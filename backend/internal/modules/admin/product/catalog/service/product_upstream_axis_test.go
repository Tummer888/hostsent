package service

// 上游 SKU 维度（T4.8）的单测。
//
// 这一批守的是三类真实风险：
//  1. 组合爆炸 —— 把数量型配置项（带宽/系统盘）当维度，10~1000 每个值一个 SKU；
//  2. 规格与订单不符 —— 客户选配整体覆盖 configoption，把 SKU 的 pin 冲掉，
//     于是"买 8 核、按 4 核开通"；
//  3. 重复生成 —— 编码不稳定导致重跑堆出一批同名规格。

import (
	"encoding/json"
	"strings"
	"testing"
)

// --- A 组：轴候选识别（数量型必须被排除）---

func TestIsQuantityOptionBlocksAxisCandidacy(t *testing.T) {
	// 数量型 option_type（与上游 common.php::judgeQuantity 同口径）必须判为数量型、
	// 因而不能做 SKU 维度：这类项上游收的是**数量本身**而不是子项 id，
	// 客户填的是连续值，枚举不出有限 SKU。
	// 实测形态（resource_products.raw_specs）：type 11 的 bw 带 qty 25~25、
	// type 19 的 system_disk_size 带 qty 60~512 —— 都是"填个数"。
	quantityTypes := []int{4, 7, 9, 11, 14, 15, 16, 17, 18, 19}
	for _, ot := range quantityTypes {
		if !isQuantityOption(ot) {
			t.Fatalf("option_type=%d 应判为数量型，因而不能做 SKU 维度", ot)
		}
	}
	// 离散型：可做 SKU 维度。实测 type 10/13 的 bw 与 system_disk_size 虽名为"带宽/系统盘"，
	// 但形态是「N|NMbps」这样的**离散取值**（qty 为 0/0、有 sub），因此可枚举。
	discreteTypes := []int{5, 6, 8, 10, 12, 13}
	for _, ot := range discreteTypes {
		if isQuantityOption(ot) {
			t.Fatalf("option_type=%d 是离散取值，不该判为数量型（否则无法做 SKU 维度）", ot)
		}
	}
}

// --- B 组：编码派生（确定性 + 可读 + 不同组合不撞码）---

func TestUpstreamAxisSpecCodeIsDeterministic(t *testing.T) {
	axes := []upstreamAxisCandidate{
		{Key: "cpu"},
		{Key: "memory"},
	}
	combo := map[string]upstreamAxisValue{
		"cpu":    {Raw: "8|8核", Label: "8核"},
		"memory": {Raw: "16384|16G", Label: "16G"},
	}
	code := upstreamAxisSpecCode(axes, combo)
	if code != "8c-16g" {
		t.Fatalf("编码应为 8c-16g，实际 %s", code)
	}
	// 同组合重复派生必须一致 —— 这是幂等重跑的基础。
	for i := 0; i < 3; i++ {
		if again := upstreamAxisSpecCode(axes, combo); again != code {
			t.Fatalf("编码不稳定：第 %d 次得到 %s", i+1, again)
		}
	}
}

func TestUpstreamAxisSpecCodeDistinguishesCombinations(t *testing.T) {
	axes := []upstreamAxisCandidate{{Key: "cpu"}, {Key: "memory"}}
	seen := map[string]string{}
	for _, cpu := range []upstreamAxisValue{{Raw: "4|4核", Label: "4核"}, {Raw: "8|8核", Label: "8核"}} {
		for _, mem := range []upstreamAxisValue{{Raw: "8192|8G", Label: "8G"}, {Raw: "16384|16G", Label: "16G"}} {
			combo := map[string]upstreamAxisValue{"cpu": cpu, "memory": mem}
			code := upstreamAxisSpecCode(axes, combo)
			if prev, dup := seen[code]; dup {
				t.Fatalf("不同组合派生出同一编码 %s（%s 与 %s/%s）", code, prev, cpu.Label, mem.Label)
			}
			seen[code] = cpu.Label + "/" + mem.Label
		}
	}
	if len(seen) != 4 {
		t.Fatalf("4 个组合应得 4 个不同编码，实际 %d", len(seen))
	}
}

