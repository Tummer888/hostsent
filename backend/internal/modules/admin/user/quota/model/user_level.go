// Package model 定义配额模块的数据库实体。
package model

import "time"

// UserLevel 表示分配给用户等级的配额能力配置。
type UserLevel struct {
	ID                uint64    `gorm:"primaryKey;autoIncrement"`
	Name              string    `gorm:"size:64;not null;uniqueIndex"`
	Code              string    `gorm:"size:64;not null;uniqueIndex"`
	Weight            int       `gorm:"not null;default:0"`
	Status            string    `gorm:"size:32;not null;default:active"`
	DefaultTemplateID *uint64   `gorm:"column:default_template_id"`
	MaxInstanceCount  int       `gorm:"column:max_instance_count;not null;default:0"`
	MaxCPUCores       int       `gorm:"column:max_cpu_cores;not null;default:0"`
	MaxMemoryGB       int       `gorm:"column:max_memory_gb;not null;default:0"`
	MaxDiskGB         int       `gorm:"column:max_disk_gb;not null;default:0"`
	FeatureFlags      string    `gorm:"column:feature_flags;type:text"`
	UpgradeCondition  string    `gorm:"column:upgrade_condition;size:255"`
	Description       string    `gorm:"size:255"`
	CreatedBy         uint64    `gorm:"column:created_by;not null;default:0"`
	UpdatedBy         uint64    `gorm:"column:updated_by;not null;default:0"`
	CreatedAt         time.Time `gorm:"autoCreateTime"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime"`
}

func (UserLevel) TableName() string {
	return "user_levels"
}
