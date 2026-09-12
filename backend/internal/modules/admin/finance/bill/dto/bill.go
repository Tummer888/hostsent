// Package dto 提供账单子域的数据传输结构。
package dto

// BillListQuery 账单列表查询
type BillListQuery struct {
	UserID        uint64 `form:"user_id" json:"user_id"`
	UserKeyword   string `form:"user_keyword" json:"user_keyword"` // 用户名/邮箱模糊
	BillNo        string `form:"bill_no" json:"bill_no"`           // 账单号精确
	Keyword       string `form:"keyword" json:"keyword"`           // 账单号模糊
	Period        string `form:"period" json:"period"`
	Status        string `form:"status" json:"status"`
	BillType      string `form:"bill_type" json:"bill_type"`
	InvoiceStatus string `form:"invoice_status" json:"invoice_status"`
	Page          int    `form:"page" json:"page"`
	PageSize      int    `form:"page_size" json:"page_size"`
}

// ReconcileRequest 触发对账请求
type ReconcileRequest struct {
	Period string `json:"period"` // 账期，空为全量
}

// GenerateRequest 手动生成账单（doc36 §7-2：GenerateForUser 此前无调用方）。
type GenerateRequest struct {
	UserID uint64 `json:"user_id" binding:"required"`
	Period string `json:"period" binding:"required"` // 账期，如 202608
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
	// 分类与拆分（doc36 §3.4）
	BillType            string  `json:"bill_type"`
	ConsumeAmount       float64 `json:"consume_amount"`
	RenewalAmount       float64 `json:"renewal_amount"`
	ChannelRefundAmount float64 `json:"channel_refund_amount"`
	RefundFeeAmount     float64 `json:"refund_fee_amount"`
	// 扣点后计入口径 = 应结 + 原路退款扣点（供收入统计使用，doc36 §3.2）。
	NetAmount float64 `json:"net_amount"`
	// 支付方式描述（doc34 F-11）：结清时记录实收金额、方式与渠道实例。
	PaidAmount    float64 `json:"paid_amount"`
	PaidMethod    string  `json:"paid_method"`
	PaidChannelID uint64  `json:"paid_channel_id"`
	PaidAt        string  `json:"paid_at"`
	// 发票（doc36 §3.3）
	InvoiceStatus string `json:"invoice_status"`
	InvoiceNo     string `json:"invoice_no"`
	InvoicedAt    string `json:"invoiced_at"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
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

// —— 发票（doc36 §3.3）——

// InvoiceApplyRequest 用户申请开票。
type InvoiceApplyRequest struct {
	BillID      uint64 `json:"bill_id" binding:"required"`
	InvoiceType string `json:"invoice_type"` // normal/special，空按普票
	Title       string `json:"title" binding:"required"`
	TaxNo       string `json:"tax_no"`
	Email       string `json:"email"`
}

// InvoiceIssueRequest 管理端开票。
type InvoiceIssueRequest struct {
	InvoiceNo string `json:"invoice_no" binding:"required"`
	FileURL   string `json:"file_url"` // 预埋：发票文件地址
	Channel   string `json:"channel"`  // 预埋：manual/tax_api
	Remark    string `json:"remark"`
}

// InvoiceListQuery 发票申请查询（管理端与用户端共用）。
type InvoiceListQuery struct {
	UserID   uint64 `form:"user_id" json:"user_id"`
	Status   string `form:"status" json:"status"`
	BillNo   string `form:"bill_no" json:"bill_no"`
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"page_size" json:"page_size"`
}

// InvoiceInfo 发票申请信息。
type InvoiceInfo struct {
	ID           uint64  `json:"id"`
	RequestNo    string  `json:"request_no"`
	BillID       uint64  `json:"bill_id"`
	BillNo       string  `json:"bill_no"`
	UserID       uint64  `json:"user_id"`
	Username     string  `json:"username"`
	InvoiceType  string  `json:"invoice_type"`
	Title        string  `json:"title"`
	TaxNo        string  `json:"tax_no"`
	Amount       float64 `json:"amount"`
	Email        string  `json:"email"`
	Status       string  `json:"status"`
	Channel      string  `json:"channel"`
	ExternalNo   string  `json:"external_no"`
	FileURL      string  `json:"file_url"`
	RejectReason string  `json:"reject_reason"`
	OperatorID   uint64  `json:"operator_id"`
	IssuedAt     string  `json:"issued_at"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

// InvoiceListResponse 发票申请列表响应。
type InvoiceListResponse struct {
	Items []InvoiceInfo `json:"items"`
	Meta  ListMeta      `json:"meta"`
}

// InvoiceFileInfo 发票文件获取结果（doc36 §3.3 预埋：下载/邮件共用）。
type InvoiceFileInfo struct {
	RequestNo string `json:"request_no"`
	BillNo    string `json:"bill_no"`
	FileURL   string `json:"file_url"`
	// Delivered=false 表示本轮仅登记未接入（如邮件下发），前端按提示语展示。
	Delivered bool `json:"delivered"`
}
