package service

// SPU/SKU 正交轴的单测（T4.7）。
//
// 这一批测试守的是三个已发生的故障：
//  1. 组合数爆炸 —— 区间型参数被当成轴，带宽 10~100 每兆一个 SKU；
//  2. SKU 重复 —— 编码带时间戳后缀，重跑一次多一批同名规格；
//  3. 编码与配置不符 —— 商品名写 6G、开通下发 4G（基线被配置项默认值覆盖）。

import (
	"encoding/json"
	"strings"
	"testing"

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
