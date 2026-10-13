// Package model 定义用户中心主机（实例）数据模型。
package model

import "time"

// Instance 用户侧云主机记录（映射 instances 表）。
type Instance struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	InstanceID  string `gorm:"column:instance_id;size:64;uniqueIndex" json:"instance_id"`
	ProviderID  uint64 `gorm:"column:provider_id;not null;index" json:"provider_id"`
	UserID      uint64 `gorm:"column:user_id;not null;index" json:"user_id"`
	ProductID   uint64 `gorm:"column:product_id" json:"product_id"`
	Name        string `gorm:"size:100" json:"name"`
	CPU         int    `json:"cpu"`
	Memory      int    `json:"memory"`
	Disk        int    `json:"disk"`
	DiskType    string `gorm:"column:disk_type;size:20" json:"disk_type"`
	Bandwidth   int    `json:"bandwidth"`
	OS          string `gorm:"size:50" json:"os"`
	Region      string `gorm:"size:50" json:"region"`
	Zone        string `gorm:"size:50" json:"zone"`
	Status      string `gorm:"size:30;default:creating" json:"status"`
	PrivateIP   string `gorm:"column:private_ip;size:15" json:"private_ip"`
	PublicIP    string `gorm:"column:public_ip;size:15" json:"public_ip"`
	RawData     string `gorm:"column:raw_data;type:text" json:"-"`
	BillingMode string `gorm:"column:billing_mode;size:20;default:hourly" json:"billing_mode"`
	// ActorUserID 开通该实例的真实操作人（P4-09）；ActorName 由查询 join users 填充，不落库。
	ActorUserID uint64 `gorm:"column:actor_user_id;index" json:"actor_user_id"`
	ActorName   string `gorm:"-" json:"actor_name"`
	// ---- 待开通占位（下单即建）与来源订单 ----
	// OrderID 来源订单（重新开通按它重投履约任务）；ProviderInstanceID 上游实例号，
	// 为空表示"尚未在上游开通"（待开通占位行）。
	OrderID            uint64 `gorm:"column:order_id;index" json:"order_id"`
	ProviderInstanceID string `gorm:"column:provider_instance_id;size:128" json:"provider_instance_id"`
	// LifecycleStage 生命周期阶段（active/grace/suspended/destroyed）：与服务状态
	// status 是两个维度——到期停机会把阶段推进到 suspended，status 同时被置 stopped；
	// 前端要把「到期暂停」与「用户自己关机」区分开，必须两个字段一起看。
	LifecycleStage string     `gorm:"column:lifecycle_stage;size:20" json:"lifecycle_stage"`
	CreatedAt      time.Time  `gorm:"autoCreateTime" json:"created_at"`
	ExpireAt       *time.Time `gorm:"column:expire_at" json:"expire_at"`
	UpdatedAt      time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名。
func (Instance) TableName() string { return "instances" }
