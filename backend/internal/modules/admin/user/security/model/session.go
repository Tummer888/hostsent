package model

import "time"

type Session struct {
	ID                uint64     `gorm:"primaryKey;autoIncrement"`
	SessionID         string     `gorm:"column:session_id;size:128;not null;uniqueIndex"`
	UserID            uint64     `gorm:"column:user_id;not null;index;index:idx_user_sessions_subject_user,priority:2"`
	Username          string     `gorm:"size:64;not null;index"`
	Platform          string     `gorm:"size:32;not null;index"`
	IP                string     `gorm:"column:ip;size:64;index"`
	UserAgent         string     `gorm:"column:user_agent;size:255"`
	DeviceFingerprint string     `gorm:"column:device_fingerprint;size:255"`
	LoginAt           time.Time  `gorm:"column:login_at;not null;index"`
	LastActiveAt      time.Time  `gorm:"column:last_active_at;not null;index"`
	ExpiredAt         *time.Time `gorm:"column:expired_at;index"`
	Status            string     `gorm:"size:32;not null;index"`
	// SubjectType 会话主体域（user/admin），与 login_logs 同义；见 login_log.go。
	// 客户会话与员工后台会话共用本表，user_id 因此落在两个 ID 空间上。
	SubjectType   string     `gorm:"column:subject_type;size:16;not null;default:user;index:idx_user_sessions_subject_user,priority:1"`
	RiskFlag      string     `gorm:"column:risk_flag;size:32;index"`
	RevokedReason string     `gorm:"column:revoked_reason;size:255"`
	RevokedBy     *uint64    `gorm:"column:revoked_by;index"`
	RevokedAt     *time.Time `gorm:"column:revoked_at"`
	CreatedAt     time.Time  `gorm:"autoCreateTime"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime"`
}

func (Session) TableName() string {
	return "user_sessions"
}
