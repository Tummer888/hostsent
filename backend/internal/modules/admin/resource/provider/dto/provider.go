package dto

import (
	commondto "hostsent/backend/internal/modules/admin/resource/common/dto"
	"hostsent/backend/internal/pkg/upstream"
)

// ListMeta 通用分页元信息（复用 common 包定义）
type ListMeta = commondto.ListMeta

// ProviderListQuery 上游提供商列表查询
type ProviderListQuery struct {
	Keyword      string `form:"keyword" json:"keyword"`
	ProviderType string `form:"provider_type" json:"provider_type"`
	Kind         string `form:"kind" json:"kind"` // upstream/compute（链路分组，T1.3）
	Status       int    `form:"status" json:"status"`
	Page         int    `form:"page" json:"page"`
	PageSize     int    `form:"page_size" json:"page_size"`
}

// ProviderCreateRequest 添加上游提供商
type ProviderCreateRequest struct {
	Name             string `json:"name" binding:"required"`
	ProviderType     string `json:"provider_type" binding:"required"`
	APIEndpoint      string `json:"api_endpoint" binding:"required"`
	APIKey           string `json:"api_key"`
	APISecret        string `json:"api_secret"`
	Region           string `json:"region"`
	ContactWay       string `json:"contact_way"`
	Des              string `json:"des"`
	UpstreamType     string `json:"upstream_type"`
	ZjmfFinanceAPIID uint64 `json:"zjmf_finance_api_id"`
	Port             string `json:"port"`
	Secure           bool   `json:"secure"`
	Disabled         bool   `json:"disabled"`
	UserPrefix       string `json:"user_prefix"`
	AccountType      string `json:"account_type"`
	Status           int    `json:"status"`
	SyncEnabled      bool   `json:"sync_enabled"`
	SyncInterval     int    `json:"sync_interval"`
	// ---- 契约层（P2/T2.3~T2.4）----
	// Credentials 动态凭证（由渠道类型的 CredentialSchema 驱动，secret 字段加密落库）；
	// 与 api_key/api_secret 同时传入时以本字段为准（旧字段保留兼容）。
	Credentials    map[string]string `json:"credentials"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	RetryMax       int               `json:"retry_max"`
	RateLimitQPS   int               `json:"rate_limit_qps"`
	// PriceChangeThreshold 上游成本价变动自动应用阈值（比例，0.05=5%，P3/T3.4）。
	PriceChangeThreshold float64 `json:"price_change_threshold"`
	// OpsConsoleURL 上游/平台侧运维控制台地址，后台一键跳转（本轮 S2）。
	OpsConsoleURL string `json:"ops_console_url"`
}

// ProviderUpdateRequest 更新上游提供商
type ProviderUpdateRequest struct {
	Name             string `json:"name"`
	APIEndpoint      string `json:"api_endpoint"`
	APIKey           string `json:"api_key"`
	APISecret        string `json:"api_secret"`
	Region           string `json:"region"`
	ContactWay       string `json:"contact_way"`
	Des              string `json:"des"`
	UpstreamType     string `json:"upstream_type"`
	ZjmfFinanceAPIID uint64 `json:"zjmf_finance_api_id"`
	Port             string `json:"port"`
	Secure           bool   `json:"secure"`
	Disabled         bool   `json:"disabled"`
	UserPrefix       string `json:"user_prefix"`
	AccountType      string `json:"account_type"`
	Status           int    `json:"status"`
	SyncEnabled      bool   `json:"sync_enabled"`
	SyncInterval     int    `json:"sync_interval"`
	// Credentials 动态凭证；值为空或等于当前脱敏回显时不覆盖已存值。
	Credentials    map[string]string `json:"credentials"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	RetryMax       int               `json:"retry_max"`
	RateLimitQPS   int               `json:"rate_limit_qps"`
	// PriceChangeThreshold 上游成本价变动自动应用阈值（比例，0.05=5%，P3/T3.4）。
	PriceChangeThreshold float64 `json:"price_change_threshold"`
	// OpsConsoleURL 上游/平台侧运维控制台地址，后台一键跳转（本轮 S2）。
	OpsConsoleURL string `json:"ops_console_url"`
}

