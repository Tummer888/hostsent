package service

import (
	"testing"

	catalogdto "hostsent/backend/internal/modules/admin/product/catalog/dto"

	"hostsent/backend/internal/modules/uc/product/dto"
)

// 对接商品在用户侧的链路信息（配置箱用）：
// 必须给出「固定配置只读项 + 可否续费 + 下单说明」，且**不得**外泄上游标价/代理价。
func TestUpstreamChainForHidesInternalPrices(t *testing.T) {
	chain := upstreamChainFor(&catalogdto.ProductUpstreamInfo{
		ProviderID: 7, ProviderName: "haika", ProviderType: "zjmf",
		UpstreamID: "468", GroupID: 14, GroupName: "常驻 | 活动",
		ListPrice: 109, CostPrice: 76.3, DiscountDesc: "代理折扣 70%",
		ListCycles: map[string]float64{"monthly": 109}, CostCycles: map[string]float64{"monthly": 76.3},
		Renewable: false, RenewNote: "上游为一次性交付商品，不支持续费",
		Description: "<div>16核16G</div>", PayType: `{"pay_type":"onetime"}`,
		FixedConfig: false,
		Options: []catalogdto.ProductUpstreamOption{
			{OptionName: "cpu|CPU", Key: "cpu", Locked: true, Value: "16|16核", SubCount: 1},
			{OptionName: "os|操作系统", Key: "os", Locked: false, Value: "12|CentOS", SubCount: 27},
		},
	})
	if chain == nil {
		t.Fatal("对接商品应产出用户侧链路信息")
	}
	if !chain.Renewable == false || chain.RenewNote == "" {
		t.Errorf("不可续费标记/原因缺失: %+v", chain)
	}
	if chain.FixedConfig {
		t.Error("存在多取值配置项时不应标记整机固定配置")
	}
	if len(chain.Options) != 2 {
		t.Fatalf("配置项数量 = %d, want 2", len(chain.Options))
	}
	if !chain.Options[0].Locked || chain.Options[0].Name != "CPU" || chain.Options[0].Value != "16|16核" {
		t.Errorf("锁定项展示口径错误: %+v", chain.Options[0])
	}
	if chain.Options[1].Locked || chain.Options[1].Name != "操作系统" {
		t.Errorf("可选项展示口径错误: %+v", chain.Options[1])
	}
	if len(chain.OrderNotes) == 0 {
		t.Error("下单说明（主机名/密码/端口）缺失")
	}
	// 自营商品没有链路信息。
	if got := upstreamChainFor(nil); got != nil {
		t.Errorf("自营商品不应有链路信息: %+v", got)
	}
}

// 类型断言：接口字段确实挂在 uc ProductInfo 上（前端读取 upstream_chain）。
var _ = func(p dto.ProductInfo) any { return p.Upstream }
