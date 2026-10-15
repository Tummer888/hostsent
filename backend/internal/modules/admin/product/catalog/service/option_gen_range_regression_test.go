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

// D. 长周期加价必须随周期变（用户反馈"换周期加价不变"）。
//
// 实测故障：编辑页每个取值只填一个「元/月」，季度/年度恒为 0，
// priceForCycle 在 0 时回落月价 —— 月付 13 元的内存 8G，年付也只加 13 元。
// 口径：留空档位按「月价 × 周期月数」折算（季 ×3、年 ×12）；显式填的不动。
func TestDeriveMissingCyclePrices(t *testing.T) {
	mk := func(monthly, quarter, annual float64) []optionValue {
		return []optionValue{{Value: "8192", Monthly: monthly, Quarter: quarter, Annual: annual}}
	}

	t.Run("留空档位按月数折算", func(t *testing.T) {
		vs := mk(13, 0, 0)
		deriveMissingCyclePrices(vs)
		if vs[0].Quarter != 39 {
			t.Errorf("季付应 13×3=39，实际 %.2f", vs[0].Quarter)
		}
		if vs[0].Annual != 156 {
			t.Errorf("年付应 13×12=156，实际 %.2f", vs[0].Annual)
		}
	})

	t.Run("显式填写的档位不被覆盖", func(t *testing.T) {
		vs := mk(13, 30, 120) // 运营自己定了更优惠的长周期价
		deriveMissingCyclePrices(vs)
		if vs[0].Quarter != 30 || vs[0].Annual != 120 {
			t.Errorf("显式价应保留，实际 %.2f / %.2f", vs[0].Quarter, vs[0].Annual)
		}
	})

	t.Run("月价 0（不加价）不折算", func(t *testing.T) {
		vs := mk(0, 0, 0)
		deriveMissingCyclePrices(vs)
		if vs[0].Quarter != 0 || vs[0].Annual != 0 {
			t.Errorf("不加价的取值不该被折算，实际 %.2f / %.2f", vs[0].Quarter, vs[0].Annual)
		}
	})

	t.Run("一次性不折算（无月数口径）", func(t *testing.T) {
		vs := mk(13, 0, 0)
		deriveMissingCyclePrices(vs)
		if vs[0].Onetime != 0 {
			t.Errorf("一次性不该被折算，实际 %.2f", vs[0].Onetime)
		}
	})
}

// E. 裸值 label 不得覆盖目录中文名（用户要求区分"传值"与"展示名"）。
//
// 实测故障：建品页在平台目录加载前就取过档位，label 被固化成裸值（os 传 "62"），
// 提交后覆盖掉目录中文名，下单界面只显示 ID。
func TestOverrideLabelKeepsCatalogName(t *testing.T) {
	cases := []struct {
		name      string
		requested string
		value     string
		catalog   string
		want      string
	}{
		{"裸值等于取值 → 用目录名", "62", "62", "CentOS / CentOS-7.9.2111-x64", "CentOS / CentOS-7.9.2111-x64"},
		{"裸值等于取值且目录没有 → 用取值", "1", "1", "", "1"},
		{"请求给了真展示名 → 以请求为准", "6G", "6144", "6G", "6G"},
		{"请求没给 label → 用目录名", "", "6144", "6G", "6G"},
	}
	for _, c := range cases {
		if got := overrideLabel(c.requested, c.value, c.catalog); got != c.want {
			t.Errorf("%s：得到 %q，期望 %q", c.name, got, c.want)
		}
	}
}

// F. 数量型（带宽/数据盘/系统盘/IP 数）必须能定价。
//
// 实测故障（用户反馈）：后台建品时带宽、数据盘这些项**没法定价** ——
// 建品页原本没有单价输入，后端的占位 sub 又把四档价格写死成 0，
// 于是只能建完商品再去详情页补。数量型没有离散取值，价格挂在"每单位"上。
//
// 口径同枚举取值：没填的长周期档按「月单价 × 周期月数」折算；显式填的以填的为准。
func TestRangeOptionUnitPricing(t *testing.T) {
	t.Run("只填月单价 → 长周期按月数折算", func(t *testing.T) {
		acc := &optionAccumulator{meta: SpecOptionMeta{OptionKey: "bw", Widget: SpecWidgetQty}}
		acc.applyOverride(dto.ProductOptionOverride{
			OptionKey: "bw", MinValue: fptr(10), MaxValue: fptr(100),
			UnitPriceMonthly: fptr(1.5),
		})
		subs := acc.toConfigOption()["sub"].([]map[string]interface{})
		p := subs[0]["pricings"].([]map[string]interface{})[0]
		if p["monthly"] != 1.5 {
			t.Errorf("月单价应 1.5，实际 %v", p["monthly"])
		}
		if p["quarterly"] != 4.5 {
			t.Errorf("季单价应 1.5×3=4.5，实际 %v", p["quarterly"])
		}
		if p["annually"] != 18.0 {
			t.Errorf("年单价应 1.5×12=18，实际 %v", p["annually"])
		}
	})

	t.Run("显式填长周期单价 → 不被折算覆盖", func(t *testing.T) {
		acc := &optionAccumulator{meta: SpecOptionMeta{OptionKey: "system_disk_size", Widget: SpecWidgetQty}}
		acc.applyOverride(dto.ProductOptionOverride{
			OptionKey: "system_disk_size", MinValue: fptr(30), MaxValue: fptr(30),
			UnitPriceMonthly: fptr(2.0), UnitPriceAnnually: fptr(30.0),
		})
		subs := acc.toConfigOption()["sub"].([]map[string]interface{})
		p := subs[0]["pricings"].([]map[string]interface{})[0]
		if p["annually"] != 30.0 {
			t.Errorf("显式年单价应保留 30，实际 %v", p["annually"])
		}
		if p["monthly"] != 2.0 {
			t.Errorf("月单价应 2，实际 %v", p["monthly"])
		}
	})

	t.Run("没定价 → 保持 0（不加价）", func(t *testing.T) {
		acc := &optionAccumulator{meta: SpecOptionMeta{OptionKey: "ip_num", Widget: SpecWidgetQty}}
		acc.applyOverride(dto.ProductOptionOverride{
			OptionKey: "ip_num", MinValue: fptr(1), MaxValue: fptr(20),
		})
		subs := acc.toConfigOption()["sub"].([]map[string]interface{})
		p := subs[0]["pricings"].([]map[string]interface{})[0]
		if p["monthly"] != 0.0 || p["annually"] != 0.0 {
			t.Errorf("未定价应保持 0，实际 %v / %v", p["monthly"], p["annually"])
		}
	})
}
