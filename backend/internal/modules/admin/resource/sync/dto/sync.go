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
