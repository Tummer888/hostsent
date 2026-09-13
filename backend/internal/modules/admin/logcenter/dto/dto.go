// Package dto 日志中心的请求/响应类型（doc92 §9.2/§9.3）。
package dto

// ColumnInfo 一列的元数据（前端据此动态渲染 26 个源的表格）。
type ColumnInfo struct {
	Key    string            `json:"key"`
	Label  string            `json:"label"`
	Type   string            `json:"type"`
	Width  int               `json:"width"`
	Hidden bool              `json:"hidden"`
	Masked bool              `json:"masked"`
	Enum   map[string]string `json:"enum_map,omitempty"`
}

// SourceInfo 一个日志源的定义。
type SourceInfo struct {
	Key                  string            `json:"key"`
	DisplayName          string            `json:"display_name"`
	Group                string            `json:"group"`
	GroupLabel           string            `json:"group_label"`
	Class                string            `json:"class"`
	Cleanable            bool              `json:"cleanable"`
	TimeColumn           string            `json:"time_column"`
	Columns              []ColumnInfo      `json:"columns"`
	FilterColumns        map[string]string `json:"filter_columns"`
	Searchable           bool              `json:"searchable"`
	DefaultAction        string            `json:"default_action"`
	DefaultRetentionDays int               `json:"default_retention_days"`
	IndependentPage      string            `json:"independent_page"`
	Remark               string            `json:"remark"`
	// Rows 当前行数（列表徽标）。
	Rows int64 `json:"rows"`
	// HasIndex 清理 SQL 是否走索引（诊断展示，不参与门禁）。
	HasIndex bool `json:"has_index"`
}

// CatalogResponse 日志源目录。
type CatalogResponse struct {
	Groups []GroupInfo  `json:"groups"`
	Items  []SourceInfo `json:"items"`
	// Actions 动作枚举（前端 select 用，含中文标签）。
	Actions []EnumOption `json:"actions"`
	// Classes 分级枚举。
	Classes []EnumOption `json:"classes"`
}

// GroupInfo 分组。
type GroupInfo struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// EnumOption 枚举选项。
type EnumOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// StatsResponse 日志中心统计（左侧徽标 + 采集侧计数）。
type StatsResponse struct {
	TotalRows    int64            `json:"total_rows"`
	RowsByGroup  map[string]int64 `json:"rows_by_group"`
	RowsBySource map[string]int64 `json:"rows_by_source"`
	// Upstream 采集侧计数：让运营能看见「日志系统自己丢了多少」（doc92 §3.1）。
	UpstreamRecorded int64 `json:"upstream_recorded"`
	UpstreamDropped  int64 `json:"upstream_dropped"`
	// JobRuns24h 近 24 小时任务执行次数与失败数。
	JobRuns24h   int64 `json:"job_runs_24h"`
	JobFailed24h int64 `json:"job_failed_24h"`
	// ActiveCleanupJob 当前在跑的清理任务 ID（0 = 无）。
	ActiveCleanupJob uint64 `json:"active_cleanup_job"`
	// ExportFiles 导出文件数与占用空间。
	ExportFiles     int64 `json:"export_files"`
	ExportTotalSize int64 `json:"export_total_size"`
}

// QueryRequest 日志查询参数。
type QueryRequest struct {
	Source   string `form:"source"`
	From     string `form:"from"`
	To       string `form:"to"`
	Keyword  string `form:"keyword"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// QueryResponse 日志查询结果（items 为原始行 map，列定义由 catalog 下发）。
type QueryResponse struct {
	Source   string           `json:"source"`
	Items    []map[string]any `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	From     string           `json:"from"`
	To       string           `json:"to"`
}

// DetailResponse 单行详情（含被截断字段的完整内容）。
type DetailResponse struct {
	Source string         `json:"source"`
	Item   map[string]any `json:"item"`
}

// ExportRequest 导出请求（列表页异步导出）。
type ExportRequest struct {
	Source  string            `json:"source" binding:"required"`
	Format  string            `json:"format"` // csv / jsonl
	From    string            `json:"from"`
	To      string            `json:"to"`
	Keyword string            `json:"keyword"`
	Filters map[string]string `json:"filters"`
}

// ExportResult 导出结果（返回 file_id，文件在导出文件列表里出现）。
type ExportResult struct {
	FileID   uint64 `json:"file_id"`
	FileName string `json:"file_name"`
	RowCount int64  `json:"row_count"`
	FileSize int64  `json:"file_size"`
	SHA256   string `json:"sha256"`
}

// ExportFileInfo 导出文件信息。
type ExportFileInfo struct {
	ID           uint64 `json:"id"`
	SourceKey    string `json:"source_key"`
	DisplayName  string `json:"display_name"`
	JobID        uint64 `json:"job_id"`
	Format       string `json:"format"`
	FileName     string `json:"file_name"`
	FileSize     int64  `json:"file_size"`
	RowCount     int64  `json:"row_count"`
	SHA256       string `json:"sha256"`
	PeriodFrom   string `json:"period_from"`
	PeriodTo     string `json:"period_to"`
	OperatorID   uint64 `json:"operator_id"`
	OperatorName string `json:"operator_name"`
	Status       string `json:"status"`
	FailReason   string `json:"fail_reason"`
	ExpiresAt    string `json:"expires_at"`
	CreatedAt    string `json:"created_at"`
	// BoundJob true 时该文件是清理任务的备份凭据，不可删除（doc92 §8.3）。
	BoundJob bool `json:"bound_job"`
}

