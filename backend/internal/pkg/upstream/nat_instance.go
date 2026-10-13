package upstream

import (
	"encoding/json"
	"strconv"
	"strings"
)

// NAT 机型判定的共享实现（doc122）。
//
// 放这里而不是各服务层各写一份：管理端与用户中心都要判断"这台机器是不是 NAT 机型"，
// 判定口径分叉会导致同一条实例在两边显示不同的连接方式。判据来自平台写参数留下的
// raw_data，属平台知识，归上游包。

// IsNatInstance 判断实例是否走 NAT 链路（无独立 IP）。
//
// 依据优先级（都是平台侧事实）：
//  1. raw_data.is_nat_host —— 面板自己的机型判定，最权威；
//  2. raw_data 里 nat_acl_limit / nat_web_limit >= 0 —— 开通时开了转发，
//     面板语义下这等价于"不分配独立 IP"；
//  3. ip_num == 0 且存在转发能力 —— 前两项都缺时的兜底（存量行）。
//
// 不能只看 public_ip 是否为空：待开通/异常实例的 public_ip 也是空的，
// 那些实例并不是 NAT 机型。判定错了的后果是前端把 VPS 显示成 NAT（或反之），
// 客户照着错的地址去连。
func IsNatInstance(rawData string) bool {
	m := parseRawData(rawData)
	if m == nil {
		return false
	}
	if v, ok := m["is_nat_host"]; ok {
		if b, ok := boolish(v); ok {
			return b
		}
	}
	acl, hasACL := rawDataNumber(m, "nat_acl_limit")
	web, hasWeb := rawDataNumber(m, "nat_web_limit")
	if hasACL && acl >= 0 {
		return true
	}
	if hasWeb && web >= 0 {
		return true
	}
	ipNum, hasIPNum := rawDataNumber(m, "ip_num")
	return hasIPNum && ipNum == 0
}

// NatLimitsFromRaw 从 raw_data 读转发能力上限（nat_acl_limit / nat_web_limit）。
// 返回 (acl, web, 是否读到)：面板口径 -1=不支持，>=0=支持（0=不限制）。
// 用于前端展示"该机器是否开放了转发能力"，与 IsNatInstance 的判据同源。
func NatLimitsFromRaw(rawData string) (int, int, bool) {
	m := parseRawData(rawData)
	if m == nil {
		return 0, 0, false
	}
	acl, hasACL := rawDataNumber(m, "nat_acl_limit")
	web, hasWeb := rawDataNumber(m, "nat_web_limit")
	if !hasACL && !hasWeb {
		return 0, 0, false
	}
	return int(acl), int(web), true
}

// parseRawData 解析实例 raw_data JSON；空/非法返回 nil。
func parseRawData(rawData string) map[string]interface{} {
	s := strings.TrimSpace(rawData)
	if s == "" || s == "null" {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	return m
}

// rawDataNumber 读数值字段并告知是否存在：0 是有效值（ip_num=0 正是 NAT 的标志），
// 不能拿"值为 0"当缺失判断。
func rawDataNumber(m map[string]interface{}, key string) (float64, bool) {
	raw, ok := m[key]
	if !ok || raw == nil {
		return 0, false
	}
	switch v := raw.(type) {
	case float64:
		return v, true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(s, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

// boolish 兼容 bool 与 "true"/"false"/"1"/"0" 字符串两种形态。
func boolish(v interface{}) (bool, bool) {
	switch t := v.(type) {
	case bool:
		return t, true
	case float64:
		return t != 0, true
	case string:
		s := strings.TrimSpace(strings.ToLower(t))
		switch s {
		case "true", "1", "yes":
			return true, true
		case "false", "0", "no":
			return false, true
		}
	}
	return false, false
}
