package model

import "time"

// ResourceQuota 记录某个用户在某个配额编码下的当前配额状态。
type ResourceQuota struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement"`
	UserID         uint64    `gorm:"column:user_id;not null;index;uniqueIndex:idx_resource_quotas_user_code"`
	QuotaCode      string    `gorm:"column:quota_code;size:64;not null;uniqueIndex:idx_resource_quotas_user_code"`
	QuotaName      string    `gorm:"column:quota_name;size:64;not null"`
	QuotaType      string    `gorm:"column:quota_type;size:32;not null"`
	LimitValue     float64   `gorm:"column:limit_value;type:decimal(15,2);not null;default:0"`
	UsedValue      float64   `gorm:"column:used_value;type:decimal(15,2);not null;default:0"`
	AvailableValue float64   `gorm:"column:available_value;type:decimal(15,2);not null;default:0"`
	Unit           string    `gorm:"size:32;not null;default:count"`
	Status         string    `gorm:"size:32;not null;default:active"`
	Source         string    `gorm:"size:32;not null;default:system"`
	TemplateID     *uint64   `gorm:"column:template_id"`
	LevelID        *uint64   `gorm:"column:level_id"`
	IsOverallocated bool     `gorm:"column:is_overallocated;not null;default:false"`
	UpdatedBy      uint64    `gorm:"column:updated_by;not null;default:0"`
	LastAdjustedAt time.Time `gorm:"column:last_adjusted_at;autoCreateTime"`
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`
}

func (ResourceQuota) TableName() string {
	return "resource_quotas"
}
