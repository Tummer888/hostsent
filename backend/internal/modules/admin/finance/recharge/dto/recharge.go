// Package dto 提供充值子域的数据传输结构。
package dto

// RechargeCreateRequest 线下充值登记请求
type RechargeCreateRequest struct {
	UserID uint64  `json:"user_id" binding:"required"`
	Amount float64 `json:"amount" binding:"required"`
	Method string  `json:"method" binding:"required"` // alipay/wechat/manual
	Remark string  `json:"remark"`
}

// RechargeListQuery 充值单列表查询
type RechargeListQuery struct {
	UserID    uint64 `form:"user_id" json:"user_id"`
	Status    string `form:"status" json:"status"`
	Method    string `form:"method" json:"method"`
	StartTime string `form:"start_time" json:"start_time"`
	EndTime   string `form:"end_time" json:"end_time"`
	Page      int    `form:"page" json:"page"`
	PageSize  int    `form:"page_size" json:"page_size"`
}

// RechargeApproveRequest 充值确认到账请求
type RechargeApproveRequest struct {
	ChannelTx string `json:"channel_tx"` // 渠道交易号
	Remark    string `json:"remark"`
}

// RechargeInfo 充值单信息
type RechargeInfo struct {
	ID         uint64  `json:"id"`
	RechargeNo string  `json:"recharge_no"`
	UserID     uint64  `json:"user_id"`
	Amount     float64 `json:"amount"`
	Method     string  `json:"method"`
	Status     string  `json:"status"`
	ChannelTx  string  `json:"channel_tx"`
	PaidAt     string  `json:"paid_at"`
	Remark     string  `json:"remark"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

// RechargeListResponse 充值单列表响应
type RechargeListResponse struct {
	Items []RechargeInfo `json:"items"`
	Meta  ListMeta       `json:"meta"`
}
