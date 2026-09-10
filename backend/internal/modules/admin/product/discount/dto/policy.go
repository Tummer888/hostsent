// Package dto 提供折扣策略（price policy）子域的数据传输结构。
package dto

// PolicyQuery 折扣策略列表查询。
type PolicyQuery struct {
	Keyword  string `form:"keyword" json:"keyword"`
	Status   string `form:"status" json:"status"`
	Scope    string `form:"scope" json:"scope"`
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"page_size" json:"page_size"`
}

// PolicyItemRequest 策略条目（作用域为 category/product 时的目标覆盖）。
type PolicyItemRequest struct {
	TargetType    string  `json:"target_type"`
	TargetID      uint64  `json:"target_id"`
	DiscountType  string  `json:"discount_type"`
	DiscountValue float64 `json:"discount_value"`
}

// PolicyRequest 创建/更新折扣策略。
type PolicyRequest struct {
	Name          string              `json:"name" binding:"required"`
	Code          string              `json:"code" binding:"required"`
	DiscountType  string              `json:"discount_type" binding:"required,oneof=rate amount"`
	DiscountValue float64             `json:"discount_value"`
	Scope         string              `json:"scope"`
	Priority      int                 `json:"priority"`
	EffectiveFrom string              `json:"effective_from"`
	EffectiveTo   string              `json:"effective_to"`
	Status        string              `json:"status"`
	Remark        string              `json:"remark"`
	Items         []PolicyItemRequest `json:"items"`
}

// PolicyItemInfo 策略条目信息。
type PolicyItemInfo struct {
	ID            uint64  `json:"id"`
	TargetType    string  `json:"target_type"`
	TargetID      uint64  `json:"target_id"`
	DiscountType  string  `json:"discount_type"`
	DiscountValue float64 `json:"discount_value"`
}

// PolicyInfo 折扣策略信息。
type PolicyInfo struct {
	ID            uint64           `json:"id"`
	Name          string           `json:"name"`
	Code          string           `json:"code"`
	DiscountType  string           `json:"discount_type"`
	DiscountValue float64          `json:"discount_value"`
	Scope         string           `json:"scope"`
	Priority      int              `json:"priority"`
	EffectiveFrom string           `json:"effective_from"`
	EffectiveTo   string           `json:"effective_to"`
	Status        string           `json:"status"`
	Remark        string           `json:"remark"`
	Items         []PolicyItemInfo `json:"items"`
	CreatedAt     string           `json:"created_at"`
	UpdatedAt     string           `json:"updated_at"`
}

// ListMeta 分页元信息。
type ListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// PolicyListResponse 折扣策略列表响应。
type PolicyListResponse struct {
	Items []PolicyInfo `json:"items"`
	Meta  ListMeta     `json:"meta"`
}

// QuoteResponse 预结算（P5-05）响应：直接复用算价 Quote 结构，此处仅作文档标注。
type QuoteResponse struct {
	OriginalAmount float64 `json:"original_amount"`
	DiscountAmount float64 `json:"discount_amount"`
	FinalAmount    float64 `json:"final_amount"`
	PolicyID       *uint64 `json:"price_policy_id"`
	Source         string  `json:"discount_source"`
}
