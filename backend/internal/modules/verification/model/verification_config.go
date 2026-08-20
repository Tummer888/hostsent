// Package model 提供实名认证模块的数据模型。
package model

import "time"

// VerificationConfig 表示实名认证配置项。
type VerificationConfig struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	ConfigKey   string    `gorm:"column:config_key;size:128;not null;uniqueIndex"`
	ConfigGroup string    `gorm:"column:config_group;size:64;not null;index"`
	ConfigValue string    `gorm:"column:config_value;type:text;not null"`
	ValueType   string    `gorm:"column:value_type;size:32;not null"`
	Status      string    `gorm:"column:status;size:32;not null;index"`
	Description string    `gorm:"column:description;size:255"`
	UpdatedBy   uint64    `gorm:"column:updated_by;not null"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// TableName 返回实名认证配置表名。
func (VerificationConfig) TableName() string {
	return "verification_configs"
}
