package model

import (
	"strings"

	"gorm.io/gorm"
)

// 可空 jsonb 列的空值归一化。
//
// Postgres 的 jsonb 不接受空串（22P02: invalid input syntax for type json），
// 而 Go 里 string 字段的零值就是 ""。三个模型都有可空 jsonb 列，调用方一旦忘了
// 转 NULL 就会在 Create 时炸掉 —— 清理任务创建路径上的 `export_file_ids` 正是
// 这个情况（doc92 §10 场景 13：任务创建直接 50001，一行未删但任务建不出来）。
//
// 这里在 BeforeCreate 里把空串写成 JSON 字面量 `null`（对 jsonb 合法）。
// 读路径的空值判定已统一（decodeStrings/decodeSummary 对 ""/null 均返回 nil），
// 因此不需要把模型字段改成 *string 去牵连所有调用点。
//
// 只挂 BeforeCreate 不挂 BeforeSave：更新路径用的是 map Updates + nullIfEmpty，
// 那里已处理过；在 BeforeSave 里改字段会把 map 更新意外切成 struct 更新。
func nullJSON(s string) string {
	if strings.TrimSpace(s) == "" {
		return "null"
	}
	return s
}

func (j *LogCleanupJob) BeforeCreate(_ *gorm.DB) error {
	j.Sources = nullJSON(j.Sources)
	j.ExportFileIDs = nullJSON(j.ExportFileIDs)
	return nil
}

func (f *LogExportFile) BeforeCreate(_ *gorm.DB) error {
	f.Filters = nullJSON(f.Filters)
	return nil
}

func (l *JobRunLog) BeforeCreate(_ *gorm.DB) error {
	l.Summary = nullJSON(l.Summary)
	return nil
}
