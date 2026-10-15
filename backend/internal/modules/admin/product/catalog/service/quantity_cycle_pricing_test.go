package service

import (
	"context"
	"testing"

	catalogmodel "hostsent/backend/internal/modules/admin/product/catalog/model"
	"hostsent/backend/internal/pkg/billingcycle"
)

// qtyRowForCycleTest 数量型配置项：系统盘 80 免费、81 起加价。
// 三个周期各有单价（上游真实数据：0.80/月、2.40/季、9.60/年）。
func qtyRowForCycleTest() *catalogmodel.ProductConfigOption {
	return &catalogmodel.ProductConfigOption{
		// Source/UpstreamKey 决定选配键：上游项用上游配置项 id（= 下单时 config_selections 的键）。
		Source:       catalogmodel.ConfigSourceUpstream,
		UpstreamKey:  241017,
		OptionType:   19,
		Widget:       catalogmodel.ConfigWidgetQty,
		OptionName:   "system_disk_size|系统盘",
		MinValue:     floatPtr(80),
		MaxValue:     floatPtr(1024),
		DefaultValue: "80",
		Subs: []catalogmodel.ProductConfigOptionSub{
			{OptionName: "系统盘", QtyMinimum: 80, QtyMaximum: 80, PriceMonthly: 0},
			{OptionName: "系统盘", QtyMinimum: 81, QtyMaximum: 1024,
				PriceMonthly: 0.8, PriceQuarterly: 2.4, PriceAnnually: 9.6},
		},
	}
}

// TestQuantityUnitPriceForCycle 数量型单位加价必须随周期变。
//
// 实测故障（用户反馈）：原实现恒取月度单价，于是选年付时"套餐主体按年计价、
// 加配的盘却仍只收一个月"——数据盘加 20GB，年付应加 192 元（20×9.6），实际只加 16 元。
func TestQuantityUnitPriceForCycle(t *testing.T) {
	row := qtyRowForCycleTest()
	cases := []struct {
		cycle string
		want  float64
	}{
		{billingcycle.Monthly, 0.8},
		{billingcycle.Quarterly, 2.4},
		{billingcycle.Annually, 9.6},
	}
	for _, c := range cases {
		if got := quantityUnitPriceForCycle(row, c.cycle); got != c.want {
			t.Errorf("%s 单位加价 = %v, want %v", c.cycle, got, c.want)
		}
	}
}

// TestQuantityUnitPriceForCycleFallback 该周期无独立单价时回落月度价。
//
// 上游老数据可能只填了月价；回落保证这类商品不会被算成 0 元加价
// （0 元加价比报错更糟：客户白拿配置且无人察觉）。
func TestQuantityUnitPriceForCycleFallback(t *testing.T) {
	row := &catalogmodel.ProductConfigOption{
		Source:      catalogmodel.ConfigSourceUpstream,
		UpstreamKey: 241021,
		OptionType:  14,
		Widget:      catalogmodel.ConfigWidgetQty,
		Subs: []catalogmodel.ProductConfigOptionSub{
			{OptionName: "数据盘", PriceMonthly: 0.8}, // 只有月价
		},
	}
	for _, cycle := range []string{billingcycle.Quarterly, billingcycle.Annually, billingcycle.Onetime} {
		if got := quantityUnitPriceForCycle(row, cycle); got != 0.8 {
			t.Errorf("%s 无独立价应回落月价 0.8, got %v", cycle, got)
		}
	}
	// 全部为 0（免费项）时不该被误判成"有价"，返回 0。
	free := &catalogmodel.ProductConfigOption{
		Source:      catalogmodel.ConfigSourceUpstream,
		UpstreamKey: 241021,
		OptionType:  14,
		Subs:        []catalogmodel.ProductConfigOptionSub{{OptionName: "数据盘"}},
	}
	if got := quantityUnitPriceForCycle(free, billingcycle.Monthly); got != 0 {
		t.Errorf("免费项单位加价 = %v, want 0", got)
	}
}

// TestResolveOptionPricingQuantityScalesByCycle 端到端：数量型加价按周期放大。
//
// 这是用户举的那个例子：主体价按年付算时，加配的盘也必须按年算，
// 而不是只加一个月的钱。
func TestResolveOptionPricingQuantityScalesByCycle(t *testing.T) {
	row := qtyRowForCycleTest()
	repo := &fakeProductRepo{configOptions: []*catalogmodel.ProductConfigOption{row}}
	svc := &productService{repo: repo}

	cases := []struct {
		cycle string
		want  float64 // 100GB → 超量 20 单位
	}{
		{billingcycle.Monthly, 16},   // 20 × 0.8
		{billingcycle.Quarterly, 48}, // 20 × 2.4
		{billingcycle.Annually, 192}, // 20 × 9.6
	}
	for _, c := range cases {
		got, err := svc.ResolveOptionPricing(context.Background(), 7, c.cycle, map[string]string{"241017": "100"})
		if err != nil {
			t.Fatalf("%s ResolveOptionPricing: %v", c.cycle, err)
		}
		if got.Amount != c.want {
			t.Errorf("%s 加价 = %v, want %v（配置项未随周期放大）", c.cycle, got.Amount, c.want)
		}
	}

	// 默认值（80，档位内含量）在任何周期都不加价。
	for _, cycle := range []string{billingcycle.Monthly, billingcycle.Annually} {
		got, err := svc.ResolveOptionPricing(context.Background(), 7, cycle, map[string]string{"241017": "80"})
		if err != nil {
			t.Fatalf("%s ResolveOptionPricing: %v", cycle, err)
		}
		if got.Amount != 0 {
			t.Errorf("%s 按默认值应不加价, got %v", cycle, got.Amount)
		}
	}
}
