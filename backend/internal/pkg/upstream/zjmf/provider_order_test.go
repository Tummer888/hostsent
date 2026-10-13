package zjmf

import (
	"encoding/json"
	"reflect"
	"strings"
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

// TestBuildCartConfigOptionExplicitMergeNotReplace 显式 configoption 必须与配置组**合并**，
// 不能替换。
//
// 场景：客户只改了「操作系统」（显式表仅一项），其余 CPU/内存/区域等固定配置
// 只能由 config_groups 还原。若显式表一出现就直接返回，固定配置全部丢失，
// 上游会按自己的默认值开通 —— 卖的是 4核4G，开出来的可能完全不同。
func TestBuildCartConfigOptionExplicitMergeNotReplace(t *testing.T) {
	var groups []ConfigGroup
	if err := json.Unmarshal([]byte(configGroups308), &groups); err != nil {
		t.Fatalf("unmarshal config groups failed: %v", err)
	}
	extra := map[string]interface{}{
		"config_groups": groups,
		// 客户在界面上选了 Debian（上游选项 159898 → 取值 321171）
		"configoption": map[string]interface{}{"159898": "321171"},
	}
	got := buildCartConfigOption(extra)

	// 显式选择生效
	if got["159898"] != "321171" {
		t.Errorf("显式选择应覆盖配置组推导值: os = %q, want 321171", got["159898"])
	}
	// 其余固定配置仍在（不能被显式表挤掉）
	want := map[string]string{
		"159897": "321169", // area
		"159899": "321187", // cpu
		"159900": "321188", // memory
		"159901": "321189", // bw
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("固定配置项 %s 丢失或被改写: got %q, want %q（完整: %v）", k, got[k], v, got)
		}
	}
}

// TestBuildCartConfigOptionExplicitOnly 无配置组时退化为仅用显式表（不 panic、不丢参）。
func TestBuildCartConfigOptionExplicitOnly(t *testing.T) {
	got := buildCartConfigOption(map[string]interface{}{
		"configoption": map[string]interface{}{"159898": "321171"},
	})
	if got["159898"] != "321171" || len(got) != 1 {
		t.Fatalf("无配置组时应只用显式表: %v", got)
	}
}

// TestDefaultHostnameUniquePerOrder 同一商品两次开通的主机名必须不同。
//
// 上游（魔方财务）对重名主机不下单也不报错：新主机卡在 domainstatus=Pending、
// dcimid=0，账单已付却永远不开通。实测事故：订单 842/843 分别复用商品名
// hs-4-4-15mbps / hs-2-4-10mbps（与已存在主机同名），主机 2057/2059 卡 Pending
// 数十分钟；同批唯一命名的主机 2056/2058 一分钟内 Active。
func TestDefaultHostnameUniquePerOrder(t *testing.T) {
	const product = "香港|六区·轻量 4-4 15Mbps"
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		// 每次用不同 token 模拟不同订单。
		token := "0123456789abcdef0123456789abc" + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26))
		got := defaultHostname(product, token)
		if seen[got] {
			t.Fatalf("主机名重复: %q（第 %d 次）；重名会让上游把主机卡在 Pending", got, i+1)
		}
		seen[got] = true
	}
}

// TestDefaultHostnameStablePerOrder 同一订单（同 token）重试必须得到同一个主机名
// —— 重试在上游眼里要像"同一个开通请求"，主机名跳变会让上游再建一台。
func TestDefaultHostnameStablePerOrder(t *testing.T) {
	const product = "香港|六区·轻量 2-4 10Mbps"
	const token = "50bbd03014388dcdc51a4c4d06953ead"
	first := defaultHostname(product, token)
	for i := 0; i < 5; i++ {
		if got := defaultHostname(product, token); got != first {
			t.Fatalf("同一订单主机名不稳定: got %q, want %q", got, first)
		}
	}
}

// TestDefaultHostnameRules 默认主机名仍须满足上游约束（长度、字符集、字母开头）。
func TestDefaultHostnameRules(t *testing.T) {
	const minLen = 10
	cases := []struct{ name, token string }{
		{"香港|六区·轻量 4-4 15Mbps", "abcdef0123456789abcdef0123456789"},
		{"4核4G", "abcdef0123456789abcdef0123456789"},
		{"香港|六区·轻量 4-4 15Mbps", ""}, // 未装配令牌时退回随机后缀
		{"a", ""},
		{strings.Repeat("超长商品名称很长的名字", 8), "abcdef0123456789abcdef0123456789"},
	}
	for _, c := range cases {
		got := defaultHostname(c.name, c.token)
		if len(got) < minLen {
			t.Errorf("defaultHostname(%q,%q) = %q（%d 位），应至少 %d 位", c.name, c.token, got, len(got), minLen)
		}
		if len(got) > 50 {
			t.Errorf("defaultHostname(%q,%q) = %q（%d 位），超过 50 位", c.name, c.token, got, len(got))
		}
		if got[0] < 'a' || got[0] > 'z' {
			t.Errorf("defaultHostname(%q,%q) = %q，应以字母开头", c.name, c.token, got)
		}
		for _, r := range got {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
				t.Errorf("defaultHostname(%q,%q) = %q，含非法字符 %q", c.name, c.token, got, r)
			}
		}
	}
}

