package specatom

import (
	"encoding/json"
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
