// Package model 定义用户等级模块的数据库实体。
package model

import "time"

// UserLevel 表示用户等级。
//
// 等级是「消费升级」的载体：按累计消费自动升级（只升不降），等级本身不参与折扣计算，
// 只用于承载权益（feature_flags）与后续的子账号数量上限。
// 原先与等级放在一起的资源配额（模板/上限/调整记录）已移除，见 migrations/017。
type UserLevel struct {
	ID               uint64    `gorm:"primaryKey;autoIncrement"`
	Name             string    `gorm:"size:64;not null;uniqueIndex"`
	Code             string    `gorm:"size:64;not null;uniqueIndex"`
	Weight           int       `gorm:"not null;default:0"` // 等级权重，用于比较高低（升级判定）
	Status           string    `gorm:"size:32;not null;default:active"`
	FeatureFlags     string    `gorm:"column:feature_flags;type:text"`
	UpgradeCondition string    `gorm:"column:upgrade_condition;size:255"`
	Description      string    `gorm:"size:255"`
	CreatedBy        uint64    `gorm:"column:created_by;not null;default:0"`
	UpdatedBy        uint64    `gorm:"column:updated_by;not null;default:0"`
	CreatedAt        time.Time `gorm:"autoCreateTime"`
	UpdatedAt        time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名。
func (UserLevel) TableName() string {
	return "user_levels"
}
