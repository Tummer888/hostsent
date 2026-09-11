// Package model 提供上游提供商模块的数据模型。
package model

import (
	"time"

	"gorm.io/gorm"
)

// 渠道链路类型（双链路重构）：upstream=上游转售，compute=算力平台（自营执行器）。
const (
	KindUpstream = "upstream"
	KindCompute  = "compute"
)

// ResourceProvider 上游提供商
type ResourceProvider struct {
	ID               uint64     `gorm:"primaryKey;autoIncrement"`
	Name             string     `gorm:"size:100;not null"`
	ProviderType     string     `gorm:"column:provider_type;size:50;not null;index"`
	APIEndpoint      string     `gorm:"column:api_endpoint;size:255;not null"`
	APIKey           string     `gorm:"column:api_key;size:255;not null"`
	APISecret        string     `gorm:"column:api_secret;size:255;not null"`
	Region           string     `gorm:"size:50"`
	ContactWay       string     `gorm:"column:contact_way;size:100"`
	Des              string     `gorm:"column:des;size:255"`
	UpstreamType     string     `gorm:"column:upstream_type;size:50"`         // 接口类型：zjmf_api 等
	ZjmfFinanceAPIID uint64     `gorm:"column:zjmf_finance_api_id;default:0"` // createApi 返回的凭证 id
	Port             string     `gorm:"column:port;size:20"`                  // 魔方云：接口端口
	Secure           bool       `gorm:"column:secure;default:false"`          // 魔方云：是否 https(1是 0否)
	Disabled         bool       `gorm:"column:disabled;default:false"`        // 魔方云：是否禁用(0启用 1禁用)
	UserPrefix       string     `gorm:"column:user_prefix;size:50"`           // 魔方云：财务标识
	AccountType      string     `gorm:"column:account_type;size:20"`          // 魔方云：账号类型 admin/agent
	Status           int        `gorm:"default:1;index"`
	SyncEnabled      bool       `gorm:"column:sync_enabled;default:true"`
	SyncInterval     int        `gorm:"column:sync_interval;default:3600"`
	LastSyncAt       *time.Time `gorm:"column:last_sync_at"`
	TotalCPU         int        `gorm:"column:total_cpu;default:0"`
	TotalMemory      int        `gorm:"column:total_memory;default:0"`
	TotalDisk        int        `gorm:"column:total_disk;default:0"`
	UsedCPU          int        `gorm:"column:used_cpu;default:0"`
	UsedMemory       int        `gorm:"column:used_memory;default:0"`
	UsedDisk         int        `gorm:"column:used_disk;default:0"`
	// 链路类型：upstream=上游转售（目录/定价/生命周期在上游），compute=算力平台（自营执行器）
	Kind string `gorm:"column:kind;size:16;not null;default:upstream"`
	// 同步健康：熔断暂停、连续失败计数、最近错误与最近成功时间（P0/T0.2~T0.3）
	SyncPaused          bool       `gorm:"column:sync_paused;not null;default:false"`
	ConsecutiveFailures int        `gorm:"column:consecutive_failures;not null;default:0"`
	LastSyncError       string     `gorm:"column:last_sync_error;type:text"`
	LastSuccessAt       *time.Time `gorm:"column:last_success_at"`
	// ---- 契约层（P2/T2.3~T2.4）----
	// Credentials 可扩展凭证 JSON（凭证字段级加密，见 credentials.go）。
	// 旧 api_key/api_secret 保留一版只读；新代码一律优先读本列。
	Credentials string `gorm:"column:credentials;type:jsonb"`
	// TimeoutSeconds 单次上游调用超时（秒）；<=0 取 transport.DefaultTimeout。
	TimeoutSeconds int `gorm:"column:timeout_seconds;not null;default:0"`
	// RetryMax 额外重试次数（传输层退避重试）；<=0 不重试。
	RetryMax int `gorm:"column:retry_max;not null;default:0"`
	// RateLimitQPS 渠道级令牌桶 QPS；<=0 表示不限流。
	RateLimitQPS int `gorm:"column:rate_limit_qps;not null;default:0"`
	// PriceChangeThreshold 上游成本价变动自动应用阈值（比例，0.05=5%）；
	// 超过阈值写 price_change_events 待人工确认，绝不静默改售价（P3/T3.4）。
	PriceChangeThreshold float64        `gorm:"column:price_change_threshold;type:numeric(10,4);not null;default:0.05"`
	CreatedAt            time.Time      `gorm:"autoCreateTime"`
	UpdatedAt            time.Time      `gorm:"autoUpdateTime"`
	DeletedAt            gorm.DeletedAt `gorm:"index"`
}

// TableName 指定表名
func (ResourceProvider) TableName() string {
	return "resource_providers"
}

// ProviderType 渠道类型注册表（T2.2）：把 provider_service 里硬编码的类型展示 map
// 改为查表，新增一个渠道系统只需插入一行 + 一个适配器包。
// DescriptorJSON 存 CapabilityDescriptor（含 CredentialSchema 与平台字段字典）。
type ProviderType struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement"`
	Type           string    `gorm:"column:type;size:50;not null;uniqueIndex:uk_provider_types_type"`
	Name           string    `gorm:"size:100;not null"`
	Kind           string    `gorm:"column:kind;size:16;not null;default:upstream"`
	DescriptorJSON string    `gorm:"column:descriptor;type:jsonb"`
	Icon           string    `gorm:"column:icon;size:64"`
	DocURL         string    `gorm:"column:doc_url;size:255"`
	AdapterVersion string    `gorm:"column:adapter_version;size:32"`
	SortOrder      int       `gorm:"column:sort_order;not null;default:0"`
	Status         int       `gorm:"not null;default:1"`
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (ProviderType) TableName() string {
	return "provider_types"
}

// ResourcePool 资源池
type ResourcePool struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement"`
	ProviderID  uint64     `gorm:"column:provider_id;not null;uniqueIndex:idx_pool_provider_upstream"`
	UpstreamID  string     `gorm:"column:upstream_id;size:64;not null;uniqueIndex:idx_pool_provider_upstream"`
	Name        string     `gorm:"size:100;not null"`
	PoolType    string     `gorm:"column:pool_type;size:20"`
	TotalCPU    int        `gorm:"column:total_cpu;default:0"`
	TotalMemory int        `gorm:"column:total_memory;default:0"`
	TotalDisk   int        `gorm:"column:total_disk;default:0"`
	UsedCPU     int        `gorm:"column:used_cpu;default:0"`
	UsedMemory  int        `gorm:"column:used_memory;default:0"`
	UsedDisk    int        `gorm:"column:used_disk;default:0"`
	Status      int        `gorm:"default:1"`
	LastSyncAt  *time.Time `gorm:"column:last_sync_at"`
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (ResourcePool) TableName() string {
	return "resource_pools"
}
