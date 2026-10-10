//go:build live

package db

import (
	"context"
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	finbillmodel "hostsent/backend/internal/modules/admin/finance/bill/model"
	costmodel "hostsent/backend/internal/modules/admin/finance/cost/model"
	referralmodel "hostsent/backend/internal/modules/admin/finance/referral/model"
	discountmodel "hostsent/backend/internal/modules/admin/product/discount/model"
	flashdiscountmodel "hostsent/backend/internal/modules/admin/product/flashdiscount/model"
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
		{
			// 065 补齐风控的两处结构缺口（doc06 §4.3/§4.4）：
			//   - risk_events.subject_type：user_id 承载 users.id 与 admins.id
			//     两个 ID 空间且会撞号，不区分域会把员工异常记到客户头上；
			//   - login_logs 的设备指纹索引 + 规则引擎的两个查询索引。
			// 默认值 'user' 必须与模型一致，否则 AutoMigrate 会尝试改列。
			name:   "065_risk_engine",
			file:   "../../../migrations/065_risk_engine.sql",
			models: []interface{}{&securitymodel.RiskEvent{}, &securitymodel.LoginLog{}},
			columns: map[string][]string{
				"risk_events": {"subject_type"},
			},
			indexes: []string{
				"idx_risk_events_subject_user",
				"idx_risk_events_rule_open",
				"idx_login_logs_device_fingerprint",
			},
		},
		{
			// 068 新增风险事件处置流水表（doc06 §4.3 处置联动）。
			//
			// 改造前 risk_events 只有一列 handle_note / handled_by，任何一次操作都会
			// 覆盖上一次的痕迹；而且「拉黑」「失效会话」两个动作根本不写回事件 ——
			// 运营点完页面状态不变，看起来像没生效。本表把每个动作记成一行，
			// 形成事件下的处置时间线。
			name:   "068_risk_event_actions",
			file:   "../../../migrations/068_risk_event_actions.sql",
			models: []interface{}{&securitymodel.RiskEventAction{}},
			columns: map[string][]string{
				"risk_event_actions": {"id", "event_id", "action", "operator_id", "note", "detail", "created_at"},
			},
			indexes: []string{"idx_risk_event_actions_event"},
		},
		{
			// 078 建成本管理三表（doc111）：成本项配置 / 上游余额快照 / 上游充值记录。
			// 快照表的 (provider_id, snapshot_date) 唯一键是「同渠道同日只有一个余额真值」
			// 的保证（重复抓取走覆盖），台账推算「期初+充值−期末=消耗」依赖它；
			// 模型与 DDL 的列/索引必须一致，否则启动期 AutoMigrate 会去改表。
			name:   "078_cost_management",
			file:   "../../../migrations/078_cost_management.sql",
			models: []interface{}{&costmodel.CostItem{}, &costmodel.UpstreamBalanceSnapshot{}, &costmodel.UpstreamBalanceTopup{}},
			columns: map[string][]string{
				"cost_items":                 {"id", "name", "category", "amount", "cycle", "occurred_on", "effective_from", "effective_to", "subject", "remark", "status", "operator_id", "created_at", "updated_at"},
				"upstream_balance_snapshots": {"id", "provider_id", "snapshot_date", "balance", "currency", "source", "remark", "created_at", "updated_at"},
				"upstream_balance_topups":    {"id", "provider_id", "occurred_on", "amount", "remark", "operator_id", "created_at", "updated_at"},
			},
			indexes: []string{
				"uk_ub_snapshot_provider_date",
				"idx_ub_snapshot_provider_date",
				"idx_ub_topup_provider_date",
				"idx_cost_items_category",
				"idx_cost_items_status",
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

// TestLiveLifecycleMergedIntoInstances 验证 066：生命周期管理整域并入实例管理。
//   - 迁移文件可重复执行（幂等）；
//   - 旧一级域菜单 /lifecycle（含 3 个叶子）已删除，不残留重复入口；
//   - 新的 /instances/lifecycle 目录与 3 个叶子已由 seed 插入；
//   - lifecycle:* 权限码保留（接口仍按它鉴权）。
//
// 顺序要求：先跑迁移，再重启后端 —— 新行由 seed 在启动时插入，
// 未重启时本用例会在「新菜单未插入」处失败，这是预期的提示。
func TestLiveLifecycleMergedIntoInstances(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}

	sqlBytes, err := os.ReadFile("../../../migrations/066_merge_lifecycle_into_instances.sql")
	if err != nil {
		t.Fatalf("读取迁移文件失败: %v", err)
	}
	for i := 1; i <= 2; i++ {
		if err := db.Exec(string(sqlBytes)).Error; err != nil {
			t.Fatalf("第 %d 次执行迁移失败: %v", i, err)
		}
	}

	var oldMenus int64
	if err := db.Raw(
		"SELECT count(*) FROM menus WHERE platform = 'admin' AND (path = '/lifecycle' OR path LIKE '/lifecycle/%')",
	).Scan(&oldMenus).Error; err != nil {
		t.Fatalf("查询旧生命周期菜单失败: %v", err)
	}
	if oldMenus != 0 {
		t.Errorf("旧一级域菜单 /lifecycle* 应已删除，仍存在 %d 行", oldMenus)
	}

	wantPaths := []string{
		"/instances/lifecycle",
		"/instances/lifecycle/expiring",
		"/instances/lifecycle/renewals",
		"/instances/lifecycle/policy",
	}
	for _, p := range wantPaths {
		var n int64
		if err := db.Raw(
			"SELECT count(*) FROM menus WHERE platform = 'admin' AND path = ? AND status = 'active'", p,
		).Scan(&n).Error; err != nil {
			t.Fatalf("查询菜单 %s 失败: %v", p, err)
		}
		if n != 1 {
			t.Errorf("菜单 %s 应存在且唯一（seed 需在后端重启后写入），实际 %d 行", p, n)
		}
	}

	var perms int64
	if err := db.Raw(
		"SELECT count(*) FROM permissions WHERE code LIKE 'lifecycle%'",
	).Scan(&perms).Error; err != nil {
		t.Fatalf("查询 lifecycle 权限失败: %v", err)
	}
	if perms < 6 {
		t.Errorf("lifecycle:* 权限码应保留，实际 %d 个", perms)
	}
}

// TestLivePaymentMergedIntoSystem 验证 067：支付中心整域并入系统管理。
//   - 迁移文件可重复执行（幂等）；
//   - 旧一级域 /payment（含 3 个二级分类与 8 个叶子）已删除，不残留重复入口；
//   - 新的 /system/payment 二级目录与 8 个叶子已由 seed 插入；
//   - payment:* 权限码保留（接口仍按它鉴权）。
//
// 顺序要求：先跑迁移，再重启后端 —— 新行由 seed 在启动时插入，
// 未重启时本用例会在「新菜单未插入」处失败，这是预期的提示。
func TestLivePaymentMergedIntoSystem(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}

	sqlBytes, err := os.ReadFile("../../../migrations/067_merge_payment_into_system.sql")
	if err != nil {
		t.Fatalf("读取迁移文件失败: %v", err)
	}
	for i := 1; i <= 2; i++ {
		if err := db.Exec(string(sqlBytes)).Error; err != nil {
			t.Fatalf("第 %d 次执行迁移失败: %v", i, err)
		}
	}

	var oldMenus int64
	if err := db.Raw(
		"SELECT count(*) FROM menus WHERE platform = 'admin' AND (path = '/payment' OR path LIKE '/payment/%')",
	).Scan(&oldMenus).Error; err != nil {
		t.Fatalf("查询旧支付中心菜单失败: %v", err)
	}
	if oldMenus != 0 {
		t.Errorf("旧一级域菜单 /payment* 应已删除，仍存在 %d 行", oldMenus)
	}

	wantPaths := []string{
		"/system/payment",
		"/system/payment/overview",
		"/system/payment/channels",
		"/system/payment/methods",
		"/system/payment/orders",
		"/system/payment/callbacks",
		"/system/payment/refunds",
		"/system/payment/payouts",
		"/system/payment/recon",
	}
	for _, p := range wantPaths {
		var n int64
		if err := db.Raw(
			"SELECT count(*) FROM menus WHERE platform = 'admin' AND path = ? AND status = 'active'", p,
		).Scan(&n).Error; err != nil {
			t.Fatalf("查询菜单 %s 失败: %v", p, err)
		}
		if n != 1 {
			t.Errorf("菜单 %s 应存在且唯一（seed 需在后端重启后写入），实际 %d 行", p, n)
		}
	}

	var perms int64
	if err := db.Raw(
		"SELECT count(*) FROM permissions WHERE code LIKE 'payment%'",
	).Scan(&perms).Error; err != nil {
		t.Fatalf("查询 payment 权限失败: %v", err)
	}
	if perms < 8 {
		t.Errorf("payment:* 权限码应保留，实际 %d 个", perms)
	}
}

// TestLiveSalesAndNotificationMergedIntoSystem 验证 068：销售中心与消息中心
// 整域并入系统管理。
//   - 迁移文件可重复执行（幂等）；
//   - 旧一级域 /sales、/notification（含各自叶子）已删除，不残留重复入口；
//   - 新的 /system/{sales,notification} 二级目录与各自叶子已由 seed 插入；
//   - 公告管理页面随 notification 模块搬迁，seed 的 component 指向新目录；
//   - sales:* / notify:* 权限码保留（接口仍按它们鉴权）。
//
// 顺序要求：先跑迁移，再重启后端 —— 新行由 seed 在启动时插入，
// 未重启时本用例会在「新菜单未插入」处失败，这是预期的提示。
func TestLiveSalesAndNotificationMergedIntoSystem(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}

	sqlBytes, err := os.ReadFile("../../../migrations/068_merge_sales_and_notification_into_system.sql")
	if err != nil {
		t.Fatalf("读取迁移文件失败: %v", err)
	}
	for i := 1; i <= 2; i++ {
		if err := db.Exec(string(sqlBytes)).Error; err != nil {
			t.Fatalf("第 %d 次执行迁移失败: %v", i, err)
		}
	}

	var oldMenus int64
	if err := db.Raw(
		`SELECT count(*) FROM menus WHERE platform = 'admin'
		   AND (path = '/sales' OR path LIKE '/sales/%'
		     OR path = '/notification' OR path LIKE '/notification/%')`,
	).Scan(&oldMenus).Error; err != nil {
		t.Fatalf("查询旧销售/消息菜单失败: %v", err)
	}
	if oldMenus != 0 {
		t.Errorf("旧一级域菜单 /sales*、/notification* 应已删除，仍存在 %d 行", oldMenus)
	}

	wantPaths := []string{
		"/system/notification",
		"/system/notification/records",
		"/system/notification/templates",
		"/system/notification/channels",
		"/system/notification/sms-templates",
		"/system/notification/broadcast",
		"/system/notification/deliveries",
		"/system/sales",
		"/system/sales/customers",
		"/system/sales/commissions",
		"/system/sales/withdrawals",
		"/system/sales/performance",
	}
	for _, p := range wantPaths {
		var n int64
		if err := db.Raw(
			"SELECT count(*) FROM menus WHERE platform = 'admin' AND path = ? AND status = 'active'", p,
		).Scan(&n).Error; err != nil {
			t.Fatalf("查询菜单 %s 失败: %v", p, err)
		}
		if n != 1 {
			t.Errorf("菜单 %s 应存在且唯一（seed 需在后端重启后写入），实际 %d 行", p, n)
		}
	}

	// 公告管理页面随 notification 模块搬迁：任何菜单都不应再指向旧页面目录
	// （菜单路径本身在迁移 069 会从 /content/announcements 改到 /system/content/announcements，
	//  这里只锁定「页面目录不再等于模块旧归属」，不依赖具体路径）。
	var staleComp int64
	if err := db.Raw(
		"SELECT count(*) FROM menus WHERE platform = 'admin' AND component = 'notification/announcements/index'",
	).Scan(&staleComp).Error; err != nil {
		t.Fatalf("查询公告管理旧组件引用失败: %v", err)
	}
	if staleComp != 0 {
		t.Errorf("不应再有菜单指向 notification/announcements/index（页面已随模块迁入 system/），实际 %d 行", staleComp)
	}

	for _, code := range []string{"sales:%", "notify:%"} {
		var n int64
		if err := db.Raw(
			"SELECT count(*) FROM permissions WHERE code LIKE ?", code,
		).Scan(&n).Error; err != nil {
			t.Fatalf("查询 %s 权限失败: %v", code, err)
		}
		if n < 4 {
			t.Errorf("%s 权限码应保留，实际 %d 个", code, n)
		}
	}
}

