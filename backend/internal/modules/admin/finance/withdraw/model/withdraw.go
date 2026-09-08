// Package model 提供提现子域的数据模型。
package model

import "time"

// 提现单状态
const (
	WithdrawStatusPending  string = "pending"  // 待审核
	WithdrawStatusApproved string = "approved" // 审核通过（已出账）
	WithdrawStatusRejected string = "rejected" // 已驳回
	WithdrawStatusPaid     string = "paid"     // 已打款
	WithdrawStatusFailed   string = "failed"   // 打款失败
)

// Withdraw 提现单。
type Withdraw struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement"`
	WithdrawNo  string     `gorm:"column:withdraw_no;size:64;uniqueIndex;not null"`
	UserID      uint64     `gorm:"column:user_id;index;not null"`
	Amount      float64    `gorm:"type:decimal(15,2);not null"`
	Channel     string     `gorm:"size:20"`  // bank/alipay
	Account     string     `gorm:"size:128"` // 收款账户
	Status      string     `gorm:"size:20;not null;default:pending;index"`
	AuditBy     uint64     `gorm:"column:audit_by"`
	AuditByName string     `gorm:"column:audit_by_name;size:50"`
	AuditedAt   *time.Time `gorm:"column:audited_at"`
	PaidAt      *time.Time `gorm:"column:paid_at"`
	Remark      string     `gorm:"size:255"`
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (Withdraw) TableName() string {
	return "withdrawals"
}
