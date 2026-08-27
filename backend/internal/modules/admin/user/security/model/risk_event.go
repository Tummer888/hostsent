package model

import "time"

type RiskEvent struct {
	ID               uint64     `gorm:"primaryKey;autoIncrement"`
	RiskType         string     `gorm:"column:risk_type;size:64;not null;index"`
	RiskLevel        string     `gorm:"column:risk_level;size:32;not null;index"`
	UserID           uint64     `gorm:"column:user_id;not null;index"`
	Username         string     `gorm:"size:64;not null;index"`
	IP               string     `gorm:"column:ip;size:64;index"`
	DeviceFingerprint string    `gorm:"column:device_fingerprint;size:255"`
	RuleCode         string     `gorm:"column:rule_code;size:128;not null;index"`
	Summary          string     `gorm:"size:255;not null"`
	DetailPayload    string     `gorm:"column:detail_payload;type:text"`
	OccurCount       int        `gorm:"column:occur_count;not null;default:1"`
	FirstOccurredAt  time.Time  `gorm:"column:first_occurred_at;not null;index"`
	LastOccurredAt   time.Time  `gorm:"column:last_occurred_at;not null;index"`
	Status           string     `gorm:"size:32;not null;index"`
	HandledBy        *uint64    `gorm:"column:handled_by;index"`
	HandledAt        *time.Time `gorm:"column:handled_at"`
	HandleNote       string     `gorm:"column:handle_note;size:255"`
	CreatedAt        time.Time  `gorm:"autoCreateTime"`
	UpdatedAt        time.Time  `gorm:"autoUpdateTime"`
}

func (RiskEvent) TableName() string {
	return "risk_events"
}