func TestUpstreamAxisSpecCodeNonNumericValue(t *testing.T) {
	// os 这类没有数字的维度：用参数名 + 清洗后的标签，且要能区分取值。
	axes := []upstreamAxisCandidate{{Key: "os"}}
	a := upstreamAxisSpecCode(axes, map[string]upstreamAxisValue{"os": {Raw: "12|CentOS", Label: "CentOS", UpstreamSubID: 12}})
	b := upstreamAxisSpecCode(axes, map[string]upstreamAxisValue{"os": {Raw: "17|Ubuntu", Label: "Ubuntu", UpstreamSubID: 17}})
	if a == b {
		t.Fatalf("不同 os 取值不该得到同一编码：%s", a)
	}
	if !strings.Contains(a, "os") {
		t.Fatalf("非数字维度编码应带参数名，实际 %s", a)
	}
}

// --- C 组：pin 形态（决定履约能否钉住组合）---

func TestUpstreamAxisPinsUseUpstreamIDs(t *testing.T) {
	// 键必须是**上游配置项 id**、值必须是**上游取值 id**：
	// 上游 cart 按 configoption[选项id]=取值id 取参，用本地词汇（"cpu"）下不去，
	// 上游会按默认配置开通 —— 那就等于 SKU 白拆了。
	// 实测形态（resource_products 76）：cpu 的 option id 是 199985、
	// 其 8核 取值的 sub id 是 737892。
	axes := []upstreamAxisCandidate{
		{Key: "cpu", UpstreamOptionID: 199985},
		{Key: "memory", UpstreamOptionID: 199986},
	}
	combo := map[string]upstreamAxisValue{
		"cpu":    {UpstreamSubID: 737892, Raw: "8|8核", Label: "8核"},
		"memory": {UpstreamSubID: 737898, Raw: "16384|16G", Label: "16G"},
	}
	pins := upstreamAxisPins(axes, combo)
	if len(pins) != 2 {
		t.Fatalf("应有 2 个 pin，实际 %d", len(pins))
	}
	if pins["199985"] != "737892" {
		t.Fatalf("cpu pin 应为 {199985: 737892}，实际 %v", pins)
	}
	if pins["199986"] != "737898" {
		t.Fatalf("memory pin 应为 {199986: 737898}，实际 %v", pins)
	}
	// 不允许出现本地参数名做键（那正是被修掉的缺陷）。
	if _, bad := pins["cpu"]; bad {
		t.Fatalf("pin 的键不能用本地参数名，实际 %v", pins)
	}
}

func TestUpstreamAxisPinsSkipsIncompleteAxis(t *testing.T) {
	// 上游选项 id 或取值 id 缺失时不能产出 pin：宁可少下发一个参数
	// （由商品配置项兜底），也不要下发一个上游认不出的键。
	axes := []upstreamAxisCandidate{{Key: "cpu", UpstreamOptionID: 0}}
	combo := map[string]upstreamAxisValue{"cpu": {UpstreamSubID: 737892, Label: "8核"}}
	if pins := upstreamAxisPins(axes, combo); len(pins) != 0 {
		t.Fatalf("缺上游选项 id 时不该产出 pin，实际 %v", pins)
	}
	axes2 := []upstreamAxisCandidate{{Key: "cpu", UpstreamOptionID: 199985}}
	combo2 := map[string]upstreamAxisValue{"cpu": {UpstreamSubID: 0, Label: "8核"}}
	if pins := upstreamAxisPins(axes2, combo2); len(pins) != 0 {
		t.Fatalf("缺上游取值 id 时不该产出 pin，实际 %v", pins)
	}
}

// --- D 组：configoption 合并（修"客户选配冲掉 SKU pin"）---