// TestLiveContentMergedIntoSystem 验证 069：内容管理整域并入系统管理。
//   - 迁移文件可重复执行（幂等）；
//   - 旧一级域 /content（含 4 个叶子）已删除，不残留重复入口；
//   - 新的 /system/content 二级目录与 4 个叶子已由 seed 插入；
//   - 公告管理沿用 notify:announcement 且页面仍指向 notification 模块目录；
//   - content:* 权限码保留（接口仍按它们鉴权）。
//
// 顺序要求：先跑迁移，再重启后端 —— 新行由 seed 在启动时插入，
// 未重启时本用例会在「新菜单未插入」处失败，这是预期的提示。
func TestLiveContentMergedIntoSystem(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}

	sqlBytes, err := os.ReadFile("../../../migrations/069_merge_content_into_system.sql")
	if err != nil {
		t.Fatalf("读取迁移文件失败: %v", err)
	}
	for i := 1; i <= 2; i++ {
		if err := db.Exec(string(sqlBytes)).Error; err != nil {
			t.Fatalf("第 %d 次执行迁移失败: %v", i, err)
		}
	}

	var oldMenus int64
	if err := db.Raw(
		"SELECT count(*) FROM menus WHERE platform = 'admin' AND (path = '/content' OR path LIKE '/content/%')",
	).Scan(&oldMenus).Error; err != nil {
		t.Fatalf("查询旧内容菜单失败: %v", err)
	}
	if oldMenus != 0 {
		t.Errorf("旧一级域菜单 /content* 应已删除，仍存在 %d 行", oldMenus)
	}

	wantPaths := []string{
		"/system/content",
		"/system/content/articles",
		"/system/content/categories",
		"/system/content/links",
		"/system/content/announcements",
	}
	for _, p := range wantPaths {
		var n int64
		if err := db.Raw(
			"SELECT count(*) FROM menus WHERE platform = 'admin' AND path = ? AND status = 'active'", p,
		).Scan(&n).Error; err != nil {
			t.Fatalf("查询菜单 %s 失败: %v", p, err)
		}
		if n != 1 {
			t.Errorf("菜单 %s 应存在且唯一（seed 需在后端重启后写入），实际 %d 行", p, n)
		}
	}

	var annComp string
	if err := db.Raw(
		"SELECT component FROM menus WHERE platform = 'admin' AND path = '/system/content/announcements'",
	).Scan(&annComp).Error; err != nil {
		t.Fatalf("查询公告管理菜单失败: %v", err)
	}
	if annComp != "system/notification/announcements/index" {
		t.Errorf("公告管理页面应仍归 notification 模块（doc102 M2-4），实际 %q", annComp)
	}

	var perms int64
	if err := db.Raw(
		"SELECT count(*) FROM permissions WHERE code LIKE 'content%'",
	).Scan(&perms).Error; err != nil {
		t.Fatalf("查询 content 权限失败: %v", err)
	}
	if perms < 4 {
		t.Errorf("content:* 权限码应保留，实际 %d 个", perms)
	}
}

