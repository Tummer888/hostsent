package service

// SPU/SKU 正交轴的单测（T4.7）。
//
// 这一批测试守的是三个已发生的故障：
//  1. 组合数爆炸 —— 区间型参数被当成轴，带宽 10~100 每兆一个 SKU；
//  2. SKU 重复 —— 编码带时间戳后缀，重跑一次多一批同名规格；
//  3. 编码与配置不符 —— 商品名写 6G、开通下发 4G（基线被配置项默认值覆盖）。

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"hostsent/backend/internal/modules/admin/product/catalog/dto"
	"hostsent/backend/internal/pkg/specatom"
)

// 测试用的原子字典：够覆盖 cpu/memory/disk/bw/os/region 的反查即可。
func seedAxisTestDictionary(t *testing.T) {
	t.Helper()
	dict := specatom.NewDictionary()
	dict.Replace([]specatom.Atom{
		{Key: specatom.KeyCPU, Name: "CPU", Unit: "core", ValueType: "int", Status: 1,
			PlatformFields: map[string]specatom.AtomPlatformField{
				"dcimcloud": {Read: "cpu_num", Write: "cpu"},
			}},
		{Key: specatom.KeyMemory, Name: "内存", Unit: "MB", ValueType: "int", Status: 1,
			PlatformFields: map[string]specatom.AtomPlatformField{
				"dcimcloud": {Read: "memory_size", Write: "memory"},
			}},
		{Key: specatom.KeyDisk, Name: "系统盘", Unit: "GB", ValueType: "int", Status: 1,
			PlatformFields: map[string]specatom.AtomPlatformField{
				"dcimcloud": {Read: "disk_size", Write: "system_disk_size"},
			}},
		{Key: specatom.KeyBandwidth, Name: "带宽", Unit: "Mbps", ValueType: "int", Status: 1,
			PlatformFields: map[string]specatom.AtomPlatformField{
				"dcimcloud": {Read: "bandwidth", Write: "bw"},
			}},
		{Key: specatom.KeyOS, Name: "操作系统", ValueType: "string", Status: 1,
			PlatformFields: map[string]specatom.AtomPlatformField{
				"dcimcloud": {Read: "os_image", Write: "os"},
			}},
		{Key: specatom.KeyRegion, Name: "区域", ValueType: "string", Status: 1,
			PlatformFields: map[string]specatom.AtomPlatformField{
				"dcimcloud": {Read: "region", Write: "area"},
			}},
	})
	specatom.SetGlobal(dict)
}

// --- A 组：轴计划校验（区间型不能做轴、空取值、组合数上限）---

func TestBuildAxisPlanRejectsRangeOption(t *testing.T) {
	// 带宽配成滑动区间又想参与正交 —— 必须在建品前拦下，否则就是组合数爆炸的源头。
	sel := map[string]optionSelection{
		"bw": {Range: []float64{10, 100}, Default: "10", SKUAxis: true},
	}
	_, err := buildAxisPlan(sel)
	if err == nil {
		t.Fatal("区间型参数标为正交轴应当报错，实际通过了")
	}
	if !strings.Contains(err.Error(), "取值范围") {
		t.Fatalf("错误信息应说明是区间型不能做轴，实际：%v", err)
	}
	if !strings.Contains(err.Error(), "bw") {
		t.Fatalf("错误信息应指出参数名，实际：%v", err)
	}
}

func TestBuildAxisPlanRejectsEmptyValues(t *testing.T) {
	sel := map[string]optionSelection{
		"memory": {Values: []string{"  ", ""}, SKUAxis: true},
	}
	_, err := buildAxisPlan(sel)
	if err == nil {
		t.Fatal("轴参数没有有效取值应当报错")
	}
	if !strings.Contains(err.Error(), "没有勾选任何取值") {
		t.Fatalf("错误信息应说明缺取值，实际：%v", err)
	}
}

func TestBuildAxisPlanRejectsTooManyCombinations(t *testing.T) {
	// 8 × 11 = 88 > 64：应当在展开前就拒绝，并给出可读原因。
	cpu := []string{"1", "2", "4", "8", "12", "16", "24", "32"}
	mem := []string{"1024", "2048", "4096", "6144", "8192", "12288", "16384", "24576", "32768", "65536", "131072"}
	sel := map[string]optionSelection{
		"cpu":    {Values: cpu, SKUAxis: true},
		"memory": {Values: mem, SKUAxis: true},
	}
	_, err := buildAxisPlan(sel)
	if err == nil {
		t.Fatal("88 个组合超出上限应当报错")
	}
	if !strings.Contains(err.Error(), "88") || !strings.Contains(err.Error(), "上限") {
		t.Fatalf("错误信息应报出组合数与上限，实际：%v", err)
	}
}

