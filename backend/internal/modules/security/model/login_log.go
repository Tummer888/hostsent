package model

import "time"

type LoginLog struct {
	ID                uint64    `gorm:"primaryKey;autoIncrement"`
	UserID            uint64    `gorm:"column:user_id;not null;index"`
	Username          string    `gorm:"size:64;not null;index"`
	LoginType         string    `gorm:"column:login_type;size:32;not null;index"`
	Result            string    `gorm:"size:32;not null;index"`
	FailureReason     string    `gorm:"column:failure_reason;size:255"`
	IP                string    `gorm:"column:ip;size:64;not null;index"`
	IPRegion          string    `gorm:"column:ip_region;size:128"`
	UserAgent         string    `gorm:"column:user_agent;size:255"`
	DeviceFingerprint string    `gorm:"column:device_fingerprint;size:255"`
	Platform          string    `gorm:"size:32;not null;index"`
	RiskFlag          string    `gorm:"column:risk_flag;size:32;index"`
	CreatedAt         time.Time `gorm:"autoCreateTime"`
}

func (LoginLog) TableName() string {
	return "login_logs"
}
