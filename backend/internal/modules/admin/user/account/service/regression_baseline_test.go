package service

// 回归基线 R1/R2/R4/R5（doc104 §2 表 + §10.5）。
//
// 这四项都是「本轮改造最容易踩坏」的既有约定：R1 的部分更新语义、R2 的唯一冲突
// 409 映射、R4 的详情聚合按节降级、R5 的默认用户组不变量。它们本身不是新功能，
// 但新功能（软删除列、部分唯一索引、注销信息段、OAuth 自动注册）每一条都要从
// 它们身上经过，所以用可执行的断言钉住，而不是只在文档里写一句「不得退化」。
//
// R3（列表无 N+1）与 R6（联表别名 -:migration）在别的文件里断言：
// R3 需要真实 SQL 计数（live 用例），R6 是结构约束（model/user_alias_test.go）。

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/account/dto"
	"hostsent/backend/internal/modules/admin/user/account/model"
)

// ---------- R2：唯一冲突必须映射为哨兵错误（handler 据此回 409）----------

// TestMapUserWriteErr_RecognizesBothIndexNames 覆盖 R2 的核心风险点。
//
// 迁移 051 把 idx_users_username / idx_users_email 换成了带 WHERE deleted_at IS NULL
// 的 uk_users_username_active / uk_users_email_active。约束名变了，而错误文本匹配
// 是唯一的识别手段（PG 的 23505 不带列名），只认新名会让**老库（迁移未跑）**上的
// 冲突退化成 500，只认旧名会让新库退化。两个名字都必须认。
func TestMapUserWriteErr_RecognizesBothIndexNames(t *testing.T) {
	cases := []struct {
		name       string
		constraint string
		want       error
	}{
		{"新索引名-用户名", "uk_users_username_active", ErrUsernameTaken},
		{"新索引名-邮箱", "uk_users_email_active", ErrEmailTaken},
		{"旧索引名-用户名", "idx_users_username", ErrUsernameTaken},
		{"旧索引名-邮箱", "idx_users_email", ErrEmailTaken},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := mapUserWriteErr(pgUniqueViolation(tc.constraint), "someone", "someone@example.com")
			if !errors.Is(err, tc.want) {
				t.Fatalf("约束 %s 应映射为 %v，实际 %v", tc.constraint, tc.want, err)
			}
		})
	}
}

// TestMapUserWriteErr_NonUniqueErrorPassesThrough 非唯一冲突必须原样透传：
// 把「数据库连不上」翻译成「用户名已存在」会让运营对着一个不存在的冲突改半天账号名。
func TestMapUserWriteErr_NonUniqueErrorPassesThrough(t *testing.T) {
	raw := errors.New("connection refused")
	if got := mapUserWriteErr(raw, "u", "e@example.com"); !errors.Is(got, raw) {
		t.Fatalf("非唯一冲突错误必须原样返回，实际 %v", got)
	}
}

// TestIsUniqueViolation_DetectsPgErrorCode 23505 的识别不依赖文本。
func TestIsUniqueViolation_DetectsPgErrorCode(t *testing.T) {
	if !isUniqueViolation(pgUniqueViolation("uk_users_username_active")) {
		t.Fatal("PG 23505 未被识别为唯一冲突")
	}
	if isUniqueViolation(nil) {
		t.Fatal("nil 不应被识别为唯一冲突")
	}
	if isUniqueViolation(errors.New("some other failure")) {
		t.Fatal("普通错误不应被识别为唯一冲突")
	}
}

// pgUniqueViolation 造一个 PG 23505 错误。
//
// 用真实的 *pgconn.PgError 而不是 fmt.Errorf：isUniqueViolation 走的是
// errors.As 分支，文本分支只是驱动被替换时的兜底，两者都要能被覆盖到。
// 错误文本里必须带约束名 —— mapUserWriteErr 正是从文本里取约束名的。
func pgUniqueViolation(constraint string) error {
	return &pgconn.PgError{
		Code:    "23505",
		Message: fmt.Sprintf(`duplicate key value violates unique constraint %q`, constraint),
	}
}

// ---------- R4：详情聚合按节降级 ----------

// fakeUserDetailRepo 只实现详情聚合用到的三个方法。
type fakeUserDetailRepo struct {
	rolesErr    error
	permErr     error
	countsErr   error
	counts      *model.UserDetailCounts
	roles       []model.UserRoleBrief
	permissions []string
}

func (f *fakeUserDetailRepo) ListCustomerPermissions(context.Context, uint64) ([]string, error) {
	if f.permErr != nil {
		return nil, f.permErr
	}
	return f.permissions, nil
}

func (f *fakeUserDetailRepo) ListRbacRolesByUserID(context.Context, uint64) ([]model.UserRoleBrief, error) {
	if f.rolesErr != nil {
		return nil, f.rolesErr
	}
	return f.roles, nil
}

