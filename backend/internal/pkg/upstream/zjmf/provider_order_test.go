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
		// 日付：此前落到 default→monthly，会让本地按天推进账期而上游按月开单。
		"daily": "day", "day": "day",
		// 两年/三年与别名写法，覆盖全部年份档位。
		"biennially": "biennially", "triennially": "triennially",
		"yearly": "annually", "annual": "annually",
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

// TestHostHeaderFlexibleNumbers 回归（实测故障：「开通后点同步刷新」必报
// `解析上游响应 data 失败: json: cannot unmarshal number into Go struct field
// .host_data.port of type string`）：
//
// 上游 host.port 是 int(11)，空值发 0/null、非空发数字；upstream_cost 同样可能是
// 数字或字符串。任一个字段类型不符都会让**整段 host_data 解析失败**，
// 于是回源刷新报错、开通时的详情拉取也会静默退化（只剩最小实例）。
// 这里把三种下发形态固定为回归：数字、字符串、null。
func TestHostHeaderFlexibleNumbers(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int64
	}{
		{"端口为数字（上游常态）", `{"host_data":{"id":1934,"port":22,"upstream_cost":12.5}}`, 22},
		{"端口为字符串（个别版本）", `{"host_data":{"id":1935,"port":"2222","upstream_cost":"8"}}`, 2222},
		{"端口为 null/0（未开放）", `{"host_data":{"id":1936,"port":null,"upstream_cost":null}}`, 0},
		{"端口为 0", `{"host_data":{"id":1937,"port":0}}`, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var h HostHeader
			if err := json.Unmarshal([]byte(c.body), &h); err != nil {
				t.Fatalf("host_data 解析失败（一个字段类型不符就整段报废）: %v", err)
			}
			if got := h.HostData.Port.Int(); got != c.want {
				t.Fatalf("port = %d, want %d", got, c.want)
			}
			if got := h.HostData.UpstreamCost; c.want == 22 && float64(got) != 12.5 {
				t.Fatalf("upstream_cost = %v, want 12.5", got)
			}
			if got := h.HostData.UpstreamCost; c.want == 2222 && float64(got) != 8 {
				t.Fatalf("upstream_cost = %v, want 8（字符串形态）", got)
			}
		})
	}

	// 端口归一：0/非法一律不落库为端口 0（详情页显示「—」而不是「0」）。
	if got := portString(FlexNumber(0)); got != "" {
		t.Fatalf("portString(0) = %q, want 空串", got)
	}
	if got := portString(FlexNumber(22)); got != "22" {
		t.Fatalf("portString(22) = %q, want \"22\"", got)
	}
}

// TestSafeHostnamePadsToMinimum 短商品名必须补足到上游要求的最小长度（10 位）。
//
// 上游按「主机名长度至少10位」校验；商品名短（如「4核4G」规范化后只有 4 位）时
// 若不补齐，账单已建、款已扣才被拒，整笔开通只能人工重试。
func TestSafeHostnamePadsToMinimum(t *testing.T) {
	const minLen = 10
	for _, in := range []string{"4核4G", "hk", "hs-1", "香港|六区·轻量 4-4 15Mbps"} {
		got := safeHostname(in)
		if len(got) < minLen {
			t.Errorf("safeHostname(%q) = %q（%d 位），应至少 %d 位", in, got, len(got), minLen)
		}
		if got[0] < 'a' || got[0] > 'z' {
			t.Errorf("safeHostname(%q) = %q，应以字母开头", in, got)
		}
		if len(got) > 50 {
			t.Errorf("safeHostname(%q) = %q，超过 50 位", in, got)
		}
	}
}
