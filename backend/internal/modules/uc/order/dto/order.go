// Package dto 定义用户中心订单模块数据结构。
package dto

import "hostsent/backend/internal/pkg/pricing"

// CreateRequest 用户下单请求。
type CreateRequest struct {
	ProductID uint64 `json:"product_id" binding:"required"`
	Quantity  int    `json:"quantity"`
}

// OrderInfo 用户可见订单信息。
type OrderInfo struct {
	ID          uint64  `json:"id"`
	OrderNo     string  `json:"order_no"`
	ProductID   uint64  `json:"product_id"`
	ProductName string  `json:"product_name"`
	Specs       string  `json:"specs"`
	PriceModel  string  `json:"price_model"`
	TotalAmount float64 `json:"total_amount"`
	PaidAmount  float64 `json:"paid_amount"`
	Status      string  `json:"status"`
	PayMethod   string  `json:"pay_method"`
	// ActorID/ActorName 下单的真实操作人（子账号下单可见，P4-09）
	ActorID   uint64 `json:"actor_user_id"`
	ActorName string `json:"actor_name"`
	// 算价明细（P5-04/P5-06）：原价、优惠、实付与折扣来源，用于订单列表/详情展示。
	OriginalAmount float64 `json:"original_amount"`
	DiscountAmount float64 `json:"discount_amount"`
	FinalAmount    float64 `json:"final_amount"`
	DiscountSource string  `json:"discount_source"`
	CreatedAt      string  `json:"created_at"`
}

// QuoteRequest 预结算请求（P5-05）：不落库、不扣款。
type QuoteRequest struct {
	ProductID uint64 `json:"product_id" binding:"required"`
	Quantity  int    `json:"quantity"`
}

// QuoteInfo 预结算价格明细。
type QuoteInfo struct {
	OriginalAmount float64        `json:"original_amount"`
	DiscountAmount float64        `json:"discount_amount"`
	FinalAmount    float64        `json:"final_amount"`
	PolicyID       *uint64        `json:"price_policy_id"`
	Source         string         `json:"discount_source"`
	Snapshot       []pricing.Rule `json:"price_snapshot"`
}

// ListQuery 我的订单查询。
type ListQuery struct {
	Status   string `form:"status" json:"status"`
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"page_size" json:"page_size"`
}

// ListResponse 我的订单列表响应。
type ListResponse struct {
	Items    []OrderInfo `json:"items"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Total    int64       `json:"total"`
}
