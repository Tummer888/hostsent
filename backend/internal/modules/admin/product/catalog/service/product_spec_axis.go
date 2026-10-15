package service

// SPU / SKU 正交轴（T4.7）。
//
// 背景：一个自营商品是 SPU（如「魔方云 4核6G」），它下面可以有多个 SKU（可卖形态）。
// 运营在配置档里为每个平台参数勾选取值时，有些参数是**规格维度的离散差异**
// （CPU 4核/8核、内存 4G/8G/16G），客户选它等于换一台机器 —— 这类参数应当
// 正交展开成多个 SKU，各 SKU 独立定价与库存。
//
// 而另一些参数是**同一台机器上的可调项**（带宽滑杆、数据盘大小、IP 数、快照数），
// 客户选它只是加配 —— 这类参数不该参与正交，否则带宽 10~100 每兆一个 SKU，
// 组合数指数爆炸。
//
// 区分方式就是在配置档的勾选结果上显式标注：
//
//	{"memory": {"values": ["4096","8192"], "sku_axis": true},
//	 "cpu":    {"values": ["4","8"],        "sku_axis": true},
//	 "bw":     {"range": [10,100],          "default": "10"}}
//
// 规则（服务端强校验，不依赖前端自觉）：
//   - 标了 sku_axis 的参数必须是离散取值（有 values、没有 range）；
//     区间型参数开放给客户后是连续值，无法枚举成 SKU。
//   - 取值至少 1 个才会成为轴；只有 1 个取值的轴不产生正交（等价于固定配置）。
//   - 全部轴的组合数有上限，超限直接拒绝并提示，避免一次建出上百个 SKU。

import (
	"fmt"
	"sort"
	"strings"
)

const (
	// skuAxisMaxCombinations 单次按轴展开的 SKU 数上限。
	//
	// 为什么要有上限：CPU 8 档 × 内存 11 档 × 系统盘 5 档 = 440 个 SKU，
	// 一次建品写出几百行 SKU + 几百行矩阵行 + 几百行绑定，既撑爆页面也没人维护。
	// 真需要这么多形态的商品应当拆成多个商品，或在建品页分批生成。
	skuAxisMaxCombinations = 64
	// skuAxisMaxPerOption 单轴取值数上限（超过说明这个参数不该做轴）。
	skuAxisMaxPerOption = 32
)

// axisOption 一个参与正交的轴：参数名 + 有序离散取值。
type axisOption struct {
	Key    string
	Values []string
}

// axisPlan 一次轴展开的结果。
type axisPlan struct {
	Axes []axisOption
	// Combinations 笛卡尔积组合（每个组合是 参数名 → 取值），顺序稳定。
	Combinations []map[string]string
}

// Empty 该配置档没有可正交的轴。
func (p *axisPlan) Empty() bool { return p == nil || len(p.Axes) == 0 }

// buildAxisPlan 从配置档勾选结果里挑出轴并展开笛卡尔积。
//
// 返回的错误都是运营可读的：直接把原因写清楚（哪个参数、为什么不能做轴、超了什么限），
// 因为这条路径的输入就是运营在界面上勾的框。
func buildAxisPlan(sel map[string]optionSelection) (*axisPlan, error) {
	plan := &axisPlan{}
	for _, key := range sortedOptionKeys(sel) {
		item := sel[key]
		if !item.SKUAxis {
			continue
		}
		// 轴必须是离散的：区间型参数在客户侧是连续取值，无法预先枚举成 SKU。
		if len(item.Range) == 2 {
			return nil, fmt.Errorf(
				"配置项「%s」被标记为参与 SKU 正交，但它配的是取值范围（动态区间）。"+
					"参与正交的参数必须是离散取值，请改为勾选具体值，或取消该项的「参与 SKU 正交」",
				key)
		}
		values := dedupeNonEmpty(item.Values)
		if len(values) == 0 {
			return nil, fmt.Errorf(
				"配置项「%s」被标记为参与 SKU 正交，但没有勾选任何取值；"+
					"请至少勾一个取值，或取消该项的「参与 SKU 正交」", key)
		}
		if len(values) > skuAxisMaxPerOption {
			return nil, fmt.Errorf(
				"配置项「%s」勾了 %d 个取值，超出单个正交参数的上限 %d 个；"+
					"请减少取值，或取消该项的「参与 SKU 正交」",
				key, len(values), skuAxisMaxPerOption)
		}
		plan.Axes = append(plan.Axes, axisOption{Key: key, Values: values})
	}
	if len(plan.Axes) == 0 {
		return plan, nil
	}
	total := 1
	for _, axis := range plan.Axes {
		total *= len(axis.Values)
		if total > skuAxisMaxCombinations {
			return nil, fmt.Errorf(
				"参与 SKU 正交的参数组合出 %d 个规格，超出上限 %d 个；"+
					"请减少参数或取值（带宽/数据盘这类连续量不要勾选「参与 SKU 正交」）",
				total, skuAxisMaxCombinations)
		}
	}
	plan.Combinations = cartesian(plan.Axes)
	return plan, nil
}

// cartesian 展开笛卡尔积；第一个轴变化最慢（外层循环），保证顺序稳定可复现。
// 稳定顺序很重要：SKU 的排序、编码、生成顺序都引用它。
func cartesian(axes []axisOption) []map[string]string {
	if len(axes) == 0 {
		return nil
	}
	out := []map[string]string{{}}
	for _, axis := range axes {
		next := make([]map[string]string, 0, len(out)*len(axis.Values))
		for _, base := range out {
			for _, v := range axis.Values {
				combo := make(map[string]string, len(base)+1)
				for k, val := range base {
					combo[k] = val
				}
				combo[axis.Key] = v
				next = append(next, combo)
			}
		}
		out = next
	}
	// 组合数为 1（所有轴都只有 1 个取值）时没有正交意义，交给调用方按"无轴"处理。
	return out
}

// dedupeNonEmpty 去空白、去空串、去重并保持原顺序。
func dedupeNonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// axisKeys 轴参数名（有序），用于给 SKU 编码按固定顺序拼段。
func (p *axisPlan) axisKeys() []string {
	if p.Empty() {
		return nil
	}
	keys := make([]string, 0, len(p.Axes))
	for _, axis := range p.Axes {
		keys = append(keys, axis.Key)
	}
	return keys
}

// splitByAxis 把配置档分成两类：参与正交的轴参数 / 仍是客户选配项的其余参数。
// 轴参数不再生成客户配置项 —— 客户是通过"选规格"而不是"改配置"来选它的。
func splitByAxis(sel map[string]optionSelection) (axes map[string]bool, rest map[string]optionSelection) {
	axes = map[string]bool{}
	rest = map[string]optionSelection{}
	for key, item := range sel {
		if item.SKUAxis {
			axes[key] = true
			continue
		}
		rest[key] = item
	}
	return axes, rest
}

// sortCombos 按轴的声明顺序把组合排成稳定的可读串（仅用于日志/测试断言）。
func sortCombos(combos []map[string]string, keys []string) []string {
	out := make([]string, 0, len(combos))
	for _, combo := range combos {
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+"="+combo[k])
		}
		out = append(out, strings.Join(parts, ","))
	}
	sort.Strings(out)
	return out
}
