// Package dto 提供定价与计费（pricing）子域的数据传输结构。
package dto

// ListMeta 通用分页元信息
type ListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// PricingQuery 价格策略列表查询
type PricingQuery struct {
	ProductID    uint64 `form:"product_id" json:"product_id"`
	BillingMode  string `form:"billing_mode" json:"billing_mode"`
	Status       int    `form:"status" json:"status"`
	Page         int    `form:"page" json:"page"`
	PageSize     int    `form:"page_size" json:"page_size"`
}

// PricingRequest 创建/更新价格策略
type PricingRequest struct {
	ProductID      uint64  `json:"product_id" binding:"required"`
	BillingMode    string  `json:"billing_mode" binding:"required"`
	UnitPrice      float64 `json:"unit_price"`
	MinBillingUnit int     `json:"min_billing_unit"`
	TierPricing    string  `json:"tier_pricing"`
	TierDiscount   string  `json:"tier_discount"`
	Status         int     `json:"status"`
}

// PricingInfo 价格策略信息
type PricingInfo struct {
	ID             uint64  `json:"id"`
	ProductID      uint64  `json:"product_id"`
	BillingMode    string  `json:"billing_mode"`
	UnitPrice      float64 `json:"unit_price"`
	MinBillingUnit int     `json:"min_billing_unit"`
	TierPricing    string  `json:"tier_pricing"`
	TierDiscount   string  `json:"tier_discount"`
	Status         int     `json:"status"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

// PricingListResponse 价格策略列表响应
type PricingListResponse struct {
	Items []PricingInfo `json:"items"`
	Meta  ListMeta      `json:"meta"`
}