// ExportFileListResponse 导出文件列表。
type ExportFileListResponse struct {
	Items    []ExportFileInfo `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// CleanupPreviewRequest 预演请求（dry_run 的执行入口）。
type CleanupPreviewRequest struct {
	Sources    []string `json:"sources"`
	BeforeTime string   `json:"before_time"`
}

// CleanupPreviewItem 逐源预演结果。
type CleanupPreviewItem struct {
	SourceKey      string `json:"source_key"`
	DisplayName    string `json:"display_name"`
	Action         string `json:"action"`
	RetentionDays  int    `json:"retention_days"`
	BeforeTime     string `json:"before_time"`
	TotalRows      int64  `json:"total_rows"`
	EligibleRows   int64  `json:"eligible_rows"`
	GuardedRows    int64  `json:"guarded_rows"`
	EarliestTime   string `json:"earliest_time"`
	LatestTime     string `json:"latest_time"`
	EstimatedBytes int64  `json:"estimated_bytes"`
	Enabled        bool   `json:"enabled"`
}

// CleanupPreviewResponse 预演结果。
type CleanupPreviewResponse struct {
	Items        []CleanupPreviewItem `json:"items"`
	EligibleRows int64                `json:"eligible_rows"`
	GuardedRows  int64                `json:"guarded_rows"`
	// ExportBeforeDelete 当前开关值：false 时页面置灰执行按钮（doc92 §8.3）。
	ExportBeforeDelete bool   `json:"export_before_delete"`
	Message            string `json:"message"`
}

// CleanupRunRequest 执行清理请求。
type CleanupRunRequest struct {
	Sources    []string `json:"sources"`
	BeforeTime string   `json:"before_time"`
	// ConfirmText 必须等于 "DELETE"（高危操作防误点，doc92 §8.3）。
	ConfirmText string `json:"confirm_text"`
}

// CleanupJobInfo 清理任务信息。
type CleanupJobInfo struct {
	ID             uint64   `json:"id"`
	JobNo          string   `json:"job_no"`
	TriggerType    string   `json:"trigger_type"`
	Status         string   `json:"status"`
	StatusLabel    string   `json:"status_label"`
	Sources        []string `json:"sources"`
	BeforeTime     string   `json:"before_time"`
	DryRun         bool     `json:"dry_run"`
	ExportRequired bool     `json:"export_required"`
	ExportFileIDs  []uint64 `json:"export_file_ids"`
	ScannedRows    int64    `json:"scanned_rows"`
	DeletedRows    int64    `json:"deleted_rows"`
	ExpiredRows    int64    `json:"expired_rows"`
	Progress       int      `json:"progress"`
	CurrentSource  string   `json:"current_source"`
	ErrorMessage   string   `json:"error_message"`
	OperatorID     uint64   `json:"operator_id"`
	OperatorName   string   `json:"operator_name"`
	StartedAt      string   `json:"started_at"`
	FinishedAt     string   `json:"finished_at"`
	CreatedAt      string   `json:"created_at"`
	// Active true 时前端每 3 秒轮询一次进度。
	Active bool `json:"active"`
}

// CleanupJobListResponse 清理任务列表。
type CleanupJobListResponse struct {
	Items    []CleanupJobInfo `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// CleanupJobDetailResponse 任务详情（含逐源进度与导出文件）。
type CleanupJobDetailResponse struct {
	Job         CleanupJobInfo   `json:"job"`
	ExportFiles []ExportFileInfo `json:"export_files"`
}

// PolicyInfo 保留策略（含注册表元数据，策略页一张表展示全部信息）。
type PolicyInfo struct {
	SourceKey       string `json:"source_key"`
	DisplayName     string `json:"display_name"`
	Group           string `json:"group"`
	GroupLabel      string `json:"group_label"`
	Class           string `json:"class"`
	Cleanable       bool   `json:"cleanable"`
	Action          string `json:"action"`
	ActionLabel     string `json:"action_label"`
	RetentionDays   int    `json:"retention_days"`
	ArchiveCron     string `json:"archive_cron"`
	BatchSize       int    `json:"batch_size"`
	Enabled         bool   `json:"enabled"`
	LastRunAt       string `json:"last_run_at"`
	LastDeleted     int64  `json:"last_deleted"`
	Remark          string `json:"remark"`
	IndependentPage string `json:"independent_page"`
	// DefaultRetentionDays seed 默认值：低于它时前端给警示（doc92 §8.4）。
	DefaultRetentionDays int    `json:"default_retention_days"`
	TimeColumn           string `json:"time_column"`
	Table                string `json:"table"`
	DeleteGuard          string `json:"delete_guard"`
	// Configured true 表示库里已有策略行（否则展示的是注册表默认值）。
	Configured bool `json:"configured"`
}

// PolicyUpdateRequest 策略更新请求。
type PolicyUpdateRequest struct {
	Action        string `json:"action" binding:"required"`
	RetentionDays int    `json:"retention_days"`
	BatchSize     int    `json:"batch_size"`
	Enabled       bool   `json:"enabled"`
	ArchiveCron   string `json:"archive_cron"`
	Remark        string `json:"remark"`
}
