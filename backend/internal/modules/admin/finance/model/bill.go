package model

import "time"

// 账单状态
const (
	BillStatusUnpaid string = "unpaid" // 未结
	BillStatusPaid   string = "paid"   // 已结清
	BillStatusClosed string = "closed" // 已关账
)

// Bill 账单：按账期归集消费与退款，形成对账口径。
type Bill struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	BillNo       string    `gorm:"column:bill_no;size:64;uniqueIndex;not null"`
	UserID       uint64    `gorm:"column:user_id;index;not null"`
	Period       string    `gorm:"size:20;not null"`                                           // 账期，如 202608
	TotalAmount  float64   `gorm:"column:total_amount;type:decimal(15,2);not null;default:0"`  // 本期应结（消费-退款）
	RefundAmount float64   `gorm:"column:refund_amount;type:decimal(15,2);not null;default:0"` // 本期退款
	Status       string    `gorm:"size:20;not null;default:unpaid"`                            // unpaid/paid/closed
	Detail       string    `gorm:"type:jsonb"`                                                 // 明细聚合摘要 JSON
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (Bill) TableName() string {
	return "bills"
}
