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
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
}

// RefundListResponse 退款单列表响应
type RefundListResponse struct {
	Items []RefundInfo `json:"items"`
	Meta  ListMeta     `json:"meta"`
}

// RefundApproveRequest 退款审核请求（通过 / 驳回备注，可空）
type RefundApproveRequest struct {
	Remark string `json:"remark"`
}
