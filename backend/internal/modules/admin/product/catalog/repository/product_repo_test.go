package repository

import (
	"encoding/json"
	"testing"

	"hostsent/backend/internal/modules/admin/product/catalog/model"
	upstreamzjmf "hostsent/backend/internal/pkg/upstream/zjmf"
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
// zjmf.ConfigGroup 完全契约兼容（供 buildCartConfigOption 消费）。
// 列口径：upstream_key 列 = 上游自身 id（下单键），source_key 列 = 上游再上一层 id（链路参考）。
func TestBuildConfigGroupsMatchesUpstream(t *testing.T) {
	rows := []*model.ProductConfigOption{
		{
			ProductID: 7, UpstreamKey: 159899, SourceKey: "710549", Source: model.ConfigSourceUpstream,
			OptionName: "cpu|CPU", OptionType: 2, SortOrder: 0,
			Subs: []model.ProductConfigOptionSub{
				{UpstreamKey: 321187, SourceKey: "4387511", Source: model.ConfigSourceUpstream,
					OptionName: "16|16核", Hidden: 0, SortOrder: 0,
					PriceMonthly: 0, PriceAnnually: 99, PriceQuarterly: 0, PriceOnetime: 0},
			},
		},
		{
			ProductID: 7, UpstreamKey: 159900, SourceKey: "710550", Source: model.ConfigSourceUpstream,
			OptionName: "memory|内存", OptionType: 2, SortOrder: 1,
			Subs: []model.ProductConfigOptionSub{
				{UpstreamKey: 321188, SourceKey: "4387516", Source: model.ConfigSourceUpstream,
					OptionName: "16384|16G", Hidden: 1, SortOrder: 0,
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
	// 以 zjmf.ConfigGroup 契约反解，验证下游 buildCartConfigOption 可用。
	var groups []upstreamzjmf.ConfigGroup
	if err := json.Unmarshal(b, &groups); err != nil {
		t.Fatalf("重建结果不满足上游 ConfigGroup 契约: %v (%s)", err, b)
	}
	if len(groups) != 1 || len(groups[0].Options) != 2 {
		t.Fatalf("重建组/选项数量错误: %+v", groups)
	}
	cpu := groups[0].Options[0]
	if cpu.ID != 159899 || cpu.UpstreamID != 710549 || cpu.OptionName != "cpu|CPU" || len(cpu.Sub) != 1 {
		t.Errorf("cpu 重建错误: %+v", cpu)
	}
	if cpu.Sub[0].ID != 321187 || cpu.Sub[0].UpstreamID != 4387511 {
		t.Errorf("cpu 子项 id 口径错误: %+v", cpu.Sub[0])
	}
	mem := groups[0].Options[1]
	if mem.ID != 159900 || mem.Sub[0].Hidden != 1 || float64(mem.Sub[0].Pricings[0].Monthly) != 19.5 {
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
	var rebuilt []upstreamzjmf.ConfigGroup
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
	// id 必须是上游自身 id（cart configoption 的键）：上游 ConfigOptions::filterConfigOptions
	// 只认自身 id，用 upstream_id 会被整段过滤 → 按上游默认配置开通。
	if cpu.ID != 10 {
		t.Errorf("cpu.ID = %d, want 10（上游自身 id，下单键）", cpu.ID)
	}
	if cpu.Sub[0].ID != 100 {
		t.Errorf("cpu 子项 ID = %d, want 100（上游自身子项 id，下单取值）", cpu.Sub[0].ID)
	}
	if cpu.Sub[0].UpstreamID != 4387511 || float64(cpu.Sub[0].Pricings[0].Annually) != 99 {
		t.Errorf("cpu 子项/价格重建错误: %+v", cpu.Sub)
	}
	// 重建结果直接喂给下单参数构建（适配器 buildCartConfigOption 取 id / sub.id，
	// 见 zjmf::TestBuildCartConfigOptionFromGroups）：两侧 id 口径必须一致。
	mem := rebuilt[0].Options[1]
	if mem.UpstreamID != 710550 || len(mem.Sub) != 1 || mem.Sub[0].Hidden != 0 || float64(mem.Sub[0].Pricings[0].Monthly) != 19.5 {
		t.Errorf("memory 重建错误: %+v", mem)
	}
	if mem.ID != 11 || mem.Sub[0].ID != 200 {
		t.Errorf("memory 自身 id 丢失: option=%d sub=%d", mem.ID, mem.Sub[0].ID)
	}
}
