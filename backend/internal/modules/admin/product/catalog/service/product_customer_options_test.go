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
		ProviderType: "dcimcloud",
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

// ===== 上游对接项（source=upstream）的选配暴露 =====

// upstreamOptionRows 构造一个「上游固定配置 + 上游多值项 + 自营开通项」的集合，
// 形态与真实导入商品一致（选项/取值名带「键|说明」前缀，取值名带 ^ 版本明细）。
func upstreamOptionRows() []*catalogmodel.ProductConfigOption {
	return []*catalogmodel.ProductConfigOption{
		{
			ID: 10, Source: catalogmodel.ConfigSourceUpstream, UpstreamKey: 163749,
			OptionName: "cpu|CPU", OptionKey: "", Widget: catalogmodel.ConfigWidgetSelect,
			Subs: []catalogmodel.ProductConfigOptionSub{
				{OptionName: "4|4核", UpstreamKey: 565658},
			},
		},
		{
			ID: 11, Source: catalogmodel.ConfigSourceUpstream, UpstreamKey: 163751,
			OptionName: "os|操作系统", Widget: catalogmodel.ConfigWidgetSelect,
			Subs: []catalogmodel.ProductConfigOptionSub{
				{OptionName: "12|CentOS^CentOS-7.6.1810-x64", UpstreamKey: 565660},
				{OptionName: "62|CentOS^CentOS-7.9.2111-x64", UpstreamKey: 565661},
				{OptionName: "75|Debian^Debian-12.0_x64", UpstreamKey: 565664},
			},
		},
		{
			ID: 12, Source: catalogmodel.ConfigSourceUpstream, UpstreamKey: 163744,
			OptionName: "network_type|网络类型", Widget: catalogmodel.ConfigWidgetSelect,
			Subs: []catalogmodel.ProductConfigOptionSub{
				{OptionName: "vpc|VPC网络", UpstreamKey: 565652},
				{OptionName: "normal|经典网络", UpstreamKey: 565653},
			},
		},
		{
			ID: 13, Source: catalogmodel.ConfigSourceSelf, SourceKey: "hostname",
			OptionKey: "hostname", OptionName: "主机名", Widget: catalogmodel.ConfigWidgetText,
		},
	}
}

