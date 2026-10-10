package mofangyun

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"hostsent/backend/internal/pkg/upstream"
)

// 目录与官方参数表的逐项对齐校验。
//
// 这份测试挡的是三类真实踩过的坑：
//   - 把说明文字当默认值（cpu_limit 的「以平台设置为准」、「不支持」），
//     前台拿它当默认选中项，数量型直接被当数字解析 → 客户下单报「取值必须是数字」；
//   - 数量型默认值落在 min~max 之外，前台步进器一打开就是非法值；
//   - 特殊取值（-1/0/auto/多段值）没有含义说明，运营/客户只能回头翻文档。
//
// docDefaults 是官方文档「未传递默认值」列的口径快照：文档给了具体值的照抄，
// 文档写的是「不支持/以平台设置为准」这类**没有可下发取值**的一律留空。

// docDefaults 文档口径的默认值：key → 期望的 Default。
// 空串表示「文档给了说明但平台侧没有对应该值，必须留空」。
var docDefaults = map[string]string{
	"cpu":                  "",
	"memory":               "",
	"system_disk_size":     "50",
	"store":                "",
	"data_disk_size":       "0",
	"os":                   "",
	"type":                 "host",
	"area":                 "",
	"node":                 "",
	"node_group":           "",
	"node_priority":        "1",
	"ip_group":             "",
	"network_type":         "",
	"bw":                   "0",
	"in_bw":                "0",
	"flow_limit":           "0",
	"flow_way":             "all",
	"traffic_bill_type":    "month",
	"ip_num":               "",
	"ipv6_num":             "0",
	"IP_MACBond":           "1",
	"port":                 "auto",
	"snap_num":             "2",
	"backup_num":           "2",
	"nat_acl_limit":        "-1",
	"nat_web_limit":        "-1",
	"system_disk_io_limit": "0,0,0,0",
	"data_disk_io_limit":   "0,0,0,0",
	"cpu_limit":            "",
	"advanced_cpu":         "",
	"advanced_bw":          "",
	"resource_package":     "",
	"gpu_num":              "",
	"niccard":              "",
}

// docLabels 文档口径的中文名（改中文名要同步改文档，故一并钉住）。
var docLabels = map[string]string{
	"area": "数据中心", "node": "节点", "os": "操作系统", "cpu": "CPU",
	"memory": "内存", "system_disk_size": "系统盘", "network_type": "网络类型",
	"bw": "带宽（上下行）", "in_bw": "流入带宽（上行）", "flow_limit": "流量",
	"flow_way": "流量方向", "ip_num": "IP 数量", "data_disk_size": "数据盘",
	"snap_num": "快照数量", "backup_num": "备份数量", "nat_acl_limit": "NAT 转发",
	"nat_web_limit": "共享建站", "system_disk_io_limit": "系统盘性能",
	"data_disk_io_limit": "数据盘性能", "ip_group": "IP 分组", "node_group": "节点分组",
	"node_priority": "节点选择优先级", "IP_MACBond": "IP-MAC 绑定",
	"cpu_limit": "CPU 限制", "traffic_bill_type": "流量计费周期", "type": "云节点类型",
	"advanced_cpu": "智能 CPU", "advanced_bw": "智能带宽", "port": "端口",
	"ipv6_num": "IPv6 数量", "resource_package": "资源包", "gpu_num": "GPU 数量",
	"niccard": "网卡驱动",
}

// TestOptionCatalog_DefaultIsSendable 默认值必须是「可以直接下发平台的值」。
// 数量型必须是落在区间内的数字；枚举型必须是自己声明过的取值；留空表示不下发。
func TestOptionCatalog_DefaultIsSendable(t *testing.T) {
	for _, spec := range optionCatalog() {
		def := strings.TrimSpace(spec.Default)
		if def == "" {
			continue // 留空 = 不给默认（平台口径由平台决定）
		}
		if len(spec.Options) > 0 {
			// 枚举型：默认值必须在候选里，除非文档明确给了候选外的默认（快照/备份默认 2）。
			if !isListedOption(spec, def) {
				switch spec.Key {
				case "snap_num", "backup_num":
					// 文档：取值 -1/0/5，未传递默认 2 个。2 是平台侧默认，不在候选里。
					if def != "2" {
						t.Errorf("%s: 枚举默认值 %q 不在候选中且不是文档给的 2", spec.Key, def)
					}
				default:
					t.Errorf("%s: 枚举默认值 %q 不在 Options 候选中", spec.Key, def)
				}
			}
			continue
		}
		// 数量型：必须是数字，且落在区间内（前台步进器直接用它做初值）。
		num, err := strconv.ParseFloat(def, 64)
		if err != nil {
			t.Errorf("%s: 默认值 %q 不是数字（说明文字不能当默认值）", spec.Key, def)
			continue
		}
		if spec.MinValue != nil && num < *spec.MinValue {
			t.Errorf("%s: 默认值 %v 小于下限 %v", spec.Key, num, *spec.MinValue)
		}
		if spec.MaxValue != nil && num > *spec.MaxValue {
			t.Errorf("%s: 默认值 %v 大于上限 %v", spec.Key, num, *spec.MaxValue)
		}
	}
}

