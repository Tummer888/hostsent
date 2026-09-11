package dto

import commondto "hostsent/backend/internal/modules/admin/resource/common/dto"

// ListMeta 通用分页元信息（复用 common 包定义）
type ListMeta = commondto.ListMeta

// SyncTaskListQuery 同步任务列表查询
type SyncTaskListQuery struct {
	ProviderID uint64 `form:"provider_id" json:"provider_id"`
	TaskType   string `form:"task_type" json:"task_type"`
	Status     string `form:"status" json:"status"`
	Page       int    `form:"page" json:"page"`
	PageSize   int    `form:"page_size" json:"page_size"`
}

// SyncLogListQuery 同步日志列表查询
type SyncLogListQuery struct {
	ProviderID uint64 `form:"provider_id" json:"provider_id"`
	TaskID     uint64 `form:"task_id" json:"task_id"`
	Status     string `form:"status" json:"status"`
	Page       int    `form:"page" json:"page"`
	PageSize   int    `form:"page_size" json:"page_size"`
}

// InstanceListQuery 实例列表查询
type InstanceListQuery struct {
	ProviderID uint64 `form:"provider_id" json:"provider_id"`
	UserID     uint64 `form:"user_id" json:"user_id"`
	Status     string `form:"status" json:"status"`
	Keyword    string `form:"keyword" json:"keyword"`
	Page       int    `form:"page" json:"page"`
	PageSize   int    `form:"page_size" json:"page_size"`
}

// CreateSyncTaskRequest 触发同步任务
type CreateSyncTaskRequest struct {
	ProviderID uint64 `json:"provider_id" binding:"required"`
	TaskType   string `json:"task_type" binding:"required"`
}

// SyncTaskInfo 同步任务信息
type SyncTaskInfo struct {
	ID           uint64  `json:"id"`
	ProviderID   uint64  `json:"provider_id"`
	TaskType     string  `json:"task_type"`
	Status       string  `json:"status"`
	TotalCount   int     `json:"total_count"`
	SuccessCount int     `json:"success_count"`
	ErrorMessage string  `json:"error_message"`
	StartedAt    *string `json:"started_at"`
	CompletedAt  *string `json:"completed_at"`
	CreatedAt    string  `json:"created_at"`
}

// SyncLogInfo 同步日志信息
type SyncLogInfo struct {
	ID           uint64 `json:"id"`
	TaskID       uint64 `json:"task_id"`
	ProviderID   uint64 `json:"provider_id"`
	SyncType     string `json:"sync_type"`
	Status       string `json:"status"`
	TotalCount   int    `json:"total_count"`
	SuccessCount int    `json:"success_count"`
	ErrorMessage string `json:"error_message"`
	Details      string `json:"details"`
	CreatedAt    string `json:"created_at"`
}

// InstanceInfo 实例信息
type InstanceInfo struct {
	ID          uint64  `json:"id"`
	InstanceID  string  `json:"instance_id"`
	ProviderID  uint64  `json:"provider_id"`
	UserID      uint64  `json:"user_id"`
	ProductID   uint64  `json:"product_id"`
	SourceMode  string  `json:"source_mode"`
	Name        string  `json:"name"`
	CPU         int     `json:"cpu"`
	Memory      int     `json:"memory"`
	Disk        int     `json:"disk"`
	DiskType    string  `json:"disk_type"`
	Bandwidth   int     `json:"bandwidth"`
	OS          string  `json:"os"`
	Region      string  `json:"region"`
	Zone        string  `json:"zone"`
	Status      string  `json:"status"`
	PrivateIP   string  `json:"private_ip"`
	PublicIP    string  `json:"public_ip"`
	BillingMode string  `json:"billing_mode"`
	ExpireAt    *string `json:"expire_at"`
	CreatedAt   string  `json:"created_at"`
}

// SyncTaskListResponse 同步任务列表响应
type SyncTaskListResponse struct {
	Items []SyncTaskInfo `json:"items"`
	Meta  ListMeta       `json:"meta"`
}

// SyncLogListResponse 同步日志列表响应
type SyncLogListResponse struct {
	Items []SyncLogInfo `json:"items"`
	Meta  ListMeta      `json:"meta"`
}

// InstanceListResponse 实例列表响应
type InstanceListResponse struct {
	Items []InstanceInfo `json:"items"`
	Meta  ListMeta       `json:"meta"`
}

// ============================================================================
// P3 同步框架 DTO（T3.2 ~ T3.5）
// ============================================================================

// SyncScheduleListQuery 调度配置列表查询
type SyncScheduleListQuery struct {
	ProviderID uint64 `form:"provider_id" json:"provider_id"`
	Enabled    *bool  `form:"enabled" json:"enabled"`
	Page       int    `form:"page" json:"page"`
	PageSize   int    `form:"page_size" json:"page_size"`
}