func TestBuildAxisPlanIgnoresRangeOptionWithoutAxisFlag(t *testing.T) {
	// 反面：区间型参数**没**标轴时完全正常（它是客户加配项，不是规格维度）。
	sel := map[string]optionSelection{
		"bw":     {Range: []float64{10, 100}, Default: "10"},
		"memory": {Values: []string{"4096", "8192"}, SKUAxis: true},
	}
	plan, err := buildAxisPlan(sel)
	if err != nil {
		t.Fatalf("未标轴的区间参数不该报错：%v", err)
	}
	if len(plan.Axes) != 1 || plan.Axes[0].Key != "memory" {
		t.Fatalf("应只有 memory 一个轴，实际：%+v", plan.Axes)
	}
	if len(plan.Combinations) != 2 {
		t.Fatalf("2 个内存取值应展开 2 个组合，实际 %d", len(plan.Combinations))
	}
}

// --- B 组：笛卡尔积 ---

func TestBuildAxisPlanCartesianProduct(t *testing.T) {
	sel := map[string]optionSelection{
		"cpu":    {Values: []string{"4", "8"}, SKUAxis: true},
		"memory": {Values: []string{"4096", "8192", "16384"}, SKUAxis: true},
	}
	plan, err := buildAxisPlan(sel)
	if err != nil {
		t.Fatalf("展开失败：%v", err)
	}
	if len(plan.Combinations) != 6 {
		t.Fatalf("2 × 3 应为 6 个组合，实际 %d", len(plan.Combinations))
	}
	// 组合必须无重复且覆盖全部配对。
	seen := map[string]bool{}
	for _, combo := range plan.Combinations {
		key := combo["cpu"] + "|" + combo["memory"]
		if seen[key] {
			t.Fatalf("组合重复：%s", key)
		}
		seen[key] = true
	}
	for _, cpu := range []string{"4", "8"} {
		for _, mem := range []string{"4096", "8192", "16384"} {
			if !seen[cpu+"|"+mem] {
				t.Fatalf("缺少组合 %s|%s", cpu, mem)
			}
		}
	}
	// 顺序稳定：同一份配置两次展开结果一致（编码/排序都依赖它）。
	plan2, _ := buildAxisPlan(sel)
	if got, want := strings.Join(sortCombos(plan.Combinations, plan.axisKeys()), ";"),
		strings.Join(sortCombos(plan2.Combinations, plan2.axisKeys()), ";"); got != want {
		t.Fatalf("展开顺序不稳定：\n%s\n%s", got, want)
	}
}

func TestBuildAxisPlanNoAxes(t *testing.T) {
	// 没有任何轴 → Empty，调用方走原有单 SKU 路径。
	sel := map[string]optionSelection{
		"memory": {Values: []string{"4096", "8192"}},
		"bw":     {Range: []float64{1, 100}, Default: "10"},
	}
	plan, err := buildAxisPlan(sel)
	if err != nil {
		t.Fatalf("无轴不该报错：%v", err)
	}
	if !plan.Empty() {
		t.Fatal("没有 sku_axis 标记时应判定为空计划")
	}
}

// --- C 组：编码派生（确定性、可读、避免重复）---

func TestDeriveAxisSpecCodeIsDeterministicAndReadable(t *testing.T) {
	seedAxisTestDictionary(t)
	axisKeys := []string{"cpu", "memory", "system_disk_size"}
	combo := map[string]string{"cpu": "4", "memory": "6144", "system_disk_size": "100"}
	code := deriveAxisSpecCode("general", "dcimcloud", axisKeys, combo)
	if code != "general-4c6g-100g" {
		t.Fatalf("编码应可读且按行业写法，实际：%s", code)
	}
	// 同一组合重复派生必须完全一致 —— 这是幂等重跑的基础（时间戳后缀会让它每次都变）。
	for i := 0; i < 3; i++ {
		if again := deriveAxisSpecCode("general", "dcimcloud", axisKeys, combo); again != code {
			t.Fatalf("编码不稳定：第 %d 次得到 %s", i+1, again)
		}
	}
}

func TestDeriveAxisSpecCodeDistinguishesCombinations(t *testing.T) {
	seedAxisTestDictionary(t)
	axisKeys := []string{"cpu", "memory"}
	codes := map[string]string{}
	for _, cpu := range []string{"4", "8"} {
		for _, mem := range []string{"4096", "8192"} {
			combo := map[string]string{"cpu": cpu, "memory": mem}
			code := deriveAxisSpecCode("general", "dcimcloud", axisKeys, combo)
			if prev, dup := codes[code]; dup {
				t.Fatalf("不同组合派生出同一编码 %s（prev=%s，当前 cpu=%s mem=%s）", code, prev, cpu, mem)
			}
			codes[code] = cpu + "/" + mem
		}
	}
	if len(codes) != 4 {
		t.Fatalf("4 个组合应得 4 个不同编码，实际 %d", len(codes))
	}
}