func (f *fakeUserDetailRepo) Counts(context.Context, uint64) (*model.UserDetailCounts, error) {
	if f.countsErr != nil {
		return nil, f.countsErr
	}
	if f.counts == nil {
		return &model.UserDetailCounts{}, nil
	}
	return f.counts, nil
}

// TestDetailAggregate_DegradesPerSection 覆盖 R4。
//
// 三个数据域分属不同模块，任何一张表异常都不该让管理员看不到用户资料本身。
// 失败的段名必须出现在 Degraded 里 —— 只降级不标记等于静默展示 0，比 500 更难排查。
func TestDetailAggregate_DegradesPerSection(t *testing.T) {
	userRepo := &fakeUserRepo{user: activeUser()}
	detailRepo := &fakeUserDetailRepo{
		rolesErr:  errors.New("roles table unavailable"),
		permErr:   errors.New("permissions table unavailable"),
		countsErr: errors.New("counts query timeout"),
	}
	svc := NewUserDetailService(userRepo, detailRepo)

	resp, err := svc.GetAggregate(context.Background(), 190)
	if err != nil {
		t.Fatalf("非 profile 段失败不得让整个详情页报错，实际: %v", err)
	}
	if resp.Profile.ID != 190 {
		t.Fatalf("profile 必须照常返回，实际 id=%d", resp.Profile.ID)
	}
	want := map[string]bool{"rbac_roles": false, "permissions": false, "summary": false}
	for _, seg := range resp.Degraded {
		if _, ok := want[seg]; !ok {
			t.Fatalf("Degraded 出现未知段名 %q", seg)
		}
		want[seg] = true
	}
	for seg, seen := range want {
		if !seen {
			t.Fatalf("失败段 %q 未记入 Degraded（实际 %v）", seg, resp.Degraded)
		}
	}
}

// TestDetailAggregate_ProfileFailureIsFatal 身份本身读不到才是真的失败。
//
// 与上一条配对：降级不能宽到「连用户都不存在也返回 200 + 空 profile」，
// 那会让前端渲染出一个全是空值的详情页。
func TestDetailAggregate_ProfileFailureIsFatal(t *testing.T) {
	userRepo := &fakeUserRepo{} // user == nil → FindByID 返回 ErrRecordNotFound
	svc := NewUserDetailService(userRepo, &fakeUserDetailRepo{})

	if _, err := svc.GetAggregate(context.Background(), 190); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("profile 读取失败必须返回错误，实际: %v", err)
	}
}

// TestDetailAggregate_DegradedAlwaysInitialized 无失败时 degraded 必须是空数组而非 null。
//
// 前端 `degraded.join('、')` 对 null 会抛异常；后端返回 `[]` 才让「没有降级段」
// 与「字段缺失」在契约上可区分。
func TestDetailAggregate_DegradedAlwaysInitialized(t *testing.T) {
	svc := NewUserDetailService(&fakeUserRepo{user: activeUser()}, &fakeUserDetailRepo{})
	resp, err := svc.GetAggregate(context.Background(), 190)
	if err != nil {
		t.Fatalf("聚合失败: %v", err)
	}
	if resp.Degraded == nil {
		t.Fatal("degraded 必须是空数组而不是 nil（前端会 join）")
	}
	if len(resp.Degraded) != 0 {
		t.Fatalf("全部成功时不应有降级段，实际 %v", resp.Degraded)
	}
}

// ---------- R5：默认用户组是不变量 ----------

// fakeUserGroupRepo 只实现用户组服务的六个方法。
type fakeUserGroupRepo struct {
	group      *model.UserGroup
	defaultID  uint64
	created    *model.UserGroup
	updated    *model.UserGroup
	deletedID  uint64
	createErr  error
	findErr    error
	updateErr  error
	defaultErr error
}

func (f *fakeUserGroupRepo) List(context.Context, dto.UserGroupListQuery) ([]model.UserGroup, int64, error) {
	return nil, 0, errUnexpectedRepoCall
}

func (f *fakeUserGroupRepo) FindByID(_ context.Context, id uint64) (*model.UserGroup, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if f.group == nil || f.group.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	copied := *f.group
	return &copied, nil
}

func (f *fakeUserGroupRepo) Create(_ context.Context, group *model.UserGroup) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.created = group
	return nil
}

func (f *fakeUserGroupRepo) Update(_ context.Context, group *model.UserGroup) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updated = group
	return nil
}

func (f *fakeUserGroupRepo) Delete(_ context.Context, id uint64) error {
	f.deletedID = id
	return nil
}

func (f *fakeUserGroupRepo) DefaultGroupID(context.Context) (uint64, error) {
	return f.defaultID, f.defaultErr
}