// TestParseCloudSpecsPrefersSaleSpecs 回归（实测：商品「4-4 15Mbps」被解析成 30Mbps）。
//
// 上游同时下发语义相近的配置项，而匹配用的是"包含"：
//   - `bw`（售卖带宽 15Mbps）与 `in_bw`（流入带宽 30Mbps）—— `in_bw` 含 `bw`；
//   - `system_disk_size`（系统盘 40G）与 `data_disk_size`（数据盘 200G）—— 后者含 `disk`。
//
// 旧实现按遍历顺序赋值，附属配置项会把真正的售卖规格覆盖掉（实测数据：
// upstream 528 的 bandwidth 被写成 30、disk 被数据盘覆盖）。
// 现改为"先精确匹配、再宽松兜底"且同字段只认首个命中，钉死这两个键的优先级。
func TestParseCloudSpecsPrefersSaleSpecs(t *testing.T) {
	// 依 upstream 528（香港|六区·轻量 4-4 15Mbps）的真实 config_groups 精简构造。
	groups := []ConfigGroup{{
		Options: []ConfigOption{
			{OptionName: "area|区域", Sub: []ConfigSub{{OptionName: "2|CN^中国香港"}}},
			{OptionName: "bw|带宽", Sub: []ConfigSub{{OptionName: "15|15Mbps"}}},
			{OptionName: "in_bw|流入带宽", Sub: []ConfigSub{{OptionName: "30|30Mbps"}}},
			{OptionName: "cpu|CPU", Sub: []ConfigSub{{OptionName: "4|4核"}}},
			{OptionName: "memory|内存", Sub: []ConfigSub{{OptionName: "4096|4G"}}},
			{OptionName: "system_disk_size|系统盘", Sub: []ConfigSub{{OptionName: "40|40G"}}},
			{OptionName: "data_disk_size|数据盘", Sub: []ConfigSub{{OptionName: "200|200G"}}},
			{OptionName: "ip_num|IP数量", Sub: []ConfigSub{{OptionName: "1|1IP"}}},
		},
	}}

	spec := parseCloudSpecs(groups)
	if spec.CPU != 4 {
		t.Errorf("CPU = %d, want 4", spec.CPU)
	}
	if spec.Memory != 4096 {
		t.Errorf("Memory = %d, want 4096（MB）", spec.Memory)
	}
	// 关键：售卖带宽取 bw(15)，不能被 in_bw(30) 覆盖。
	if spec.Bandwidth != 15 {
		t.Errorf("Bandwidth = %d, want 15（bw 售卖带宽，不是 in_bw 流入带宽 %d）", spec.Bandwidth, 30)
	}
	// 关键：系统盘取 system_disk_size(40)，不能被 data_disk_size(200) 覆盖。
	if spec.Disk != 40 {
		t.Errorf("Disk = %d, want 40（系统盘，不是数据盘 200）", spec.Disk)
	}
}

// TestParseCloudSpecsOrderIndependent 两种下发顺序都必须得到同一结果。
func TestParseCloudSpecsOrderIndependent(t *testing.T) {
	// 附属项在前：旧实现（按顺序覆盖）会在这里出错。
	groups := []ConfigGroup{{
		Options: []ConfigOption{
			{OptionName: "in_bw|流入带宽", Sub: []ConfigSub{{OptionName: "30|30Mbps"}}},
			{OptionName: "data_disk_size|数据盘", Sub: []ConfigSub{{OptionName: "200|200G"}}},
			{OptionName: "bw|带宽", Sub: []ConfigSub{{OptionName: "15|15Mbps"}}},
			{OptionName: "system_disk_size|系统盘", Sub: []ConfigSub{{OptionName: "40|40G"}}},
		},
	}}
	spec := parseCloudSpecs(groups)
	if spec.Bandwidth != 15 {
		t.Errorf("Bandwidth = %d, want 15（与配置项顺序无关）", spec.Bandwidth)
	}
	if spec.Disk != 40 {
		t.Errorf("Disk = %d, want 40（与配置项顺序无关）", spec.Disk)
	}
}

// TestParseCloudSpecsFallsBackToLooseMatch 只有宽松键（无精确键）时仍能识别。
//
// 上游部分商品的系统盘键名是中文「系统盘」，带宽键可能是 bandwidth；
// 精确轮匹配不到，必须靠宽松轮兜底（否则规格整段丢失）。
func TestParseCloudSpecsFallsBackToLooseMatch(t *testing.T) {
	groups := []ConfigGroup{{
		Options: []ConfigOption{
			{OptionName: "带宽", Sub: []ConfigSub{{OptionName: "20|20Mbps"}}},
			{OptionName: "系统盘空间", Sub: []ConfigSub{{OptionName: "60|60G"}}},
			{OptionName: "内存大小", Sub: []ConfigSub{{OptionName: "8192|8G"}}},
		},
	}}
	spec := parseCloudSpecs(groups)
	if spec.Bandwidth != 20 {
		t.Errorf("Bandwidth = %d, want 20（宽松匹配「带宽」）", spec.Bandwidth)
	}
	if spec.Disk != 60 {
		t.Errorf("Disk = %d, want 60（宽松匹配「系统盘空间」）", spec.Disk)
	}
	if spec.Memory != 8192 {
		t.Errorf("Memory = %d, want 8192（宽松匹配「内存大小」）", spec.Memory)
	}
}