func TestDeriveAxisSpecCodeHandlesNonAtomAxis(t *testing.T) {
	seedAxisTestDictionary(t)
	// port / snap_num 这类没有原子映射的轴：用「参数名+取值」拼段，非字母数字被清掉。
	code := deriveAxisSpecCode("general", "dcimcloud", []string{"port"}, map[string]string{"port": "auto"})
	if code != "general-portauto" {
		t.Fatalf("无原子映射的轴应回落为参数名+取值，实际：%s", code)
	}
}

// --- D 组：轴取值写入原子取值与平台参数 ---

func TestApplyAxisToSpecValuesTranslatesToAtomKeys(t *testing.T) {
	seedAxisTestDictionary(t)
	base := `{"compute.cpu":2,"compute.memory":2048}`
	combo := map[string]string{"cpu": "8", "memory": "8192"}
	out, leftover, err := applyAxisToSpecValues("dcimcloud", base, combo)
	if err != nil {
		t.Fatalf("写入失败：%v", err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("结果不是合法 JSON：%v", err)
	}
	// 轴值必须覆盖基线，且是数字类型（平台接口不接受字符串数字）。
	if got := valueAsInt(m, specatom.KeyCPU); got != 8 {
		t.Fatalf("CPU 应为 8，实际 %v", m[specatom.KeyCPU])
	}
	if got := valueAsInt(m, specatom.KeyMemory); got != 8192 {
		t.Fatalf("内存应为 8192，实际 %v", m[specatom.KeyMemory])
	}
	if _, isFloat := m[specatom.KeyCPU].(float64); !isFloat {
		t.Fatalf("CPU 应是 JSON 数字，实际类型 %T", m[specatom.KeyCPU])
	}
	if len(leftover) != 0 {
		t.Fatalf("cpu/memory 都有原子映射，不该有 leftover，实际 %v", leftover)
	}
}

func TestApplyAxisToSpecValuesCollectsNonAtomLeftover(t *testing.T) {
	seedAxisTestDictionary(t)
	// snap_num 没有原子映射：不该被塞进 specs（那里放的是平台无关的原子取值），
	// 而要交给调用方写进平台参数，否则这个轴等于没生效。
	out, leftover, err := applyAxisToSpecValues("dcimcloud", `{"compute.cpu":4}`, map[string]string{"snap_num": "-1"})
	if err != nil {
		t.Fatalf("写入失败：%v", err)
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(out), &m)
	if _, exists := m["snap_num"]; exists {
		t.Fatal("无原子映射的参数不该进 specs")
	}
	if leftover["snap_num"] != "-1" {
		t.Fatalf("应收集为 leftover，实际 %v", leftover)
	}
}

func TestApplyAxisToPlatformParamsAxisWins(t *testing.T) {
	seedAxisTestDictionary(t)
	// 模板平台参数里已有 memory=2048，轴组合是 8192 —— 轴必须赢。
	base := `{"memory":"2048","area":"1"}`
	combo := map[string]string{"memory": "8192"}
	out, err := applyAxisToPlatformParams(base, combo, nil)
	if err != nil {
		t.Fatalf("写入失败：%v", err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("结果非法 JSON：%v", err)
	}
	if m["memory"] != "8192" {
		t.Fatalf("轴取值应覆盖模板值，实际 %v", m["memory"])
	}
	if m["area"] != "1" {
		t.Fatalf("无关参数应保留，实际 %v", m["area"])
	}
}

// --- E 组：名称模板渲染 ---

func TestRenderNameTemplate(t *testing.T) {
	cases := []struct {
		name     string
		tpl      string
		combo    map[string]string
		specs    string
		expected string
	}{
		{
			name:     "核数与内存按 GB 展示",
			tpl:      "{cpu}核{memory}G {os}",
			combo:    map[string]string{"os": "62"},
			specs:    `{"compute.cpu":8,"compute.memory":16384}`,
			expected: "8核16G 62",
		},
		{
			name:     "内存 MB 占位符",
			tpl:      "{cpu}核{memory_mb}MB",
			combo:    nil,
			specs:    `{"compute.cpu":4,"compute.memory":6144}`,
			expected: "4核6144MB",
		},
		{
			name:     "未提供的占位符被清掉，不留 {xxx}",
			tpl:      "{cpu}核{memory}G {bw}Mbps",
			combo:    nil,
			specs:    `{"compute.cpu":4,"compute.memory":4096}`,
			expected: "4核4G Mbps",
		},
		{
			name:     "空模板返回空串（调用方回落到模板名）",
			tpl:      "",
			combo:    nil,
			specs:    `{"compute.cpu":4}`,
			expected: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderNameTemplate(tc.tpl, tc.combo, tc.specs)
			if got != tc.expected {
				t.Fatalf("期望 %q，实际 %q", tc.expected, got)
			}
		})
	}
}

// --- F 组：基线对齐（修「商品名 6G、开通 4G」）---

func TestAlignSpecValuesWithOptionDefaults(t *testing.T) {
	seedAxisTestDictionary(t)
	// 复现真实故障数据：模板结构字段给 memory=6144（6G），
	// 而内存配置项的 default 是 4096（4G）。对齐后 SKU 基线必须是 4096，
	// 与客户不带选配时的实际开通值一致。
	base := `{"compute.cpu":4,"compute.memory":6144,"storage.system.size":100}`
	sel := map[string]optionSelection{
		"memory": {Values: []string{"6144", "4096", "8192"}, Default: "4096"},
	}
	out, err := alignSpecValuesWithOptionDefaults("dcimcloud", base, sel)
	if err != nil {
		t.Fatalf("对齐失败：%v", err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("结果非法 JSON：%v", err)
	}
	if got := valueAsInt(m, specatom.KeyMemory); got != 4096 {
		t.Fatalf("内存基线应对齐到配置项默认值 4096，实际 %d", got)
	}
	// 没开放给客户的参数保持原样。
	if got := valueAsInt(m, specatom.KeyDisk); got != 100 {
		t.Fatalf("未涉及的参数不该被改动，实际 disk=%d", got)
	}
}

func TestAlignSpecValuesSkipsAxisOptions(t *testing.T) {
	seedAxisTestDictionary(t)
	// 轴参数的取值由组合决定，不能被"默认值对齐"覆盖回去。
	base := `{"compute.cpu":4,"compute.memory":8192}`
	sel := map[string]optionSelection{
		"memory": {Values: []string{"4096", "8192"}, Default: "4096", SKUAxis: true},
	}
	out, err := alignSpecValuesWithOptionDefaults("dcimcloud", base, sel)
	if err != nil {
		t.Fatalf("对齐失败：%v", err)
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(out), &m)
	if got := valueAsInt(m, specatom.KeyMemory); got != 8192 {
		t.Fatalf("轴参数不该被对齐逻辑改写，实际 %d", got)
	}
}

func TestAlignSpecValuesIgnoresOptionsWithoutDefault(t *testing.T) {
	seedAxisTestDictionary(t)
	base := `{"compute.memory":6144}`
	// 没声明 default 的配置项：无法判断"默认开通什么"，保持模板基线不动。
	out, err := alignSpecValuesWithOptionDefaults("dcimcloud", base, map[string]optionSelection{
		"memory": {Values: []string{"4096", "8192"}},
	})
	if err != nil {
		t.Fatalf("对齐失败：%v", err)
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(out), &m)
	if got := valueAsInt(m, specatom.KeyMemory); got != 6144 {
		t.Fatalf("未声明默认值时不该改动基线，实际 %d", got)
	}
}

func TestAlignSpecValuesAcceptsNumericDefault(t *testing.T) {
	seedAxisTestDictionary(t)
	// default 在库里有字符串与数字两种写法（migrations/111 写的是数字），都要能读。
	var sel map[string]optionSelection
	if err := json.Unmarshal([]byte(`{"memory":{"values":["4096","8192"],"default":4096}}`), &sel); err != nil {
		t.Fatalf("数字 default 应能解析：%v", err)
	}
	out, err := alignSpecValuesWithOptionDefaults("dcimcloud", `{"compute.memory":8192}`, sel)
	if err != nil {
		t.Fatalf("对齐失败：%v", err)
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(out), &m)
	if got := valueAsInt(m, specatom.KeyMemory); got != 4096 {
		t.Fatalf("数字形态的 default 也该生效，实际 %d", got)
	}
}

// --- G 组：轴参数不再生成客户配置项（消灭双真源）---

func TestSplitByAxisSeparatesAxisAndRest(t *testing.T) {
	sel := map[string]optionSelection{
		"memory": {Values: []string{"4096", "8192"}, SKUAxis: true},
		"cpu":    {Values: []string{"4", "8"}, SKUAxis: true},
		"bw":     {Range: []float64{10, 100}, Default: "10"},
	}
	axes, rest := splitByAxis(sel)
	if len(axes) != 2 || !axes["memory"] || !axes["cpu"] {
		t.Fatalf("两个轴参数都应被识别，实际 %v", axes)
	}
	if len(rest) != 1 || rest["bw"].SKUAxis {
		t.Fatalf("带宽应留在客户选配项里，实际 %v", rest)
	}
	if _, leaked := rest["memory"]; leaked {
		t.Fatal("轴参数不该出现在客户选配项里")
	}
}

// --- C 组：逐规格定价（T4.7 建品页的"规格定价表"）---

// TestMatchAxisPriceRequiresExactKeySet 键集合必须完全一致。
// 只比对交集会让运营给 {cpu:4} 定的价顺带命中 {cpu:4, memory:8192}，
// 那个规格于是拿不到自己该有的价（这正是"价格对不上配置"的一类表现）。
func TestMatchAxisPriceRequiresExactKeySet(t *testing.T) {
	prices := []dto.ProductAxisSpecPrice{
		{Values: map[string]string{"cpu": "4"}, Price: 99},
	}
	if _, ok := matchAxisPrice(prices, map[string]string{"cpu": "4", "memory": "8192"}); ok {
		t.Fatal("{cpu:4} 的定价不该命中 {cpu:4, memory:8192} 的组合")
	}
	if got, ok := matchAxisPrice(prices, map[string]string{"cpu": "4"}); !ok || got.Price != 99 {
		t.Fatalf("键集合一致时应命中，got ok=%v price=%v", ok, got.Price)
	}
}

// TestMatchAxisPriceComparesTrimmedValues 取值比对忽略两侧空白（前端提交的键可能带空格）。
func TestMatchAxisPriceComparesTrimmedValues(t *testing.T) {
	prices := []dto.ProductAxisSpecPrice{
		{Values: map[string]string{"cpu": " 8 "}, Price: 199},
	}
	if got, ok := matchAxisPrice(prices, map[string]string{"cpu": "8"}); !ok || got.Price != 199 {
		t.Fatalf("去空白后应命中，got ok=%v price=%v", ok, got.Price)
	}
}

// TestMatchAxisPriceSelectsRightCombination 多组合各自命中自己的价（2核/4核各填各的）。
func TestMatchAxisPriceSelectsRightCombination(t *testing.T) {
	prices := []dto.ProductAxisSpecPrice{
		{Values: map[string]string{"cpu": "2"}, Price: 59},
		{Values: map[string]string{"cpu": "4"}, Price: 99},
		{Values: map[string]string{"cpu": "8"}, Price: 199},
	}
	for combo, want := range map[string]float64{"2": 59, "4": 99, "8": 199} {
		got, ok := matchAxisPrice(prices, map[string]string{"cpu": combo})
		if !ok || got.Price != want {
			t.Fatalf("cpu=%s 应命中 %v，实际 ok=%v price=%v", combo, want, ok, got.Price)
		}
	}
	if _, ok := matchAxisPrice(prices, map[string]string{"cpu": "16"}); ok {
		t.Fatal("未定价的组合不该命中任何一项")
	}
}

// TestBaseAxisSpecPrice 无轴档位的定价用空 Values 项表示（前端就渲染一行）。
func TestBaseAxisSpecPrice(t *testing.T) {
	prices := []dto.ProductAxisSpecPrice{
		{Values: map[string]string{"cpu": "4"}, Price: 99},
		{Values: map[string]string{}, Price: 29, CostPrice: 11, Cycles: map[string]float64{"monthly": 29}},
	}
	got, ok := baseAxisSpecPrice(prices)
	if !ok {
		t.Fatal("应能取到无轴档位的定价")
	}
	if got.Price != 29 || got.CostPrice != 11 {
		t.Fatalf("无轴项取值不对：price=%v cost=%v", got.Price, got.CostPrice)
	}
	// 只有带轴项时不该误取第一项当无轴价：那会把某个规格的价当成"总体默认价"。
	if _, ok := baseAxisSpecPrice([]dto.ProductAxisSpecPrice{{Values: map[string]string{"cpu": "4"}, Price: 99}}); ok {
		t.Fatal("没有空 Values 项时不应命中")
	}
	if _, ok := baseAxisSpecPrice(nil); ok {
		t.Fatal("空列表不应命中")
	}
}

// recordingMatrixWriter 记录写入的矩阵行，供断言。
type recordingMatrixWriter struct {
	rows   []CyclePriceRow
	specID uint64
}

func (w *recordingMatrixWriter) SaveSpecPrices(_ context.Context, _ uint64, specID uint64, rows []CyclePriceRow) error {
	w.specID = specID
	w.rows = append(w.rows, rows...)
	return nil
}

// TestSaveSpecCyclePricesRejectsZeroPrice 0 元周期价必须报错而不是跳过。
//
// 跳过会让该规格留下一张"部分周期"的矩阵；而矩阵语义是"有任意一行就是该规格
// 全部周期的权威"，客户选那些被跳过的周期时会被硬拒（不是回落到商品级价）。
func TestSaveSpecCyclePricesRejectsZeroPrice(t *testing.T) {
	svc := &productService{}
	w := &recordingMatrixWriter{}
	svc.cycleMatrixWriter = w

	err := svc.saveSpecCyclePrices(context.Background(), 7, 3, map[string]float64{
		"monthly":   99,
		"quarterly": 0,
	}, 40)
	if err == nil {
		t.Fatal("含 0 元周期价应报错")
	}
	if !strings.Contains(err.Error(), "必须大于 0") {
		t.Fatalf("错误应说明价格必须大于 0，实际：%v", err)
	}
	if len(w.rows) != 0 {
		t.Fatalf("报错时不该写入任何矩阵行，实际写入 %d 行", len(w.rows))
	}
}

// TestSaveSpecCyclePricesWritesAllCycles 全部周期都有价时整表写入。
func TestSaveSpecCyclePricesWritesAllCycles(t *testing.T) {
	svc := &productService{}
	w := &recordingMatrixWriter{}
	svc.cycleMatrixWriter = w

	err := svc.saveSpecCyclePrices(context.Background(), 7, 3, map[string]float64{
		"annually":  999,
		"monthly":   99,
		"quarterly": 279,
	}, 40)
	if err != nil {
		t.Fatalf("写矩阵失败：%v", err)
	}
	if w.specID != 3 {
		t.Fatalf("应写到 spec 3，实际 %d", w.specID)
	}
	if len(w.rows) != 3 {
		t.Fatalf("应写 3 个周期行，实际 %d", len(w.rows))
	}
	// 稳定顺序：年付/月付/季付按字典序，与实现一致，便于比对与复现。
	if w.rows[0].Cycle != "annually" || w.rows[1].Cycle != "monthly" || w.rows[2].Cycle != "quarterly" {
		t.Fatalf("周期顺序不稳定：%v %v %v", w.rows[0].Cycle, w.rows[1].Cycle, w.rows[2].Cycle)
	}
	for _, row := range w.rows {
		if row.CostPrice != 40 || row.Source != "manual" {
			t.Fatalf("成本价与来源应随行带上：%+v", row)
		}
	}
}

// TestSaveSpecCyclePricesRejectsUnknownCycle 不认识的周期名报错（静默丢弃会让运营以为价填上了）。
func TestSaveSpecCyclePricesRejectsUnknownCycle(t *testing.T) {
	svc := &productService{}
	svc.cycleMatrixWriter = &recordingMatrixWriter{}
	err := svc.saveSpecCyclePrices(context.Background(), 7, 3, map[string]float64{"fortnightly": 10}, 0)
	if err == nil || !strings.Contains(err.Error(), "不支持的计费周期") {
		t.Fatalf("未知周期应报错，实际：%v", err)
	}
}

// TestSaveSpecCyclePricesNoWriterIsNoop 未装配矩阵写入能力时静默跳过（价格能力未装配不该拖垮建品）。
func TestSaveSpecCyclePricesNoWriterIsNoop(t *testing.T) {
	svc := &productService{}
	if err := svc.saveSpecCyclePrices(context.Background(), 7, 3, map[string]float64{"monthly": 99}, 0); err != nil {
		t.Fatalf("未装配 writer 不该报错：%v", err)
	}
}

// --- D 组：压缩定价（T4.8：套餐价 + 逐取值加价）---

// testAxisPlan 造一个 cpu×memory 的计划，供压缩定价用例复用。
func testAxisPlan() *axisPlan {
	plan := &axisPlan{Axes: []axisOption{
		{Key: "cpu", Values: []string{"2", "4", "8"}},
		{Key: "memory", Values: []string{"2048", "4096", "8192"}},
	}}
	plan.Combinations = cartesian(plan.Axes)
	return plan
}

// TestAxisChargeComboAddsValueSurcharges 套餐价 + 各取值加价叠加，基线取值不加价。
// 这是压缩定价的核心口径：运营只填"4核 +10、8核 +20"，不再逐 SKU 填价。
func TestAxisChargeComboAddsValueSurcharges(t *testing.T) {
	plan := testAxisPlan()
	charges := &dto.ProductAxisChargePlan{
		BaseValues: map[string]string{"cpu": "2", "memory": "2048"},
		BasePrice:  100,
		BaseCost:   60,
		Values: map[string]map[string]float64{
			"cpu":    {"4": 10, "8": 20},
			"memory": {"4096": 10, "8192": 20},
		},
		CostValues: map[string]map[string]float64{
			"cpu":    {"4": 6, "8": 12},
			"memory": {"4096": 6, "8192": 12},
		},
	}
	want := map[string][2]float64{
		"2|2048": {100, 60},
		"4|2048": {110, 66},
		"8|2048": {120, 72},
		"2|4096": {110, 66},
		"4|4096": {120, 72},
		"8|4096": {130, 78},
		"2|8192": {120, 72},
		"4|8192": {130, 78},
		"8|8192": {140, 84},
	}
	for _, combo := range plan.Combinations {
		key := combo["cpu"] + "|" + combo["memory"]
		price, cost, _, err := axisChargeCombo(combo, charges, plan, nil)
		if err != nil {
			t.Fatalf("%s 展开失败：%v", key, err)
		}
		expect, ok := want[key]
		if !ok {
			t.Fatalf("出现了预期外的组合 %s", key)
		}
		if price != expect[0] || cost != expect[1] {
			t.Fatalf("%s 应为 价%v/成本%v，实际 价%v/成本%v", key, expect[0], expect[1], price, cost)
		}
	}
}

// TestAxisChargeComboBaselineFromTemplateDefault 方案未声明基线时用模板默认值。
// 模板里"默认选中 4核"才是运营心里的基线，比"取值列表第一个"更可靠。
func TestAxisChargeComboBaselineFromTemplateDefault(t *testing.T) {
	plan := testAxisPlan()
	charges := &dto.ProductAxisChargePlan{
		BasePrice: 100,
		Values:    map[string]map[string]float64{"cpu": {"2": 5, "8": 20}, "memory": {"2048": 5}},
	}
	defaults := map[string]string{"cpu": "4", "memory": "4096"}
	// 全取默认值的组合 = 套餐价（基线不加价）。
	price, _, _, err := axisChargeCombo(map[string]string{"cpu": "4", "memory": "4096"}, charges, plan, defaults)
	if err != nil || price != 100 {
		t.Fatalf("默认取值应为基线、价 = 套餐价 100，实际 %v（err=%v）", price, err)
	}
	// 换成非默认取值就要叠加上去。
	price, _, _, _ = axisChargeCombo(map[string]string{"cpu": "8", "memory": "2048"}, charges, plan, defaults)
	if price != 125 {
		t.Fatalf("8核2G 应为 100+20+5=125，实际 %v", price)
	}
}

// TestAxisChargeComboIgnoresBaselineSurcharge 基线取值即便在 Values 里填了加价也不生效。
// 基线已含在套餐价里，再叠一次就是重复计费。
func TestAxisChargeComboIgnoresBaselineSurcharge(t *testing.T) {
	plan := testAxisPlan()
	charges := &dto.ProductAxisChargePlan{
		BaseValues: map[string]string{"cpu": "2", "memory": "2048"},
		BasePrice:  100,
		Values:     map[string]map[string]float64{"cpu": {"2": 999}},
	}
	price, _, _, err := axisChargeCombo(map[string]string{"cpu": "2", "memory": "2048"}, charges, plan, nil)
	if err != nil {
		t.Fatalf("展开失败：%v", err)
	}
	if price != 100 {
		t.Fatalf("基线取值不该叠加加价，应为 100，实际 %v", price)
	}
}

// TestAxisChargeComboRejectsInvalidBaseline 声明的基线必须是该轴真实取值。
// 声明一个不存在的值时，真正的基线会被当成"非基线"多收一份加价。
func TestAxisChargeComboRejectsInvalidBaseline(t *testing.T) {
	plan := testAxisPlan()
	charges := &dto.ProductAxisChargePlan{
		BaseValues: map[string]string{"cpu": "16"}, // 16 核不在取值里
		BasePrice:  100,
		Values:     map[string]map[string]float64{"cpu": {"2": 5}},
	}
	// 非法基线被忽略 → 回落到第一个取值（2核）作基线 → 2核不加价。
	price, _, _, err := axisChargeCombo(map[string]string{"cpu": "2", "memory": "2048"}, charges, plan, nil)
	if err != nil {
		t.Fatalf("展开失败：%v", err)
	}
	if price != 100 {
		t.Fatalf("非法基线应回落到首个取值，2核应为套餐价 100，实际 %v", price)
	}
}

// TestAxisChargeComboNegativeSurchargeRejected 负加价必须报错。
// 允许负值会让某些规格算出低于成本甚至负数的价，那属于"单独定价"的语义，不是加价。
func TestAxisChargeComboNegativeSurchargeRejected(t *testing.T) {
	plan := testAxisPlan()
	charges := &dto.ProductAxisChargePlan{
		BasePrice: 100,
		Values:    map[string]map[string]float64{"cpu": {"4": -10}},
	}
	if _, _, _, err := axisChargeCombo(map[string]string{"cpu": "4", "memory": "2048"}, charges, plan, nil); err == nil {
		t.Fatal("负加价应报错")
	}
}

// TestAxisChargeComboScalesSurchargeByCycleRatio 长周期加价按该周期的基准倍率放大。
//
// 套餐月付 100 / 年付 1000（相当于只收 10 个月），那么"4核 +10"在年付应算 +100，
// 而不是按月数 ×12 算成 +120 —— 否则优惠只在套餐上生效、加价反而多收，
// 高配年付会贵得不成比例。
func TestAxisChargeComboScalesSurchargeByCycleRatio(t *testing.T) {
	plan := testAxisPlan()
	charges := &dto.ProductAxisChargePlan{
		BaseValues: map[string]string{"cpu": "2", "memory": "2048"},
		BasePrice:  100,
		BaseCycles: map[string]float64{"monthly": 100, "quarterly": 300, "annually": 1000},
		Values:     map[string]map[string]float64{"cpu": {"4": 10}},
	}
	_, _, cycles, err := axisChargeCombo(map[string]string{"cpu": "4", "memory": "2048"}, charges, plan, nil)
	if err != nil {
		t.Fatalf("展开失败：%v", err)
	}
	want := map[string]float64{"monthly": 110, "quarterly": 330, "annually": 1100}
	for cycle, expect := range want {
		if got := cycles[cycle]; got != expect {
			t.Fatalf("%s 应为 %v（加价按基准倍率放大），实际 %v", cycle, expect, got)
		}
	}
}

// TestAxisChargeComboFallsBackToMonthsWithoutBaseCycles 套餐没给逐周期价时按月数放大。
func TestAxisChargeComboFallsBackToMonthsWithoutBaseCycles(t *testing.T) {
	plan := testAxisPlan()
	charges := &dto.ProductAxisChargePlan{
		BasePrice:  100,
		BaseCycles: map[string]float64{"annually": 1200},
		Values:     map[string]map[string]float64{"cpu": {"4": 10}},
	}
	_, _, cycles, err := axisChargeCombo(map[string]string{"cpu": "4", "memory": "2048"}, charges, plan, nil)
	if err != nil {
		t.Fatalf("展开失败：%v", err)
	}
	// 年付基准 1200/100 = 12 倍 → 加价 +120。
	if got := cycles["annually"]; got != 1320 {
		t.Fatalf("年付应为 1200+120=1320，实际 %v", got)
	}
}

// TestAxisChargeComboNoCyclesWhenPlanOmitsThem 方案没给逐周期价时不凭空造周期行。
//
// 只填月付的商品不该被写出季付价 —— 那会让运营以为季付已定价。
// 长周期的补满由 expandCyclesForPlan 按可售周期列表负责。
func TestAxisChargeComboNoCyclesWhenPlanOmitsThem(t *testing.T) {
	plan := testAxisPlan()
	charges := &dto.ProductAxisChargePlan{
		BasePrice: 100,
		BaseCost:  60,
		Values:    map[string]map[string]float64{"cpu": {"4": 10}},
	}
	_, _, cycles, err := axisChargeCombo(map[string]string{"cpu": "4", "memory": "2048"}, charges, plan, nil)
	if err != nil {
		t.Fatalf("展开失败：%v", err)
	}
	if len(cycles) != 0 {
		t.Fatalf("方案未定周期价时不该产出周期行，实际 %v", cycles)
	}
}

// TestExpandCyclesForPlanFillsEverySellableCycle 可售周期必须全部补满。
// 少写一个周期，客户选它时会被硬拒（某规格有任意一行即为其全部周期的权威）。
func TestExpandCyclesForPlanFillsEverySellableCycle(t *testing.T) {
	sellable := []string{"monthly", "quarterly", "annually"}
	// 已知周期价优先，未给的按月数推导。
	got := expandCyclesForPlan(100, sellable, map[string]float64{"annually": 1000})
	if got["monthly"] != 100 {
		t.Fatalf("月付应为 100，实际 %v", got["monthly"])
	}
	if got["quarterly"] != 300 {
		t.Fatalf("季付未给价时应按 3 个月推导为 300，实际 %v", got["quarterly"])
	}
	if got["annually"] != 1000 {
		t.Fatalf("已给的年付价应原样保留 1000，实际 %v", got["annually"])
	}
}

// TestExpandCyclesForPlanSkipsUnknownCycles 无法识别的周期名不产出（不静默塞一个月付价）。
func TestExpandCyclesForPlanSkipsUnknownCycles(t *testing.T) {
	got := expandCyclesForPlan(100, []string{"monthly", "fortnightly"}, nil)
	if _, ok := got["fortnightly"]; ok {
		t.Fatal("未知周期不该被推导出价")
	}
	if got["monthly"] != 100 {
		t.Fatalf("月付应为 100，实际 %v", got["monthly"])
	}
}

// TestExpandCyclesForPlanNoMonthlyNoExpansion 没有月付价时不做推导（无基准可依）。
func TestExpandCyclesForPlanNoMonthlyNoExpansion(t *testing.T) {
	known := map[string]float64{"monthly": 0}
	got := expandCyclesForPlan(0, []string{"monthly", "annually"}, known)
	if len(got) != len(known) {
		t.Fatalf("无月付价时不该扩展，实际 %v", got)
	}
}

// TestNormalizeCycleListDedupesAndNormalizes 周期列表归一化（别名 month → monthly）。
func TestNormalizeCycleListDedupesAndNormalizes(t *testing.T) {
	got := normalizeCycleList([]string{"month", "monthly", "year", "", "  "})
	want := []string{"monthly", "annually"}
	if len(got) != len(want) {
		t.Fatalf("应为 %v，实际 %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("应为 %v，实际 %v", want, got)
		}
	}
}
