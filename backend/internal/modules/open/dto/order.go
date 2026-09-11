package dto

import (
	ucorderdto "hostsent/backend/internal/modules/uc/order/dto"
)

// OpenOrderRequest 代客下单请求（doc16 §8.3 POST /orders）。
type OpenOrderRequest struct {
	ProductID uint64 `json:"product_id" binding:"required"`
	// SpecCode 规格变体编码；商品挂有 SKU 时必填。
	SpecCode string `json:"spec_code"`
	Quantity int    `json:"quantity"`
	// CustomerRef 下游自己的终端客户标识（代客下单，落 orders.channel_customer_ref）。
	CustomerRef string `json:"customer_ref"`
	// ClientRequestID 幂等键，由 handler 从 X-Client-Request-Id 头注入。
	ClientRequestID string `json:"-"`
}

// OpenOrderResult 代客下单结果。
type OpenOrderResult struct {
	Order ucorderdto.OrderInfo `json:"order"`
	// CustomerRef 回显下游终端客户标识。
	CustomerRef string `json:"customer_ref,omitempty"`
	// IdempotentlyReplayed true 表示同 client_request_id 重放返回首次结果。
	IdempotentlyReplayed bool `json:"idempotently_replayed"`
}