// TestLiveReferralAndPointsMergedIntoFinance 验证 070：推广返现与积分中心
// 整域并入财务管理。
//   - 迁移文件可重复执行（幂等）；
//   - 旧 admin 一级域 /referral、/points（含各自叶子）已删除；
//   - 用户端（platform=user）同名的 /points 与 /referral 及其子项必须原样保留；
//   - 新的 /finance/{referral,points} 二级目录与各自叶子已由 seed 插入；
//   - referral:* / point:* 权限码保留（接口仍按它们鉴权）。
//
// 顺序要求：先跑迁移，再重启后端 —— 新行由 seed 在启动时插入，
// 未重启时本用例会在「新菜单未插入」处失败，这是预期的提示。
func TestLiveReferralAndPointsMergedIntoFinance(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}

	sqlBytes, err := os.ReadFile("../../../migrations/070_merge_referral_and_points_into_finance.sql")
	if err != nil {
		t.Fatalf("读取迁移文件失败: %v", err)
	}
	for i := 1; i <= 2; i++ {
		if err := db.Exec(string(sqlBytes)).Error; err != nil {
			t.Fatalf("第 %d 次执行迁移失败: %v", i, err)
		}
	}

	var oldAdminMenus int64
	if err := db.Raw(
		`SELECT count(*) FROM menus WHERE platform = 'admin'
		   AND (path = '/referral' OR path LIKE '/referral/%'
		     OR path = '/points' OR path LIKE '/points/%')`,
	).Scan(&oldAdminMenus).Error; err != nil {
		t.Fatalf("查询旧推广/积分菜单失败: %v", err)
	}
	if oldAdminMenus != 0 {
		t.Errorf("旧 admin 一级域菜单 /referral*、/points* 应已删除，仍存在 %d 行", oldAdminMenus)
	}

	// 用户端同名菜单不能被误删（迁移只作用于 admin）。
	for _, p := range []string{"/points", "/referral", "/referral/overview", "/referral/invitees", "/referral/materials"} {
		var n int64
		if err := db.Raw(
			"SELECT count(*) FROM menus WHERE platform = 'user' AND path = ? AND status = 'active'", p,
		).Scan(&n).Error; err != nil {
			t.Fatalf("查询用户端菜单 %s 失败: %v", p, err)
		}
		if n != 1 {
			t.Errorf("用户端菜单 %s 应原样保留，实际 %d 行", p, n)
		}
	}

	wantPaths := []string{
		"/finance/referral",
		"/finance/referral/cashbacks",
		"/finance/referral/withdrawals",
		"/finance/referral/invitees",
		"/finance/points",
		"/finance/points/overview",
		"/finance/points/rules",
		"/finance/points/accounts",
		"/finance/points/transactions",
	}
	for _, p := range wantPaths {
		var n int64
		if err := db.Raw(
			"SELECT count(*) FROM menus WHERE platform = 'admin' AND path = ? AND status = 'active'", p,
		).Scan(&n).Error; err != nil {
			t.Fatalf("查询菜单 %s 失败: %v", p, err)
		}
		if n != 1 {
			t.Errorf("菜单 %s 应存在且唯一（seed 需在后端重启后写入），实际 %d 行", p, n)
		}
	}

	for _, code := range []string{"referral:%", "point:%"} {
		var n int64
		if err := db.Raw(
			"SELECT count(*) FROM permissions WHERE code LIKE ?", code,
		).Scan(&n).Error; err != nil {
			t.Fatalf("查询 %s 权限失败: %v", code, err)
		}
		if n < 3 {
			t.Errorf("%s 权限码应保留，实际 %d 个", code, n)
		}
	}
}

