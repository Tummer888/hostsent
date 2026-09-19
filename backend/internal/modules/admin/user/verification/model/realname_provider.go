// Package model 提供实名认证模块的数据模型。
package model

import "time"

// RealnameProvider 实名核验服务商配置（realname_providers，迁移 051 建表）。
//
// 形态逐列对齐 captcha_providers：descriptor 存能力描述符快照（便于审计「当时的
// 字段定义是什么」），credentials 存字段级加密后的凭证 JSON。
type RealnameProvider struct {
	ID           uint64 `gorm:"primaryKey;autoIncrement"`
	ProviderType string `gorm:"column:provider_type;size:50;not null;index"`
	Name         string `gorm:"size:100;not null"`
	// Mode manual / api / redirect，由描述符决定，落库便于列表直接展示。
	Mode         string     `gorm:"size:20;not null;default:'manual'"`
	Descriptor   string     `gorm:"type:jsonb"`
	Credentials  string     `gorm:"type:jsonb"`
	Endpoint     string     `gorm:"size:255;not null;default:''"`
	Priority     int64      `gorm:"not null;default:0"`
	HealthStatus string     `gorm:"column:health_status;size:20;not null;default:''"`
	LastError    string     `gorm:"column:last_error;type:text"`
	LastCheckAt  *time.Time `gorm:"column:last_check_at"`
	// Status 1 启用 / 0 停用（对齐 captcha_providers 的整数状态）。
	Status    int        `gorm:"not null;default:1"`
	IsDefault bool       `gorm:"column:is_default;not null;default:false"`
	Remark    string     `gorm:"size:255;not null;default:''"`
	CreatedAt time.Time  `gorm:"autoCreateTime"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime"`
	DeletedAt *time.Time `gorm:"index"`
}

// TableName 返回实名核验服务商表名。
func (RealnameProvider) TableName() string {
	return "realname_providers"
}
