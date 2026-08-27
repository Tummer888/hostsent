// Package model 定义配额模块的数据库实体。
package model

import "time"

// QuotaTemplate 表示一套可复用的配额模板定义。
type QuotaTemplate struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	Name        string    `gorm:"size:64;not null;uniqueIndex"`
	Code        string    `gorm:"size:64;not null;uniqueIndex"`
	Scope       string    `gorm:"size:32;not null;default:default"`
	Status      string    `gorm:"size:32;not null;default:active"`
	Description string    `gorm:"size:255"`
	Version     int       `gorm:"not null;default:1"`
	CreatedBy   uint64    `gorm:"column:created_by;not null;default:0"`
	UpdatedBy   uint64    `gorm:"column:updated_by;not null;default:0"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (QuotaTemplate) TableName() string {
	return "quota_templates"
}