// SyncScheduleUpdateRequest 更新调度配置：节奏/启停/优先级/时间窗。
type SyncScheduleUpdateRequest struct {
	IntervalSeconds         *int  `json:"interval_seconds"`
	FullSyncIntervalSeconds *int  `json:"full_sync_interval_seconds"`
	Enabled                 *bool `json:"enabled"`
	Priority                *int  `json:"priority"`
	// WindowStart/WindowEnd：允许执行的小时区间 0-23；同时传 null 表示不限。
	WindowStart *int16 `json:"window_start"`
	WindowEnd   *int16 `json:"window_end"`
	// WindowClear 为 true 时清空时间窗（与传 null 区分：JSON 无法区分缺省与 null）。
	WindowClear bool `json:"window_clear"`
	// ResetNextRun 为 true 时把 next_run_at 置空，使该调度立即到期。
	ResetNextRun bool `json:"reset_next_run"`
}

// SyncScheduleInfo 调度配置信息
type SyncScheduleInfo struct {
	ID                      uint64  `json:"id"`
	ProviderID              uint64  `json:"provider_id"`
	ProviderName            string  `json:"provider_name"`
	Scope                   string  `json:"scope"`
	ScopeName               string  `json:"scope_name"`
	IntervalSeconds         int     `json:"interval_seconds"`
	FullSyncIntervalSeconds int     `json:"full_sync_interval_seconds"`
	Enabled                 bool    `json:"enabled"`
	Priority                int     `json:"priority"`
	WindowStart             *int16  `json:"window_start"`
	WindowEnd               *int16  `json:"window_end"`
	LastRunAt               *string `json:"last_run_at"`
	NextRunAt               *string `json:"next_run_at"`
	LastStatus              string  `json:"last_status"`
	LastError               string  `json:"last_error"`
}

// SyncScheduleListResponse 调度配置列表响应
type SyncScheduleListResponse struct {
	Items []SyncScheduleInfo `json:"items"`
	Meta  ListMeta           `json:"meta"`
}

// SyncScopeMeta 已注册的 scope 说明（供前端下拉/图例）。
type SyncScopeMeta struct {
	Scope           string `json:"scope"`
	Name            string `json:"name"`
	DefaultInterval int    `json:"default_interval_seconds"`
}

// PriceChangeListQuery 调价事件列表查询
type PriceChangeListQuery struct {
	ProviderID uint64 `form:"provider_id" json:"provider_id"`
	Status     string `form:"status" json:"status"`
	Page       int    `form:"page" json:"page"`
	PageSize   int    `form:"page_size" json:"page_size"`
}

// PriceChangeInfo 调价事件信息
type PriceChangeInfo struct {
	ID                uint64   `json:"id"`
	ProviderID        uint64   `json:"provider_id"`
	Scope             string   `json:"scope"`
	ResourceProductID uint64   `json:"resource_product_id"`
	UpstreamID        string   `json:"upstream_id"`
	ProductID         uint64   `json:"product_id"`
	Field             string   `json:"field"`
	OldValue          *float64 `json:"old_value"`
	NewValue          *float64 `json:"new_value"`
	ChangeRatio       *float64 `json:"change_ratio"`
	Threshold         *float64 `json:"threshold"`
	Status            string   `json:"status"`
	Applied           bool     `json:"applied"`
	Remark            string   `json:"remark"`
	CreatedAt         string   `json:"created_at"`
	HandledAt         *string  `json:"handled_at"`
}

// PriceChangeListResponse 调价事件列表响应
type PriceChangeListResponse struct {
	Items []PriceChangeInfo `json:"items"`
	Meta  ListMeta          `json:"meta"`
}

// PriceChangeHandleRequest 确认/驳回调价（支持批量）。
type PriceChangeHandleRequest struct {
	IDs    []uint64 `json:"ids" binding:"required"`
	Action string   `json:"action" binding:"required"` // confirm | reject
	Remark string   `json:"remark"`
}

// SyncDiffListQuery 差异记录列表查询
type SyncDiffListQuery struct {
	ProviderID uint64 `form:"provider_id" json:"provider_id"`
	TaskID     uint64 `form:"task_id" json:"task_id"`
	Scope      string `form:"scope" json:"scope"`
	Action     string `form:"action" json:"action"`
	Page       int    `form:"page" json:"page"`
	PageSize   int    `form:"page_size" json:"page_size"`
}

// SyncDiffInfo 差异记录信息
type SyncDiffInfo struct {
	ID          uint64 `json:"id"`
	TaskID      uint64 `json:"task_id"`
	ProviderID  uint64 `json:"provider_id"`
	Scope       string `json:"scope"`
	Action      string `json:"action"`
	LocalID     uint64 `json:"local_id"`
	ExternalID  string `json:"external_id"`
	Field       string `json:"field"`
	OldValue    string `json:"old_value"`
	NewValue    string `json:"new_value"`
	Disposition string `json:"disposition"`
	Remark      string `json:"remark"`
	CreatedAt   string `json:"created_at"`
}

// SyncDiffListResponse 差异记录列表响应
type SyncDiffListResponse struct {
	Items []SyncDiffInfo `json:"items"`
	Meta  ListMeta       `json:"meta"`
}

// SyncDiffSummaryProvider 按渠道汇总的差异统计（对账页真实数据源）。
type SyncDiffSummaryProvider struct {
	ProviderID uint64           `json:"provider_id"`
	Total      int64            `json:"total"`
	ByAction   map[string]int64 `json:"by_action"`
}

// SyncDiffSummaryResponse 差异汇总响应
type SyncDiffSummaryResponse struct {
	Items []SyncDiffSummaryProvider `json:"items"`
}
