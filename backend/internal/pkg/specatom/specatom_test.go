package specatom

import (
	"encoding/json"
	"fmt"
	"testing"

	"hostsent/backend/internal/pkg/model"
)

func newTestDict(t *testing.T) *Dictionary {
	t.Helper()
	d := NewDictionary()
	d.Replace([]Atom{
		{Key: KeyCPU, Name: "CPU", ValueType: "int", MinValue: ptr(1.0), Status: 1},
		{Key: KeyMemory, Name: "内存", ValueType: "int", MinValue: ptr(128.0), Status: 1},
		{Key: KeyDisk, Name: "系统盘", ValueType: "int", MinValue: ptr(20.0), Status: 1},
		{Key: KeyDiskType, Name: "系统盘类型", ValueType: "enum",
			EnumValues: json.RawMessage(`["ssd","hdd"]`), Status: 1},
		{Key: "network.traffic", Name: "流量", ValueType: "int", Status: 1},
		{Key: "compute.custom", Name: "自定义", ValueType: "string", Status: 1},
	})
	return d
}

func ptr(v float64) *float64 { return &v }

// 字号为空字典时完全不阻断（P2 预埋期策略）。
func TestValidateSkipsWhenDictionaryEmpty(t *testing.T) {
	d := NewDictionary()
	if issues := d.Validate(model.StandardProductSpec{CPU: -5}); issues != nil {
		t.Fatalf("empty dictionary should not validate, got %v", issues)
	}
}

func TestValidateFixedFields(t *testing.T) {
	d := newTestDict(t)
	issues := d.Validate(model.StandardProductSpec{CPU: 2, Memory: 4096, Disk: 50, DiskType: "ssd"})
	if len(issues) != 0 {
		t.Fatalf("valid spec rejected: %v", issues)
	}

	issues = d.Validate(model.StandardProductSpec{CPU: 0, Memory: 64, Disk: 10, DiskType: "nvme"})
	if len(issues) != 3 {
		t.Fatalf("expected 3 issues (below min x2 + enum), got %d: %v", len(issues), issues)
	}
}

// 零值视为"未提供"，不判 required——保护存量只填部分字段的数据。
func TestValidateSkipsZeroValues(t *testing.T) {
	d := newTestDict(t)
	if issues := d.Validate(model.StandardProductSpec{}); len(issues) != 0 {
		t.Fatalf("zero values should be skipped, got %v", issues)
	}
}

func TestValidateExtra(t *testing.T) {
	d := newTestDict(t)
	// 未登记 key 报错；已登记 key 按类型校验；平台专有 key 放行。
	issues := d.Validate(model.StandardProductSpec{Extra: map[string]interface{}{
		"unknown.key":     1,
		"network.traffic": "abc",
		"upstream_pid":    "123",
	}})
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d: %v", len(issues), issues)
	}
	keys := map[string]bool{}
	for _, i := range issues {
		keys[i.Key] = true
	}
	if !keys["unknown.key"] || !keys["network.traffic"] {
		t.Fatalf("issue keys mismatch: %v", issues)
	}
}

func TestValidateExtraRegisteredAtom(t *testing.T) {
	d := newTestDict(t)
	issues := d.Validate(model.StandardProductSpec{Extra: map[string]interface{}{"compute.custom": "ok"}})
	if len(issues) != 0 {
		t.Fatalf("registered string atom rejected: %v", issues)
	}
}

func TestReplaceSkipsDisabledAtoms(t *testing.T) {
	d := NewDictionary()
	d.Replace([]Atom{
		{Key: KeyCPU, ValueType: "int", Status: 0},
		{Key: KeyMemory, ValueType: "int", Status: 1},
	})
	if _, ok := d.Get(KeyCPU); ok {
		t.Fatal("disabled atom should not be loaded")
	}
	if d.Len() != 1 {
		t.Fatalf("expected 1 atom, got %d", d.Len())
	}
}