// ProviderInfo 上游提供商信息
type ProviderInfo struct {
	ID               uint64  `json:"id"`
	Name             string  `json:"name"`
	ProviderType     string  `json:"provider_type"`
	APIEndpoint      string  `json:"api_endpoint"`
	APIKey           string  `json:"api_key"`
	APISecret        string  `json:"api_secret"`
	Region           string  `json:"region"`
	ContactWay       string  `json:"contact_way"`
	Des              string  `json:"des"`
	UpstreamType     string  `json:"upstream_type"`
	ZjmfFinanceAPIID uint64  `json:"zjmf_finance_api_id"`
	Port             string  `json:"port"`
	Secure           bool    `json:"secure"`
	Disabled         bool    `json:"disabled"`
	UserPrefix       string  `json:"user_prefix"`
	AccountType      string  `json:"account_type"`
	Status           int     `json:"status"`
	SyncEnabled      bool    `json:"sync_enabled"`
	SyncInterval     int     `json:"sync_interval"`
	LastSyncAt       *string `json:"last_sync_at"`
	// 链路与同步健康（P0）：kind 区分上游转售/算力平台；sync_paused 为熔断暂停，
	// last_sync_error 展示暂停原因，供后台「一键恢复」判断。
	Kind                string  `json:"kind"`
	SyncPaused          bool    `json:"sync_paused"`
	ConsecutiveFailures int     `json:"consecutive_failures"`
	LastSyncError       string  `json:"last_sync_error"`
	LastSuccessAt       *string `json:"last_success_at"`
	TotalCPU            int     `json:"total_cpu"`
	TotalMemory         int     `json:"total_memory"`
	TotalDisk           int     `json:"total_disk"`
	UsedCPU             int     `json:"used_cpu"`
	UsedMemory          int     `json:"used_memory"`
	UsedDisk            int     `json:"used_disk"`
	// ---- 契约层（P2/T2.3~T2.4）----
	// Credentials 脱敏凭证快照（secret 字段仅首尾 2 位），供动态表单回显。
	Credentials    map[string]string `json:"credentials"`
	CredentialKeys []string          `json:"credential_keys"`
	// CredentialError 非空表示凭证解密失败（L8：显式报错而非回退明文），需重新录入。
	CredentialError string `json:"credential_error"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
	RetryMax        int    `json:"retry_max"`
	RateLimitQPS    int    `json:"rate_limit_qps"`
	// PriceChangeThreshold 上游成本价变动自动应用阈值（比例，P3/T3.4）。
	PriceChangeThreshold float64 `json:"price_change_threshold"`
	// OpsConsoleURL 上游/平台侧运维控制台地址，后台一键跳转（本轮 S2）。
	OpsConsoleURL string `json:"ops_console_url"`
	// Capabilities 渠道能力描述符，供后台能力矩阵展示。
	Capabilities upstream.CapabilityDescriptor `json:"capabilities"`
	CreatedAt    string                        `json:"created_at"`
	UpdatedAt    string                        `json:"updated_at"`
}

// ProviderListResponse 提供商列表响应
type ProviderListResponse struct {
	Items []ProviderInfo `json:"items"`
	Meta  ListMeta       `json:"meta"`
}

// ProviderTypeItem 支持的提供商类型（来自 provider_types 注册表）
type ProviderTypeItem struct {
	Type string `json:"type"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	// Implemented 是否已注册适配器（false 时后台可展示但连接测试会明确失败）。
	Implemented    bool                          `json:"implemented"`
	AdapterVersion string                        `json:"adapter_version"`
	DocURL         string                        `json:"doc_url"`
	Icon           string                        `json:"icon"`
	Capabilities   upstream.CapabilityDescriptor `json:"capabilities"`
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
