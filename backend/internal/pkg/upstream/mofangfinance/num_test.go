package mofangfinance

import "testing"

// TestNumAcceptsIntegerTypes 回归：host/header 的 nextduedate 经 HostHeader 解析后是 int64，
// num 若不认整型会让链路 A 续费拿不到上游权威账期、静默回退本地顺延（T5.2 语义破坏）。
func TestNumAcceptsIntegerTypes(t *testing.T) {
	cases := []struct {
		name string
		in   interface{}
		want float64
	}{
		{"int", int(1798761600), 1798761600},
		{"int32", int32(1798761600), 1798761600},
		{"int64", int64(1798761600), 1798761600},
		{"uint", uint(1798761600), 1798761600},
		{"uint64", uint64(1798761600), 1798761600},
		{"float64", float64(1.5), 1.5},
		{"string", "1798761600", 1798761600},
	}
	for _, c := range cases {
		got, ok := num(c.in)
		if !ok || got != c.want {
			t.Errorf("num(%s) = (%v, %v), want (%v, true)", c.name, got, ok, c.want)
		}
	}
	if _, ok := num(struct{}{}); ok {
		t.Error("num(struct{}) should not be ok")
	}
}
