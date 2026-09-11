// Package model 提供资源同步子域的数据模型。
package model

import "time"

// SyncTask 同步任务
type SyncTask struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement"`
	ProviderID   uint64     `gorm:"column:provider_id;not null;index"`
	TaskType     string     `gorm:"column:task_type;size:50;not null"`
	Status       string     `gorm:"size:20;default:pending;index"`
	TotalCount   int        `gorm:"column:total_count;default:0"`
	SuccessCount int        `gorm:"column:success_count;default:0"`
	ErrorMessage string     `gorm:"column:error_message;type:text"`
	StartedAt    *time.Time `gorm:"column:started_at"`
	CompletedAt  *time.Time `gorm:"column:completed_at"`
	CreatedAt    time.Time  `gorm:"autoCreateTime"`
}

// TableName 指定表名
func (SyncTask) TableName() string {
	return "sync_tasks"
}

// SyncLog 同步日志
type SyncLog struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	TaskID       uint64    `gorm:"column:task_id;index"`
	ProviderID   uint64    `gorm:"column:provider_id;not null;index"`
	SyncType     string    `gorm:"column:sync_type;size:50;not null"`
	Status       string    `gorm:"size:20;default:success;index"`
	TotalCount   int       `gorm:"column:total_count;default:0"`
	SuccessCount int       `gorm:"column:success_count;default:0"`
	ErrorMessage string    `gorm:"column:error_message;type:text"`
	Details      string    `gorm:"type:text"` // JSON 以 text 存储
	CreatedAt    time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名
func (SyncLog) TableName() string {
	return "sync_logs"
}

// 实例链路判据（P1/T1.2，D6）：单一字段区分自营与上游，禁止混用。
const (
	// SourceModeSelf 经我方订单/后台开通的自营链路实例。
	SourceModeSelf = "self"
	// SourceModeUpstream 由上游渠道同步发现的上游链路实例。
	SourceModeUpstream = "upstream"
)

// Instance 统一实例主数据
type Instance struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement"`
	InstanceID  string `gorm:"column:instance_id;size:64;uniqueIndex;not null"`
	ProviderID  uint64 `gorm:"column:provider_id;not null;index"`
	UserID      uint64 `gorm:"column:user_id;not null;index"`
	ProductID   uint64 `gorm:"column:product_id"`
	Name        string `gorm:"size:100;not null"`
	CPU         int    `gorm:"not null"`
	Memory      int    `gorm:"not null"`
	Disk        int    `gorm:"not null"`
	DiskType    string `gorm:"column:disk_type;size:20"`
	Bandwidth   int    `gorm:"default:0"`
	OS          string `gorm:"size:50"`
	Region      string `gorm:"size:50"`
	Zone        string `gorm:"size:50"`
	Status      string `gorm:"size:30;default:creating;index"`
	PrivateIP   string `gorm:"column:private_ip;size:15"`
	PublicIP    string `gorm:"column:public_ip;size:15"`
	RawData     string `gorm:"column:raw_data;type:text"` // JSON 以 text 存储
	BillingMode string `gorm:"column:billing_mode;size:20;default:hourly"`
	// ActorUserID 开通该实例的真实操作人（子账号下单时为主账号名下的子账号，P4-09）
	ActorUserID uint64 `gorm:"column:actor_user_id;index"`
	// OrderID 开通该实例的来源订单；存量数据为 0（见 61 实施计划 §4.1）。
	OrderID uint64 `gorm:"column:order_id;index"`
	// Remark 管理员内部备注。注意：同步流程（UpsertInstances）不得覆盖此列。
	Remark string `gorm:"size:255"`
	// LastSyncedAt 最近一次单实例回源上游的时间。
	LastSyncedAt *time.Time `gorm:"column:last_synced_at"`
	// ---- 双链路语义列（P1/T1.2）----
	// SourceMode 链路判据：self=经我方订单/后台开通，upstream=由上游同步发现。
	SourceMode string `gorm:"column:source_mode;size:16"`
	// SellProductID 自营链路售出商品（products.id）；上游链路为空。
	SellProductID uint64 `gorm:"column:sell_product_id"`
	// UpstreamProductID 上游链路资源商品（resource_products.id）；自营链路为空。
	UpstreamProductID uint64 `gorm:"column:upstream_product_id"`
	// ProviderInstanceID 上游/平台侧的实例号（= instance_id，独立列便于多平台对接）。
	ProviderInstanceID string `gorm:"column:provider_instance_id;size:128"`
	// UpstreamOrderID 上游订单号（链路 A 续费/对账用）。
	UpstreamOrderID string `gorm:"column:upstream_order_id;size:128"`
	// LifecycleStage 生命周期阶段（T5.4 推进器使用）。
	LifecycleStage string     `gorm:"column:lifecycle_stage;size:24"`
	CreatedAt      time.Time  `gorm:"autoCreateTime"`
	ExpireAt       *time.Time `gorm:"column:expire_at"`
	UpdatedAt      time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (Instance) TableName() string {
	return "instances"
}
