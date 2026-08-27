package model

import "time"

type Session struct {
	ID              uint64     `gorm:"primaryKey;autoIncrement"`
	SessionID       string     `gorm:"column:session_id;size:128;not null;uniqueIndex"`
	UserID          uint64     `gorm:"column:user_id;not null;index"`
	Username        string     `gorm:"size:64;not null;index"`
	Platform        string     `gorm:"size:32;not null;index"`
	IP              string     `gorm:"column:ip;size:64;index"`
	IPRegion        string     `gorm:"column:ip_region;size:128"`
	UserAgent       string     `gorm:"column:user_agent;size:255"`
	DeviceFingerprint string   `gorm:"column:device_fingerprint;size:255"`
	LoginAt         time.Time  `gorm:"column:login_at;not null;index"`
	LastActiveAt    time.Time  `gorm:"column:last_active_at;not null;index"`
	ExpiredAt       *time.Time `gorm:"column:expired_at;index"`
	Status          string     `gorm:"size:32;not null;index"`
	RiskFlag        string     `gorm:"column:risk_flag;size:32;index"`
	RevokedReason   string     `gorm:"column:revoked_reason;size:255"`
	RevokedBy       *uint64    `gorm:"column:revoked_by;index"`
	RevokedAt       *time.Time `gorm:"column:revoked_at"`
	CreatedAt       time.Time  `gorm:"autoCreateTime"`
	UpdatedAt       time.Time  `gorm:"autoUpdateTime"`
}

func (Session) TableName() string {
	return "user_sessions"
}
