//go:build live

package repository

import (
	"context"
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
)

// TestPendingInstanceLifecycle 针对真实库验证「待开通实例占位」的三步生命周期：
// 下单建占位（幂等）→ 开通成功就地转正（不留第二行）→ 订单作废删除占位。
//
// 用户实测问题：开通失败后实例模块里什么都看不到、无法重新开通。
// 占位行是"待开通"可见性的载体，转正/删除的 SQL 语义必须与运行期一致。
//
// 仅在 -tags live 时运行：go test -tags live ./internal/modules/admin/resource/sync/repository/
func TestPendingInstanceLifecycle(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Skipf("无可用测试库：%v", err)
	}
	// 事务包裹并回滚，避免污染真实库。
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("开启事务失败: %v", tx.Error)
	}
	defer tx.Rollback()

	ctx := context.Background()
	repo := NewSyncRepository(tx)

	const orderID = uint64(9999201)
	const userID = uint64(2236) // 取真实账号：instances.user_id 有外键
	placeholder := func() *syncmodel.Instance {
		return &syncmodel.Instance{
			InstanceID:        "pending-LIVETEST-9999201",
			ProviderID:        7,
			UserID:            userID,
			Name:              "live-test-pending",
			CPU:               4,
			Memory:            4096,
			Disk:              40,
			Status:            "creating",
			BillingMode:       "monthly",
			OrderID:           orderID,
			SourceMode:        "upstream",
			SellProductID:     176,
			UpstreamProductID: 120,
		}
	}

	// 1) 建占位（幂等：重复下单/重试不会建出第二行）。
	if err := repo.CreatePendingInstance(ctx, placeholder()); err != nil {
		t.Fatalf("建占位失败: %v", err)
	}
	if err := repo.CreatePendingInstance(ctx, placeholder()); err != nil {
		t.Fatalf("重复建占位应幂等无错: %v", err)
	}
	var n int64
	if err := tx.Model(&syncmodel.Instance{}).Where("order_id = ?", orderID).Count(&n).Error; err != nil {
		t.Fatalf("计数失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("占位行数 = %d, want 1（幂等失效会留下幽灵实例）", n)
	}

	// 2) 开通成功：占位行就地转正（实例号换成上游号、状态/规格/IP/账期一次补齐）。
	real := placeholder()
	real.InstanceID = "1999992001"
	real.ProviderInstanceID = "1999992001"
	real.Status = "running"
	real.PublicIP = "203.0.113.9"
	real.OS = "CentOS"
	adopted, err := repo.AdoptPendingInstance(ctx, orderID, real)
	if err != nil {
		t.Fatalf("转正失败: %v", err)
	}
	if !adopted {
		t.Fatal("转正未命中占位行：会出现「待开通」幽灵行 + 真实行两条记录")
	}
	var row syncmodel.Instance
	if err := tx.Where("order_id = ?", orderID).Take(&row).Error; err != nil {
		t.Fatalf("转正后查询失败: %v", err)
	}
	if row.InstanceID != "1999992001" || row.ProviderInstanceID != "1999992001" || row.Status != "running" || row.PublicIP != "203.0.113.9" {
		t.Fatalf("转正字段未写入: %+v", row)
	}

	// 3) 已转正的行不再被"删除占位"误删（订单取消/关单只清未开通的占位）。
	deleted, err := repo.DeletePendingInstanceByOrder(ctx, orderID)
	if err != nil {
		t.Fatalf("删除占位失败: %v", err)
	}
	if deleted != 0 {
		t.Fatal("已开通（已转正）的实例不得被占位清理删除")
	}

	// 4) 未开通的占位：删除生效。
	second := placeholder()
	second.InstanceID = "pending-LIVETEST-9999202"
	second.OrderID = 9999202
	if err := repo.CreatePendingInstance(ctx, second); err != nil {
		t.Fatalf("建第二个占位失败: %v", err)
	}
	deleted, err = repo.DeletePendingInstanceByOrder(ctx, 9999202)
	if err != nil {
		t.Fatalf("删除第二个占位失败: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("删除行数 = %d, want 1", deleted)
	}
}
