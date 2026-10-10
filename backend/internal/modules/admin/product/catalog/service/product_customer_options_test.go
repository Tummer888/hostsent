package service

import (
	"context"
	"testing"

	catalogmodel "hostsent/backend/internal/modules/admin/product/catalog/model"
)

// floatPtr 取指针（MinValue/MaxValue 用）。
func floatPtr(v float64) *float64 { return &v }

// selfOptionRows 构造一个"CPU 单选 + 带宽数量型"的最小选配项集合。
func selfOptionRows() []*catalogmodel.ProductConfigOption {
	return []*catalogmodel.ProductConfigOption{
		{
			ID: 1, Source: catalogmodel.ConfigSourceSelf, SourceKey: "cpu", OptionKey: "cpu",
			OptionName: "CPU", Widget: catalogmodel.ConfigWidgetRadio, Required: true,
			Unit: "核", SortOrder: 1,
			Subs: []catalogmodel.ProductConfigOptionSub{
				{OptionName: "2核", SourceKey: "2", PriceMonthly: 0},
				{OptionName: "4核", SourceKey: "4", PriceMonthly: 20},
				{OptionName: "8核", SourceKey: "8", PriceMonthly: 60},
			},
		},
		{
			ID: 2, Source: catalogmodel.ConfigSourceSelf, SourceKey: "bw", OptionKey: "bw",
			OptionName: "带宽", Widget: catalogmodel.ConfigWidgetQty, Unit: "Mbps",
			DefaultValue: "5", MinValue: floatPtr(1), MaxValue: floatPtr(100), SortOrder: 2,
			Subs: []catalogmodel.ProductConfigOptionSub{
				// 数量型：子项单价 = 每单位加价（从最小值起算）。
				{OptionName: "带宽", PriceMonthly: 2},
			},
		},
	}
}

// TestResolveOptionPricingEnumAndQty 枚举型按选中值加价，数量型按超量部分加价。
func TestResolveOptionPricingEnumAndQty(t *testing.T) {
	repo := &fakeProductRepo{configOptions: selfOptionRows()}
	svc := &productService{repo: repo}

	res, err := svc.ResolveOptionPricing(context.Background(), 1, "monthly", map[string]string{
		"cpu": "8", "bw": "25",
	})
	if err != nil {
		t.Fatalf("ResolveOptionPricing: %v", err)
	}
	// CPU 8核 = 60；带宽 25 → 超量 20（25-默认 5）× 2 = 40。
	if res.Selections["cpu"] != "8" || res.Selections["bw"] != "25" {
		t.Errorf("选择未归一: %+v", res.Selections)
	}
	if res.Amount != 100 {
		t.Errorf("加价小计 = %v, want 100", res.Amount)
	}
	if len(res.Items) != 2 {
		t.Fatalf("应有 2 条明细，实际 %d", len(res.Items))
	}
}

// TestResolveOptionPricingRequiredDefault 必选项未选时用默认补齐（不加价）。
func TestResolveOptionPricingRequiredDefault(t *testing.T) {
	repo := &fakeProductRepo{configOptions: selfOptionRows()}
	svc := &productService{repo: repo}

	res, err := svc.ResolveOptionPricing(context.Background(), 1, "monthly", nil)
	if err != nil {
		t.Fatalf("ResolveOptionPricing: %v", err)
	}
	// 必选 CPU 取首个可见值 2核（价 0），带宽取默认 5（=最小值，无加价）。
	if res.Selections["cpu"] != "2" {
		t.Errorf("必选项应补默认首个取值，实际 %q", res.Selections["cpu"])
	}
	if res.Selections["bw"] != "5" {
		t.Errorf("数量型应补默认值 5，实际 %q", res.Selections["bw"])
	}
	if res.Amount != 0 {
		t.Errorf("默认取值不应产生加价，实际 %v", res.Amount)
	}
}

// TestResolveOptionPricingRejectsUnknown 选了不存在的取值直接报错（不静默降级）。
func TestResolveOptionPricingRejectsUnknown(t *testing.T) {
	repo := &fakeProductRepo{configOptions: selfOptionRows()}
	svc := &productService{repo: repo}

	if _, err := svc.ResolveOptionPricing(context.Background(), 1, "monthly", map[string]string{"cpu": "64"}); err == nil {
		t.Fatal("非法取值应报错")
	}
}

