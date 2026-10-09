package db

import (
	systemmodel "hostsent/backend/internal/modules/admin/system/config/model"
)

// riskSystemConfigs 风控规则开关与阈值默认值（doc06 §4.3/§4.4，迁移 065 同口径双写）。
//
// 双写的理由与 captchaSystemConfigs 相同：迁移负责既有库，这里负责新建库/容器重启，
// 两处必须一致，否则「全新部署」与「升级部署」的默认阈值会不一样。
//
// 默认值取「宁可漏报不要误报」：
//   - risk_rules_enabled=true —— 这条链路只写风险事件、不阻断任何登录，
//     开着不会改变既有行为，只是让「异常行为监控」页从空壳变成有真实数据；
//   - risk_device_change_enabled=true —— 换设备用户自己能感知，误报代价只是运营看一眼；
//   - risk_new_ip_enabled=false —— 移动网络、企业 NAT、CDN 回源下「新 IP」是常态，
//     默认打开会瞬间淹没真正需要看的事件。运营确认自家用户分布后可自行打开。
//
// 只有黑名单会阻断登录，而黑名单必须由运营显式添加 —— 因此这些默认值
// 不会导致任何「升级后突然登不进来」。
func riskSystemConfigs() []systemmodel.SystemConfig {
	return []systemmodel.SystemConfig{
		{ConfigKey: "risk_rules_enabled", ConfigValue: "true", ValueType: systemmodel.ValueTypeBool, Group: systemmodel.ConfigGroupSecurity, Description: "异常行为规则引擎总开关（只写事件、不阻断登录）", SortOrder: 50, Status: systemmodel.StatusActive},
		{ConfigKey: "risk_fail_threshold", ConfigValue: "5", ValueType: systemmodel.ValueTypeInt, Group: systemmodel.ConfigGroupSecurity, Description: "连续登录失败触发风险事件的次数阈值", SortOrder: 51, Status: systemmodel.StatusActive},
		{ConfigKey: "risk_fail_window_minutes", ConfigValue: "10", ValueType: systemmodel.ValueTypeInt, Group: systemmodel.ConfigGroupSecurity, Description: "连续登录失败统计窗口（分钟）", SortOrder: 52, Status: systemmodel.StatusActive},
		{ConfigKey: "risk_multi_account_fail_accounts", ConfigValue: "8", ValueType: systemmodel.ValueTypeInt, Group: systemmodel.ConfigGroupSecurity, Description: "同一 IP 失败的不同账号数阈值（撞库判定）", SortOrder: 53, Status: systemmodel.StatusActive},
		{ConfigKey: "risk_multi_account_fail_window_minutes", ConfigValue: "10", ValueType: systemmodel.ValueTypeInt, Group: systemmodel.ConfigGroupSecurity, Description: "撞库统计窗口（分钟）", SortOrder: 54, Status: systemmodel.StatusActive},
		{ConfigKey: "risk_login_burst_count", ConfigValue: "30", ValueType: systemmodel.ValueTypeInt, Group: systemmodel.ConfigGroupSecurity, Description: "同一 IP 登录请求次数阈值（频率异常）", SortOrder: 55, Status: systemmodel.StatusActive},
		{ConfigKey: "risk_login_burst_window_minutes", ConfigValue: "5", ValueType: systemmodel.ValueTypeInt, Group: systemmodel.ConfigGroupSecurity, Description: "登录频率统计窗口（分钟）", SortOrder: 56, Status: systemmodel.StatusActive},
		{ConfigKey: "risk_device_change_enabled", ConfigValue: "true", ValueType: systemmodel.ValueTypeBool, Group: systemmodel.ConfigGroupSecurity, Description: "是否对设备指纹变更生成风险事件", SortOrder: 57, Status: systemmodel.StatusActive},
		{ConfigKey: "risk_new_ip_enabled", ConfigValue: "false", ValueType: systemmodel.ValueTypeBool, Group: systemmodel.ConfigGroupSecurity, Description: "是否对新 IP 登录生成风险事件（NAT 环境易误报）", SortOrder: 58, Status: systemmodel.StatusActive},
		{ConfigKey: "risk_notify_critical", ConfigValue: "false", ValueType: systemmodel.ValueTypeBool, Group: systemmodel.ConfigGroupSecurity, Description: "严重级风险事件是否额外推送站内通知", SortOrder: 59, Status: systemmodel.StatusActive},
	}
}
