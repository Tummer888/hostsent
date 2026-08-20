// Package model 提供实名认证模块的数据模型。
package model

import "time"

// VerificationReviewLog 表示实名认证审核日志。
type VerificationReviewLog struct {
	ID               uint64    `gorm:"primaryKey;autoIncrement"`
	ApplicationID    uint64    `gorm:"column:application_id;not null;index"`
	FromStatus       string    `gorm:"column:from_status;size:32;not null"`
	ToStatus         string    `gorm:"column:to_status;size:32;not null;index"`
	Action           string    `gorm:"column:action;size:32;not null"`
	OperatorID       uint64    `gorm:"column:operator_id;not null;index"`
	OperatorName     string    `gorm:"column:operator_name;size:64;not null"`
	Note             string    `gorm:"column:note;size:255"`
	RejectReasonCode string    `gorm:"column:reject_reason_code;size:64"`
	RejectReason     string    `gorm:"column:reject_reason;size:255"`
	CreatedAt        time.Time `gorm:"autoCreateTime"`
}

// TableName 返回实名认证审核日志表名。
func (VerificationReviewLog) TableName() string {
	return "verification_review_logs"
}
