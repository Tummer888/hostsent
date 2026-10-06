//go:build live

package db

import (
	"context"
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	finbillmodel "hostsent/backend/internal/modules/admin/finance/bill/model"
	discountmodel "hostsent/backend/internal/modules/admin/product/discount/model"
	flashdiscountmodel "hostsent/backend/internal/modules/admin/product/flashdiscount/model"
	referralmodel "hostsent/backend/internal/modules/admin/referral/model"
	levelmodel "hostsent/backend/internal/modules/admin/user/account/level/model"
	usergroupmodel "hostsent/backend/internal/modules/admin/user/account/model"
	usergrouprepo "hostsent/backend/internal/modules/admin/user/account/repository"
	agentlevelmodel "hostsent/backend/internal/modules/admin/user/agentlevel/model"
	securitymodel "hostsent/backend/internal/modules/admin/user/security/model"
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
		{
			// 054 只加一列：账单的「本期充值合计」。充值此前完全没有账单口径，
			// 这一列是充值侧与账单侧唯一的对接点，模型与 DDL 必须一致。
			name:    "054_bill_recharge_amount",
			file:    "../../../migrations/054_bill_recharge_amount.sql",
			models:  []interface{}{&finbillmodel.Bill{}},
			columns: map[string][]string{"bills": {"recharge_amount"}},
		},
		{
			// 055 把账单拆成两类来源：按期消费账单（user_id+period 唯一）与
			// 逐笔充值账单（source_type+source_no 唯一）。两个唯一索引都必须带
			// WHERE 谓词，否则同账期的第二笔充值无处落脚。
			name:   "055_bill_source_split",
			file:   "../../../migrations/055_bill_source_split.sql",
			models: []interface{}{&finbillmodel.Bill{}},
			columns: map[string][]string{
				"bills": {"source_type", "source_no", "recharge_amount"},
			},
			indexes: []string{"uk_bills_user_period", "uk_bills_source"},
		},
		{
			// 056 是员工域 / 客户域分离的收口：roles.scope 纠正 + 登录日志与会话
			// 的域判别列。subject_type 让「员工后台登录」不再被算进同 ID 的客户
			// 详情（admins 与 users 有 7 组撞号），代登录仍留在客户域。
			name:   "056_staff_user_domain_split",
			file:   "../../../migrations/056_staff_user_domain_split.sql",
			models: []interface{}{&securitymodel.LoginLog{}, &securitymodel.Session{}},
			columns: map[string][]string{
				"login_logs":    {"subject_type"},
				"user_sessions": {"subject_type"},
			},
			indexes: []string{"idx_login_logs_subject_user", "idx_user_sessions_subject_user"},
		},
		{
			// 057 把用户等级重规划为六级会员阶梯（白银→…→王者），并做两件兜底：
			// user_level_id 为空的行落到最低等级、一条角色都没有的客户账号补
			// roles.code='user'。断言迁移可重复执行且收敛结果唯一（不新增等级行）。
			name:   "057_user_level_membership_ladder",
			file:   "../../../migrations/057_user_level_membership_ladder.sql",
			models: []interface{}{&levelmodel.UserLevel{}},
			columns: map[string][]string{
				"user_levels": {"name", "code", "weight", "status", "upgrade_threshold", "max_sub_accounts", "benefits"},
			},
			indexes: []string{"idx_user_levels_code", "idx_user_levels_name"},
		},
		{
			// 058 给等级加 icon / color 两列：运营在管理端自定义图标与配色，
			// 用户端按同一套值渲染等级徽章。两列 NOT NULL DEFAULT ''，
			// 空值语义是「按权重回落默认」而非「未回填」，故不回填存量行。
			name:   "058_user_level_icon_color",
			file:   "../../../migrations/058_user_level_icon_color.sql",
			models: []interface{}{&levelmodel.UserLevel{}},
			columns: map[string][]string{
				"user_levels": {"icon", "color"},
			},
		},
		{
			// 059 是 doc108 的落地点：代理独立成「等级 + 折扣矩阵」，用户组退回纯分类。
			// 断言三件事：新表列集合与模型一致、users.agent_level_id 存在、
			// user_groups 的 price_policy_id / is_agent_group 两列确实已消失
			// （模型侧也已移除，否则 AutoMigrate 会把它们加回来）。
			name:   "059_agent_levels_and_group_purify",
			file:   "../../../migrations/059_agent_levels_and_group_purify.sql",
			models: []interface{}{&agentlevelmodel.AgentLevel{}, &agentlevelmodel.AgentLevelDiscount{}},
			columns: map[string][]string{
				"agent_levels":          {"id", "name", "code", "weight", "status", "description", "created_at", "updated_at"},
				"agent_level_discounts": {"id", "agent_level_id", "target_type", "target_id", "discount_rate", "created_at", "updated_at"},
				"users":                 {"agent_level_id"},
				"product_categories":    {"cost_rate"},
			},
			indexes: []string{"uk_agent_levels_code", "uk_agent_level_discounts", "idx_agent_level_discounts_target", "idx_users_agent_level_id"},
		},
		{
			// 061 是 §8I 的落地点：商品分组（分类/商品集合）× 折扣组（等级→费率）。
			// 两张新表是录入组织层，算价仍只认 agent_level_discounts 逐格矩阵。
			name: "061_product_groups_and_discount_schemes",
			file: "../../../migrations/061_product_groups_and_discount_schemes.sql",
			models: []interface{}{
				&agentlevelmodel.ProductGroup{},
				&agentlevelmodel.ProductGroupItem{},
				&agentlevelmodel.DiscountScheme{},
				&agentlevelmodel.DiscountSchemeItem{},
			},
			columns: map[string][]string{
				"product_groups":        {"id", "name", "code", "description", "status", "created_at", "updated_at"},
				"product_group_items":   {"id", "group_id", "target_type", "target_id", "created_at"},
				"discount_schemes":      {"id", "name", "code", "product_group_id", "status", "created_at", "updated_at"},
				"discount_scheme_items": {"id", "scheme_id", "agent_level_id", "discount_rate", "created_at", "updated_at"},
			},
			indexes: []string{
				"uk_product_groups_code", "uk_product_group_items", "idx_product_group_items_target",
				"uk_discount_schemes_code", "idx_discount_schemes_group",
				"uk_discount_scheme_items", "idx_discount_scheme_items_level",
			},
		},
		{
			// 062 是 §8J 的落地点：面向**用户**的限时活动折扣。
			// 算价通过 pricing.Deps.PromotionRule 命中它，且代理用户不参与（见 service）。
			name: "062_flash_discounts",
			file: "../../../migrations/062_flash_discounts.sql",
			models: []interface{}{
				&flashdiscountmodel.FlashDiscount{},
				&flashdiscountmodel.FlashDiscountItem{},
			},
			columns: map[string][]string{
				"flash_discounts":      {"id", "name", "code", "description", "discount_type", "discount_value", "scope", "start_at", "end_at", "status", "remark", "created_at", "updated_at"},
				"flash_discount_items": {"id", "discount_id", "target_type", "target_id", "created_at"},
			},
			indexes: []string{
				"uk_flash_discounts_code", "idx_flash_discounts_window",
				"uk_flash_discount_items", "idx_flash_discount_items_target",
			},
		},
		{
			// 063 把折扣组的商品分组绑定从单值列换成关联表（doc108 §8K）：
			// 一个折扣组可绑多个商品分组共用同一套费率，一个分组仍只归一个折扣组。
			name: "064_scheme_multi_groups",
			file: "../../../migrations/064_scheme_multi_groups.sql",
			models: []interface{}{
				&agentlevelmodel.DiscountScheme{},
				&agentlevelmodel.DiscountSchemeGroup{},
			},
			columns: map[string][]string{
				"discount_scheme_groups": {"id", "scheme_id", "group_id", "created_at"},
			},
			indexes: []string{
				"uk_discount_scheme_groups",       // (scheme_id, group_id)
				"uk_discount_scheme_groups_group", // (group_id)：一个分组只归一个折扣组
			},
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

// TestLiveAgentMenuMergedIntoGroups 验证 060：二级菜单「代理等级」已并入
// 用户组管理页的代理分组分区，独立菜单行不复存在；agent_level:* 权限码保留
// （分区内的按钮与接口仍按它鉴权）。
func TestLiveAgentMenuMergedIntoGroups(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}

	sqlBytes, err := os.ReadFile("../../../migrations/060_merge_agent_menu_into_groups.sql")
	if err != nil {
		t.Fatalf("读取迁移文件失败: %v", err)
	}
	for i := 1; i <= 2; i++ {
		if err := db.Exec(string(sqlBytes)).Error; err != nil {
			t.Fatalf("第 %d 次执行迁移失败: %v", i, err)
		}
	}

	var menus int64
	if err := db.Raw(
		"SELECT count(*) FROM menus WHERE platform = 'admin' AND path = '/users/agents'",
	).Scan(&menus).Error; err != nil {
		t.Fatalf("查询代理等级菜单失败: %v", err)
	}
	if menus != 0 {
		t.Errorf("独立菜单 /users/agents 应已并入用户组管理，仍存在 %d 行", menus)
	}

	var perms int64
	if err := db.Raw(
		"SELECT count(*) FROM permissions WHERE code LIKE 'agent_level:%'",
	).Scan(&perms).Error; err != nil {
		t.Fatalf("查询 agent_level 权限失败: %v", err)
	}
	if perms < 4 {
		t.Errorf("agent_level:* 权限码应保留，实际 %d 个", perms)
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

	// doc108 起用户组不再承载折扣：price_policy_id 列已由迁移 059 删除，
	// 因此这里改为断言列不存在（比"值为空"更强的保证：结构上就不可能绑折扣）。
	var policyCols int64
	if err := db.Raw(
		"SELECT count(*) FROM information_schema.columns WHERE table_name = 'user_groups' AND column_name IN ('price_policy_id','is_agent_group')",
	).Scan(&policyCols).Error; err != nil {
		t.Fatalf("查询 user_groups 折扣列失败: %v", err)
	}
	if policyCols != 0 {
		t.Errorf("user_groups 不应再有 price_policy_id / is_agent_group 列，实际仍有 %d 列", policyCols)
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
