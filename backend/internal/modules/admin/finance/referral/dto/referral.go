// Package dto 提供推广邀请返现子域的请求/响应结构。
package dto

// ListMeta 分页元信息。
type ListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// Profile 返现概览（用户端）。
type Profile struct {
	InviteCode        string  `json:"invite_code"`         // 我的邀请码
	Enabled           bool    `json:"enabled"`             // 返现功能是否开启
	Balance           float64 `json:"balance"`             // 可用返现余额
	Frozen            float64 `json:"frozen"`              // 提现申请冻结
	TotalIncome       float64 `json:"total_income"`        // 累计返现收入
	TotalOut          float64 `json:"total_out"`           // 累计流出
	PendingWithdraw   float64 `json:"pending_withdraw"`    // 待审核提现金额
	InviteeCount      int64   `json:"invitee_count"`       // 已邀请人数
	MinWithdrawAmount float64 `json:"min_withdraw_amount"` // 最低提现金额
	FirstOrderRate    float64 `json:"first_order_rate"`    // 首单返现比率（展示用）
	SubsequentRate    float64 `json:"subsequent_rate"`     // 后续下单返现比率
	RenewalRate       float64 `json:"renewal_rate"`        // 续费返现比率
}

// CashbackInfo 返现台账行。
type CashbackInfo struct {
	ID              uint64  `json:"id"`
	TxNo            string  `json:"tx_no"`
	Type            string  `json:"type"`
	Direction       int     `json:"direction"`
	Amount          float64 `json:"amount"`
	BalanceAfter    float64 `json:"balance_after"`
	OrderID         uint64  `json:"order_id"`
	OrderNo         string  `json:"order_no"`
	InviteeUserID   uint64  `json:"invitee_user_id"`
	InviteeUsername string  `json:"invitee_username"`
	InviterUserID   uint64  `json:"inviter_user_id"`
	InviterUsername string  `json:"inviter_username"`
	RefNo           string  `json:"ref_no"`
	Remark          string  `json:"remark"`
	CreatedAt       string  `json:"created_at"`
}

// CashbackListResponse 返现台账分页响应。
type CashbackListResponse struct {
	Items []CashbackInfo `json:"items"`
	Meta  ListMeta       `json:"meta"`
}

// InviteeInfo 被邀请人贡献行。
type InviteeInfo struct {
	UserID        uint64  `json:"user_id"`
	Username      string  `json:"username"`
	Email         string  `json:"email"`
	InvitedAt     string  `json:"invited_at"`
	CashbackTotal float64 `json:"cashback_total"`
	OrderCount    int64   `json:"order_count"`
}

// InviteeListResponse 邀请列表分页响应。
type InviteeListResponse struct {
	Items []InviteeInfo `json:"items"`
	Meta  ListMeta      `json:"meta"`
}

// WithdrawRequest 用户端提现申请。
type WithdrawRequest struct {
	Amount  float64 `json:"amount" binding:"required"`
	Channel string  `json:"channel"` // bank/alipay
	Account string  `json:"account"` // 收款账户
}

// TransferRequest 用户端返现转入现金余额。
type TransferRequest struct {
	Amount float64 `json:"amount" binding:"required"`
}

// WithdrawalInfo 提现单。
type WithdrawalInfo struct {
	ID          uint64  `json:"id"`
	WithdrawNo  string  `json:"withdraw_no"`
	UserID      uint64  `json:"user_id"`
	Username    string  `json:"username"`
	Amount      float64 `json:"amount"`
	Channel     string  `json:"channel"`
	Account     string  `json:"account"`
	Status      string  `json:"status"`
	AuditBy     uint64  `json:"audit_by"`
	AuditByName string  `json:"audit_by_name"`
	AuditedAt   string  `json:"audited_at"`
	Remark      string  `json:"remark"`
	CreatedAt   string  `json:"created_at"`
}

// WithdrawalListResponse 提现单分页响应。
type WithdrawalListResponse struct {
	Items []WithdrawalInfo `json:"items"`
	Meta  ListMeta         `json:"meta"`
}

// WithdrawalAuditRequest 提现审核请求。
type WithdrawalAuditRequest struct {
	Remark string `json:"remark"`
}

// LedgerQuery 管理端返现台账查询。
type LedgerQuery struct {
	UserID   uint64 `form:"user_id"`
	Type     string `form:"type"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// WithdrawalQuery 提现单查询（用户端与管理端共用）。
type WithdrawalQuery struct {
	UserID   uint64 `form:"user_id"`
	Status   string `form:"status"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// Query 通用分页查询（邀请列表等）。
type Query struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}