// TestResolveOptionPricingRejectsOutOfRange 数量型超区间报错。
func TestResolveOptionPricingRejectsOutOfRange(t *testing.T) {
	repo := &fakeProductRepo{configOptions: selfOptionRows()}
	svc := &productService{repo: repo}

	if _, err := svc.ResolveOptionPricing(context.Background(), 1, "monthly", map[string]string{"bw": "999"}); err == nil {
		t.Fatal("超上限应报错")
	}
	if _, err := svc.ResolveOptionPricing(context.Background(), 1, "monthly", map[string]string{"bw": "abc"}); err == nil {
		t.Fatal("非数字应报错")
	}
}

// TestResolveOptionPricingNoOptionsPassThrough 商品无选配项时原样回传（存量行为不变）。
func TestResolveOptionPricingNoOptionsPassThrough(t *testing.T) {
	repo := &fakeProductRepo{}
	svc := &productService{repo: repo}

	res, err := svc.ResolveOptionPricing(context.Background(), 1, "monthly", map[string]string{"x": "y"})
	if err != nil {
		t.Fatalf("ResolveOptionPricing: %v", err)
	}
	if res.Amount != 0 || res.Selections["x"] != "y" {
		t.Errorf("无选配项应原样透传，实际 %+v", res)
	}
}

// TestResolveOptionPricingCyclePrice 周期价优先：年付取值用 price_annually。
func TestResolveOptionPricingCyclePrice(t *testing.T) {
	rows := selfOptionRows()
	rows[0].Subs[2].PriceAnnually = 600
	repo := &fakeProductRepo{configOptions: rows}
	svc := &productService{repo: repo}

	res, err := svc.ResolveOptionPricing(context.Background(), 1, "annually", map[string]string{"cpu": "8"})
	if err != nil {
		t.Fatalf("ResolveOptionPricing: %v", err)
	}
	if res.Amount != 600 {
		t.Errorf("年付应取 price_annually=600，实际 %v", res.Amount)
	}
}

// TestCustomerOptionsShape 用户侧结构：控件/必选/分组/加价都要带出来。
func TestCustomerOptionsShape(t *testing.T) {
	repo := &fakeProductRepo{configOptions: selfOptionRows()}
	svc := &productService{repo: repo}

	groups, err := svc.CustomerOptions(context.Background(), 1)
	if err != nil {
		t.Fatalf("CustomerOptions: %v", err)
	}
	if len(groups) != 1 || len(groups[0].Options) != 2 {
		t.Fatalf("应返回 1 组 2 项，实际 %+v", groups)
	}
	cpu := groups[0].Options[0]
	if cpu.Widget != catalogmodel.ConfigWidgetRadio || !cpu.Required || len(cpu.Values) != 3 {
		t.Errorf("CPU 项结构不对：%+v", cpu)
	}
	if cpu.Values[2].Value != "8" || cpu.Values[2].PriceMonthly != 60 {
		t.Errorf("取值加价未带出：%+v", cpu.Values[2])
	}
	bw := groups[0].Options[1]
	if bw.Widget != catalogmodel.ConfigWidgetQty || bw.MinValue == nil || *bw.MinValue != 1 {
		t.Errorf("数量型结构不对：%+v", bw)
	}
}

// TestCustomerOptionsGroupSelect 有分组标签时升级为分组下拉（操作系统场景）。
func TestCustomerOptionsGroupSelect(t *testing.T) {
	rows := []*catalogmodel.ProductConfigOption{{
		Source: catalogmodel.ConfigSourceSelf, SourceKey: "os", OptionKey: "os",
		OptionName: "操作系统", Widget: catalogmodel.ConfigWidgetSelect, Required: true,
		Subs: []catalogmodel.ProductConfigOptionSub{
			{OptionName: "Ubuntu 22.04", SourceKey: "12", GroupLabel: "Ubuntu"},
			{OptionName: "Windows 2022", SourceKey: "30", GroupLabel: "Windows"},
		},
	}}
	svc := &productService{repo: &fakeProductRepo{configOptions: rows}}

	groups, err := svc.CustomerOptions(context.Background(), 1)
	if err != nil {
		t.Fatalf("CustomerOptions: %v", err)
	}
	opt := groups[0].Options[0]
	if opt.Widget != "group_select" {
		t.Errorf("有分组应升级为 group_select，实际 %q", opt.Widget)
	}
	if opt.Values[0].Group != "Ubuntu" {
		t.Errorf("取值分组标签丢失：%+v", opt.Values[0])
	}
}

