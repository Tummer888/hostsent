package service

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	"hostsent/backend/internal/modules/admin/order/model"
	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// TestActivateProvisioningAdoptsExistingHost 回归（实测用户场景）：
//
//	订单开通失败（上游余额不足）→ 用户/运营在**上游侧**充值并开通
//	→ 本地重试时必须把上游那台机器接回来，而不是直接标"服务中"或一直卡创建中。
//
// provisioning 分支此前的两处错误：
//  1. 「已记录实例 → 直接激活」用的是"本地是否有该商品的实例行"，而下单即建的
//     待开通占位行同样满足该条件 —— 机器没开出来订单就被标 active；
//  2. 修正①后若只看"实例行存在"而不回源，仍无法把上游主机的真实状态带回来。
//
// 现在：先回源（provision）拿真实状态，running 才激活；creating 则保持 provisioning，
// 由滞留推进器继续跟进。
func TestActivateProvisioningAdoptsExistingHost(t *testing.T) {
	cases := []struct {
		name        string
		hostStatus  pkgmodel.StandardInstanceStatus
		hasInstance bool
		wantStatus  string
	}{
		{"上游主机已就绪 → 服务中", pkgmodel.InstanceStatusRunning, true, model.OrderStatusActive},
		{"上游主机仍创建中 → 保持开通中", pkgmodel.InstanceStatusCreating, true, model.OrderStatusProvisioning},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var recorded []string
			adapter := NewUpstreamProvisionAdapter(ProvisionDeps{
				BuildProvisionRequest: func(context.Context, uint64, string, string, string, map[string]string) (interface{}, error) {
					return provisionRequestForTest(), nil
				},
				BuildProviderConfig: func(context.Context, uint64) (*upstream.ProviderConfig, error) {
					return &upstream.ProviderConfig{ID: 7, Type: "zjmf"}, nil
				},
				CreateInstance: func(context.Context, *upstream.ProviderConfig, *pkgmodel.CreateInstanceRequest) (*pkgmodel.StandardInstance, error) {
					return &pkgmodel.StandardInstance{
						ProviderID: 7,
						UpstreamID: "2071",
						Status:     c.hostStatus,
					}, nil
				},
				RecordInstance: func(_ context.Context, inst *pkgmodel.StandardInstance, _ *model.Order) error {
					recorded = append(recorded, inst.UpstreamID)
					return nil
				},
				HasInstance: func(context.Context, uint64, uint64) (bool, error) {
					return c.hasInstance, nil
				},
				Logger: zap.NewNop(),
			})

			order := &model.Order{
				ID: 848, UserID: 2236, ProductID: 176,
				Status: model.OrderStatusProvisioning,
			}
			if err := adapter.Activate(context.Background(), order); err != nil {
				t.Fatalf("Activate failed: %v", err)
			}
			if order.Status != c.wantStatus {
				t.Fatalf("订单状态 = %q, want %q（主机状态 %q）", order.Status, c.wantStatus, c.hostStatus)
			}
			// 无论是否激活，上游主机的真实状态都必须回写本地实例。
			if len(recorded) == 0 {
				t.Fatal("必须把回源拿到的上游主机状态落库（否则本地看不到已开通的机器）")
			}
		})
	}
}

// TestActivatePaidRecoversWhenUpstreamPaid 订单处于 paid（本地上次失败）时，
// 重试同样要能接管上游已有的机器，并在就绪时置服务中。
func TestActivatePaidRecoversWhenUpstreamPaid(t *testing.T) {
	order := &model.Order{ID: 849, UserID: 2236, ProductID: 175, Status: model.OrderStatusPaid}
	var provisioned bool
	adapter := NewUpstreamProvisionAdapter(ProvisionDeps{
		BuildProvisionRequest: func(context.Context, uint64, string, string, string, map[string]string) (interface{}, error) {
			return provisionRequestForTest(), nil
		},
		BuildProviderConfig: func(context.Context, uint64) (*upstream.ProviderConfig, error) {
			return &upstream.ProviderConfig{ID: 7, Type: "zjmf"}, nil
		},
		CreateInstance: func(context.Context, *upstream.ProviderConfig, *pkgmodel.CreateInstanceRequest) (*pkgmodel.StandardInstance, error) {
			provisioned = true
			return &pkgmodel.StandardInstance{ProviderID: 7, UpstreamID: "2070", Status: pkgmodel.InstanceStatusRunning}, nil
		},
		RecordInstance: func(context.Context, *pkgmodel.StandardInstance, *model.Order) error { return nil },
		HasInstance:    func(context.Context, uint64, uint64) (bool, error) { return false, nil },
		Logger:         zap.NewNop(),
	})

	if err := adapter.Activate(context.Background(), order); err != nil {
		t.Fatalf("Activate failed: %v", err)
	}
	if !provisioned {
		t.Fatal("paid 订单重试必须回到上游核对（可能已被上游开通）")
	}
	if order.Status != model.OrderStatusActive {
		t.Fatalf("订单状态 = %q, want active（上游主机 running）", order.Status)
	}
}

// TestActivateKeepsRetryableOnTransientError 上游网络错误必须保持可重试状态，
// 不得把订单标成服务中（否则用户拿到一台不存在的机器）。
func TestActivateKeepsRetryableOnTransientError(t *testing.T) {
	order := &model.Order{ID: 850, UserID: 2236, ProductID: 175, Status: model.OrderStatusPaid}
	adapter := NewUpstreamProvisionAdapter(ProvisionDeps{
		BuildProvisionRequest: func(context.Context, uint64, string, string, string, map[string]string) (interface{}, error) {
			return provisionRequestForTest(), nil
		},
		BuildProviderConfig: func(context.Context, uint64) (*upstream.ProviderConfig, error) {
			return &upstream.ProviderConfig{ID: 7, Type: "zjmf"}, nil
		},
		CreateInstance: func(context.Context, *upstream.ProviderConfig, *pkgmodel.CreateInstanceRequest) (*pkgmodel.StandardInstance, error) {
			return nil, errors.New("connection reset by peer")
		},
		RecordInstance: func(context.Context, *pkgmodel.StandardInstance, *model.Order) error { return nil },
		HasInstance:    func(context.Context, uint64, uint64) (bool, error) { return false, nil },
		Logger:         zap.NewNop(),
	})

	if err := adapter.Activate(context.Background(), order); err == nil {
		t.Fatal("上游错误必须向上抛（交由工作池退避重试）")
	}
	if order.Status != model.OrderStatusPaid {
		t.Fatalf("订单状态 = %q, want paid（失败不得推进）", order.Status)
	}
}

// testProvisionRequest 履约请求替身：providerID()/extractConfigOptions() 按字段名反射读取，
// 因此只需同名同类型的字段（不必依赖 catalog 包的具体类型）。
type testProvisionRequest struct {
	ProviderID    uint64                 `json:"provider_id"`
	ConfigOptions map[string]interface{} `json:"config_options"`
}

// provisionRequestForTest 造一个带提供商 ID 的开通请求（providerID() 能解析出 7）。
func provisionRequestForTest() testProvisionRequest {
	return testProvisionRequest{
		ProviderID:    7,
		ConfigOptions: map[string]interface{}{"upstream_pid": "529"},
	}
}
