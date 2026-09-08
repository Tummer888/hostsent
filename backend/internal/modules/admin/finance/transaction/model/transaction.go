// Package model 提供资金流水子域的数据模型。
package model

import "time"

// 资金流水类型
const (
	TxTypeRecharge   string = "recharge"   // 充值
	TxTypeConsume    string = "consume"    // 消费（订单支付扣余额）
	TxTypeRefund     string = "refund"     // 退款（订单退款回余额）
	TxTypeCommission string = "commission" // 佣金（分销入账）
	TxTypeSettlement string = "settlement" // 结算（分销/提现出账）
	TxTypeAdjust     string = "adjust"     // 调账（赠送/扣减）
)

// 资金方向
const (
	DirectionIncome  int = 1  // 收入
	DirectionExpense int = -1 // 支出
)

// WalletTransaction 资金流水台账。只增不改不删，落库后不允许更新金额。
// 以 (user_id, biz_type, ref_no) 唯一约束保证同一来源只记一次账（幂等）。
type WalletTransaction struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement"`
	TxNo          string    `gorm:"column:tx_no;size:64;uniqueIndex;not null"` // 流水号，如 20260830W00001
	UserID        uint64    `gorm:"column:user_id;uniqueIndex:uk_biz,priority:1;index;not null"`
	Type          string    `gorm:"size:32;not null;index"`                                         // recharge/consume/refund/commission/settlement/adjust
	Direction     int       `gorm:"not null"`                                                       // 1=收入 -1=支出
	Amount        float64   `gorm:"type:decimal(15,2);not null"`                                    // 变动金额
	BalanceBefore float64   `gorm:"column:balance_before;type:decimal(15,2);not null"`              // 变动前余额
	BalanceAfter  float64   `gorm:"column:balance_after;type:decimal(15,2);not null"`               // 变动后余额
	OrderID       uint64    `gorm:"column:order_id;index"`                                          // 关联订单（可为空）
	OrderNo       string    `gorm:"column:order_no;size:64"`                                        // 订单号快照
	RefNo         string    `gorm:"column:ref_no;uniqueIndex:uk_biz,priority:3;size:64"`            // 关联单号（退款/结算/充值单号）
	BizType       string    `gorm:"column:biz_type;uniqueIndex:uk_biz,priority:2;size:32;not null"` // 业务标识，用于幂等
	Remark        string    `gorm:"size:255"`
	OperatorID    uint64    `gorm:"column:operator_id"` // 操作人（0=系统）
	CreatedAt     time.Time `gorm:"autoCreateTime;index"`
}

// TableName 指定表名
func (WalletTransaction) TableName() string {
	return "wallet_transactions"
}
