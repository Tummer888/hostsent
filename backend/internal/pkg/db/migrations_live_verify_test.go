//go:build live

package db

import (
	"context"
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	discountmodel "hostsent/backend/internal/modules/admin/product/discount/model"
	referralmodel "hostsent/backend/internal/modules/admin/referral/model"
	usergroupmodel "hostsent/backend/internal/modules/admin/user/account/model"
	usergrouprepo "hostsent/backend/internal/modules/admin/user/account/repository"
)

// 迁移与模型双写一致性验证（R1/R4）：针对真实库确认
//   - 迁移文件可重复执行（幂等）；
//   - AutoMigrate 的模型列集合与迁移 DDL 一致（否则启动期 AutoMigrate 会报错）；
//   - 关键字段/唯一索引确实存在。
//
// 仅在 -tags live 时运行：go test -tags live ./internal/pkg/db/。
func TestLivePhaseMigrations(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}

	cases := []struct {
		name    string
		file    string
		models  []interface{}
		columns map[string][]string
		indexes []string
	}{
		{
			name:   "024_p5_price_policies",
			file:   "../../../migrations/024_p5_price_policies.sql",
			models: []interface{}{&discountmodel.PricePolicy{}, &discountmodel.PricePolicyItem{}},
			columns: map[string][]string{
				"price_policies":     {"id", "name", "code", "discount_type", "discount_value", "scope", "priority", "effective_from", "effective_to", "status", "remark", "created_at", "updated_at"},
				"price_policy_items": {"id", "policy_id", "target_type", "target_id", "discount_type", "discount_value", "created_at"},
				"orders":             {"original_amount", "discount_amount", "final_amount", "price_policy_id", "discount_source", "price_snapshot"},
				"order_items":        {"original_amount", "discount_amount", "final_amount"},
			},
			indexes: []string{"uk_pp_items", "uk_price_policies_code"},
		},
		{
			// 026 同时做两件事：建返现三表 + users 邀请列（本用例校验），并退役代理域 5 表。
			// 代理表的 DROP 不在断言范围（迁移内已带存在性守卫，重复执行安全）。
			name:   "026_drop_agent_domain_and_referral",
			file:   "../../../migrations/026_drop_agent_domain_and_referral.sql",
			models: []interface{}{&referralmodel.ReferralAccount{}, &referralmodel.ReferralTransaction{}, &referralmodel.ReferralWithdrawal{}},
			columns: map[string][]string{
				"users":                 {"invite_code", "inviter_user_id", "invited_at"},
				"referral_accounts":     {"id", "user_id", "balance", "frozen", "total_income", "total_out", "version", "created_at", "updated_at"},
				"referral_transactions": {"id", "tx_no", "user_id", "type", "direction", "amount", "balance_before", "balance_after", "biz_type", "ref_no", "order_id", "order_no", "inviter_user_id", "invitee_user_id", "remark", "operator_id", "created_at"},
				"referral_withdrawals":  {"id", "withdraw_no", "user_id", "amount", "channel", "account", "status", "audit_by", "audit_by_name", "audited_at", "remark", "created_at", "updated_at"},
			},
			indexes: []string{"uk_referral_user", "uk_referral_tx_no", "uk_referral_biz", "uk_referral_wd_no", "uk_users_invite_code", "idx_users_inviter_user_id"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sqlBytes, err := os.ReadFile(tc.file)
			if err != nil {
				t.Fatalf("读取迁移文件失败: %v", err)
			}
			for i := 1; i <= 2; i++ {
				if err := db.Exec(string(sqlBytes)).Error; err != nil {
					t.Fatalf("第 %d 次执行迁移失败: %v", i, err)
				}
			}
			if err := db.AutoMigrate(tc.models...); err != nil {
				t.Fatalf("AutoMigrate 失败（迁移与模型不一致）: %v", err)
			}
			for table, want := range tc.columns {
				assertColumns(t, db, table, want...)
			}
			for _, idx := range tc.indexes {
				assertIndex(t, db, idx)
			}
		})
	}
}

