//go:build live

package db

import (
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	discountmodel "hostsent/backend/internal/modules/admin/product/discount/model"
	distributionmodel "hostsent/backend/internal/modules/admin/user/distribution/model"
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
			name:   "025_p6_agent_pricing",
			file:   "../../../migrations/025_p6_agent_pricing.sql",
			models: []interface{}{&distributionmodel.AgentLevel{}, &distributionmodel.Commission{}},
			columns: map[string][]string{
				"agent_levels":             {"price_policy_id"},
				"distribution_commissions": {"order_id"},
			},
			indexes: []string{"idx_agent_levels_price_policy_id", "uk_commissions_order_agent"},
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
