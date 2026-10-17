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

	"hostsent/backend/internal/modules/admin/product/catalog/dto"
	"hostsent/backend/internal/pkg/billingcycle"
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

// matchAxisPrice 在逐规格定价列表里找匹配某个取值组合的那一项。
//
// 匹配口径：键集合完全一致、且每个取值都相等。要求"键集合一致"是为了避免误命中 ——
// 若只比对交集，运营给 {cpu:4} 定的价会顺带命中 {cpu:4, memory:8192}，
// 那个规格就拿不到自己该有的价。运营只需要给想单独定价的规格填项。
func matchAxisPrice(prices []dto.ProductAxisSpecPrice, combo map[string]string) (dto.ProductAxisSpecPrice, bool) {
	for _, item := range prices {
		if len(item.Values) != len(combo) {
			continue
		}
		matched := true
		for k, want := range item.Values {
			got, ok := combo[k]
			if !ok || strings.TrimSpace(got) != strings.TrimSpace(want) {
				matched = false
				break
			}
		}
		if matched {
			return item, true
		}
	}
	return dto.ProductAxisSpecPrice{}, false
}

// baseAxisSpecPrice 取出"没有轴维度"那一档的定价（Values 为空的项）。
//
// 为什么需要：配置档没标 sku_axis 时前端仍会渲染一行规格（它就是"这个档位"本身），
// 那行的定价同样要落到矩阵里 —— 否则自营单规格商品的周期价在建品页填了却写不进去，
// 又回到"填了价、客户看到的是旧价"的老问题。
func baseAxisSpecPrice(prices []dto.ProductAxisSpecPrice) (dto.ProductAxisSpecPrice, bool) {
	return matchAxisPrice(prices, nil)
}

// axisComboWithDefaults 把轴组合补全成"该规格完整的取值集合"，供逐规格定价匹配用。
//
// 为什么要补：前端只提交它勾选过的轴取值组合；组合的键集合必须与运营填价时看到的
// 规格维度一致，否则匹配会因键数不等而全部落空。
func axisComboWithDefaults(plan *axisPlan, combo map[string]string) map[string]string {
	out := make(map[string]string, len(combo))
	for k, v := range combo {
		out[k] = v
	}
	if plan == nil {
		return out
	}
	// 组合本身已含全部轴取值（plan.Combinations 是完整笛卡尔积），这里只做补齐兜底。
	for _, axis := range plan.Axes {
		if _, ok := out[axis.Key]; ok {
			continue
		}
		if len(axis.Values) > 0 {
			out[axis.Key] = axis.Values[0]
		}
	}
	return out
}

// resolveAxisBaseline 定出每个轴的基线取值（该取值不加价、已含在套餐价里）。
//
// 优先级：压缩方案显式声明 > 模板该轴的默认值 > 该轴第一个取值。
// 必须**每轴都有且只有一个**基线，否则套餐价没有归属：某个轴谁都不算基线，
// 展开出来的每个组合都会额外叠一份加价；两个取值都算基线则会互相抵消。
func resolveAxisBaseline(plan *axisPlan, charges *dto.ProductAxisChargePlan, defaults map[string]string) map[string]string {
	out := map[string]string{}
	if plan == nil {
		return out
	}
	for _, axis := range plan.Axes {
		if charges != nil {
			if v := strings.TrimSpace(charges.BaseValues[axis.Key]); v != "" {
				// 声明的基线必须是该轴真实取值，否则那个组合根本不存在，
				// 而真正的基线取值会被当成"非基线"多收一份加价。
				if containsString(axis.Values, v) {
					out[axis.Key] = v
					continue
				}
			}
		}
		if v := strings.TrimSpace(defaults[axis.Key]); v != "" && containsString(axis.Values, v) {
			out[axis.Key] = v
			continue
		}
		if len(axis.Values) > 0 {
			out[axis.Key] = axis.Values[0]
		}
	}
	return out
}

// axisChargeOf 取某轴某取值的加价；基线取值与未填写的取值都是 0。
//
// 返回**原始值**（含负数）而不是"大于 0 才算"：负加价是必须被拒绝的输入，
// 在这里悄悄换成 0 会让运营以为降价生效了，实际卖的还是原价。
func axisChargeOf(charges *dto.ProductAxisChargePlan, key, value string, baseline map[string]string) float64 {
	if charges == nil {
		return 0
	}
	if strings.TrimSpace(baseline[key]) == strings.TrimSpace(value) {
		return 0
	}
	if byValue, ok := charges.Values[key]; ok {
		return byValue[value]
	}
	return 0
}

// axisCostChargeOf 同 axisChargeOf，取的是成本加价。
func axisCostChargeOf(charges *dto.ProductAxisChargePlan, key, value string, baseline map[string]string) float64 {
	if charges == nil {
		return 0
	}
	if strings.TrimSpace(baseline[key]) == strings.TrimSpace(value) {
		return 0
	}
	if byValue, ok := charges.CostValues[key]; ok {
		return byValue[value]
	}
	return 0
}

