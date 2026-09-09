package repository

import (
	"encoding/json"
	"testing"

	"hostsent/backend/internal/modules/admin/product/catalog/model"
	upstreamfinance "hostsent/backend/internal/pkg/upstream/mofangfinance"
)

// configGroupsJSON 模拟上游 resource_products.raw_specs->'config_groups' 的真实形态
// （extractConfigGroups 的产物），含字符串价格（FlexFloat 兼容）。
const configGroupsJSON = `[
  {
    "id": 1, "name": "基础配置", "description": "基础",
    "options": [
      {
        "id": 10, "option_name": "cpu", "option_type": 2, "upstream_id": 710549,
        "hidden": 0, "sort_order": 0,
        "sub": [
          {"id": 100, "option_name": "4核", "upstream_id": 4387511, "hidden": 0, "sort_order": 0,
           "pricings": [{"monthly": "0.00", "annually": "99.00", "quarterly": "0.00", "onetime": "0.00"}]}
        ]
      },
      {
        "id": 11, "option_name": "memory", "option_type": 2, "upstream_id": 710550,
        "hidden": 0, "sort_order": 1,
        "sub": [
          {"id": 200, "option_name": "8G", "upstream_id": 4387512, "hidden": 0, "sort_order": 0,
           "pricings": [{"monthly": 19.5, "annually": 195, "quarterly": 0, "onetime": 0}]}
        ]
      }
    ]
  }
]`

// TestParseConfigGroups 验证 config_groups 解析保留上游 id 与字符串/数字价格。
func TestParseConfigGroups(t *testing.T) {
	var groups []interface{}
	if err := json.Unmarshal([]byte(configGroupsJSON), &groups); err != nil {
		t.Fatalf("解析测试 JSON 失败: %v", err)
	}
	parsed, err := parseConfigGroups(groups)
	if err != nil {
		t.Fatalf("parseConfigGroups 失败: %v", err)
	}
	if len(parsed) != 1 || len(parsed[0].Options) != 2 {
		t.Fatalf("期望 1 组 2 配置项，得到 %d 组 %d 配置项", len(parsed), len(parsed[0].Options))
	}
	cpu := parsed[0].Options[0]
	if cpu.UpstreamID != 710549 || cpu.OptionName != "cpu" || cpu.OptionType != 2 {
		t.Errorf("cpu 选项解析错误: %+v", cpu)
	}
	if len(cpu.Sub) != 1 || cpu.Sub[0].UpstreamID != 4387511 || cpu.Sub[0].Pricings[0].Annually != flexFloat(99) {
		t.Errorf("cpu 子项/价格解析错误: %+v", cpu.Sub)
	}
	mem := parsed[0].Options[1]
	if len(mem.Sub) != 1 || mem.Sub[0].Pricings[0].Monthly != flexFloat(19.5) {
		t.Errorf("memory 数字价格解析错误: %+v", mem.Sub[0].Pricings)
	}
}

