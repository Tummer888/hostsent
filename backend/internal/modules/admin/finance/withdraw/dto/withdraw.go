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

// WithdrawAuditRequest 提现审核请求
type WithdrawAuditRequest struct {
	Remark string `json:"remark"`
}

// WithdrawInfo 提现单信息
type WithdrawInfo struct {
	ID          uint64  `json:"id"`
	WithdrawNo  string  `json:"withdraw_no"`
	UserID      uint64  `json:"user_id"`
	Amount      float64 `json:"amount"`
	Channel     string  `json:"channel"`
	Account     string  `json:"account"`
	Status      string  `json:"status"`
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
