package mofangfinance

import (
	"encoding/json"
	"reflect"
	"testing"
)

// configGroups308 收录上游 宁波电信 16-16 30Mbps（upstream_pid=308）的 config_groups 片段，
// 用于验证 buildCartConfigOption 能依据 config_groups 还原 configoption[option_upstream_id] = sub_upstream_id。
const configGroups308 = `[
  {"id":26945,"name":"宁波电信高防一区 16H16G配置项组","options":[
    {"id":159897,"option_name":"area|区域","option_type":12,"upstream_id":710547,"sub":[
      {"id":321169,"option_name":"2|浙江^宁波","upstream_id":4387466,"hidden":0}]},
    {"id":159898,"option_name":"os|操作系统","option_type":5,"upstream_id":710548,"sub":[
      {"id":321170,"option_name":"12|CentOS^CentOS-7.6.1810-x64","upstream_id":4387467,"hidden":0},
      {"id":321171,"option_name":"14|Debian^Debian-9.12.1-x64","upstream_id":4387468,"hidden":0}]},
    {"id":159899,"option_name":"cpu|CPU","option_type":6,"upstream_id":710549,"sub":[
      {"id":321187,"option_name":"16|16核","upstream_id":4387511,"hidden":0}]},
    {"id":159900,"option_name":"memory|内存","option_type":8,"upstream_id":710550,"sub":[
      {"id":321188,"option_name":"16384|16G","upstream_id":4387516,"hidden":0}]},
    {"id":159901,"option_name":"bw|带宽","option_type":11,"upstream_id":710551,"sub":[
      {"id":321189,"option_name":"30Mbps","upstream_id":4387526,"hidden":0}]}
  ]}
]`

func TestBuildCartConfigOptionFromGroups(t *testing.T) {
	var groups []ConfigGroup
	if err := json.Unmarshal([]byte(configGroups308), &groups); err != nil {
		t.Fatalf("unmarshal config groups failed: %v", err)
	}
	extra := map[string]interface{}{"config_groups": groups}
	got := buildCartConfigOption(extra)
	want := map[string]string{
		"710547": "4387466",
		"710548": "4387467", // os 多值：未指定 os 时取首个可见项（CentOS）
		"710549": "4387511",
		"710550": "4387516",
		"710551": "4387526",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("configoption mismatch:\n got=%v\nwant=%v", got, want)
	}
}

func TestBuildCartConfigOptionOsMatch(t *testing.T) {
	var groups []ConfigGroup
	if err := json.Unmarshal([]byte(configGroups308), &groups); err != nil {
		t.Fatalf("unmarshal config groups failed: %v", err)
	}
	extra := map[string]interface{}{
		"config_groups": groups,
		"os":            "debian",
	}
	got := buildCartConfigOption(extra)
	if got["710548"] != "4387468" {
		t.Fatalf("expected os=debian -> sub 4387468, got %q", got["710548"])
	}
}

func TestSafeHostname(t *testing.T) {
	cases := map[string]string{
		"宁波电信|一区·标准 16-32 30Mbps": "hs-16-32-30mbps",
		"hs-probe-123":                "hs-probe-123",
		"":                            "hs-",
	}
	for in, expected := range cases {
		got := safeHostname(in)
		if expected == "hs-" {
			if got == "" || got[:3] != "hs-" {
				t.Fatalf("safeHostname(%q) = %q, want hs- prefix", in, got)
			}
			continue
		}
		if got != expected {
			t.Fatalf("safeHostname(%q) = %q, want %q", in, got, expected)
		}
	}
}

func TestNormalizeBillingCycle(t *testing.T) {
	cases := map[string]string{
		"monthly": "monthly", "quarterly": "quarterly",
		"semiannually": "semiannually", "annually": "annually",
		"fixed": "onetime", "hourly": "hour", "": "monthly",
	}
	for in, want := range cases {
		if got := normalizeBillingCycle(in); got != want {
			t.Fatalf("normalizeBillingCycle(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestHostHeaderAssignedIPs 上游 host/header 的 assignedips 可能是数组或字符串，需兼容解析。
func TestHostHeaderAssignedIPs(t *testing.T) {
	// 数组形态（本账户首个 ip 可能命中）
	var a HostHeader
	if err := json.Unmarshal([]byte(`{"host_data":{"id":1934,"assignedips":[]}}`), &a); err != nil {
		t.Fatalf("assignedips array unmarshal failed: %v", err)
	}
	if a.HostData.AssignedIPs != "" {
		t.Fatalf("empty array should yield empty string, got %q", a.HostData.AssignedIPs)
	}
	// 字符串形态
	var b HostHeader
	if err := json.Unmarshal([]byte(`{"host_data":{"id":1935,"assignedips":"1.2.3.4,5.6.7.8"}}`), &b); err != nil {
		t.Fatalf("assignedips string unmarshal failed: %v", err)
	}
	if b.HostData.AssignedIPs != "1.2.3.4,5.6.7.8" {
		t.Fatalf("string assignedips mismatch, got %q", b.HostData.AssignedIPs)
	}
}
