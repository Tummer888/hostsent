// Package dto 提供提现子域的数据传输结构。
package dto

// WithdrawListQuery 提现单列表查询
type WithdrawListQuery struct {
	UserID    uint64 `form:"user_id" json:"user_id"`
	Status    string `form:"status" json:"status"`
	StartTime string `form:"start_time" json:"start_time"`
	EndTime   string `form:"end_time" json:"end_time"`
	Page      int    `form:"page" json:"page"`
	PageSize  int    `form:"page_size" json:"page_size"`
}

// WithdrawApplyRequest 用户提现申请（资金入口，主账号）。
type WithdrawApplyRequest struct {
	Amount      float64 `json:"amount" binding:"required"`
	Channel     string  `json:"channel" binding:"required"` // bank/alipay
	AccountNo   string  `json:"account_no" binding:"required"`
	AccountName string  `json:"account_name" binding:"required"`
	BankName    string  `json:"bank_name"`
	PayoutMode  string  `json:"payout_mode"` // api/manual，空按平台默认（manual）
	Remark      string  `json:"remark"`
}

// WithdrawAuditRequest 提现审核请求
type WithdrawAuditRequest struct {
	Remark string `json:"remark"`
}

// WithdrawPayoutRequest 打款完成登记请求
type WithdrawPayoutRequest struct {
	ChannelTx  string `json:"channel_tx"`
	ReceiptURL string `json:"receipt_url"`
	Remark     string `json:"remark"`
}

// WithdrawInfo 提现单信息
type WithdrawInfo struct {
	ID          uint64  `json:"id"`
	WithdrawNo  string  `json:"withdraw_no"`
	UserID      uint64  `json:"user_id"`
	Amount      float64 `json:"amount"`
	Channel     string  `json:"channel"`
	ChannelName string  `json:"channel_name"`
	Account     string  `json:"account"`
	AccountName string  `json:"account_name"`
	BankName    string  `json:"bank_name"`
	Status      string  `json:"status"`
	PayoutNo    string  `json:"payout_no"`
	PayoutMode  string  `json:"payout_mode"`
	ChannelTx   string  `json:"channel_tx"`
	AuditBy     uint64  `json:"audit_by"`
	AuditByName string  `json:"audit_by_name"`
	AuditedAt   string  `json:"audited_at"`
	PaidAt      string  `json:"paid_at"`
	Remark      string  `json:"remark"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// WithdrawListResponse 提现单列表响应
type WithdrawListResponse struct {
	Items []WithdrawInfo `json:"items"`
	Meta  ListMeta       `json:"meta"`
}
