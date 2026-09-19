package model

import (
	"reflect"
	"strings"
	"testing"
)

// 回归基线 R6（doc104 §2 表）：联表只读别名必须写 `gorm:"->;-:migration"`。
//
// 为什么这是一条「必须用测试钉住」的约定而不是注释：
//
//	`->`      只读权限：GORM 不会把该字段写回库（联表别名本来也不是真实列）。
//	`-:migration` 让 AutoMigrate 跳过建列。
//
// 只写 `->` 而漏了 `-:migration` 时，AutoMigrate 会在 users 表上**建出一个多余的
// 真实列**（user_group_name、sales_admin_name …）。那一刻起这张表里就有两个同名
// 概念：真实列恒为 NULL（联表别名不参与写入），而查询用 SELECT users.* 带出的
// 又是别名值——本地看着正常，重启跑一次迁移后数据就开始漂移。
//
// 本轮新增了 deleted_by_name 这个联表别名，正是这条基线最容易踩坏的时刻。
func TestUserAliasFieldsAreReadOnlyAndNotMigrated(t *testing.T) {
	// 联表别名白名单：这些字段在 users 表里**不存在**真实列，只能由查询的
	// LEFT JOIN 别名带出。新增别名时把它加进来，测试会替我们检查 gorm tag。
	aliases := []string{
		"DeletedByName",
		"UserGroupName",
		"UserLevelName",
		"UserLevelCode",
		"InviterName",
		"SalesAdminName",
		"OwnerName",
	}

	typ := reflect.TypeOf(User{})
	for _, name := range aliases {
		field, ok := typ.FieldByName(name)
		if !ok {
			t.Fatalf("联表别名字段 %s 不存在（是否被改名或删除？）", name)
		}
		tag := field.Tag.Get("gorm")
		if !strings.Contains(tag, "->") {
			t.Errorf("%s 缺少只读权限 `->`，GORM 会尝试把它写回库（tag=%q）", name, tag)
		}
		if !strings.Contains(tag, "-:migration") {
			t.Errorf("%s 缺少 `-:migration`，AutoMigrate 会在 users 表建出多余列（tag=%q）", name, tag)
		}
	}
}

// TestUserSoftDeleteColumnsUsePointerTime 覆盖 doc104 §4.2 的形态选择。
//
// deleted_at 必须是 *time.Time 而不是 gorm.DeletedAt：全仓库有大量
// Table("users") 裸查询（等级仓储、工单前置条件、会员仓储），gorm.DeletedAt 的
// 隐式过滤会在这些路径上静默生效或静默失效——同一个查询换个写法就换了语义。
// 换成 gorm.DeletedAt 会连带改变那些路径的行为，且不会有任何编译错误。
func TestUserSoftDeleteColumnsUsePointerTime(t *testing.T) {
	typ := reflect.TypeOf(User{})

	deletedAt, ok := typ.FieldByName("DeletedAt")
	if !ok {
		t.Fatal("users 模型缺少 DeletedAt（注销标记）")
	}
	if got := deletedAt.Type.String(); got != "*time.Time" {
		t.Fatalf("DeletedAt 必须是 *time.Time（显式 WHERE deleted_at IS NULL），实际 %s", got)
	}
	if !strings.Contains(deletedAt.Tag.Get("gorm"), "column:deleted_at") {
		t.Errorf("DeletedAt 的列名必须显式为 deleted_at，实际 tag=%q", deletedAt.Tag.Get("gorm"))
	}

	// 实名信任信号同理：NULL 与「零值时间」必须可区分，只有指针能做到。
	verifiedAt, ok := typ.FieldByName("RealNameVerifiedAt")
	if !ok {
		t.Fatal("users 模型缺少 RealNameVerifiedAt（实名唯一信任信号）")
	}
	if got := verifiedAt.Type.String(); got != "*time.Time" {
		t.Fatalf("RealNameVerifiedAt 必须是 *time.Time，实际 %s", got)
	}

	// 唯一索引必须带部分条件：注销用户的 username/email 不该永久占用。
	username, _ := typ.FieldByName("Username")
	if tag := username.Tag.Get("gorm"); !strings.Contains(tag, "WHERE:deleted_at IS NULL") {
		t.Errorf("Username 的唯一索引必须带 WHERE deleted_at IS NULL（部分唯一索引），实际 tag=%q", tag)
	}
	email, _ := typ.FieldByName("Email")
	if tag := email.Tag.Get("gorm"); !strings.Contains(tag, "WHERE:deleted_at IS NULL") {
		t.Errorf("Email 的唯一索引必须带 WHERE deleted_at IS NULL（部分唯一索引），实际 tag=%q", tag)
	}
}
