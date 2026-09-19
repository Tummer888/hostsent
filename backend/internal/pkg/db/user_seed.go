package db

import (
	systemmodel "hostsent/backend/internal/modules/admin/system/model"
)

// userDeletionSystemConfigs 用户注销与留存期相关开关（doc104 §4.1）。
//
// 与迁移 052 同口径双写：既有库靠迁移写入，新建库靠本 seed 写入，
// 两条路径的键名/默认值必须完全一致，否则「新库和老库行为不同」。
//
// 留存期默认 180 天：制度要求个人数据保留一定时间（例如 6 个月）以备查证，
// 到期后再做完整硬删除。与日志中心的 log_retention_days 取同一数值，
// 避免两套保留期口径互相打架。
func userDeletionSystemConfigs() []systemmodel.SystemConfig {
	return []systemmodel.SystemConfig{
		{
			ConfigKey:   "user.deletion_retention_days",
			ConfigValue: "180",
			ValueType:   systemmodel.ValueTypeInt,
			Group:       systemmodel.ConfigGroupSecurity,
			Description: "用户注销后数据留存天数，到期后硬删除（最小 1 天，0 或非法值按 180 处理）",
			SortOrder:   10,
			Status:      systemmodel.StatusActive,
		},
	}
}
