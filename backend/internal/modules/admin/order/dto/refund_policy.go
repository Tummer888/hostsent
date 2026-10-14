package dto

// RefundPolicyInfo 退款策略（管理端展示）。
type RefundPolicyInfo struct {
	ID          uint64 `json:"id"`
	SourceMode  string `json:"source_mode"`  // self / upstream / ""（不限）
	CategoryID  uint64 `json:"category_id"`  // 0 = 该来源下全部商品（含子孙分类）
	Refundable  bool   `json:"refundable"`   // 是否允许用户自助申请退款
	WindowHours int    `json:"window_hours"` // 可退时限（小时）；0 = 不限
	AutoApprove bool   `json:"auto_approve"` // 免人工审核自动退款
	Status      string `json:"status"`
	SortOrder   int    `json:"sort_order"`
	Remark      string `json:"remark"`
	// CategoryName 分类名（回填展示用，0 时为空）。
	CategoryName string `json:"category_name,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// RefundPolicyListResponse 策略列表响应。
type RefundPolicyListResponse struct {
	Items []RefundPolicyInfo `json:"items"`
}

// RefundPolicyRequest 新建/更新退款策略。
type RefundPolicyRequest struct {
	SourceMode  string `json:"source_mode"` // self / upstream / ""（不限）
	CategoryID  uint64 `json:"category_id"`
	Refundable  bool   `json:"refundable"`
	WindowHours int    `json:"window_hours"`
	AutoApprove bool   `json:"auto_approve"`
	Status      string `json:"status"`
	SortOrder   int    `json:"sort_order"`
	Remark      string `json:"remark"`
}

// UserRefundRequest 用户自助申请退款（uc）。
type UserRefundRequest struct {
	// Reason 退款原因（必填：客户诉求是审核与后续复盘的依据）。
	Reason string `json:"reason" binding:"required"`
	// RefundMode 期望去向：balance=退回余额 / channel=原路退回。
	// 余额支付的订单只能选 balance（服务端强制，前端同步锁定）。
	RefundMode string `json:"refund_mode"`
}

// UserRefundResult 用户申请退款结果。
type UserRefundResult struct {
	RefundNo string `json:"refund_no"`
	// AutoApproved=true 表示免人工审核已自动退款（此后仍有事后复核）。
	AutoApproved bool    `json:"auto_approved"`
	Amount       float64 `json:"amount"`
	// Status 退款单当前状态（pending=待审核 / approved=已通过）。
	Status string `json:"status"`
	// Hint 给用户的提示文案（例如"已原路退回，等待财务处理"）。
	Hint string `json:"hint"`
}

// OrderRefundPolicyHint 订单可退性提示（订单详情/列表下发，前端据此决定是否展示申请入口）。
type OrderRefundPolicyHint struct {
	// Refundable 当前是否可申请退款（含时限与策略判定）。
	Refundable bool `json:"refundable"`
	// WindowHours 策略配置的可退时限（小时）；0 = 不限。
	WindowHours int `json:"window_hours"`
	// Deadline 可退截止时间（RFC3339）；不限时为空串。
	Deadline string `json:"deadline,omitempty"`
	// AutoApprove 命中免审核自动退款（退回余额 + 全额 + 在时限内）。
	AutoApprove bool `json:"auto_approve"`
	// ChannelAllowed 是否允许选择原路退回（有渠道支付记录的订单才可为 true）。
	ChannelAllowed bool `json:"channel_allowed"`
	// MaxAmount 可退上限（订单实付 - 已生效退款）。
	MaxAmount float64 `json:"max_amount"`
	// Reason 不可退原因（Refundable=false 时给用户看的一句话）。
	Reason string `json:"reason,omitempty"`
}