// TestLiveUserGroupsDefaultAndPriority 验证 028 用户组收尾迁移：
//   - 迁移文件可重复执行（R4 幂等）；
//   - priority 列已删除（模型侧也已移除，AutoMigrate 不会把它加回来）；
//   - 默认组唯一约束（部分唯一索引）确实存在；
//   - 不存在指向不存在用户组的悬空 users.user_group_id；
//   - is_default 至多一行。
func TestLiveUserGroupsDefaultAndPriority(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}

	sqlBytes, err := os.ReadFile("../../../migrations/028_user_groups_default_and_drop_priority.sql")
	if err != nil {
		t.Fatalf("读取迁移文件失败: %v", err)
	}
	for i := 1; i <= 2; i++ {
		if err := db.Exec(string(sqlBytes)).Error; err != nil {
			t.Fatalf("第 %d 次执行迁移失败: %v", i, err)
		}
	}

	var priorityCols int64
	if err := db.Raw(
		"SELECT count(*) FROM information_schema.columns WHERE table_name = 'user_groups' AND column_name = 'priority'",
	).Scan(&priorityCols).Error; err != nil {
		t.Fatalf("查询 priority 列失败: %v", err)
	}
	if priorityCols != 0 {
		t.Errorf("user_groups.priority 应已删除，实际仍存在 %d 列", priorityCols)
	}

	assertIndex(t, db, "uk_user_groups_single_default")

	var defaults int64
	if err := db.Raw("SELECT count(*) FROM user_groups WHERE is_default = true").Scan(&defaults).Error; err != nil {
		t.Fatalf("查询默认组数量失败: %v", err)
	}
	if defaults > 1 {
		t.Errorf("默认用户组应至多一个，实际 %d", defaults)
	}

	var dangling int64
	if err := db.Raw(
		"SELECT count(*) FROM users u WHERE u.user_group_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM user_groups g WHERE g.id = u.user_group_id)",
	).Scan(&dangling).Error; err != nil {
		t.Fatalf("查询悬空 user_group_id 失败: %v", err)
	}
	if dangling != 0 {
		t.Errorf("存在 %d 条指向不存在用户组的 users.user_group_id", dangling)
	}
}

// TestLiveDefaultUserGroupSeeded 验证默认用户组的种子语义（is_default 兜底）：
//   - seedDefaultUserGroup 幂等（重复执行不会产生第二个默认组）；
//   - 执行后恰好存在一个 is_default 组（注册/建号的兜底目标）；
//   - code=default 的种子组不绑定折扣策略，避免新用户静默获得折扣。
func TestLiveDefaultUserGroupSeeded(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}

	for i := 1; i <= 2; i++ {
		if err := seedDefaultUserGroup(db); err != nil {
			t.Fatalf("第 %d 次执行 seedDefaultUserGroup 失败: %v", i, err)
		}
	}

	var defaults int64
	if err := db.Raw("SELECT count(*) FROM user_groups WHERE is_default = true").Scan(&defaults).Error; err != nil {
		t.Fatalf("查询默认组数量失败: %v", err)
	}
	if defaults != 1 {
		t.Errorf("seedDefaultUserGroup 后默认用户组应恰好一个，实际 %d", defaults)
	}

	var seededWithPolicy int64
	if err := db.Raw(
		"SELECT count(*) FROM user_groups WHERE code = 'default' AND price_policy_id IS NOT NULL",
	).Scan(&seededWithPolicy).Error; err != nil {
		t.Fatalf("查询默认组折扣策略失败: %v", err)
	}
	if seededWithPolicy != 0 {
		t.Errorf("code=default 的默认用户组不应绑定折扣策略，实际 %d 个", seededWithPolicy)
	}
}

// TestLiveUserGroupDefaultSwitch 验证默认组切换不会撞唯一索引：
// 仓储的 Create/Update 必须「先清后写」（同一事务），否则把另一个组置为默认时会
// 触发 uk_user_groups_single_default（SQLSTATE 23505），运营侧表现为保存失败。
// 测试结束后恢复 code=default 组的默认标记，避免影响线上兜底行为。
func TestLiveUserGroupDefaultSwitch(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}
	repo := usergrouprepo.NewUserGroupRepository(db)
	ctx := context.Background()

	t.Cleanup(func() {
		db.Exec("DELETE FROM user_groups WHERE code IN ('zz_def_a', 'zz_def_b')")
		db.Exec("UPDATE user_groups SET is_default = true WHERE code = 'default'")
	})

	groupA := &usergroupmodel.UserGroup{Name: "zz默认组A", Code: "zz_def_a", Status: "active", IsDefault: true}
	if err := repo.Create(ctx, groupA); err != nil {
		t.Fatalf("创建默认组 A 失败: %v", err)
	}

	// 创建第二个默认组：应先自动取消 A 的默认标记，而不是撞唯一索引。
	groupB := &usergroupmodel.UserGroup{Name: "zz默认组B", Code: "zz_def_b", Status: "active", IsDefault: true}
	if err := repo.Create(ctx, groupB); err != nil {
		t.Fatalf("创建默认组 B 失败（默认组唯一性未先清后写）: %v", err)
	}

	// 再把 A 改回默认（走 Update 路径）。
	reloaded, err := repo.FindByID(ctx, groupA.ID)
	if err != nil {
		t.Fatalf("读取默认组 A 失败: %v", err)
	}
	if reloaded.IsDefault {
		t.Errorf("创建 B 后 A 的 is_default 应已清空")
	}
	reloaded.IsDefault = true
	if err := repo.Update(ctx, reloaded); err != nil {
		t.Fatalf("切换默认组回 A 失败: %v", err)
	}

	var defaults int64
	if err := db.Raw("SELECT count(*) FROM user_groups WHERE is_default = true").Scan(&defaults).Error; err != nil {
		t.Fatalf("查询默认组数量失败: %v", err)
	}
	if defaults != 1 {
		t.Errorf("切换后默认用户组应恰好一个，实际 %d", defaults)
	}
	var currentCode string
	if err := db.Raw("SELECT code FROM user_groups WHERE is_default = true LIMIT 1").Scan(&currentCode).Error; err != nil {
		t.Fatalf("查询当前默认组失败: %v", err)
	}
	if currentCode != "zz_def_a" {
		t.Errorf("当前默认组应为 zz_def_a，实际 %q", currentCode)
	}
}