// TestBuildConfigGroupsMatchesUpstream 验证子表行重建的 config_groups 结构与上游
// mofangfinance.ConfigGroup 完全契约兼容（供 buildCartConfigOption 消费）。
func TestBuildConfigGroupsMatchesUpstream(t *testing.T) {
	rows := []*model.ProductConfigOption{
		{
			ProductID: 7, UpstreamKey: 710549, OptionName: "cpu", OptionType: 2, SortOrder: 0,
			Subs: []model.ProductConfigOptionSub{
				{UpstreamKey: 4387511, OptionName: "4核", Hidden: 0, SortOrder: 0,
					PriceMonthly: 0, PriceAnnually: 99, PriceQuarterly: 0, PriceOnetime: 0},
			},
		},
		{
			ProductID: 7, UpstreamKey: 710550, OptionName: "memory", OptionType: 2, SortOrder: 1,
			Subs: []model.ProductConfigOptionSub{
				{UpstreamKey: 4387512, OptionName: "8G", Hidden: 1, SortOrder: 0,
					PriceMonthly: 19.5, PriceAnnually: 195, PriceQuarterly: 0, PriceOnetime: 0},
			},
		},
	}
	out := buildConfigGroups(rows)
	if len(out) != 1 {
		t.Fatalf("期望 1 组，得到 %d", len(out))
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	// 以 mofangfinance.ConfigGroup 契约反解，验证下游 buildCartConfigOption 可用。
	var groups []upstreamfinance.ConfigGroup
	if err := json.Unmarshal(b, &groups); err != nil {
		t.Fatalf("重建结果不满足上游 ConfigGroup 契约: %v (%s)", err, b)
	}
	if len(groups) != 1 || len(groups[0].Options) != 2 {
		t.Fatalf("重建组/选项数量错误: %+v", groups)
	}
	cpu := groups[0].Options[0]
	if cpu.UpstreamID != 710549 || cpu.OptionName != "cpu" || len(cpu.Sub) != 1 || cpu.Sub[0].UpstreamID != 4387511 {
		t.Errorf("cpu 重建错误: %+v", cpu)
	}
	mem := groups[0].Options[1]
	if mem.Sub[0].Hidden != 1 || float64(mem.Sub[0].Pricings[0].Monthly) != 19.5 {
		t.Errorf("memory 重建错误: %+v", mem)
	}
}

// TestBuildConfigGroupsEmpty 验证空子表返回空（触发上游惰性回退）。
func TestBuildConfigGroupsEmpty(t *testing.T) {
	out := buildConfigGroups(nil)
	if len(out) != 0 {
		t.Fatalf("空输入应返回空，得到 %v", out)
	}
}

// TestConfigGroupsRoundTrip 验证「解析 → 子表行 → 重建」闭环：上游 upstream_id/option_name、
// 子项 upstream_id/hidden 与价格都应保留，供 buildCartConfigOption 消费。
func TestConfigGroupsRoundTrip(t *testing.T) {
	var groups []interface{}
	if err := json.Unmarshal([]byte(configGroupsJSON), &groups); err != nil {
		t.Fatalf("解析测试 JSON 失败: %v", err)
	}
	parsed, err := parseConfigGroups(groups)
	if err != nil {
		t.Fatalf("parseConfigGroups 失败: %v", err)
	}
	rows := buildConfigOptionRows(parsed, 7)

	// 模拟 SaveConfigOptions 落库后回填的子项 OptionID。
	for _, row := range rows {
		for i := range row.Subs {
			row.Subs[i].OptionID = row.ID
		}
	}

	out := buildConfigGroups(rows)
	if len(out) != 1 {
		t.Fatalf("期望 1 组，得到 %d", len(out))
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	var rebuilt []upstreamfinance.ConfigGroup
	if err := json.Unmarshal(b, &rebuilt); err != nil {
		t.Fatalf("重建不满足上游契约: %v (%s)", err, b)
	}
	if len(rebuilt) != 1 || len(rebuilt[0].Options) != 2 {
		t.Fatalf("重建组/选项数量错误: %+v", rebuilt)
	}
	cpu := rebuilt[0].Options[0]
	if cpu.UpstreamID != 710549 || cpu.OptionName != "cpu" || cpu.OptionType != 2 || len(cpu.Sub) != 1 {
		t.Errorf("cpu 重建错误: %+v", cpu)
	}
	if cpu.Sub[0].UpstreamID != 4387511 || float64(cpu.Sub[0].Pricings[0].Annually) != 99 {
		t.Errorf("cpu 子项/价格重建错误: %+v", cpu.Sub)
	}
	mem := rebuilt[0].Options[1]
	if mem.UpstreamID != 710550 || len(mem.Sub) != 1 || mem.Sub[0].Hidden != 0 || float64(mem.Sub[0].Pricings[0].Monthly) != 19.5 {
		t.Errorf("memory 重建错误: %+v", mem)
	}
}