// TestLiveSystemSettingsGroupedUnderConfig 验证 071：系统配置升级为二级目录，
// 验证码配置 / 第三方登录 / 操作审计 收进同组。
//   - 迁移文件可重复执行（幂等）；
//   - 三条旧一级叶子路径已删除；
//   - /system/config 就地升级为 directory（无 component），且仍唯一；
//   - 四个新叶子（basic / captcha / oauth / audit-logs）已由 seed 插入且组件正确；
//   - 权限码全部保留（system:config:* / captcha:* / oauth:* / security:audit:list）。
//
// 顺序要求：先跑迁移，再重启后端 —— 新行由 seed 在启动时插入，
// 未重启时本用例会在「新菜单未插入」处失败，这是预期的提示。
func TestLiveSystemSettingsGroupedUnderConfig(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}

	sqlBytes, err := os.ReadFile("../../../migrations/071_group_system_settings_under_config.sql")
	if err != nil {
		t.Fatalf("读取迁移文件失败: %v", err)
	}
	for i := 1; i <= 2; i++ {
		if err := db.Exec(string(sqlBytes)).Error; err != nil {
			t.Fatalf("第 %d 次执行迁移失败: %v", i, err)
		}
	}

	var oldMenus int64
	if err := db.Raw(
		"SELECT count(*) FROM menus WHERE platform = 'admin' AND path IN ('/system/captcha','/system/oauth')",
	).Scan(&oldMenus).Error; err != nil {
		t.Fatalf("查询旧入口失败: %v", err)
	}
	if oldMenus != 0 {
		t.Errorf("旧一级叶子 /system/{captcha,oauth} 应已删除，仍存在 %d 行", oldMenus)
	}

	// 操作审计：本迁移删的是「直接挂 /system 下」的旧行；迁移 072 会让同一路径
	// 以日志中心子项的身份重新出现，故这里断言「不得再是 /system 的直接子项」。
	var auditUnderSystem int64
	if err := db.Raw(
		`SELECT count(*) FROM menus m JOIN menus p ON p.id = m.parent_id
		  WHERE m.platform = 'admin' AND m.path = '/system/audit-logs' AND p.path = '/system'`,
	).Scan(&auditUnderSystem).Error; err != nil {
		t.Fatalf("查询操作审计归属失败: %v", err)
	}
	if auditUnderSystem != 0 {
		t.Errorf("/system/audit-logs 不应再直挂 /system（迁移 072 已改挂日志中心），实际 %d 行", auditUnderSystem)
	}

	// 分组节点：就地升级为目录，且只能有一行。
	var groupType, groupComp string
	var groupCount int64
	if err := db.Raw(
		"SELECT count(*) FROM menus WHERE platform = 'admin' AND path = '/system/config'",
	).Scan(&groupCount).Error; err != nil {
		t.Fatalf("查询 /system/config 失败: %v", err)
	}
	if groupCount != 1 {
		t.Fatalf("/system/config 应唯一，实际 %d 行", groupCount)
	}
	if err := db.Raw(
		"SELECT type, COALESCE(component,'') FROM menus WHERE platform = 'admin' AND path = '/system/config'",
	).Row().Scan(&groupType, &groupComp); err != nil {
		t.Fatalf("读取 /system/config 行失败: %v", err)
	}
	if groupType != "directory" || groupComp != "" {
		t.Errorf("/system/config 应为目录且无组件，实际 type=%q component=%q", groupType, groupComp)
	}

	// 操作审计在迁移 072 已改挂日志中心，故不在此列（由 072 的用例负责）。
	wantComponents := map[string]string{
		"/system/config/basic":   "system/config/index",
		"/system/config/captcha": "system/captcha/index",
		"/system/config/oauth":   "system/oauth/index",
	}
	for p, comp := range wantComponents {
		var got string
		if err := db.Raw(
			"SELECT component FROM menus WHERE platform = 'admin' AND path = ? AND status = 'active'", p,
		).Scan(&got).Error; err != nil {
			t.Fatalf("查询菜单 %s 失败: %v", p, err)
		}
		if got != comp {
			t.Errorf("菜单 %s 的 component 应为 %q，实际 %q", p, comp, got)
		}
	}

	for _, code := range []string{"system:config:%", "captcha:%", "oauth:%"} {
		var n int64
		if err := db.Raw("SELECT count(*) FROM permissions WHERE code LIKE ?", code).Scan(&n).Error; err != nil {
			t.Fatalf("查询 %s 权限失败: %v", code, err)
		}
		if n < 1 {
			t.Errorf("%s 权限码应保留，实际 %d 个", code, n)
		}
	}
	var auditPerms int64
	if err := db.Raw(
		"SELECT count(*) FROM permissions WHERE code = 'security:audit:list'",
	).Scan(&auditPerms).Error; err != nil {
		t.Fatalf("查询 security:audit:list 失败: %v", err)
	}
	if auditPerms < 1 {
		t.Error("security:audit:list 权限码应保留（操作审计页在本组内仍用它鉴权）")
	}
}

