package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"hostsent/backend/internal/modules/admin/product/spec/model"
)

// 机型判定涉及的平台参数名（魔方云写参数口径，与 dcimcloud 适配器的配置项目录一致）。
const (
	paramIPNum  = "ip_num"
	paramNatACL = "nat_acl_limit"
	paramNatWeb = "nat_web_limit"
)

// normalizeAndValidateMachineKind 归一机型取值并校验其与平台参数基线是否自洽（doc122）。
// 返回归一后的机型（vps / nat / ”），供入库。
//
// 魔方云两条开通链路共用 POST /clouds，机型完全由参数取值决定，面板不校验组合：
//   - 独立 IP（vps）：ip_num >= 1 且 nat_*_limit = -1；
//   - NAT 机型：ip_num = 0 且 nat_acl_limit / nat_web_limit 至少一个 >= 0，
//     机器没有独立 IP，客户经节点转发地址 nat_host_ip:ext_port 访问。
//
// 为什么必须在保存时校验：最危险的配错不是「参数被面板拒」而是「机器开出来了但客户
// 连不上」——把 NAT 机型的转发能力也置为 -1，得到一台既无独立 IP 又无转发入口的机器，
// 面板不会报错，客户拿到手才发现无法连接。另一类是机型与参数相反（NAT 档位配了
// ip_num>=1），客户付 NAT 的价钱拿到带独立 IP 的机器。两类都宁可在此拦住。
//
// kind 为 ” 时跳过校验：存量模板没有机型维度，不能因为新增校验就让老数据改不动。
func normalizeAndValidateMachineKind(kind string, platformParams, optionSelections json.RawMessage) (string, error) {
	normalized := strings.TrimSpace(strings.ToLower(kind))
	if normalized == "" {
		return model.MachineKindUnset, nil
	}
	if normalized != model.MachineKindVPS && normalized != model.MachineKindNAT {
		return "", fmt.Errorf("机型「%s」不合法（可选 vps / nat）", kind)
	}

	params, err := decodeParamObject(platformParams, "平台参数")
	if err != nil {
		return "", err
	}
	ipNum, hasIPNum, err := paramNumber(params, paramIPNum)
	if err != nil {
		return "", err
	}
	acl, hasACL, err := paramNumber(params, paramNatACL)
	if err != nil {
		return "", err
	}
	web, hasWeb, err := paramNumber(params, paramNatWeb)
	if err != nil {
		return "", err
	}

	switch normalized {
	case model.MachineKindNAT:
		if !hasIPNum {
			// 目录把 ip_num 声明为必选：不下发平台直接拒绝开通，必须显式给 0。
			return "", fmt.Errorf("NAT 机型必须在「平台映射」里显式设置 ip_num = 0（不分配独立 IP）")
		}
		if ipNum != 0 {
			return "", fmt.Errorf("NAT 机型的 ip_num 必须为 0，当前为 %s；需要独立 IP 的档位请把机型改为「独立 IP」", formatParamNumber(ipNum))
		}
		if (!hasACL || acl < 0) && (!hasWeb || web < 0) {
			return "", fmt.Errorf("NAT 机型必须至少开通一种访问入口：nat_acl_limit（端口转发）或 nat_web_limit（共享建站）需 >= 0；" +
				"两者都为 -1 时机器既没有独立 IP 也没有转发入口，客户开出来将无法连接")
		}
		// NAT 的 ip_num 固定为 0，不能同时作为客户可选配置项暴露：
		// 客户选配的优先级高于档位基线，选了非 0 会覆盖掉这里的 0。
		selections, err := decodeParamObject(optionSelections, "选配项")
		if err != nil {
			return "", err
		}
		if _, ok := selections[paramIPNum]; ok {
			return "", fmt.Errorf("NAT 机型的 ip_num 固定为 0，不能作为客户可选配置项（请在选配项里移除「%s」）", paramIPNum)
		}
	case model.MachineKindVPS:
		if hasIPNum && ipNum < 1 {
			return "", fmt.Errorf("独立 IP 机型的 ip_num 必须 >= 1，当前为 %s；不分配独立 IP 的档位请把机型改为「NAT 机型」", formatParamNumber(ipNum))
		}
		if hasACL && acl >= 0 {
			return "", fmt.Errorf("独立 IP 机型的 nat_acl_limit 必须为 -1，当前为 %s；NAT 端口转发与独立 IP 机型互斥", formatParamNumber(acl))
		}
		if hasWeb && web >= 0 {
			return "", fmt.Errorf("独立 IP 机型的 nat_web_limit 必须为 -1，当前为 %s；共享建站与独立 IP 机型互斥", formatParamNumber(web))
		}
	}
	return normalized, nil
}

// decodeParamObject 解析平台参数 / 选配项 JSON 对象；空值与 null 视为空对象。
func decodeParamObject(raw json.RawMessage, label string) (map[string]any, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return map[string]any{}, nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return nil, fmt.Errorf("%s JSON 非法（应为 参数名 → 取值 的对象）: %w", label, err)
	}
	if parsed == nil {
		return map[string]any{}, nil
	}
	return parsed, nil
}

// paramNumber 读取平台参数里的数值：返回 (值, 是否存在, 错误)。
// 前端可能回传数字 0 或字符串 "0"，两种都要认；非数值文本直接报错，
// 不能静默当作 0 —— 那会让 "abc" 被当成合法的 NAT ip_num。
func paramNumber(params map[string]any, key string) (float64, bool, error) {
	raw, ok := params[key]
	if !ok || raw == nil {
		return 0, false, nil
	}
	switch v := raw.(type) {
	case float64:
		return v, true, nil
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return 0, true, fmt.Errorf("平台参数「%s」的值「%s」不是数字", key, v.String())
		}
		return f, true, nil
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return 0, false, nil
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, true, fmt.Errorf("平台参数「%s」的值「%s」不是数字", key, s)
		}
		return f, true, nil
	default:
		return 0, true, fmt.Errorf("平台参数「%s」应为数字", key)
	}
}

// formatParamNumber 用于报错文案：整数不带小数点（0 而不是 0.000000）。
func formatParamNumber(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
