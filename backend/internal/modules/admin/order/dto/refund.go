package dto

// RefundListQuery 退款单列表查询
type RefundListQuery struct {
	Keyword    string `form:"keyword" json:"keyword"`         // 退款单号 / 原因
	OrderID    uint64 `form:"order_id" json:"order_id"`       // 关联订单 ID
	Status     string `form:"status" json:"status"`           // 退款状态
	RefundMode string `form:"refund_mode" json:"refund_mode"` // 退款去向：balance/channel（doc36 §3.2）
	Page       int    `form:"page" json:"page"`
	PageSize   int    `form:"page_size" json:"page_size"`
}

// RefundInfo 退款单信息
type RefundInfo struct {
	ID          uint64  `json:"id"`
	RefundNo    string  `json:"refund_no"`
	OrderID     uint64  `json:"order_id"`
	OrderNo     string  `json:"order_no"`
	UserID      uint64  `json:"user_id"`
	Amount      float64 `json:"amount"`
	Reason      string  `json:"reason"`
	Status      string  `json:"status"`
	AuditBy     uint64  `json:"audit_by"`
	AuditByName string  `json:"audit_by_name"`
	AuditedAt   string  `json:"audited_at"`
	// 退款去向（doc36 §3.2）：balance 退回余额 / channel 原路退回。
	RefundMode string `json:"refund_mode"`
	// 扣点与净额：原路退回时 fee_amount 为渠道手续费，net_amount = amount + fee_amount。
	FeeAmount float64 `json:"fee_amount"`
	NetAmount float64 `json:"net_amount"`
	// 渠道退款单（原路退回时由支付中心回填）
	ChannelRefundNo     string `json:"channel_refund_no"`
	ChannelRefundStatus string `json:"channel_refund_status"`
	// 申请来源：admin=运营发起 / user=客户自助申请（后者需要事后复核）。
	Origin string `json:"origin"`
	// AuditRemark 审核备注（本次起真正落库）与 EvidenceNo 退款凭据外部单号。
	AuditRemark string `json:"audit_remark"`
	EvidenceNo  string `json:"evidence_no"`
	// 事后复核（仅免审核自动退款的单有值）
	ReviewState    string `json:"review_state"`
	ReviewRemark   string `json:"review_remark"`
	ReviewedBy     uint64 `json:"reviewed_by"`
	ReviewedByName string `json:"reviewed_by_name"`
	ReviewedAt     string `json:"reviewed_at"`
	// Attachments 凭据附件列表（详情接口填充；列表接口不填充）。
	Attachments []RefundAttachmentInfo `json:"attachments,omitempty"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
}

// RefundListResponse 退款单列表响应
type RefundListResponse struct {
	Items []RefundInfo `json:"items"`
	Meta  ListMeta     `json:"meta"`
}

// RefundApproveRequest 退款审核请求。
//
// Remark 此前只作为形参传入却从未落库 —— 审核依据（为什么退/为什么拒）
// 是资金审计的必要材料，本次随审核一并写入 order_refunds.audit_remark。
type RefundApproveRequest struct {
	Remark string `json:"remark"`
	// EvidenceNo 退款凭据外部单号（人工退款的转账/渠道流水号）：查账定位用。
	// 原路退回（channel）时尤其重要 —— 系统暂无自动退款接口，钱是人工打出去的。
	EvidenceNo string `json:"evidence_no"`
	// AttachmentIDs 本次审核上传的凭据附件（先走上传接口拿 ID，再随审核提交）。
	AttachmentIDs []uint64 `json:"attachment_ids"`
}

// RefundReviewRequest 事后复核请求（免审核自动退款后的最后一道人工确认）。
type RefundReviewRequest struct {
	// Remark 复核结论（如"已核对余额已入账，凭证相符"）。
	Remark string `json:"remark"`
}

// RefundAttachmentInfo 退款凭据附件信息。
type RefundAttachmentInfo struct {
	ID         uint64 `json:"id"`
	RefundID   uint64 `json:"refund_id"`
	FileName   string `json:"file_name"`
	FileSize   int64  `json:"file_size"`
	FileType   string `json:"file_type"`
	UploaderID uint64 `json:"uploader_id"`
	CreatedAt  string `json:"created_at"`
}
