package model

import "time"

type Blacklist struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement"`
	Type        string     `gorm:"size:32;not null;index:idx_blacklists_type_target,unique"`
	TargetValue string     `gorm:"column:target_value;size:255;not null;index:idx_blacklists_type_target,unique"`
	Status      string     `gorm:"size:32;not null;index"`
	Source      string     `gorm:"size:32;not null;index"`
	Reason      string     `gorm:"size:255"`
	EffectiveAt time.Time  `gorm:"column:effective_at;not null;index"`
	ExpiredAt   *time.Time `gorm:"column:expired_at"`
	HitCount    int        `gorm:"column:hit_count;not null;default:0"`
	CreatedBy   uint64     `gorm:"column:created_by;not null;index"`
	UpdatedBy   uint64     `gorm:"column:updated_by;not null;index"`
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`
}

func (Blacklist) TableName() string {
	return "blacklists"
}