func TestMergeConfigOptionMapKeepsPins(t *testing.T) {
	// 实测缺陷场景：SKU pin 了 cpu=1001（8核），客户另选了 os=17。
	// 旧实现是整体赋值，pin 会被替换成只剩 os 的那份 —— 履约按默认 cpu 开通。
	pins := map[string]interface{}{"163751": "1001"}
	customer := map[string]interface{}{"163752": "17"}
	merged := mergeConfigOptionMap(pins, customer)
	if merged["163751"] != "1001" {
		t.Fatalf("SKU 的 pin 必须保留，实际 %v", merged)
	}
	if merged["163752"] != "17" {
		t.Fatalf("客户选择也必须写入，实际 %v", merged)
	}
	if len(merged) != 2 {
		t.Fatalf("应合并成 2 项，实际 %d", len(merged))
	}
}

func TestMergeConfigOptionMapCustomerWinsOnSameKey(t *testing.T) {
	// 同键冲突（轴参数已从可选项排除，正常不会发生）：客户显式选择代表本次订单意图，应当赢。
	pins := map[string]interface{}{"100": "1"}
	customer := map[string]interface{}{"100": "2"}
	merged := mergeConfigOptionMap(pins, customer)
	if merged["100"] != "2" {
		t.Fatalf("同键应以客户选择为准，实际 %v", merged["100"])
	}
}

func TestMergeConfigOptionMapHandlesNilAndWrongType(t *testing.T) {
	// 上游商品的 opts["configoption"] 可能是 nil 或非 map（历史数据/手工绑定），
	// 类型断言失败不能 panic，也不能把客户选择丢掉。
	if got := mergeConfigOptionMap(nil, map[string]interface{}{"a": "1"}); got["a"] != "1" {
		t.Fatalf("base 为 nil 时应取 override，实际 %v", got)
	}
	if got := mergeConfigOptionMap("not-a-map", map[string]interface{}{"a": "1"}); got["a"] != "1" {
		t.Fatalf("base 类型非法时应取 override，实际 %v", got)
	}
	if got := mergeConfigOptionMap(map[string]interface{}{"a": "1"}, nil); got["a"] != "1" {
		t.Fatalf("override 为 nil 时应保留 base，实际 %v", got)
	}
}

// --- E 组：轴选择解析（多来源写入的容错）---

func TestParseUpstreamAxisSelection(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		expected []string
	}{
		{"标准布尔形态", `{"cpu":true,"memory":true}`, []string{"cpu", "memory"}},
		{"false 不算选中", `{"cpu":true,"memory":false}`, []string{"cpu"}},
		{"空串", "", nil},
		{"null 字面量", "null", nil},
		{"空对象", "{}", nil},
		{"数字形态兼容", `{"cpu":1,"memory":0}`, []string{"cpu"}},
		{"字符串形态兼容", `{"cpu":"true"}`, []string{"cpu"}},
		{"非法 JSON 不 panic", `{bad json`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseUpstreamAxisSelection(tc.raw)
			if len(got) != len(tc.expected) {
				t.Fatalf("期望 %v，实际 %v", tc.expected, got)
			}
			for _, k := range tc.expected {
				if !got[k] {
					t.Fatalf("应包含 %s，实际 %v", k, got)
				}
			}
		})
	}
}

func TestParseUpstreamAxisSelectionRoundTrip(t *testing.T) {
	// 保存用 json.Marshal(chosen)，读回用 parseUpstreamAxisSelection：
	// 两端口径必须闭环，否则保存了却不生效。
	chosen := map[string]bool{"cpu": true, "memory": true}
	raw, err := json.Marshal(chosen)
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}
	back := parseUpstreamAxisSelection(string(raw))
	if !back["cpu"] || !back["memory"] || len(back) != 2 {
		t.Fatalf("序列化后读回不一致：%v", back)
	}
}

// --- F 组：价格 = 基准 + 组合加价 ---

