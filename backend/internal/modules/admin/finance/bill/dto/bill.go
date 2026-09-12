// Package dto 提供账单子域的数据传输结构。
package dto

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

// BillInfo 账单信息
type BillInfo struct {
	ID           uint64  `json:"id"`
	BillNo       string  `json:"bill_no"`
	UserID       uint64  `json:"user_id"`
	Period       string  `json:"period"`
	TotalAmount  float64 `json:"total_amount"`
	RefundAmount float64 `json:"refund_amount"`
	Status       string  `json:"status"`
	// 支付方式描述（doc34 F-11）：结清时记录实收金额、方式与渠道实例。
	PaidAmount    float64 `json:"paid_amount"`
	PaidMethod    string  `json:"paid_method"`
	PaidChannelID uint64  `json:"paid_channel_id"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

// BillListResponse 账单列表响应
type BillListResponse struct {
	Items []BillInfo `json:"items"`
	Meta  ListMeta   `json:"meta"`
}

// ReconcileResponse 对账结果
type ReconcileResponse struct {
	Period        string  `json:"period"`         // 对账账期（空表示全量）
	IncomeTotal   float64 `json:"income_total"`   // 收入合计
	ExpenseTotal  float64 `json:"expense_total"`  // 支出合计
	TxCount       int64   `json:"tx_count"`       // 流水条数
	WalletBalance float64 `json:"wallet_balance"` // 钱包当前余额合计
	Diff          float64 `json:"diff"`           // 账实差异（应为 0）
	Status        string  `json:"status"`         // ok/suspicious
}
