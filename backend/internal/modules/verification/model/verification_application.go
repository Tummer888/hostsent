// Package model 提供实名认证模块的数据模型。
package model

import "time"

// VerificationApplication 表示实名认证申请主记录。
type VerificationApplication struct {
	ID               uint64     `gorm:"primaryKey;autoIncrement"`
	UserID           uint64     `gorm:"column:user_id;not null;index"`
	Username         string     `gorm:"column:username;size:64;not null;index"`
	VerificationType string     `gorm:"column:verification_type;size:32;not null;index"`
	Status           string     `gorm:"column:status;size:32;not null;index"`
	RealName         string     `gorm:"column:real_name;size:128;not null"`
	SubjectName      string     `gorm:"column:subject_name;size:255;not null"`
	IDType           string     `gorm:"column:id_type;size:32;not null"`
	IDNumberMasked   string     `gorm:"column:id_number_masked;size:64;not null"`
	MobileMasked     string     `gorm:"column:mobile_masked;size:32"`
	CountryCode      string     `gorm:"column:country_code;size:16"`
	RiskFlags        string     `gorm:"column:risk_flags;size:255"`
	SubmittedAt      time.Time  `gorm:"column:submitted_at;not null;index"`
	ReviewedAt       *time.Time `gorm:"column:reviewed_at;index"`
	ReviewedBy       *uint64    `gorm:"column:reviewed_by;index"`
	ReviewerName     string     `gorm:"column:reviewer_name;size:64"`
	RejectReasonCode string     `gorm:"column:reject_reason_code;size:64"`
	RejectReason     string     `gorm:"column:reject_reason;size:255"`
	ReviewNote       string     `gorm:"column:review_note;size:255"`
	Version          int        `gorm:"column:version;not null;default:1"`
	CreatedAt        time.Time  `gorm:"autoCreateTime"`
	UpdatedAt        time.Time  `gorm:"autoUpdateTime"`
}

// TableName 返回实名认证申请主表名。
func (VerificationApplication) TableName() string {
	return "verification_applications"
}
