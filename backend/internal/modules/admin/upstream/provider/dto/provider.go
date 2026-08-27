package dto

import commondto "hostsent/backend/internal/modules/admin/upstream/common/dto"

// ListMeta 通用分页元信息（复用 common 包定义）
type ListMeta = commondto.ListMeta

// ProviderListQuery 上游提供商列表查询
type ProviderListQuery struct {
	Keyword      string `form:"keyword" json:"keyword"`
	ProviderType string `form:"provider_type" json:"provider_type"`
	Status       int    `form:"status" json:"status"`
	Page         int    `form:"page" json:"page"`
	PageSize     int    `form:"page_size" json:"page_size"`
}

// ProviderCreateRequest 添加上游提供商
type ProviderCreateRequest struct {
	Name         string `json:"name" binding:"required"`
	ProviderType string `json:"provider_type" binding:"required"`
	APIEndpoint  string `json:"api_endpoint" binding:"required"`
	APIKey       string `json:"api_key"`
	APISecret    string `json:"api_secret"`
	Region       string `json:"region"`
	Status       int    `json:"status"`
	SyncEnabled  bool   `json:"sync_enabled"`
	SyncInterval int    `json:"sync_interval"`
}

// ProviderUpdateRequest 更新上游提供商
type ProviderUpdateRequest struct {
	Name         string `json:"name"`
	APIEndpoint  string `json:"api_endpoint"`
	APIKey       string `json:"api_key"`
	APISecret    string `json:"api_secret"`
	Region       string `json:"region"`
	Status       int    `json:"status"`
	SyncEnabled  bool   `json:"sync_enabled"`
	SyncInterval int    `json:"sync_interval"`
}

// ProviderInfo 上游提供商信息
type ProviderInfo struct {
	ID           uint64 `json:"id"`
	Name         string `json:"name"`
	ProviderType string `json:"provider_type"`
	APIEndpoint  string `json:"api_endpoint"`
	APIKey       string `json:"api_key"`
	APISecret    string `json:"api_secret"`
	Region       string `json:"region"`
	Status       int    `json:"status"`
	SyncEnabled  bool   `json:"sync_enabled"`
	SyncInterval int    `json:"sync_interval"`
	LastSyncAt   *string `json:"last_sync_at"`
	TotalCPU     int    `json:"total_cpu"`
	TotalMemory  int    `json:"total_memory"`
	TotalDisk    int    `json:"total_disk"`
	UsedCPU      int    `json:"used_cpu"`
	UsedMemory   int    `json:"used_memory"`
	UsedDisk     int    `json:"used_disk"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// ProviderListResponse 提供商列表响应
type ProviderListResponse struct {
	Items []ProviderInfo `json:"items"`
	Meta  ListMeta       `json:"meta"`
}

// ProviderTypeItem 支持的提供商类型
type ProviderTypeItem struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// TestConnectionResult 连接测试结果
type TestConnectionResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// PoolListQuery 资源池列表查询
type PoolListQuery struct {
	ProviderID uint64 `form:"provider_id" json:"provider_id"`
	Page       int    `form:"page" json:"page"`
	PageSize   int    `form:"page_size" json:"page_size"`
}

// PoolInfo 资源池信息
type PoolInfo struct {
	ID          uint64  `json:"id"`
	ProviderID  uint64  `json:"provider_id"`
	UpstreamID  string  `json:"upstream_id"`
	Name        string  `json:"name"`
	PoolType    string  `json:"pool_type"`
	TotalCPU    int     `json:"total_cpu"`
	TotalMemory int     `json:"total_memory"`
	TotalDisk   int     `json:"total_disk"`
	UsedCPU     int     `json:"used_cpu"`
	UsedMemory  int     `json:"used_memory"`
	UsedDisk    int     `json:"used_disk"`
	Status      int     `json:"status"`
	LastSyncAt  *string `json:"last_sync_at"`
}

// PoolListResponse 资源池列表响应
type PoolListResponse struct {
	Items []PoolInfo `json:"items"`
	Meta  ListMeta   `json:"meta"`
}
