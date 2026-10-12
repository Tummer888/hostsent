package zjmf

import (
	"encoding/json"
	"reflect"
	"testing"
)

// configGroups308 收录上游 宁波电信 16-16 30Mbps（upstream_pid=308）的 config_groups 片段，
// 用于验证 buildCartConfigOption 能依据 config_groups 还原
// configoption[option.id] = sub.id（上游自身 id，不是 upstream_id）。
// 依据：上游 common/logic/ConfigOptions.php::filterConfigOptions 只按 option 自身 id
// 匹配请求键，取值必须是该 option 下的 sub.id；误用 upstream_id 会被过滤掉并按上游默认值开通。
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
		"159897": "321169",
		"159898": "321170", // os 多值：未指定 os 时取首个可见项（CentOS）
		"159899": "321187",
		"159900": "321188",
		"159901": "321189",
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
	if got["159898"] != "321171" {
		t.Fatalf("expected os=debian -> sub 321171, got %q", got["159898"])
	}
}

// TestBuildCartConfigOptionNativeProduct 覆盖上游原生商品（upstream_id 全 0）：
// 键取 option.id / sub.id，历史误用 upstream_id 时这类商品会整个丢参。
func TestBuildCartConfigOptionNativeProduct(t *testing.T) {
	// 实测形态：上游 新人特惠·香港 4-4（pid=552）的 cpu 配置项。
	fixture := `[{"id":27650,"name":"配置项组","options":[
	  {"id":164026,"option_name":"cpu|CPU","option_type":6,"upstream_id":0,"sub":[
	    {"id":566823,"option_name":"4|4核","upstream_id":0,"hidden":0}]}
	]}]`
	var groups []ConfigGroup
	if err := json.Unmarshal([]byte(fixture), &groups); err != nil {
		t.Fatalf("unmarshal config groups failed: %v", err)
	}
	got := buildCartConfigOption(map[string]interface{}{"config_groups": groups})
	if got["164026"] != "566823" {
		t.Fatalf("expected 164026 -> 566823, got %v", got)
	}
}

func TestSafeHostname(t *testing.T) {
	cases := map[string]string{
		"宁波电信|一区·标准 16-32 30Mbps": "hs-16-32-30mbps",
		"hs-probe-123": "hs-probe-123",
		"":             "hs-",
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