// TestCustomerOptionsIncludesUpstreamSelectable 上游多值项下发、单值固定项不下发。
func TestCustomerOptionsIncludesUpstreamSelectable(t *testing.T) {
	svc := &productService{repo: &fakeProductRepo{configOptions: upstreamOptionRows()}}

	groups, err := svc.CustomerOptions(context.Background(), 1)
	if err != nil {
		t.Fatalf("CustomerOptions: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("应返回 1 组，实际 %+v", groups)
	}
	// cpu 单值（固定配置）不下发；os / network_type / hostname 三项下发。
	if len(groups[0].Options) != 3 {
		t.Fatalf("应下发 3 项（os/network_type/hostname），实际 %d", len(groups[0].Options))
	}
	os := groups[0].Options[0]
	if os.OptionKey != "163751" {
		t.Errorf("上游项键应取上游选项 id，实际 %q", os.OptionKey)
	}
	if os.Name != "操作系统" {
		t.Errorf("上游项名应去掉「键|」前缀，实际 %q", os.Name)
	}
	if os.Values[0].Value != "565660" {
		t.Errorf("上游取值应下发上游取值 id，实际 %q", os.Values[0].Value)
	}
	if os.Values[0].Label != "CentOS-7.6.1810-x64" {
		t.Errorf("取值展示名应取 ^ 后的明细，实际 %q", os.Values[0].Label)
	}
	// 操作系统家族重复出现 → 分组下拉；网络类型各成一族 → 保持平铺。
	if os.Widget != optionWidgetGroupSelect || os.Values[0].Group != "CentOS" {
		t.Errorf("操作系统应为分组下拉，实际 %q group=%q", os.Widget, os.Values[0].Group)
	}
	net := groups[0].Options[1]
	if net.Widget != optionWidgetSelect {
		t.Errorf("网络类型各取值家族不重复，应保持平铺下拉，实际 %q", net.Widget)
	}
}

// TestResolveOptionPricingUpstreamSelectable 上游项按上游 id 归一，单值固定项忽略。
func TestResolveOptionPricingUpstreamSelectable(t *testing.T) {
	svc := &productService{repo: &fakeProductRepo{configOptions: upstreamOptionRows()}}

	res, err := svc.ResolveOptionPricing(context.Background(), 1, "monthly", map[string]string{
		"163751": "565664", // os → Debian
		"163749": "565658", // cpu 是固定配置：接受不了，忽略
	})
	if err != nil {
		t.Fatalf("ResolveOptionPricing: %v", err)
	}
	if res.Selections["163751"] != "565664" {
		t.Errorf("上游 os 选择未归一：%+v", res.Selections)
	}
	if _, ok := res.Selections["163749"]; ok {
		t.Errorf("上游单值固定项不应进入选配：%+v", res.Selections)
	}
}

// TestSanitizeSelectionsKeepsUpstreamOption 履约兜底：上游可选键放行、固定键丢弃。
func TestSanitizeSelectionsKeepsUpstreamOption(t *testing.T) {
	svc := &productService{repo: &fakeProductRepo{configOptions: upstreamOptionRows()}}

	out := svc.sanitizeSelections(context.Background(), 1, map[string]string{
		"163751":   "565664", // 可选：保留
		"163749":   "565658", // 固定项：丢弃
		"hostname": "hs-node1",
		"os":       "62", // 本地词汇不是本商品的键 → 丢弃
	})
	if out["163751"] != "565664" || out["hostname"] != "hs-node1" {
		t.Errorf("合法选择应保留：%+v", out)
	}
	if _, ok := out["163749"]; ok {
		t.Errorf("固定项应被丢弃：%+v", out)
	}
	if _, ok := out["os"]; ok {
		t.Errorf("非本商品选配键应被丢弃：%+v", out)
	}
}

// TestUpstreamConfigOptionOverride 选配映射成 configoption[上游选项id]=上游取值id。
func TestUpstreamConfigOptionOverride(t *testing.T) {
	// 形态与 repo.ConfigGroupsByProductID 一致：组 → options[]（id/sub[].id）。
	groups := []interface{}{map[string]interface{}{
		"options": []interface{}{
			map[string]interface{}{
				"id":  float64(163751),
				"sub": []interface{}{map[string]interface{}{"id": float64(565660)}, map[string]interface{}{"id": float64(565664)}},
			},
			map[string]interface{}{
				"id":  "163744",
				"sub": []interface{}{map[string]interface{}{"id": "565653"}},
			},
		},
	}}
	got := upstreamConfigOptionOverride(groups, map[string]string{"163751": "565664", "163744": "565653"})
	if got["163751"] != "565664" || got["163744"] != "565653" {
		t.Fatalf("映射不对：%+v", got)
	}
	// 取值不在该选项候选中（脏数据）不产出映射，避免把非法取值下发给上游。
	bad := upstreamConfigOptionOverride(groups, map[string]string{"163751": "999999"})
	if len(bad) != 0 {
		t.Errorf("非法取值不应产出映射：%+v", bad)
	}
}

// TestUpstreamConfigOptionOverrideQuantity 数量型配置项下发的是**数量本身**，不是子项 id。
//
// 这是"按量定价"能生效的关键：上游 ConfigOptions::filterConfigOptions 对
// judgeQuantity 为真的 option_type（4/7/9/11/14/15/16/17/18/19）直接把该数值
// 夹到 qty_minimum..qty_maximum 后作为取值用。若照枚举型按 sub id 匹配，
// 客户填的 "100"（GB）不会命中任何子项 —— 参数被丢弃，上游按默认配置开通（白扣款）。
func TestUpstreamConfigOptionOverrideQuantity(t *testing.T) {
	groups := []interface{}{map[string]interface{}{
		"options": []interface{}{
			// 数量型（系统盘）：区间 80–1024
			map[string]interface{}{
				"id":          float64(241017),
				"option_type": float64(19),
				"qty_minimum": float64(80),
				"qty_maximum": float64(1024),
				"sub":         []interface{}{map[string]interface{}{"id": float64(1036029)}},
			},
			// 枚举型（操作系统）：仍按下发取值 id
			map[string]interface{}{
				"id":  float64(241014),
				"sub": []interface{}{map[string]interface{}{"id": float64(565660)}},
			},
		},
	}}

	got := upstreamConfigOptionOverride(groups, map[string]string{
		"241017": "100",    // 要 100G 系统盘
		"241014": "565660", // 选某个操作系统
	})
	if got["241017"] != "100" {
		t.Errorf("数量型应下发数量本身，got %#v", got["241017"])
	}
	if got["241014"] != "565660" {
		t.Errorf("枚举型应下发取值 id，got %#v", got["241014"])
	}

	// 越界按上游口径夹取（上游就是"越界即夹"，报错会把整单卡死）。
	low := upstreamConfigOptionOverride(groups, map[string]string{"241017": "10"})
	if low["241017"] != "80" {
		t.Errorf("低于下限应夹到 min=80，got %#v", low["241017"])
	}
	high := upstreamConfigOptionOverride(groups, map[string]string{"241017": "99999"})
	if high["241017"] != "1024" {
		t.Errorf("高于上限应夹到 max=1024，got %#v", high["241017"])
	}

	// 非数字取值不产出映射（避免把脏值当数量下发）。
	if bad := upstreamConfigOptionOverride(groups, map[string]string{"241017": "abc"}); len(bad) != 0 {
		t.Errorf("非数字取值不应产出映射：%+v", bad)
	}
}

// TestCustomerWidgetQuantityByOptionType option_type 属于上游数量型集合时，
// 即使没有 min/max 也要判成 qty（历史行只有 option_type 可用）。
func TestCustomerWidgetQuantityByOptionType(t *testing.T) {
	for _, tp := range []int{4, 7, 9, 11, 14, 15, 16, 17, 18, 19} {
		row := &catalogmodel.ProductConfigOption{OptionType: tp}
		if got := customerWidget(row); got != optionWidgetQty {
			t.Errorf("option_type=%d 控件 = %q, want qty", tp, got)
		}
	}
	// 操作系统（5）等保持下拉。
	if got := customerWidget(&catalogmodel.ProductConfigOption{OptionType: 5}); got != optionWidgetSelect {
		t.Errorf("操作系统应保持 select, got %q", got)
	}
}

// TestCustomerSelectableUpstreamQuantity 数量型即使只有一个价档也要开放给客户。
//
// 例：数据盘只有一个 0~1024 的 0.8 元/GB 档，按"单值即固定配置"会被滤掉，
// 客户就完全没法指定盘的大小了。
func TestCustomerSelectableUpstreamQuantity(t *testing.T) {
	oneTier := &catalogmodel.ProductConfigOption{
		OptionType: 14,
		Subs: []catalogmodel.ProductConfigOptionSub{
			{OptionName: "数据盘", QtyMinimum: 0, QtyMaximum: 1024, PriceMonthly: 0.8},
		},
	}
	if !customerSelectableUpstream(oneTier) {
		t.Error("数量型单价档也必须可售（否则客户指定不了容量）")
	}
	// 非数量型的单值固定配置仍然锁定。
	fixed := &catalogmodel.ProductConfigOption{
		OptionType: 5,
		Subs:       []catalogmodel.ProductConfigOptionSub{{OptionName: "CentOS"}},
	}
	if customerSelectableUpstream(fixed) {
		t.Error("单值枚举型仍应视为固定配置")
	}
}
