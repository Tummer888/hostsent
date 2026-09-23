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
	BillTypeRecharge    string = "recharge"    // 充值账单（单笔充值，见 SourceTypeRecharge）
)

// 账单来源：决定「一张账单代表什么」，也是唯一约束的划分依据。
//
//	period   按期归集的消费账单：一个用户一个账期一张，重复生成覆盖（uk_bills_user_period）。
//	recharge 单笔充值账单：一笔充值一张，SourceNo 记充值单号，逐笔可追溯（uk_bills_source）。
//
// 为什么充值要单独成账而不是并进当期消费账单：两笔充值并进一张账单后，
// 用户拿到的凭证只有一个「本期充值合计」，对不上「我的充值单」里的逐笔记录，
// 也看不出哪笔钱对应哪次到账（见 055 迁移）。
const (
	SourceTypePeriod   string = "period"
	SourceTypeRecharge string = "recharge"
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
//
// RechargeAmount 是唯一不参与应结的金额列：充值是用户把钱打进平台，不是欠款，
// 计进 total_amount 会让应结金额虚高。它只作展示与对账（见 054 迁移）。
//
// 两种来源各自记账（见 SourceType 注释与 055 迁移）：SourceType=period 的行
// recharge_amount 恒为 0，充值额只出现在 SourceType=recharge 的单笔账单上 ——
// 同一笔钱不在两处重复计数，且每笔充值都有自己的凭证。
type Bill struct {
	ID           uint64  `gorm:"primaryKey;autoIncrement"`
	BillNo       string  `gorm:"column:bill_no;size:64;uniqueIndex;not null"`
	UserID       uint64  `gorm:"column:user_id;index;not null"`
	Period       string  `gorm:"size:20;not null"`                                           // 账期，如 202608
	TotalAmount  float64 `gorm:"column:total_amount;type:decimal(15,2);not null;default:0"`  // 本期应结
	RefundAmount float64 `gorm:"column:refund_amount;type:decimal(15,2);not null;default:0"` // 本期退款合计（余额+原路）
	Status       string  `gorm:"size:20;not null;default:unpaid"`                            // unpaid/paid/closed
	// 来源（period/recharge）：唯一约束按来源分流，见 SourceType 注释与 055 迁移。
	SourceType string `gorm:"column:source_type;size:20;not null;default:period"`
	// SourceNo 来源单据号：充值账单记充值单号（recharge_no），按期账单为空。
	SourceNo string `gorm:"column:source_no;size:64;not null;default:''"`
	// 分类与拆分（doc36 §3.4）
	BillType            string  `gorm:"column:bill_type;size:20;not null;default:consumption"`
	ConsumeAmount       float64 `gorm:"column:consume_amount;type:decimal(15,2);not null;default:0"`
	RenewalAmount       float64 `gorm:"column:renewal_amount;type:decimal(15,2);not null;default:0"`
	ChannelRefundAmount float64 `gorm:"column:channel_refund_amount;type:decimal(15,2);not null;default:0"`
	RefundFeeAmount     float64 `gorm:"column:refund_fee_amount;type:decimal(15,2);not null;default:0"`
	// 充值金额（不参与 total_amount 应结口径，见结构体注释与迁移 054）。
	// 按期账单恒为 0；充值账单等于该笔充值金额。
	RechargeAmount float64 `gorm:"column:recharge_amount;type:decimal(15,2);not null;default:0"`
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
