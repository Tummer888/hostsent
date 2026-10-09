package service

import "testing"

// 建品页就地改配置时，SKU 编码必须跟着实际取值走，否则编码与配置自相矛盾。
func TestDeriveSpecCodeFromValues(t *testing.T) {
	cases := []struct {
		name     string
		family   string
		values   string
		fallback string
		want     string
	}{
		{"按实际取值重算", "general", `{"compute.cpu":4,"compute.memory":8192,"storage.system.size":60}`, "general-2c4g-30g", "general-4c8g-60g"},
		{"无磁盘时省略", "compute", `{"compute.cpu":2,"compute.memory":2048}`, "compute-2c2g-40g", "compute-2c2g"},
		{"取值非数字回落", "general", `{"compute.cpu":"x","compute.memory":8192}`, "general-2c4g-30g", "general-2c4g-30g"},
		{"非法 JSON 回落", "general", `not-json`, "general-2c4g-30g", "general-2c4g-30g"},
		{"缺内存回落", "general", `{"compute.cpu":4}`, "general-2c4g-30g", "general-2c4g-30g"},
	}
	for _, c := range cases {
		if got := deriveSpecCodeFromValues(c.family, c.values, c.fallback); got != c.want {
			t.Errorf("%s: deriveSpecCodeFromValues = %q, want %q", c.name, got, c.want)
		}
	}
}
