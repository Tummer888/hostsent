// Package model 定义用户中心主机（实例）数据模型。
package model

import "time"

// Instance 用户侧云主机记录（映射 instances 表）。
type Instance struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	InstanceID  string     `gorm:"column:instance_id;size:64;uniqueIndex" json:"instance_id"`
	ProviderID  uint64     `gorm:"column:provider_id;not null;index" json:"provider_id"`
	UserID      uint64     `gorm:"column:user_id;not null;index" json:"user_id"`
	ProductID   uint64     `gorm:"column:product_id" json:"product_id"`
	Name        string     `gorm:"size:100" json:"name"`
	CPU         int        `json:"cpu"`
	Memory      int        `json:"memory"`
	Disk        int        `json:"disk"`
	DiskType    string     `gorm:"column:disk_type;size:20" json:"disk_type"`
	Bandwidth   int        `json:"bandwidth"`
	OS          string     `gorm:"size:50" json:"os"`
	Region      string     `gorm:"size:50" json:"region"`
	Zone        string     `gorm:"size:50" json:"zone"`
	Status      string     `gorm:"size:30;default:creating" json:"status"`
	PrivateIP   string     `gorm:"column:private_ip;size:15" json:"private_ip"`
	PublicIP    string     `gorm:"column:public_ip;size:15" json:"public_ip"`
	RawData     string     `gorm:"column:raw_data;type:text" json:"-"`
	BillingMode string     `gorm:"column:billing_mode;size:20;default:hourly" json:"billing_mode"`
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
	ExpireAt    *time.Time `gorm:"column:expire_at" json:"expire_at"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名。
func (Instance) TableName() string { return "instances" }