// TestLiveResidualMenuCleanup 验证 027 菜单清理迁移：
//   - 迁移文件可重复执行（R4 幂等）；
//   - 清理后不再存在任何无页面/重复/退役的残留菜单路径；
//   - /users/levels 恰好一行且为 active。
func TestLiveResidualMenuCleanup(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}

	sqlBytes, err := os.ReadFile("../../../migrations/027_cleanup_residual_menus.sql")
	if err != nil {
		t.Fatalf("读取迁移文件失败: %v", err)
	}
	for i := 1; i <= 2; i++ {
		if err := db.Exec(string(sqlBytes)).Error; err != nil {
			t.Fatalf("第 %d 次执行迁移失败: %v", i, err)
		}
	}

	count := func(query string, args ...interface{}) int64 {
		t.Helper()
		var n int64
		if err := db.Raw(query, args...).Scan(&n).Error; err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		return n
	}

	for _, pattern := range []string{"/users/rbac%", "/users/quota%", "/user/%"} {
		platform := "admin"
		if pattern == "/user/%" {
			platform = "user"
		}
		if n := count("SELECT count(*) FROM menus WHERE platform = ? AND path LIKE ?", platform, pattern); n != 0 {
			t.Errorf("残留菜单未清理：platform=%s path LIKE %s 仍有 %d 行", platform, pattern, n)
		}
	}
	if n := count("SELECT count(*) FROM menus WHERE platform = 'admin' AND path = '/dashboard/analysis'"); n != 0 {
		t.Errorf("残留菜单未清理：/dashboard/analysis 仍有 %d 行", n)
	}
	if n := count("SELECT count(*) FROM menus WHERE platform = 'admin' AND path IN ('/support', '/support/tickets')"); n != 0 {
		t.Errorf("残留菜单未清理：admin /support* 仍有 %d 行", n)
	}
	if n := count("SELECT count(*) FROM menus WHERE platform = 'admin' AND path = '/users/accounts/detail'"); n != 0 {
		t.Errorf("用户详情菜单应下线，仍有 %d 行", n)
	}
	if n := count("SELECT count(*) FROM menus WHERE platform = 'admin' AND path = '/users/levels' AND status = 'active'"); n != 1 {
		t.Errorf("期望 /users/levels 恰好一行 active，实际 %d", n)
	}
}

func assertColumns(t *testing.T, db *gorm.DB, table string, want ...string) {
	t.Helper()
	var got []string
	if err := db.Raw(
		"SELECT column_name FROM information_schema.columns WHERE table_name = ?", table,
	).Scan(&got).Error; err != nil {
		t.Fatalf("查询 %s 列失败: %v", table, err)
	}
	have := make(map[string]bool, len(got))
	for _, c := range got {
		have[c] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Fatalf("表 %s 缺少列 %s（实际列：%v）", table, w, got)
		}
	}
}

func assertIndex(t *testing.T, db *gorm.DB, name string) {
	t.Helper()
	var count int64
	if err := db.Raw("SELECT count(*) FROM pg_indexes WHERE indexname = ?", name).Scan(&count).Error; err != nil {
		t.Fatalf("查询索引 %s 失败: %v", name, err)
	}
	if count != 1 {
		t.Fatalf("期望索引 %s 存在，实际 %d", name, count)
	}
}
