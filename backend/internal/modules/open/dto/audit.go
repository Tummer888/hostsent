package dto

// AuditRequestItem 对账：幂等请求记录。
type AuditRequestItem struct {
	ClientRequestID string `json:"client_request_id"`
	Method          string `json:"method"`
	Path            string `json:"path"`
	Status          string `json:"status"` // processing / completed
	ResponseStatus  *int   `json:"response_status,omitempty"`
	// ResponseBody 首次响应快照（成功或业务错误），重放口径与此一致。
	ResponseBody string `json:"response_body,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}

// AuditOrderItem 对账：代客下单订单（标准金额口径，与下单响应一致）。
type AuditOrderItem struct {
	OrderNo        string  `json:"order_no"`
	ProductID      uint64  `json:"product_id"`
	ProductName    string  `json:"product_name"`
	Status         string  `json:"status"`
	OriginalAmount float64 `json:"original_amount"`
	FinalAmount    float64 `json:"final_amount"`
	PaidAmount     float64 `json:"paid_amount"`
	BillingCycle   string  `json:"billing_cycle,omitempty"`
	CustomerRef    string  `json:"customer_ref,omitempty"`
	CreatedAt      string  `json:"created_at,omitempty"`
}

// AuditListResponse 通用分页。
type AuditListResponse struct {
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}
