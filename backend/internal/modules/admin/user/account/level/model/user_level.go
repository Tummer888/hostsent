// Package model 定义用户等级模块的数据库实体。
package model

import "time"

// UserLevel 表示用户等级。
//
// 等级是「消费升级」的载体：按累计消费自动升级（只升不降），等级本身不参与折扣计算，
// 只用于承载权益（feature_flags）与后续的子账号数量上限。
// 原先与等级放在一起的资源配额（模板/上限/调整记录）已移除，见 migrations/017。
type UserLevel struct {
	ID               uint64 `gorm:"primaryKey;autoIncrement"`
	Name             string `gorm:"size:64;not null;uniqueIndex"`
	Code             string `gorm:"size:64;not null;uniqueIndex"`
	Weight           int    `gorm:"not null;default:0"` // 等级权重，用于比较高低（升级判定）
	Status           string `gorm:"size:32;not null;default:active"`
	FeatureFlags     string `gorm:"column:feature_flags;type:text"`
	UpgradeCondition string `gorm:"column:upgrade_condition;size:255"`
	// Benefits 等级权益描述（展示用，JSONB）。
	Benefits string `gorm:"column:benefits;type:jsonb"`
	// UpgradeThreshold 消费升级门槛（累计消费 ≥ 该值即达此等级）。
	UpgradeThreshold float64 `gorm:"column:upgrade_threshold;type:decimal(15,2);not null;default:0"`
	// MaxSubAccounts 该等级可创建的成员（子账号）数量上限。
	MaxSubAccounts int `gorm:"column:max_sub_accounts;not null;default:0"`
	// Icon 等级图标标识（前端图标目录的 key，如 star-filled）。
	// 存 key 而不是图标 URL：配色随主题变化（暗色模式、品牌色替换），
	// 存死图片链接会在换主题时失效，而组件图标能直接继承当前前景色。
	Icon string `gorm:"size:64;not null;default:''"`
	// Color 等级主题色（#RRGGBB）。运营自定义后，用户端与管理端同一等级同色。
	// 空值表示未配置，前端回落到按权重推导的默认色阶。
	Color string `gorm:"size:32;not null;default:''"`
	// Description 等级说明文案。
	Description string    `gorm:"size:255"`
	CreatedBy   uint64    `gorm:"column:created_by;not null;default:0"`
	UpdatedBy   uint64    `gorm:"column:updated_by;not null;default:0"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名。
func (UserLevel) TableName() string {
	return "user_levels"
}