func TestUpstreamAxisPriceAddsSurcharges(t *testing.T) {
	// 复现上游商品 112 的真实形态：基准月付 40，cpu 8核加 48、内存 16G 加 112。
	// 期望 40 + 48 + 112 = 200。
	axes := []upstreamAxisCandidate{{Key: "cpu"}, {Key: "memory"}}
	combo := map[string]upstreamAxisValue{
		"cpu":    {Monthly: 48, Quarterly: 144, Annually: 576},
		"memory": {Monthly: 112, Quarterly: 336, Annually: 1344},
	}
	if got := upstreamAxisPrice(40, axes, combo, "monthly"); got != 200 {
		t.Fatalf("月付应为 200，实际 %v", got)
	}
	if got := upstreamAxisPrice(40, axes, combo, "annually"); got != 1960 {
		t.Fatalf("年付应为 1960（40+576+1344），实际 %v", got)
	}
}

func TestUpstreamAxisPriceFallsBackToMonthly(t *testing.T) {
	// 上游多数商品只有月价：其它周期缺档时回落月价（与自营 priceForCycle 同口径）。
	axes := []upstreamAxisCandidate{{Key: "cpu"}}
	combo := map[string]upstreamAxisValue{"cpu": {Monthly: 48}}
	if got := upstreamAxisPrice(100, axes, combo, "quarterly"); got != 148 {
		t.Fatalf("季付缺价时应回落月价：期望 148，实际 %v", got)
	}
}

func TestUpstreamAxisPriceNeverNegative(t *testing.T) {
	// 加价为负（上游偶尔用负数表达优惠）时可算出负数，必须夹到 0 —— 负价格能白拿机器。
	axes := []upstreamAxisCandidate{{Key: "cpu"}}
	combo := map[string]upstreamAxisValue{"cpu": {Monthly: -200}}
	if got := upstreamAxisPrice(100, axes, combo, "monthly"); got < 0 {
		t.Fatalf("价格不该为负，实际 %v", got)
	}
}

// --- G 组：笛卡尔积 ---

func TestUpstreamAxisCombos(t *testing.T) {
	axes := []upstreamAxisCandidate{
		{Key: "cpu", Values: []upstreamAxisValue{{Raw: "4|4核"}, {Raw: "8|8核"}}},
		{Key: "memory", Values: []upstreamAxisValue{{Raw: "8192|8G"}, {Raw: "16384|16G"}, {Raw: "32768|32G"}}},
	}
	combos := upstreamAxisCombos(axes)
	if len(combos) != 6 {
		t.Fatalf("2 × 3 应为 6 个组合，实际 %d", len(combos))
	}
	seen := map[string]bool{}
	for _, combo := range combos {
		key := combo["cpu"].Raw + "|" + combo["memory"].Raw
		if seen[key] {
			t.Fatalf("组合重复：%s", key)
		}
		seen[key] = true
	}
	if len(seen) != 6 {
		t.Fatalf("应有 6 个不同组合，实际 %d", len(seen))
	}
	// 顺序稳定：两次展开结果一致（编码与排序都依赖它）。
	again := upstreamAxisCombos(axes)
	for i := range combos {
		if combos[i]["cpu"].Raw != again[i]["cpu"].Raw || combos[i]["memory"].Raw != again[i]["memory"].Raw {
			t.Fatalf("第 %d 个组合顺序不稳定", i)
		}
	}
}

func TestUpstreamAxisCombinationCount(t *testing.T) {
	plan := &upstreamAxisPlan{Axes: []upstreamAxisCandidate{
		{Key: "cpu", Values: make([]upstreamAxisValue, 8)},
		{Key: "memory", Values: make([]upstreamAxisValue, 11)},
	}}
	if got := plan.CombinationCount(); got != 88 {
		t.Fatalf("8 × 11 应为 88，实际 %d", got)
	}
	// 88 > 64 上限：界面与保存都要能据此拒绝。
	if plan.CombinationCount() <= upstreamAxisMaxCombinations {
		t.Fatalf("88 应超出上限 %d", upstreamAxisMaxCombinations)
	}
}

func TestUpstreamAxisPlanEmpty(t *testing.T) {
	if !(&upstreamAxisPlan{}).Empty() {
		t.Fatal("无轴应判定为空计划")
	}
	var nilPlan *upstreamAxisPlan
	if !nilPlan.Empty() {
		t.Fatal("nil 计划应为空")
	}
}
