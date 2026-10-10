package service

import (
	"testing"

	"hostsent/backend/internal/modules/admin/product/catalog/dto"
)

// TestOptionAccumulatorAddValuesDedup 跨档位累加：同一取值只出现一次，标签取目录名。
func TestOptionAccumulatorAddValuesDedup(t *testing.T) {
	meta := SpecOptionMeta{
		OptionKey: "os", Label: "操作系统",
		Values: []SpecOptionValueMeta{
			{Value: "12", Label: "Ubuntu 22.04", GroupLabel: "Ubuntu"},
			{Value: "13", Label: "Ubuntu 24.04", GroupLabel: "Ubuntu"},
		},
	}
	acc := &optionAccumulator{meta: meta}
	acc.addValues(optionSelection{Values: []string{"12", "13"}}, meta)
	acc.addValues(optionSelection{Values: []string{"12"}}, meta) // 重复勾选

	if len(acc.values) != 2 {
		t.Fatalf("取值应去重为 2 个，实际 %d：%+v", len(acc.values), acc.values)
	}
	if acc.values[0].Label != "Ubuntu 22.04" || acc.values[0].Group != "Ubuntu" {
		t.Errorf("标签/分组未从目录取值库回填：%+v", acc.values[0])
	}
	// 有分组标签 → 渲染成分组下拉。
	if acc.meta.Widget != SpecWidgetGroupSelect {
		t.Errorf("有分组应升级为 group_select，实际 %q", acc.meta.Widget)
	}
}

// TestOptionAccumulatorRangeIsQty 区间型参数（带宽）渲染成步进器并取并集区间。
func TestOptionAccumulatorRangeIsQty(t *testing.T) {
	meta := SpecOptionMeta{OptionKey: "bw", Label: "带宽", Unit: "Mbps"}
	acc := &optionAccumulator{meta: meta}
	acc.addValues(optionSelection{Range: []float64{1, 50}, Default: "10"}, meta)
	acc.addValues(optionSelection{Range: []float64{5, 100}}, meta)

	if acc.meta.Widget != SpecWidgetQty {
		t.Fatalf("区间型应为 qty，实际 %q", acc.meta.Widget)
	}
	if acc.meta.MinValue == nil || *acc.meta.MinValue != 1 {
		t.Errorf("区间下界应取最宽 1，实际 %v", acc.meta.MinValue)
	}
	if acc.meta.MaxValue == nil || *acc.meta.MaxValue != 100 {
		t.Errorf("区间上界应取最宽 100，实际 %v", acc.meta.MaxValue)
	}
	if acc.meta.Default != "10" {
		t.Errorf("默认值应保留 10，实际 %q", acc.meta.Default)
	}
}

// TestOptionAccumulatorOverrideReplaces 建品页就地覆盖：取值整体替换并带上加价。
func TestOptionAccumulatorOverrideReplaces(t *testing.T) {
	meta := SpecOptionMeta{OptionKey: "ip_num", Label: "IP 数量", Widget: SpecWidgetQty}
	acc := &optionAccumulator{meta: meta}
	acc.addValues(optionSelection{Values: []string{"1", "2"}}, meta)
	acc.applyOverride(dto.ProductOptionOverride{
		OptionKey: "ip_num", Label: "IP 个数", Widget: SpecWidgetRadio, Required: true,
		Values: []dto.ProductOptionValue{
			{Value: "1", Label: "1 个"},
			{Value: "2", Label: "2 个 / 加价", PriceMonthly: 10},
		},
	})

	if acc.meta.Label != "IP 个数" || !acc.meta.Required || acc.meta.Widget != SpecWidgetRadio {
		t.Errorf("覆盖后元数据不对：%+v", acc.meta)
	}
	if len(acc.values) != 2 {
		t.Fatalf("覆盖后应有 2 个取值，实际 %d", len(acc.values))
	}
	if acc.values[1].Monthly != 10 {
		t.Errorf("取值加价未带上：%+v", acc.values[1])
	}
}

// TestOptionAccumulatorToConfigOptionShape 生成的 option 结构要能被产品仓库解析回库。
func TestOptionAccumulatorToConfigOptionShape(t *testing.T) {
	meta := SpecOptionMeta{
		OptionKey: "cpu", Label: "CPU", Widget: SpecWidgetRadio, Required: true,
		Unit: "核", ProviderType: "mofangyun", SortOrder: 10,
	}
	acc := &optionAccumulator{meta: meta}
	acc.addValues(optionSelection{Values: []string{"2", "4"}}, meta)
	opt := acc.toConfigOption()

	if opt["source"] != "self" || opt["source_key"] != "cpu" {
		t.Errorf("source/source_key 不对：%v", opt)
	}
	if opt["option_type"] != 2 { // 单选
		t.Errorf("radio 应对应 option_type=2，实际 %v", opt["option_type"])
	}
	if opt["widget"] != SpecWidgetRadio || opt["required"] != true {
		t.Errorf("widget/required 不对：%v", opt)
	}
	subs, ok := opt["sub"].([]map[string]interface{})
	if !ok || len(subs) != 2 {
		t.Fatalf("sub 结构不对：%v", opt["sub"])
	}
	if subs[0]["source_key"] != "2" {
		t.Errorf("sub 的 source_key 应是平台取值 2，实际 %v", subs[0]["source_key"])
	}
	pricings, ok := subs[0]["pricings"].([]map[string]interface{})
	if !ok || len(pricings) != 1 {
		t.Fatalf("pricings 结构不对：%v", subs[0]["pricings"])
	}
}

// TestParseOptionSelectionsInvalid 非法 JSON 返回 nil，不 panic。
func TestParseOptionSelectionsInvalid(t *testing.T) {
	if got := parseOptionSelections("{not json"); got != nil {
		t.Errorf("非法 JSON 应返回 nil，实际 %+v", got)
	}
	if got := parseOptionSelections(""); got != nil {
		t.Errorf("空串应返回 nil，实际 %+v", got)
	}
	got := parseOptionSelections(`{"cpu":{"values":["2","4"]}}`)
	if len(got) != 1 || len(got["cpu"].Values) != 2 {
		t.Errorf("正常解析失败：%+v", got)
	}
}

// TestSortedOptionKeysStable 生成顺序必须稳定，否则配置项排序每次都不一样。
func TestSortedOptionKeysStable(t *testing.T) {
	keys := sortedOptionKeys(map[string]optionSelection{"os": {}, "cpu": {}, "bw": {}})
	want := []string{"bw", "cpu", "os"}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("排序不稳定：%v", keys)
		}
	}
}

// TestBuildGroupsFromAccumulatorsEmpty 没有取值时不生成空的配置组。
func TestBuildGroupsFromAccumulatorsEmpty(t *testing.T) {
	groups := buildGroupsFromAccumulators([]string{"cpu"}, map[string]*optionAccumulator{
		"cpu": {meta: SpecOptionMeta{OptionKey: "cpu"}}, // 无取值
	})
	if groups != nil {
		t.Errorf("无取值应不生成配置组，实际 %v", groups)
	}
}
