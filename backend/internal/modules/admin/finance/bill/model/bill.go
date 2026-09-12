// Package model 提供账单子域的数据模型。
package model

import "time"

// 账单状态
const (
	BillStatusUnpaid string = "unpaid" // 未结
	BillStatusPaid   string = "paid"   // 已结清
	BillStatusClosed string = "closed" // 已关账
)

// 账单分类（doc36 §3.4）：回答「这张账单是什么钱」。
const (
	BillTypeConsumption string = "consumption" // 产品消费
	BillTypeRenewal     string = "renewal"     // 续费消费
	BillTypeMixed       string = "mixed"       // 消费 + 续费混合
	BillTypeRecharge    string = "recharge"    // 充值（预埋：B 端对账口径）
)

// Bill 账单：按账期归集消费与退款，形成对账口径。
//
// 口径（doc36 §3.2/§3.4，用户确认）：
//
//	total_amount = consume_amount + renewal_amount - channel_refund_amount
//	net_amount   = total_amount - refund_fee_amount（收入统计基数，见 service.buildBillInfo）
//
// 余额退回不减少消费口径（资金仍在平台内）；原路退回本金从账单应结金额冲减，
// 渠道扣点属平台成本不进票面，只在收入统计基数是再扣一次。
type Bill struct {
	ID           uint64  `gorm:"primaryKey;autoIncrement"`
	BillNo       string  `gorm:"column:bill_no;size:64;uniqueIndex;not null"`
	UserID       uint64  `gorm:"column:user_id;index;not null"`
	Period       string  `gorm:"size:20;not null"`                                           // 账期，如 202608
	TotalAmount  float64 `gorm:"column:total_amount;type:decimal(15,2);not null;default:0"`  // 本期应结
	RefundAmount float64 `gorm:"column:refund_amount;type:decimal(15,2);not null;default:0"` // 本期退款合计（余额+原路）
	Status       string  `gorm:"size:20;not null;default:unpaid"`                            // unpaid/paid/closed
	// 分类与拆分（doc36 §3.4）
	BillType            string  `gorm:"column:bill_type;size:20;not null;default:consumption"`
	ConsumeAmount       float64 `gorm:"column:consume_amount;type:decimal(15,2);not null;default:0"`
	RenewalAmount       float64 `gorm:"column:renewal_amount;type:decimal(15,2);not null;default:0"`
	ChannelRefundAmount float64 `gorm:"column:channel_refund_amount;type:decimal(15,2);not null;default:0"`
	RefundFeeAmount     float64 `gorm:"column:refund_fee_amount;type:decimal(15,2);not null;default:0"`
	// 支付方式描述（doc34 F-11 / doc35）：账单结清时记录实际收款方式与渠道实例。
	PaidAmountFen int64      `gorm:"column:paid_amount_fen;not null;default:0"`
	PaidMethod    string     `gorm:"column:paid_method;size:32;not null;default:''"`
	PaidChannelID uint64     `gorm:"column:paid_channel_id;not null;default:0"`
	PaidAt        *time.Time `gorm:"column:paid_at"`
	// 发票（doc36 §3.3）：none/applied/issued/rejected。
	InvoiceStatus string     `gorm:"column:invoice_status;size:20;not null;default:none;index"`
	InvoiceNo     string     `gorm:"column:invoice_no;size:64;not null;default:''"`
	InvoicedAt    *time.Time `gorm:"column:invoiced_at"`
	Detail        string     `gorm:"type:jsonb"` // 明细聚合摘要 JSON
	CreatedAt     time.Time  `gorm:"autoCreateTime"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (Bill) TableName() string {
	return "bills"
}
