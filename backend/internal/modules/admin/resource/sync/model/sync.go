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
	ActorUserID uint64     `gorm:"column:actor_user_id;index"`
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
	ExpireAt    *time.Time `gorm:"column:expire_at"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (Instance) TableName() string {
	return "instances"
}
