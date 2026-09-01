package dto

// TransactionListQuery 资金流水分页查询
type TransactionListQuery struct {
	UserID    uint64 `form:"user_id" json:"user_id"`
	Type      string `form:"type" json:"type"`           // 流水类型
	Direction int    `form:"direction" json:"direction"` // 1=收入 -1=支出
	StartTime string `form:"start_time" json:"start_time"`
	EndTime   string `form:"end_time" json:"end_time"`
	Page      int    `form:"page" json:"page"`
	PageSize  int    `form:"page_size" json:"page_size"`
}

// AdjustRequest 人工调账（赠送/扣减）请求
type AdjustRequest struct {
	UserID    uint64  `json:"user_id" binding:"required"`
	Type      string  `json:"type"`                         // 默认 adjust
	Direction int     `json:"direction" binding:"required"` // 1=收入 -1=支出
	Amount    float64 `json:"amount" binding:"required"`
	BizKey    string  `json:"biz_key" binding:"required"` // 幂等键
	Remark    string  `json:"remark"`
}

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

// BillListQuery 账单列表查询
type BillListQuery struct {
	UserID   uint64 `form:"user_id" json:"user_id"`
	Period   string `form:"period" json:"period"`
	Status   string `form:"status" json:"status"`
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"page_size" json:"page_size"`
}

// ReconcileRequest 触发对账请求
type ReconcileRequest struct {
	Period string `json:"period"` // 账期，空为全量
}