// chargeRatioForCycle 该周期相对月付的加价倍率。
//
// 为什么不能直接按月数乘：套餐价本身可能已经不按"月数 × 月付"定价
// （年付基准 1000 而月付 100，即年付只算 10 个月）。加价必须跟着**同一个比例**走，
// 否则"年付省 2 个月"的优惠只在套餐上生效，加价部分反而按 12 个月收，
// 客户选高配年付时总价会出现倒挂（高配年付比低配年付贵得不成比例）。
//
// 没有基准周期价时回落到月数（至少 1），保证加价不会变成 0。
func chargeRatioForCycle(cycle string, charges *dto.ProductAxisChargePlan) float64 {
	months := billingcycle.Multiplier(cycle)
	if months <= 0 {
		months = 1
	}
	if charges == nil {
		return float64(months)
	}
	base := charges.BaseCycles[billingcycle.Normalize(cycle)]
	if base <= 0 || charges.BasePrice <= 0 {
		return float64(months)
	}
	ratio := base / charges.BasePrice
	if ratio <= 0 {
		return float64(months)
	}
	return ratio
}

// axisChargeCombo 算出某个取值组合的（售价, 成本价）与逐周期价。
//
// 口径：价 = 套餐基准价 + Σ 各取值的加价（基线不加价）。成本同口径。
// 逐周期价 = 套餐该周期基准价 + Σ 加价 × 该周期倍率；套餐没定该周期时
// 回落到"月付价 × 周期月数"，交给上层再按需推导长周期。
//
// 返回的 cycles 只包含**套餐明确给过价的周期**，其余周期由调用方推导，
// 这样"只填月付"的商品不会被这里凭空写出一个季付价。
func axisChargeCombo(
	combo map[string]string,
	charges *dto.ProductAxisChargePlan,
	plan *axisPlan,
	defaults map[string]string,
) (price float64, cost float64, cycles map[string]float64, err error) {
	if charges == nil {
		return 0, 0, nil, nil
	}
	baseline := resolveAxisBaseline(plan, charges, defaults)
	price = charges.BasePrice
	cost = charges.BaseCost
	for _, axis := range plan.Axes {
		value := combo[axis.Key]
		if value == "" {
			value = baseline[axis.Key]
		}
		amount := axisChargeOf(charges, axis.Key, value, baseline)
		if amount < 0 {
			return 0, 0, nil, fmt.Errorf(
				"配置项「%s」取值「%s」的加价不能为负（负加价请用逐规格单独定价表达）",
				axis.Key, value)
		}
		price += amount
		cost += axisCostChargeOf(charges, axis.Key, value, baseline)
	}
	if len(charges.BaseCycles) == 0 {
		return price, cost, nil, nil
	}
	cycles = make(map[string]float64, len(charges.BaseCycles))
	for cycle, base := range charges.BaseCycles {
		code := billingcycle.Normalize(cycle)
		if code == "" || base <= 0 {
			continue
		}
		ratio := chargeRatioForCycle(code, charges)
		cycles[code] = round2Price(base + (price-charges.BasePrice)*ratio)
	}
	if len(cycles) == 0 {
		cycles = nil
	}
	return price, cost, cycles, nil
}

// containsString 判断值是否在列表里（轴取值口径：忽略两侧空白）。
func containsString(list []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, item := range list {
		if strings.TrimSpace(item) == want {
			return true
		}
	}
	return false
}

// expandCyclesForPlan 把"月付价 + 已有长周期价"补满成全部可售周期的价。
//
// 为什么必须补满：每个规格会写自己的矩阵行，而矩阵语义是"某规格有任意一行，
// 它就是该规格所有周期的权威"—— 少写一个周期，客户选那个周期时会被硬拒
// （"该商品不提供季付"），而不是回落到商品级价。运营在页面上只填月付价时，
// 长周期由这里按**周期月数**推导，至少保证"能买"且不亏。
//
// 推导口径：月付 × 月数。刻意不在这里做"省 N 个月"的优惠 —— 那是运营的定价决策，
// 要么在压缩方案的 BaseCycles 里明确给出，要么在逐规格单独定价里填。
func expandCyclesForPlan(monthly float64, sellable []string, known map[string]float64) map[string]float64 {
	if monthly <= 0 {
		return known
	}
	out := make(map[string]float64, len(sellable))
	for _, raw := range sellable {
		code := billingcycle.Normalize(raw)
		if code == "" {
			continue
		}
		if price, ok := known[code]; ok && price > 0 {
			out[code] = round2Price(price)
			continue
		}
		if code == billingcycle.Monthly || billingcycle.Multiplier(code) <= 1 {
			out[code] = round2Price(monthly)
			continue
		}
		out[code] = round2Price(monthly * float64(billingcycle.Multiplier(code)))
	}
	if len(out) == 0 {
		return known
	}
	return out
}

// normalizeCycleList 归一化周期列表并去重（保持输入顺序）。
func normalizeCycleList(raw []string) []string {
	out := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, item := range raw {
		code := billingcycle.Normalize(item)
		if code == "" {
			continue
		}
		if _, dup := seen[code]; dup {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out
}
