package model

import "time"

// 退款状态
const (
	RefundStatusPending  string = "pending"  // 待审核
	RefundStatusApproved string = "approved" // 已通过
	RefundStatusRejected string = "rejected" // 已驳回
	RefundStatusDone     string = "done"     // 已退款
)

// OrderRefund 退款单
type OrderRefund struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement"`
	RefundNo    string     `gorm:"column:refund_no;size:64;uniqueIndex;not null"`
	OrderID     uint64     `gorm:"column:order_id;not null;index"`
	UserID      uint64     `gorm:"column:user_id;index"`
	Amount      float64    `gorm:"type:decimal(15,2);not null;default:0"`
	Reason      string     `gorm:"size:255"`
	Status      string     `gorm:"size:32;not null;default:pending;index"`
	AuditBy     uint64     `gorm:"column:audit_by"` // 审核人
	AuditByName string     `gorm:"column:audit_by_name;size:50"`
	AuditedAt   *time.Time `gorm:"column:audited_at"`
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (OrderRefund) TableName() string {
	return "order_refunds"
}
