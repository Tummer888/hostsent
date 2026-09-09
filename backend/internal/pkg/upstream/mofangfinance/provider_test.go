package mofangfinance

import "testing"

// TestParseCloudSpecs 验证从 prodetail 嵌套 config_groups 提取标准规格。
func TestParseCloudSpecs(t *testing.T) {
	groups := []ConfigGroup{
		{
			Name: "CPU/内存",
			Options: []ConfigOption{
				{OptionName: "cpu", Sub: []ConfigSub{{OptionName: "4|4核"}}},
				{OptionName: "memory|内存", Sub: []ConfigSub{{OptionName: "8192|8G"}}},
			},
		},
		{
			Name: "系统/网络",
			Options: []ConfigOption{
				{OptionName: "os|操作系统", Sub: []ConfigSub{{OptionName: "12|CentOS^CentOS-7.6"}}},
				{OptionName: "system_disk_size|系统盘", Sub: []ConfigSub{{OptionName: "100|100G"}}},
				{OptionName: "bw|带宽", Sub: []ConfigSub{{OptionName: "20|20M"}}},
				{OptionName: "area|区域", Sub: []ConfigSub{{OptionName: "香港"}}},
			},
		},
	}
	spec := parseCloudSpecs(groups)
	if spec.CPU != 4 {
		t.Errorf("cpu = %d, want 4", spec.CPU)
	}
	if spec.Memory != 8192 {
		t.Errorf("memory = %d, want 8192", spec.Memory)
	}
	if spec.Disk != 100 {
		t.Errorf("disk = %d, want 100", spec.Disk)
	}
	if spec.OS != "CentOS" {
		t.Errorf("os = %q, want CentOS", spec.OS)
	}
	if spec.Bandwidth != 20 {
		t.Errorf("bandwidth = %d, want 20", spec.Bandwidth)
	}
	if spec.Region != "香港" {
		t.Errorf("region = %q, want 香港", spec.Region)
	}
}

// TestMemoryToMB 验证内存：上游 "8192|8G" 数字即 MB，不做 GB 换算。
func TestMemoryToMB(t *testing.T) {
	cases := map[string]int{
		"8192|8G": 8192,
		"16384":   16384,
		"4G":      4,
		"":        0,
	}
	for in, want := range cases {
		if got := memoryToMB(in); got != want {
			t.Errorf("memoryToMB(%q) = %d, want %d", in, got, want)
		}
	}
}

// TestNormalizeOS 验证操作系统归一化。
func TestNormalizeOS(t *testing.T) {
	cases := map[string]string{
		"12|CentOS^CentOS-7.6": "CentOS",
		"CentOS":               "CentOS",
		"ubuntu|Ubuntu":        "Ubuntu",
		"":                     "",
	}
	for in, want := range cases {
		if got := normalizeOS(in); got != want {
			t.Errorf("normalizeOS(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestLeadingInt 验证前导数字解析。
func TestLeadingInt(t *testing.T) {
	cases := map[string]int{"4": 4, "2 核": 2, "30,1": 30, "8192": 8192, "abc": 0, "": 0}
	for in, want := range cases {
		if got := leadingInt(in); got != want {
			t.Errorf("leadingInt(%q) = %d, want %d", in, got, want)
		}
	}
}
