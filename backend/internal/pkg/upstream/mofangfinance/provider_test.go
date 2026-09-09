package mofangfinance

import "testing"

// TestParseCloudOptions 验证从 openapi 配置项（平铺 configoptions[]，含中文说明键）提取标准规格。
func TestParseCloudOptions(t *testing.T) {
	opts := []ConfigOption{
		{OptionName: "cpu", Sub: []ConfigSub{{OptionName: "4|4核"}}},
		{OptionName: "内存", Sub: []ConfigSub{{OptionName: "16G|16GB"}}},
		{OptionName: "操作系统", Sub: []ConfigSub{{OptionName: "centos7^CentOS 7"}}},
		{OptionName: "系统盘", Sub: []ConfigSub{{OptionName: "100|100G"}}},
		{OptionName: "带宽", Sub: []ConfigSub{{OptionName: "20|20M"}}},
		{OptionName: "数据中心", Sub: []ConfigSub{{OptionName: "香港"}}},
		{OptionName: "节点", Sub: []ConfigSub{{OptionName: "一区"}}},
	}
	spec := parseCloudOptions(opts)
	if spec.CPU != 4 {
		t.Errorf("cpu = %d, want 4", spec.CPU)
	}
	if spec.Memory != 16384 {
		t.Errorf("memory = %d, want 16384 (16G), got %d", spec.Memory, spec.Memory)
	}
	if spec.Disk != 100 {
		t.Errorf("disk = %d, want 100", spec.Disk)
	}
	if spec.OS != "centos7^CentOS 7" {
		t.Errorf("os = %q, want centos7^CentOS 7", spec.OS)
	}
	if spec.Bandwidth != 20 {
		t.Errorf("bandwidth = %d, want 20", spec.Bandwidth)
	}
	if spec.Region != "香港" {
		t.Errorf("region = %q, want 香港", spec.Region)
	}
	if spec.Zone != "一区" {
		t.Errorf("zone = %q, want 一区", spec.Zone)
	}
}

// TestMemoryToMB 验证内存单位 G→MB 换算。
func TestMemoryToMB(t *testing.T) {
	cases := map[string]int{
		"16G":   16384,
		"16GB":  16384,
		"4G":    4096,
		"16384": 16384,
		"8192":  8192,
		"":      0,
	}
	for in, want := range cases {
		if got := memoryToMB(in); got != want {
			t.Errorf("memoryToMB(%q) = %d, want %d", in, got, want)
		}
	}
}

// TestCyclePrice 验证从商品 cycle 计费周期取上游售价（product_price>0）。
func TestCyclePrice(t *testing.T) {
	cycle := []map[string]interface{}{
		{"billingcycle": "monthly", "product_price": 88.5, "setup_fee": 0},
		{"billingcycle": "annually", "product_price": 850.0, "setup_fee": 0},
	}
	price, _, bc, _ := cyclePrice(cycle)
	if price != 88.5 {
		t.Errorf("cyclePrice = %v, want 88.5", price)
	}
	if bc != "monthly" {
		t.Errorf("billingcycle = %q, want monthly", bc)
	}
	// 价格为 0 时应回退（取第一个周期）
	cycle2 := []map[string]interface{}{{"billingcycle": "onetime", "product_price": 0}}
	p2, _, _, _ := cyclePrice(cycle2)
	if p2 != 0 {
		t.Errorf("zero cycle price = %v, want 0", p2)
	}
}

// TestMatchKey 验证中文/英文配置键匹配。
func TestMatchKey(t *testing.T) {
	if !matchKey(normalizeOptionKey("内存"), "memory", "mem", "内存") {
		t.Errorf("内存 应匹配内存键")
	}
	if !matchKey(normalizeOptionKey("系统盘"), "system_disk_size", "系统盘") {
		t.Errorf("系统盘 应匹配系统盘键")
	}
	if !matchKey(normalizeOptionKey("CPU"), "cpu", "核") {
		t.Errorf("CPU 应匹配 cpu 键")
	}
	if matchKey(normalizeOptionKey("IP数量"), "cpu", "核") {
		t.Errorf("IP数量 不应匹配 cpu 键")
	}
}

// TestLeadingInt 验证前导数字解析（处理 "2 核"/"30,1"/"8192" 等）。
func TestLeadingInt(t *testing.T) {
	cases := map[string]int{
		"4":    4,
		"2 核":  2,
		"30,1": 30,
		"8192": 8192,
		"100M": 100,
		"abc":  0,
		"":     0,
	}
	for in, want := range cases {
		if got := leadingInt(in); got != want {
			t.Errorf("leadingInt(%q) = %d, want %d", in, got, want)
		}
	}
}