func TestToFloatVariants(t *testing.T) {
	cases := []struct {
		in   any
		want float64
		ok   bool
	}{
		{2, 2, true},
		{int64(3), 3, true},
		{4.5, 4.5, true},
		{json.Number("6"), 6, true},
		{"7", 7, true},
		{"abc", 0, false},
		{nil, 0, false},
	}
	for _, c := range cases {
		got, ok := toFloat(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Fatalf("toFloat(%v) = %v, %v; want %v, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

// TestFromMapToMapRoundTrip 原子取值映射与标准规格之间可往返。
func TestFromMapToMapRoundTrip(t *testing.T) {
	in := map[string]any{
		"compute.cpu":         float64(2),
		"compute.memory":      4096,
		"storage.system.size": 50,
		"placement.region":    "hk",
		"network.traffic":     100,
	}
	spec := FromMap(in)
	if spec.CPU != 2 || spec.Memory != 4096 || spec.Disk != 50 || spec.Region != "hk" {
		t.Fatalf("FromMap 固定字段解析错误: %+v", spec)
	}
	if spec.Extra["network.traffic"] != 100 {
		t.Fatalf("字典外原子应落入 Extra: %+v", spec.Extra)
	}
	out := ToMap(spec)
	for k, want := range in {
		if got, ok := out[k]; !ok || fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("ToMap[%s] = %v, want %v", k, got, want)
		}
	}
}

// TestToMapSkipsZeroValues 零值（未提供）不写入规格映射。
func TestToMapSkipsZeroValues(t *testing.T) {
	out := ToMap(model.StandardProductSpec{CPU: 1, Disk: 0, OS: ""})
	if _, ok := out[KeyDisk]; ok {
		t.Error("零值 disk 不应写入")
	}
	if _, ok := out[KeyOS]; ok {
		t.Error("空 os 不应写入")
	}
	if out[KeyCPU] != 1 {
		t.Errorf("cpu = %v, want 1", out[KeyCPU])
	}
}

// TestWriteParamsTranslatesByPlatform 按平台字段字典把原子翻译为写参数。
func TestWriteParamsTranslatesByPlatform(t *testing.T) {
	d := NewDictionary()
	d.Replace([]Atom{
		{Key: KeyCPU, ValueType: "int", Status: 1,
			PlatformFields: map[string]AtomPlatformField{"mofangyun": {Read: "cpu_num", Write: "cpu"}}},
		{Key: KeyMemory, ValueType: "int", Status: 1,
			PlatformFields: map[string]AtomPlatformField{"mofangyun": {Read: "memory_size", Write: "memory"}}},
		{Key: KeyRegion, ValueType: "string", Status: 1}, // 无平台字段：应被跳过
	})
	out := d.WriteParams("mofangyun", model.StandardProductSpec{CPU: 2, Memory: 8192, Region: "hk"})
	if out["cpu"] != 2 || out["memory"] != 8192 {
		t.Fatalf("平台写字段翻译错误: %v", out)
	}
	if len(out) != 2 {
		t.Fatalf("未登记写字段的原子应跳过，got %v", out)
	}
	if got := d.WriteParams("kvm", model.StandardProductSpec{CPU: 2}); len(got) != 0 {
		t.Fatalf("未接入平台应返回空映射，got %v", got)
	}
}

// TestRequiredMissingFor 上架门禁（T4.6）：按链路与平台过滤必填原子。
// billing.cycle 这类由订单账期决定、平台写侧为 "-" 的原子不得误伤 SKU 完整性校验。
func TestRequiredMissingFor(t *testing.T) {
	d := NewDictionary()
	d.Replace([]Atom{
		{Key: KeyCPU, ValueType: "int", Required: true, AppliesTo: "both", Status: 1,
			PlatformFields: map[string]AtomPlatformField{"mofangyun": {Write: "cpu"}}},
		{Key: KeyRegion, ValueType: "string", Required: true, AppliesTo: "both", Status: 1,
			PlatformFields: map[string]AtomPlatformField{"mofangyun": {Write: "area"}}},
		{Key: "billing.cycle", ValueType: "enum", Required: true, AppliesTo: "both", Status: 1,
			PlatformFields: map[string]AtomPlatformField{"mofangyun": {Write: "-"}}},
		{Key: "image.id", ValueType: "string", Required: true, AppliesTo: "upstream", Status: 1,
			PlatformFields: map[string]AtomPlatformField{"mofangyun": {Write: "os"}}},
	})

	// 平台过滤：billing.cycle 写侧为 "-" 不算缺失；region 缺失被报出。
	issues := d.RequiredMissingFor(map[string]any{KeyCPU: 2}, "self", "mofangyun")
	keys := map[string]bool{}
	for _, is := range issues {
		keys[is.Key] = true
	}
	if keys["billing.cycle"] {
		t.Errorf("写侧为 '-' 的原子不应参与平台必填校验: %v", issues)
	}
	if !keys[KeyRegion] {
		t.Errorf("缺失的 region 应被报出: %v", issues)
	}
	// 链路过滤：image.id 仅适用于 upstream，self 链路不校验。
	if keys["image.id"] {
		t.Errorf("applies_to=upstream 的原子不应约束 self 链路: %v", issues)
	}
	// 全量回退（未指定平台）时 billing.cycle 参与校验。
	all := d.RequiredMissingFor(map[string]any{KeyCPU: 2, KeyRegion: "hk"}, "self", "")
	found := false
	for _, is := range all {
		if is.Key == "billing.cycle" {
			found = true
		}
	}
	if !found {
		t.Errorf("未指定平台时应校验全部必填原子: %v", all)
	}
}