// TestOptionCatalog_DefaultsMatchDoc 默认值与中文名逐项对齐官方文档。
func TestOptionCatalog_DefaultsMatchDoc(t *testing.T) {
	seen := map[string]bool{}
	for _, spec := range optionCatalog() {
		seen[spec.Key] = true
		want, ok := docDefaults[spec.Key]
		if !ok {
			t.Errorf("%s: 目录里多出一项，文档口径表里没有（新增项请同步补 docs 与 docDefaults）", spec.Key)
			continue
		}
		if got := strings.TrimSpace(spec.Default); got != want {
			t.Errorf("%s: 默认值 = %q，文档口径应为 %q", spec.Key, got, want)
		}
		if wantLabel, ok := docLabels[spec.Key]; ok && spec.Label != wantLabel {
			t.Errorf("%s: 中文名 = %q，文档口径应为 %q", spec.Key, spec.Label, wantLabel)
		}
	}
	for key := range docDefaults {
		if !seen[key] {
			t.Errorf("%s: 文档口径表里有但目录里缺失", key)
		}
	}
}

// selfExplanatoryValues 取值本身就是含义、不需要再挂一句说明的配置项。
//
// 「每个取值都要有备注」这条针对的是**光看取值读不懂**的那些（-1 不开通 / 0 不限量 /
// auto 随机端口 / 0,0,0,0 不限制）。cpu 与 memory 除外：取值 4 就是 4 核、4096 就是 4G，
// 显示名已经把它写全了，再挂一份 OptionsHelp 只会在配置档页多出一行「2 2核」的重复。
var selfExplanatoryValues = map[string]bool{"cpu": true, "memory": true}

// TestOptionCatalog_HelpExplainsOpaqueValues 每个取值都要有含义说明，特殊值尤其不能漏。
func TestOptionCatalog_HelpExplainsOpaqueValues(t *testing.T) {
	for _, spec := range optionCatalog() {
		help := parseOptionsHelp(t, spec)
		if !selfExplanatoryValues[spec.Key] {
			for _, opt := range spec.Options {
				if strings.TrimSpace(help[opt.Value]) == "" {
					t.Errorf("%s: 取值 %q 缺少 OptionsHelp 说明", spec.Key, opt.Value)
				}
			}
		}
		for value, text := range help {
			if strings.TrimSpace(text) == "" {
				t.Errorf("%s: OptionsHelp[%q] 为空", spec.Key, value)
			}
		}
		if strings.TrimSpace(spec.Help) == "" {
			t.Errorf("%s: Help 为空（取值从哪来、怎么填必须写清）", spec.Key)
		}
	}
}

// TestOptionCatalog_BandwidthSemantics 带宽两个字段的语义不能被写混。
// 平台侧 bw 是上下行同值展开成 in_bw/out_bw，in_bw 只表示流入（上行）。
func TestOptionCatalog_BandwidthSemantics(t *testing.T) {
	byKey := map[string]upstream.ConfigOptionSpec{}
	for _, spec := range optionCatalog() {
		byKey[spec.Key] = spec
	}
	bw, ok := byKey["bw"]
	if !ok {
		t.Fatal("目录缺少 bw")
	}
	if !strings.Contains(bw.Help, "上下行") {
		t.Errorf("bw.Help 应说明它是上下行同值：%q", bw.Help)
	}
	inBw, ok := byKey["in_bw"]
	if !ok {
		t.Fatal("目录缺少 in_bw")
	}
	if strings.Contains(inBw.Help, "上下行") {
		t.Errorf("in_bw.Help 不应写「上下行」（它只是流入/上行）：%q", inBw.Help)
	}
	if !strings.Contains(inBw.Help, "流入") {
		t.Errorf("in_bw.Help 应点明是流入/上行带宽：%q", inBw.Help)
	}
}

// TestOptionCatalog_RequiredMatchesDoc 必选项与文档一致（平台不传会拒绝开通）。
func TestOptionCatalog_RequiredMatchesDoc(t *testing.T) {
	want := map[string]bool{"area": true, "os": true, "cpu": true, "memory": true, "network_type": true, "ip_num": true}
	for _, spec := range optionCatalog() {
		if spec.Required != want[spec.Key] {
			t.Errorf("%s: Required = %v，文档口径应为 %v（文档里 area 与 node 至少传一个）",
				spec.Key, spec.Required, want[spec.Key])
		}
	}
}

func isListedOption(spec upstream.ConfigOptionSpec, value string) bool {
	for _, o := range spec.Options {
		if o.Value == value {
			return true
		}
	}
	return false
}

func parseOptionsHelp(t *testing.T, spec upstream.ConfigOptionSpec) map[string]string {
	t.Helper()
	if strings.TrimSpace(spec.OptionsHelp) == "" {
		return nil
	}
	out := map[string]string{}
	if err := json.Unmarshal([]byte(spec.OptionsHelp), &out); err != nil {
		t.Fatalf("%s: OptionsHelp 不是合法 JSON 对象：%v（原文 %s）", spec.Key, err, spec.OptionsHelp)
	}
	return out
}
