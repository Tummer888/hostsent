package service

import (
	"testing"

	"hostsent/backend/internal/modules/admin/product/catalog/dto"
)

// 客户选配项生成的两个实测回归（自营商品缺失带宽/数据盘等配置项）。
//
// 故障现象：模板 17 里配了 bw / in_bw / ip_num / data_disk_size / system_disk_size
// 五个数量型（区间）配置项，但生成的商品只有 cpu / memory 等枚举项 ——
// 范围项全部丢失，下单表单里也就没有这些可填项，且 option_name 为空。
//
// 两个独立根因，各自都有用例锁住：
//   A. option_selections 解析：default 写成 JSON 数字时整份解析失败（见 TestParseOptionSelectionsAcceptsNumericDefault）；
//   B. 就地覆盖：范围项只带 min/max、不带 values，未标 isRange 被整项丢弃（见 TestOverrideRangeOptionKept）。

// A. 默认值的两种写法都必须能解析。
//
// 为什么会有两种写法：这一列是 jsonb，编辑器写字符串 "10"，
// 而 SQL 侧的修正（migrations/111 用 to_jsonb(numeric)）写出来是数字 10。
// 早先 Default 声明为 string，数字形态会让**整份 selections 反序列化失败**返回 nil，
// 于是商品的客户选配项全部消失 —— 不是少一个字段，是整份数据被丢掉。
func TestParseOptionSelectionsAcceptsNumericDefault(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want int // 期望解析出的参数个数
	}{
		{"数字型 default（SQL 修正后的存量数据）", `{"bw":{"range":[10,100],"default":10}}`, 1},
		{"字符串型 default（编辑器写入）", `{"bw":{"range":[10,100],"default":"10"}}`, 1},
		{"混合形态（真实模板 17 片段）", `{"bw":{"range":[10,100],"default":10},"cpu":{"values":["4"],"default":"4"}}`, 2},
		{"无 default", `{"ip_num":{"range":[1,20]}}`, 1},
		{"空/null", ``, 0},
	}
	for _, c := range cases {
		got := parseOptionSelections(c.raw)
		if len(got) != c.want {
			t.Errorf("%s：解析出 %d 项，期望 %d 项（解析失败会让商品丢掉全部配置项）",
				c.name, len(got), c.want)
		}
	}

	// 数字形态要能被读成与字符串形态等价的值。
	num := parseOptionSelections(`{"bw":{"range":[10,100],"default":10}}`)
	str := parseOptionSelections(`{"bw":{"range":[10,100],"default":"10"}}`)
	if num["bw"].Default != str["bw"].Default {
		t.Errorf("数字与字符串形态应等价：num=%q str=%q", num["bw"].Default, str["bw"].Default)
	}
}

// B. 只带 min/max 的范围项（建品页 option_overrides 的形态）不能被丢弃。
//
// buildGroupsFromAccumulators 的保留条件是「有取值 或 isRange」；
// 范围项天然没有 values，所以 isRange 必须在 applyOverride 里标上。
func TestOverrideRangeOptionKept(t *testing.T) {
	overrides := []dto.ProductOptionOverride{
		{OptionKey: "bw", MinValue: fptr(10), MaxValue: fptr(100), Default: "10"},
		{OptionKey: "system_disk_size", MinValue: fptr(30), MaxValue: fptr(30), Default: "30"},
		// 对照组：枚举项照旧保留。
		{OptionKey: "cpu", Default: "4", Values: []dto.ProductOptionValue{{Value: "4", Default: true}}},
	}
	merged := map[string]*optionAccumulator{}
	order := []string{}
	for _, ov := range overrides {
		acc := merged[ov.OptionKey]
		if acc == nil {
			acc = &optionAccumulator{meta: SpecOptionMeta{OptionKey: ov.OptionKey}}
			merged[ov.OptionKey] = acc
			order = append(order, ov.OptionKey)
		}
		acc.applyOverride(ov)
	}

	groups := buildGroupsFromAccumulators(order, merged)
	if len(groups) == 0 {
		t.Fatal("生成结果为空：范围项被整项丢弃")
	}
	opts := groups[0].(map[string]interface{})["options"].([]map[string]interface{})
	if len(opts) != 3 {
		t.Fatalf("期望 3 个配置项（2 个范围 + 1 个枚举），实际 %d", len(opts))
	}
	byKey := map[string]map[string]interface{}{}
	for _, o := range opts {
		byKey[o["option_key"].(string)] = o
	}
	for _, key := range []string{"bw", "system_disk_size"} {
		o, ok := byKey[key]
		if !ok {
			t.Errorf("范围项 %s 缺失", key)
			continue
		}
		if o["widget"] != SpecWidgetQty {
			t.Errorf("%s 的控件应为 qty，实际 %v", key, o["widget"])
		}
		if o["min_value"] == nil || o["max_value"] == nil {
			t.Errorf("%s 的区间未落库：min=%v max=%v", key, o["min_value"], o["max_value"])
		}
	}
}

func fptr(v float64) *float64 { return &v }

// C. 枚举型默认值必须落在**已开放的取值**内。
//
// 实测故障：目录里 snap_num 的平台默认是 2，而档位只开放了 -1。
// 不加校正会生成 default_value=2 + 取值只有 -1 的配置项 ——
// 客户下单时预选不出任何一项，编辑器还展示「平台默认 2」这种永远选不到的值。
// 口径：档位声明优先 → 目录默认（仅当在取值内）→ 首个取值。
func TestEnumDefaultStaysWithinOpenValues(t *testing.T) {
	meta := SpecOptionMeta{
		OptionKey: "snap_num", Label: "快照数量", Widget: SpecWidgetRadio,
		Default: "2", // 目录的平台默认，不在开放取值里
		Values:  []SpecOptionValueMeta{{Value: "-1", Label: "不开通"}, {Value: "0", Label: "不限量"}},
	}

	t.Run("目录默认不在取值内 → 回落首个取值", func(t *testing.T) {
		acc := &optionAccumulator{meta: meta}
		acc.addValues(optionSelection{Values: []string{"-1"}}, meta)
		if acc.meta.Default != "-1" {
			t.Errorf("默认值应为 -1，实际 %q", acc.meta.Default)
		}
	})

	t.Run("档位声明的默认优先", func(t *testing.T) {
		acc := &optionAccumulator{meta: meta}
		acc.addValues(optionSelection{Values: []string{"-1", "0"}, Default: "0"}, meta)
		if acc.meta.Default != "0" {
			t.Errorf("应取档位声明的 0，实际 %q", acc.meta.Default)
		}
	})

	t.Run("目录默认在取值内则保留", func(t *testing.T) {
		meta2 := meta
		meta2.Default = "0"
		acc := &optionAccumulator{meta: meta2}
		acc.addValues(optionSelection{Values: []string{"-1", "0"}}, meta2)
		if acc.meta.Default != "0" {
			t.Errorf("目录默认 0 合法，应保留，实际 %q", acc.meta.Default)
		}
	})
}
