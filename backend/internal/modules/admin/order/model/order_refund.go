package model

import "time"

// 退款状态
const (
	RefundStatusPending  string = "pending"  // 待审核
	RefundStatusApproved string = "approved" // 已通过
	RefundStatusRejected string = "rejected" // 已驳回
	RefundStatusDone     string = "done"     // 已退款
)

// 退款去向（doc36 §3.2）。审核通过后按此决定资金出口：
//   - balance：退回余额，资金仍在平台内，消费口径不变；
//   - channel：原路退回支付来源，收入口径需扣减本金与渠道扣点。
const (
	RefundModeBalance string = "balance" // 退回余额（默认）
	RefundModeChannel string = "channel" // 原路退回
)

// 退款申请来源
const (
	RefundOriginAdmin string = "admin" // 运营在管理端发起
	RefundOriginUser  string = "user"  // 客户在用户中心自助申请
)

// 事后复核状态（免审核自动退款后的最后一道人工确认）。
//
// 与 status 分开：status 是资金流状态（pending/approved/rejected），
// review_state 是「自动退出去的钱，有没有人回头核对过」——两者生命周期不同，
// 混在一个字段会让「已退款」的语义在两种场景下打架。
const (
	RefundReviewNone    string = ""        // 无需复核（人工审核路径）
	RefundReviewPending string = "pending" // 待复核（免审核自动退款后）
	RefundReviewDone    string = "done"    // 已复核
)

// 渠道退款单状态（原路退回时由支付中心回填，预埋渠道 API 对接）。
const (
	ChannelRefundStatusNone    string = ""
	ChannelRefundStatusPending string = "pending"
	ChannelRefundStatusSuccess string = "success"
	ChannelRefundStatusFailed  string = "failed"
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
	// 退款去向与扣点（doc36 §3.2）
	RefundMode string  `gorm:"column:refund_mode;size:20;not null;default:balance;index"`
	FeeAmount  float64 `gorm:"column:fee_amount;type:decimal(15,2);not null;default:0"` // 渠道扣点（原路退回）
	NetAmount  float64 `gorm:"column:net_amount;type:decimal(15,2);not null;default:0"` // 实际净流出 = 本金 + 扣点
	// 渠道退款单（原路退回时由支付中心回填）
	ChannelRefundNo     string `gorm:"column:channel_refund_no;size:64;not null;default:''"`
	ChannelRefundStatus string `gorm:"column:channel_refund_status;size:20;not null;default:''"`
	// Origin 申请来源：admin（运营发起）/ user（客户自助）。存量默认 admin。
	Origin string `gorm:"column:origin;size:16;not null;default:''"`
	// AuditRemark 审核备注：审核时填写的依据（此前 Appove/Reject 收了 remark 却从未落库）。
	AuditRemark string `gorm:"column:audit_remark;size:255"`
	// EvidenceNo 退款凭据外部单号：人工退款（尤其原路退回）时填写转账/渠道流水号，
	// 供后期查账定位；配合 refund_attachments 的凭证附件一起构成审计材料。
	EvidenceNo string `gorm:"column:evidence_no;size:128"`
	// 事后复核（免审核自动退款路径）
	ReviewState    string     `gorm:"column:review_state;size:16;not null;default:'';index"`
	ReviewRemark   string     `gorm:"column:review_remark;size:255"`
	ReviewedBy     uint64     `gorm:"column:reviewed_by"`
	ReviewedByName string     `gorm:"column:reviewed_by_name;size:50"`
	ReviewedAt     *time.Time `gorm:"column:reviewed_at"`
	CreatedAt      time.Time  `gorm:"autoCreateTime"`
	UpdatedAt      time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (OrderRefund) TableName() string {
	return "order_refunds"
}
