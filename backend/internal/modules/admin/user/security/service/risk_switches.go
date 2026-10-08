package service

import (
	"strconv"
	"strings"
)

// 风险规则配置键（system_configs，group=security）。与迁移 065 写入的默认值一一对应。
const (
	ConfigRiskRulesEnabled        = "risk_rules_enabled"
	ConfigRiskNotifyCritical      = "risk_notify_critical"
	ConfigRiskFailThreshold       = "risk_fail_threshold"
	ConfigRiskFailWindow          = "risk_fail_window_minutes"
	ConfigRiskMultiAccount        = "risk_multi_account_fail_accounts"
	ConfigRiskMultiAccountWindow  = "risk_multi_account_fail_window_minutes"
	ConfigRiskBurstCount          = "risk_login_burst_count"
	ConfigRiskBurstWindow         = "risk_login_burst_window_minutes"
	ConfigRiskDeviceChangeEnabled = "risk_device_change_enabled"
	ConfigRiskNewIPEnabled        = "risk_new_ip_enabled"
)

// riskSwitchKeys 规则引擎需要一次性载入的配置键。
var riskSwitchKeys = []string{
	ConfigRiskRulesEnabled,
	ConfigRiskNotifyCritical,
	ConfigRiskFailThreshold,
	ConfigRiskFailWindow,
	ConfigRiskMultiAccount,
	ConfigRiskMultiAccountWindow,
	ConfigRiskBurstCount,
	ConfigRiskBurstWindow,
	ConfigRiskDeviceChangeEnabled,
	ConfigRiskNewIPEnabled,
}

// RiskSwitchKeys 返回规则配置键（装配层构造配置源时使用）。
func RiskSwitchKeys() []string { return riskSwitchKeys }

// boolOr 解析布尔配置；空值/非法值回落默认。
func boolOr(raw string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		return def
	}
}

// intOr 解析整数配置；空值/非法值/负数回落默认。
//
// 负值当非法处理是有意的：把阈值配成 -1 会让「失败 >= -1」恒真，
// 每次登录都产生一条风险事件 —— 页面上会瞬间刷满告警。
func intOr(raw string, def int) int {
	v := strings.TrimSpace(raw)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def
	}
	return n
}
