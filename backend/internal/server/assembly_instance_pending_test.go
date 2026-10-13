package server

import (
	"context"
	"errors"
	"testing"

	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
)

// TestBuildInstancePlaceholder 锁定「下单即建待开通实例占位」的关键口径。
//
// 用户实测问题：订单开通失败（上游余额不足）后实例模块里什么都看不到，
// 既无法查看也没法重新开通。占位行随订单创建，开通成功时就地转正。
func TestBuildInstancePlaceholder(t *testing.T) {
	baseOrder := func() *ordermodel.Order {
		return &ordermodel.Order{
			ID:          832,
			OrderNo:     "AO223617917947318823",
			UserID:      2236,
			ProductID:   176,
			ProductName: "香港|六区·轻量 4-4 15Mbps",
			Specs:       `{"cpu":4,"memory":4096,"disk":40,"os":"CentOS"}`,
			Cycle:       "monthly",
			OperatorID:  2236,
		}
	}

	t.Run("可开通商品：建占位（待开通、无上游实例号）", func(t *testing.T) {
		var got []syncmodel.Instance
		hook := buildInstancePlaceholder(instancePlaceholderPorts{
			provisionProviderID: func(context.Context, *ordermodel.Order) uint64 { return 7 },
			productLink: func(context.Context, uint64) (string, uint64) {
				return "upstream", 120
			},
			create: func(_ context.Context, row *syncmodel.Instance) error {
				got = append(got, *row)
				return nil
			},
		}, nil)

		if err := hook(context.Background(), baseOrder()); err != nil {
			t.Fatalf("hook err: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("占位行数 = %d, want 1", len(got))
		}
		row := got[0]
		if row.InstanceID != "pending-AO223617917947318823" {
			t.Errorf("占位号 = %q", row.InstanceID)
		}
		if row.ProviderInstanceID != "" {
			t.Errorf("占位行不得带上游实例号（转正判据），实际 %q", row.ProviderInstanceID)
		}
		if row.Status != "creating" || row.OrderID != 832 || row.UserID != 2236 {
			t.Errorf("占位行关键字段错误: %+v", row)
		}
		if row.SourceMode != "upstream" || row.UpstreamProductID != 120 || row.SellProductID != 176 {
			t.Errorf("链路语义错误: source_mode=%q upstream_product_id=%d sell_product_id=%d",
				row.SourceMode, row.UpstreamProductID, row.SellProductID)
		}
		if row.ExpireAt != nil {
			t.Error("未开通的占位行不应有账期（否则生命周期推进器会当作到期实例处置）")
		}
		if row.CPU != 4 || row.Memory != 4096 || row.OS != "CentOS" {
			t.Errorf("规格快照未带上: %+v", row)
		}
	})

	t.Run("不可开通商品/续费单：不建占位", func(t *testing.T) {
		called := 0
		hook := buildInstancePlaceholder(instancePlaceholderPorts{
			provisionProviderID: func(context.Context, *ordermodel.Order) uint64 { return 0 },
			create: func(context.Context, *syncmodel.Instance) error {
				called++
				return nil
			},
		}, nil)

		if err := hook(context.Background(), baseOrder()); err != nil {
			t.Fatalf("hook err: %v", err)
		}
		if called != 0 {
			t.Fatalf("无提供商时不应建占位（虚拟商品不建），实际建成 %d 行", called)
		}

		renewal := baseOrder()
		renewal.RenewalID = 99
		hook2 := buildInstancePlaceholder(instancePlaceholderPorts{
			provisionProviderID: func(context.Context, *ordermodel.Order) uint64 { return 7 },
			create: func(context.Context, *syncmodel.Instance) error {
				called++
				return nil
			},
		}, nil)
		if err := hook2(context.Background(), renewal); err != nil {
			t.Fatalf("hook err: %v", err)
		}
		if called != 0 {
			t.Fatal("续费订单不应建占位（续费是对既有实例延期）")
		}
	})

	t.Run("落库失败向上返回（由调用方记日志，不阻断下单）", func(t *testing.T) {
		hook := buildInstancePlaceholder(instancePlaceholderPorts{
			provisionProviderID: func(context.Context, *ordermodel.Order) uint64 { return 7 },
			create: func(context.Context, *syncmodel.Instance) error {
				return errors.New("db down")
			},
		}, nil)
		if err := hook(context.Background(), baseOrder()); err == nil {
			t.Fatal("落库失败必须回传错误给调用方（调用方只记日志）")
		}
	})
}
