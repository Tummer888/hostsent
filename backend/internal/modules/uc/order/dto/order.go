// Package dto 定义用户中心订单模块数据结构。
package dto

import "hostsent/backend/internal/pkg/pricing"

// CreateRequest 用户下单请求。
type CreateRequest struct {
	ProductID uint64 `json:"product_id" binding:"required"`
	// SpecCode 规格变体编码（SKU，T4.1）；商品挂有规格时必填，未挂规格可留空。
	SpecCode string `json:"spec_code"`
	Quantity int    `json:"quantity"`
	// Cycle 计费周期（doc25）：monthly/quarterly/annually 等规范值。
	// 留空回落商品 price_model 对应周期（存量行为不变）；该周期未开放则被拒。
	Cycle string `json:"cycle"`
	// ChannelMeta 开放平台代客下单渠道信息（P6/T6.3）。仅由 open 模块程序化注入，
	// json:"-" 保证 UC 自有 HTTP 路由无法伪造渠道标记。
	ChannelMeta ChannelMeta `json:"-"`
}

// ChannelMeta 下单渠道归属：channel='open' 的订单由下游应用代客下单，
// open_app_id 指向 open_apps.id，customer_ref 存下游自己的终端客户标识。
type ChannelMeta struct {
	Channel            string
	OpenAppID          uint64
	ChannelCustomerRef string
}

// OrderInfo 用户可见订单信息。
// 注意：orders 表无 spec_code 列，SKU 编码落在 order_items（后台订单详情可见）；
// 订单的 specs 已是所选 SKU 的规格快照。
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
	// 开通履约状态（T5.1）：pending/running/success/failed/manual，供前端轮询展示；
	// 未装配工作池或非异步开通的订单为空。
	ProvisionStatus string `json:"provision_status,omitempty"`
	// ProvisionError 开通失败原因（failed/manual 时展示，便于用户/运维排查）。
	ProvisionError string `json:"provision_error,omitempty"`
}

// QuoteRequest 预结算请求（P5-05）：不落库、不扣款。
type QuoteRequest struct {
	ProductID uint64 `json:"product_id" binding:"required"`
	// SpecCode 规格变体编码（SKU，T4.1）；与下单口径一致。
	SpecCode string `json:"spec_code"`
	Quantity int    `json:"quantity"`
	// Cycle 计费周期（doc25）；与下单口径一致，留空回落商品 price_model。
	Cycle string `json:"cycle"`
}

// QuoteInfo 预结算价格明细。
type QuoteInfo struct {
	SpecCode       string         `json:"spec_code"`
	Cycle          string         `json:"cycle"`
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