// TestUserGroup_DefaultCannotBeCleared 默认组只能「转移」不能「清空」。
//
// 清空默认标记会让注册/建号失去兜底目标，新用户重新散落在 user_group_id = NULL，
// 而这条路径没有任何报错——等发现时已经积累了一批未分组账号。
func TestUserGroup_DefaultCannotBeCleared(t *testing.T) {
	repo := &fakeUserGroupRepo{group: &model.UserGroup{ID: 6, Name: "默认用户组", IsDefault: true, Status: "active"}}
	svc := NewUserGroupService(repo)

	if _, err := svc.Update(context.Background(), 6, dto.UserGroupUpdateRequest{Name: "默认用户组", IsDefault: false}); err == nil {
		t.Fatal("取消当前默认组的默认标记必须被拒")
	}
	if repo.updated != nil {
		t.Fatal("被拒时不应写库")
	}
}

// TestUserGroup_DefaultCannotBeDeleted 默认组不可删除，理由同上。
func TestUserGroup_DefaultCannotBeDeleted(t *testing.T) {
	repo := &fakeUserGroupRepo{group: &model.UserGroup{ID: 6, Name: "默认用户组", IsDefault: true, Status: "active"}}
	svc := NewUserGroupService(repo)

	if err := svc.Delete(context.Background(), 6); err == nil {
		t.Fatal("删除默认用户组必须被拒")
	}
	if repo.deletedID != 0 {
		t.Fatal("被拒时不应删库")
	}
}

// TestUserGroup_NonDefaultCanBeCleared 非默认组不受此限制（不变量只约束「有且仅有一个」）。
func TestUserGroup_NonDefaultCanBeCleared(t *testing.T) {
	repo := &fakeUserGroupRepo{group: &model.UserGroup{ID: 7, Name: "代理组", IsDefault: false, Status: "active"}}
	svc := NewUserGroupService(repo)

	if _, err := svc.Update(context.Background(), 7, dto.UserGroupUpdateRequest{Name: "代理组", Status: "active"}); err != nil {
		t.Fatalf("非默认组应可正常更新: %v", err)
	}
	if repo.updated == nil {
		t.Fatal("正常更新应写库")
	}
}

// TestUserGroup_DefaultGroupIDResolverIsShared 覆盖 R5 的「必须复用同一解析器」。
//
// 建号（UserService）、注册（uc/auth）、OAuth 自动注册（uc/oauth）三处都从
// 同一个 DefaultGroupID 取兜底组。若任何一处自己写「取第一个组」的隐式兜底，
// 就会出现「后台建号进 6 组、OAuth 注册进 1 组」这种只有对账时才发现的分裂。
// 这里断言解析器本身不做隐式兜底：未配置默认组时返回 0（调用方据此保持未分组）。
func TestUserGroup_DefaultGroupIDResolverIsShared(t *testing.T) {
	repo := &fakeUserGroupRepo{defaultID: 0}
	svc := NewUserGroupService(repo)

	id, err := svc.DefaultGroupID(context.Background())
	if err != nil {
		t.Fatalf("未配置默认组不应报错: %v", err)
	}
	if id != 0 {
		t.Fatalf("未配置默认组时必须返回 0（不做「取第一个组」的隐式兜底），实际 %d", id)
	}

	repo.defaultID = 6
	if id, err = svc.DefaultGroupID(context.Background()); err != nil || id != 6 {
		t.Fatalf("应返回配置的默认组 6，实际 id=%d err=%v", id, err)
	}
}

// TestUserService_ResolveDefaultGroupNilOnZero 建号侧对 0 的处理：保持未分组而不是报错。
func TestUserService_ResolveDefaultGroupNilOnZero(t *testing.T) {
	svc := NewUserService(&fakeUserRepo{}, nil, nil).(*userService)
	svc.SetDefaultGroupProvider(&fakeUserGroupRepo{defaultID: 0})
	if got := svc.resolveDefaultGroupID(context.Background()); got != nil {
		t.Fatalf("默认组为 0 时应返回 nil（未分组），实际 %v", *got)
	}

	svc.SetDefaultGroupProvider(&fakeUserGroupRepo{defaultID: 6})
	got := svc.resolveDefaultGroupID(context.Background())
	if got == nil || *got != 6 {
		t.Fatalf("应解析出 6，实际 %v", got)
	}

	// 解析失败同样回落未分组：注册链路不能因为用户组表异常而整体失败。
	svc.SetDefaultGroupProvider(&fakeUserGroupRepo{defaultErr: errors.New("db down")})
	if got := svc.resolveDefaultGroupID(context.Background()); got != nil {
		t.Fatalf("解析失败时应回落 nil，实际 %v", *got)
	}
}