// TestLiveAuditLogsUnderLogCenter 验证 072：操作审计由「系统配置」组改挂「日志中心」，
// 并恢复原路径 /system/audit-logs。
//   - 迁移文件可重复执行（幂等）；
//   - 071 建立的 /system/config/audit-logs 已删除；
//   - /system/audit-logs 唯一，且 parent 是 /system/log-center、component 未变；
//   - security:audit:list 权限码保留（接口与用户管理侧同一入口共用）。
func TestLiveAuditLogsUnderLogCenter(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}

	sqlBytes, err := os.ReadFile("../../../migrations/072_move_audit_logs_to_log_center.sql")
	if err != nil {
		t.Fatalf("读取迁移文件失败: %v", err)
	}
	for i := 1; i <= 2; i++ {
		if err := db.Exec(string(sqlBytes)).Error; err != nil {
			t.Fatalf("第 %d 次执行迁移失败: %v", i, err)
		}
	}

	var stale int64
	if err := db.Raw(
		"SELECT count(*) FROM menus WHERE platform = 'admin' AND path = '/system/config/audit-logs'",
	).Scan(&stale).Error; err != nil {
		t.Fatalf("查询旧路径失败: %v", err)
	}
	if stale != 0 {
		t.Errorf("/system/config/audit-logs 应已删除，仍存在 %d 行", stale)
	}

	var parentPath, component string
	var n int64
	if err := db.Raw(
		"SELECT count(*) FROM menus WHERE platform = 'admin' AND path = '/system/audit-logs'",
	).Scan(&n).Error; err != nil {
		t.Fatalf("查询 /system/audit-logs 失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("/system/audit-logs 应唯一（seed 需在后端重启后写入），实际 %d 行", n)
	}
	if err := db.Raw(
		`SELECT p.path, m.component FROM menus m JOIN menus p ON p.id = m.parent_id
		  WHERE m.platform = 'admin' AND m.path = '/system/audit-logs'`,
	).Row().Scan(&parentPath, &component); err != nil {
		t.Fatalf("读取操作审计行失败: %v", err)
	}
	if parentPath != "/system/log-center" {
		t.Errorf("操作审计的父级应为 /system/log-center，实际 %q", parentPath)
	}
	if component != "system/audit-logs/index" {
		t.Errorf("操作审计 component 应为 system/audit-logs/index，实际 %q", component)
	}

	var perms int64
	if err := db.Raw(
		"SELECT count(*) FROM permissions WHERE code = 'security:audit:list'",
	).Scan(&perms).Error; err != nil {
		t.Fatalf("查询 security:audit:list 失败: %v", err)
	}
	if perms < 1 {
		t.Error("security:audit:list 权限码应保留")
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
//   - /users/levels 不再存在（063 已归入 /users/accounts/levels）。
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
	if n := count("SELECT count(*) FROM menus WHERE platform = 'admin' AND path = '/users/levels'"); n != 0 {
		t.Errorf("旧路径 /users/levels 应已清理（063 归入账户管理），实际 %d 行", n)
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

// TestLiveFinanceModuleReorg 验证 073 财务模块整理迁移：
//   - 迁移文件可重复执行（幂等）；
//   - 资金流水改挂「资金管理」组（/finance/accounts）且组内排序为 流水1/钱包2/调账3；
//   - 目录改名生效：/finance/accounts=资金管理、/finance/bill-center=账单与对账；
//   - 5 个历史死键清零，且新键（finance.adjust_enabled 等）已由 seed 就位并归 finance 分组；
//   - 财务参数页权限码 finance:config 已登记且已授 finance_admin。
func TestLiveFinanceModuleReorg(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}

	sqlBytes, err := os.ReadFile("../../../migrations/073_finance_module_reorg.sql")
	if err != nil {
		t.Fatalf("读取迁移文件失败: %v", err)
	}
	for i := 1; i <= 2; i++ {
		if err := db.Exec(string(sqlBytes)).Error; err != nil {
			t.Fatalf("第 %d 次执行迁移失败: %v", i, err)
		}
	}

	// ① 资金流水改挂资金管理组
	var parentPath, name string
	var sortOrder int
	if err := db.Raw(
		`SELECT p.path, m.name, m.sort_order FROM menus m JOIN menus p ON p.id = m.parent_id
		  WHERE m.platform = 'admin' AND m.path = '/finance/transactions'`,
	).Row().Scan(&parentPath, &name, &sortOrder); err != nil {
		t.Fatalf("读取资金流水行失败: %v", err)
	}
	if parentPath != "/finance/accounts" {
		t.Errorf("资金流水的父级应为 /finance/accounts，实际 %q", parentPath)
	}
	if name != "资金流水" {
		t.Errorf("资金流水名称不应变化，实际 %q", name)
	}
	if sortOrder != 1 {
		t.Errorf("资金流水在组内应排第 1，实际 %d", sortOrder)
	}

	// 组内其余叶子排序：原来断言「钱包 2 / 调账 3」；用户钱包行已由迁移 074 删除，
	// 这里只校验本迁移自身的产物 —— 人工调账在组内排第 3（074 会把空洞补成 2）。
	for path, want := range map[string]int{"/finance/accounts/adjust": 3} {
		var got int
		if err := db.Raw(
			"SELECT sort_order FROM menus WHERE platform = 'admin' AND path = ?", path,
		).Scan(&got).Error; err != nil {
			t.Fatalf("读取 %s 排序失败: %v", path, err)
		}
		if got != want {
			t.Errorf("%s 的 sort_order 应为 %d，实际 %d", path, want, got)
		}
	}

	// ② 目录改名
	for path, want := range map[string]string{
		"/finance/accounts":    "资金管理",
		"/finance/bill-center": "账单与对账",
	} {
		var got string
		if err := db.Raw(
			"SELECT name FROM menus WHERE platform = 'admin' AND path = ?", path,
		).Scan(&got).Error; err != nil {
			t.Fatalf("读取 %s 名称失败: %v", path, err)
		}
		if got != want {
			t.Errorf("%s 的名称应为 %q，实际 %q", path, want, got)
		}
	}

	// ③ 死键清零
	var deadCount int64
	if err := db.Raw(
		"SELECT count(*) FROM system_configs WHERE config_key IN ('finance_billing_cycle','finance_tax_rate','finance_recon_threshold','finance_balance_warning','finance_manual_adjust_enabled')",
	).Scan(&deadCount).Error; err != nil {
		t.Fatalf("查询死键失败: %v", err)
	}
	if deadCount != 0 {
		t.Errorf("5 个历史死键应已清除，仍存在 %d 个", deadCount)
	}

	// ④ 新键就位（seed 写入 finance 分组）
	for _, key := range []string{"finance.adjust_enabled", "finance.recon_tolerance", "finance.balance_warning"} {
		var group, value string
		if err := db.Raw(
			"SELECT config_group, config_value FROM system_configs WHERE config_key = ?", key,
		).Row().Scan(&group, &value); err != nil {
			t.Fatalf("读取 %s 失败（需先重启后端写入 seed）: %v", key, err)
		}
		if group != "finance" {
			t.Errorf("%s 的分组应为 finance，实际 %q", key, group)
		}
		if value == "" {
			t.Errorf("%s 的取值不应为空", key)
		}
	}

	// ⑤ 财务参数权限码与授权
	var permCount int64
	if err := db.Raw("SELECT count(*) FROM permissions WHERE code = 'finance:config'").Scan(&permCount).Error; err != nil {
		t.Fatalf("查询 finance:config 失败: %v", err)
	}
	if permCount != 1 {
		t.Errorf("finance:config 权限码应唯一存在，实际 %d 行", permCount)
	}
	var grantCount int64
	if err := db.Raw(
		`SELECT count(*) FROM role_permissions rp
		   JOIN roles r ON r.id = rp.role_id
		   JOIN permissions p ON p.id = rp.permission_id
		  WHERE r.code = 'finance_admin' AND p.code = 'finance:config'`,
	).Scan(&grantCount).Error; err != nil {
		t.Fatalf("查询 finance_admin 授权失败: %v", err)
	}
	if grantCount < 1 {
		t.Error("finance_admin 应已获得 finance:config 授权")
	}

	// 财务参数接口的三条路由权限码均在权限目录中（finance:wallet/finance:config）
	var walletPerm int64
	if err := db.Raw("SELECT count(*) FROM permissions WHERE code = 'finance:wallet'").Scan(&walletPerm).Error; err != nil {
		t.Fatalf("查询 finance:wallet 失败: %v", err)
	}
	if walletPerm < 1 {
		t.Error("finance:wallet 权限码应存在（统计接口与流水导出沿用）")
	}
}

// TestLiveFinanceWalletsPageRemoved 验证 074 下线「用户钱包」页迁移：
//   - 迁移文件可重复执行（幂等）；
//   - 菜单行 /finance/accounts/wallets 已删除，且 seed 不再声明该路径（重建也不会回来）；
//   - 资金管理组只剩「资金流水(1)」「人工调账(2)」，排序连续、目录仍满足 R1（≥2 叶子）；
//   - 接口与权限码保留：GET /finance/wallets/:user_id 走 finance:wallet，用户详情页财务面板依赖它。
func TestLiveFinanceWalletsPageRemoved(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}

	sqlBytes, err := os.ReadFile("../../../migrations/074_remove_finance_wallets_page.sql")
	if err != nil {
		t.Fatalf("读取迁移文件失败: %v", err)
	}
	for i := 1; i <= 2; i++ {
		if err := db.Exec(string(sqlBytes)).Error; err != nil {
			t.Fatalf("第 %d 次执行迁移失败: %v", i, err)
		}
	}

	// ① 菜单行已删除
	var wallets int64
	if err := db.Raw(
		"SELECT count(*) FROM menus WHERE platform = 'admin' AND path = '/finance/accounts/wallets'",
	).Scan(&wallets).Error; err != nil {
		t.Fatalf("查询用户钱包菜单行失败: %v", err)
	}
	if wallets != 0 {
		t.Errorf("/finance/accounts/wallets 菜单行应已删除，仍有 %d 行", wallets)
	}

	// ② seed 不再声明该路径（否则重启后端会把它插回来）
	for _, item := range SeedMenus() {
		if item.Platform == "admin" && item.Path == "/finance/accounts/wallets" {
			t.Error("seed 仍在声明 /finance/accounts/wallets，重启后会重新插入")
		}
	}

	// ③ 组内两个叶子且排序连续
	rows, err := db.Raw(
		`SELECT m.path, m.sort_order FROM menus m JOIN menus p ON p.id = m.parent_id
		  WHERE m.platform = 'admin' AND p.path = '/finance/accounts' ORDER BY m.sort_order`,
	).Rows()
	if err != nil {
		t.Fatalf("查询资金管理组叶子失败: %v", err)
	}
	defer rows.Close()
	got := map[string]int{}
	for rows.Next() {
		var path string
		var order int
		if err := rows.Scan(&path, &order); err != nil {
			t.Fatalf("扫描行失败: %v", err)
		}
		got[path] = order
	}
	if len(got) != 2 {
		t.Errorf("资金管理组应只剩 2 个叶子（R1 仍成立），实际 %d 个: %v", len(got), got)
	}
	if got["/finance/transactions"] != 1 {
		t.Errorf("资金流水应排第 1，实际 %d", got["/finance/transactions"])
	}
	if got["/finance/accounts/adjust"] != 2 {
		t.Errorf("人工调账应排第 2（空洞补齐），实际 %d", got["/finance/accounts/adjust"])
	}

	// ④ 接口与权限码保留
	var permCount int64
	if err := db.Raw("SELECT count(*) FROM permissions WHERE code = 'finance:wallet'").Scan(&permCount).Error; err != nil {
		t.Fatalf("查询 finance:wallet 失败: %v", err)
	}
	if permCount != 1 {
		t.Errorf("finance:wallet 权限码应保留（用户详情页财务面板与统计接口共用），实际 %d 行", permCount)
	}
}

// TestLiveCostManagementSeed 验证成本管理（doc111）的菜单与权限四处对齐：
//   - seed 声明「成本管理」二级目录 + 3 个叶子（总览 / 成本项配置 / 上游余额台账）；
//   - 库中目录与叶子已就位、排序 1/2/3、组件路径与前端页面目录一致；
//   - 权限码 finance:cost:overview / item / balance 三个齐备，且 finance_admin 已授权；
//   - 成本管理目录满足 R1（≥2 启用叶子才留目录）。
//
// 顺序要求：先重启后端让 seed 生效，再跑本用例；未重启时会在「菜单未插入」处失败。
func TestLiveCostManagementSeed(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}

	// ① seed 必须声明目录与 3 个叶子（否则新环境重建不出来）
	wantSeed := map[string]string{
		"/finance/cost":           "",
		"/finance/cost/overview":  "finance/cost/overview/index",
		"/finance/cost/items":     "finance/cost/items/index",
		"/finance/cost/upstreams": "finance/cost/upstreams/index",
	}
	declared := map[string]string{}
	for _, item := range SeedMenus() {
		if item.Platform != "admin" {
			continue
		}
		if _, ok := wantSeed[item.Path]; ok {
			declared[item.Path] = item.Component
		}
	}
	for path, component := range wantSeed {
		got, ok := declared[path]
		if !ok {
			t.Errorf("seed 未声明成本管理菜单 %s", path)
			continue
		}
		if got != component {
			t.Errorf("seed 菜单 %s 组件应为 %q，实际 %q", path, component, got)
		}
	}

	// ② 库中已就位：目录是 directory、叶子排序连续、R1 成立
	var dirType string
	var dirName string
	if err := db.Raw(
		"SELECT type, name FROM menus WHERE platform = 'admin' AND path = '/finance/cost'",
	).Row().Scan(&dirType, &dirName); err != nil {
		t.Fatalf("查询 /finance/cost 目录失败（seed 需在后端重启后写入）: %v", err)
	}
	if dirType != "directory" {
		t.Errorf("/finance/cost 应为二级目录，实际 type=%s", dirType)
	}
	if dirName != "成本管理" {
		t.Errorf("/finance/cost 名称应为「成本管理」，实际 %q", dirName)
	}

	rows, err := db.Raw(
		`SELECT m.path, m.sort_order, m.component FROM menus m JOIN menus p ON p.id = m.parent_id
		  WHERE m.platform = 'admin' AND p.path = '/finance/cost' AND m.status = 'active'
		  ORDER BY m.sort_order`,
	).Rows()
	if err != nil {
		t.Fatalf("查询成本管理叶子失败: %v", err)
	}
	defer rows.Close()
	type leaf struct {
		order     int
		component string
	}
	got := map[string]leaf{}
	for rows.Next() {
		var path, component string
		var order int
		if err := rows.Scan(&path, &order, &component); err != nil {
			t.Fatalf("扫描行失败: %v", err)
		}
		got[path] = leaf{order: order, component: component}
	}
	if len(got) < 2 {
		t.Errorf("成本管理目录应满足 R1（≥2 启用叶子），实际 %d 个: %v", len(got), got)
	}
	wantOrder := map[string]int{"/finance/cost/overview": 1, "/finance/cost/items": 2, "/finance/cost/upstreams": 3}
	for path, order := range wantOrder {
		l, ok := got[path]
		if !ok {
			t.Errorf("成本管理叶子 %s 未入库", path)
			continue
		}
		if l.order != order {
			t.Errorf("叶子 %s 排序应为 %d，实际 %d", path, order, l.order)
		}
		if l.component != wantSeed[path] {
			t.Errorf("叶子 %s 组件应为 %q，实际 %q", path, wantSeed[path], l.component)
		}
	}

	// ③ 权限码齐备
	for _, code := range []string{"finance:cost:overview", "finance:cost:item", "finance:cost:balance"} {
		var n int64
		if err := db.Raw("SELECT count(*) FROM permissions WHERE code = ?", code).Scan(&n).Error; err != nil {
			t.Fatalf("查询权限码 %s 失败: %v", code, err)
		}
		if n != 1 {
			t.Errorf("权限码 %s 应存在且唯一，实际 %d 行", code, n)
		}
	}

	// ④ finance_admin 已获授权（成本页默认对财务管理员可见）
	var granted int64
	if err := db.Raw(
		`SELECT count(*) FROM role_permissions rp
		   JOIN roles r ON r.id = rp.role_id
		   JOIN permissions p ON p.id = rp.permission_id
		  WHERE r.code = 'finance_admin' AND p.code LIKE 'finance:cost:%'`,
	).Scan(&granted).Error; err != nil {
		t.Fatalf("查询 finance_admin 授权失败: %v", err)
	}
	if granted < 3 {
		t.Errorf("finance_admin 应获 3 个 finance:cost:* 权限，实际 %d 个", granted)
	}
}