// TestCustomerOptionsSkipsHiddenAndEmpty 隐藏取值与无取值的枚举项都不下发。
func TestCustomerOptionsSkipsHiddenAndEmpty(t *testing.T) {
	rows := []*catalogmodel.ProductConfigOption{
		{
			Source: catalogmodel.ConfigSourceSelf, SourceKey: "os", OptionName: "操作系统",
			Widget: catalogmodel.ConfigWidgetSelect,
			Subs: []catalogmodel.ProductConfigOptionSub{
				{OptionName: "已停用镜像", SourceKey: "99", Hidden: 1},
			},
		},
		{
			Source: catalogmodel.ConfigSourceSelf, SourceKey: "ricks", OptionName: "空项",
			Widget: catalogmodel.ConfigWidgetSelect,
		},
	}
	svc := &productService{repo: &fakeProductRepo{configOptions: rows}}

	groups, err := svc.CustomerOptions(context.Background(), 1)
	if err != nil {
		t.Fatalf("CustomerOptions: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("无可见取值的枚举项不应下发，实际 %+v", groups)
	}
}

// TestSanitizeSelectionsDropsUnknown 履约侧兜底：只转发该商品确实开放的参数与取值。
func TestSanitizeSelectionsDropsUnknown(t *testing.T) {
	repo := &fakeProductRepo{configOptions: selfOptionRows()}
	svc := &productService{repo: repo}

	out := svc.sanitizeSelections(context.Background(), 1, map[string]string{
		"cpu":     "8",  // 合法
		"bw":      "20", // 数量型：键合法即放行
		"os":      "62", // 该商品没开放 os → 丢弃
		"unknown": "x",  // 未开放 → 丢弃
		"cpu2":    "4",  // 未开放 → 丢弃
	})
	if out["cpu"] != "8" || out["bw"] != "20" {
		t.Errorf("合法选择应保留：%+v", out)
	}
	if _, ok := out["os"]; ok {
		t.Errorf("未开放的 os 应被丢弃：%+v", out)
	}
	if _, ok := out["unknown"]; ok {
		t.Errorf("未开放的参数应被丢弃：%+v", out)
	}
	if len(out) != 2 {
		t.Errorf("应只剩 2 项，实际 %+v", out)
	}
}

// TestSanitizeSelectionsDropsInvalidValue 取值不在候选内时丢弃（运营事后删过取值的历史订单）。
func TestSanitizeSelectionsDropsInvalidValue(t *testing.T) {
	repo := &fakeProductRepo{configOptions: selfOptionRows()}
	svc := &productService{repo: repo}

	out := svc.sanitizeSelections(context.Background(), 1, map[string]string{"cpu": "64"})
	if _, ok := out["cpu"]; ok {
		t.Errorf("非法取值应被丢弃：%+v", out)
	}
}

// TestOptionAccumulatorBuildsCustomerOptions 建品生成的 option 能被用户侧读取（端到端结构闭环）。
func TestOptionAccumulatorBuildsCustomerOptions(t *testing.T) {
	meta := SpecOptionMeta{
		OptionKey: "os", Label: "操作系统", Widget: SpecWidgetSelect, Required: true,
		ProviderType: "mofangyun",
		Values: []SpecOptionValueMeta{
			{Value: "12", Label: "Ubuntu 22.04", GroupLabel: "Ubuntu"},
		},
	}
	acc := &optionAccumulator{meta: meta}
	acc.addValues(optionSelection{Values: []string{"12"}}, meta)
	opt := acc.toConfigOption()

	// 模拟仓储回读：option → model.ProductConfigOption + sub。
	row := &catalogmodel.ProductConfigOption{
		Source:     catalogmodel.ConfigSourceSelf,
		SourceKey:  opt["source_key"].(string),
		OptionKey:  opt["option_key"].(string),
		OptionName: opt["option_name"].(string),
		Widget:     opt["widget"].(string),
		Required:   opt["required"].(bool),
		Subs:       []catalogmodel.ProductConfigOptionSub{},
	}
	for _, raw := range opt["sub"].([]map[string]interface{}) {
		row.Subs = append(row.Subs, catalogmodel.ProductConfigOptionSub{
			OptionName: raw["option_name"].(string),
			SourceKey:  raw["source_key"].(string),
			GroupLabel: raw["group_label"].(string),
			Hidden:     0,
		})
	}
	svc := &productService{repo: &fakeProductRepo{configOptions: []*catalogmodel.ProductConfigOption{row}}}
	groups, err := svc.CustomerOptions(context.Background(), 1)
	if err != nil {
		t.Fatalf("CustomerOptions: %v", err)
	}
	if len(groups) != 1 || groups[0].Options[0].OptionKey != "os" {
		t.Fatalf("生成→读取闭环失败：%+v", groups)
	}
	if groups[0].Options[0].Widget != optionWidgetGroupSelect {
		t.Errorf("有分组应渲染为分组下拉，实际 %q", groups[0].Options[0].Widget)
	}
}
