package db

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	logcatalog "hostsent/backend/internal/modules/admin/logcenter/catalog"
	logcentermodel "hostsent/backend/internal/modules/admin/logcenter/model"
	systemmodel "hostsent/backend/internal/modules/admin/system/model"
)

// logSystemConfigs 日志中心相关开关默认值（doc89 §9.1 日志部分）。
//
// 默认值取舍（doc89 §9.2）：
//   - log_cleanup_enabled=true：清理永远先导出后删除，不存在无备份被删。
//   - log_upstream_capture=true + sample_rate=100：先有可观测性，量大了改配置调采样。
func logSystemConfigs() []systemmodel.SystemConfig {
	return []systemmodel.SystemConfig{
		{ConfigKey: "log_retention_days", ConfigValue: "180", ValueType: systemmodel.ValueTypeInt, Group: "log", Description: "日志全局保留天数（硬地板 7 天）", SortOrder: 1, Status: systemmodel.StatusActive},
		{ConfigKey: "log_cleanup_enabled", ConfigValue: "true", ValueType: systemmodel.ValueTypeBool, Group: "log", Description: "启用日志自动清理调度", SortOrder: 2, Status: systemmodel.StatusActive},
		{ConfigKey: "log_cleanup_hour", ConfigValue: "3", ValueType: systemmodel.ValueTypeInt, Group: "log", Description: "日志清理执行小时（0-23，当日只跑一次）", SortOrder: 3, Status: systemmodel.StatusActive},
		{ConfigKey: "log_export_before_delete", ConfigValue: "true", ValueType: systemmodel.ValueTypeBool, Group: "log", Description: "删除前必须导出备份（关闭时清理接口拒绝执行）", SortOrder: 4, Status: systemmodel.StatusActive},
		{ConfigKey: "log_upstream_capture", ConfigValue: "true", ValueType: systemmodel.ValueTypeBool, Group: "log", Description: "采集上游厂商接口日志", SortOrder: 5, Status: systemmodel.StatusActive},
		{ConfigKey: "log_upstream_body_max_bytes", ConfigValue: "4096", ValueType: systemmodel.ValueTypeInt, Group: "log", Description: "上游正文采集上限（字节，超出截断）", SortOrder: 6, Status: systemmodel.StatusActive},
		{ConfigKey: "log_upstream_sample_rate", ConfigValue: "100", ValueType: systemmodel.ValueTypeInt, Group: "log", Description: "上游日志采样率（0-100，100=全量）", SortOrder: 7, Status: systemmodel.StatusActive},
		{ConfigKey: "log_export_retention_days", ConfigValue: "365", ValueType: systemmodel.ValueTypeInt, Group: "log", Description: "导出文件自身保留天数", SortOrder: 8, Status: systemmodel.StatusActive},
	}
}

// seedLogRetentionPolicies 逐源写入保留策略 seed（doc92 §4.2/§4.3）。
//
// 数据源就是 catalog 注册表本身 —— 26 个源的 action/保留期来自同一份代码常量，
// 不存在「seed 与注册表漂移」的可能（改了注册表即改了 seed）。
// ON CONFLICT DO NOTHING：运营调过的保留期不能被重启重置（seedAdminUser 的教训）。
func seedLogRetentionPolicies(tx *gorm.DB) error {
	sources := logcatalog.All()
	if len(sources) == 0 {
		return nil
	}
	rows := make([]logcentermodel.LogRetentionPolicy, 0, len(sources))
	for _, src := range sources {
		action := src.EffectiveAction()
		cron := ""
		if action == logcatalog.ActionArchiveOnly {
			cron = "monthly"
		}
		rows = append(rows, logcentermodel.LogRetentionPolicy{
			SourceKey:     src.Key,
			DisplayName:   src.DisplayName,
			Action:        action,
			RetentionDays: src.DefaultRetention(),
			ArchiveCron:   cron,
			BatchSize:     5000,
			Enabled:       true,
		})
	}
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "source_key"}}, DoNothing: true}).
		CreateInBatches(rows, 50).Error
}
