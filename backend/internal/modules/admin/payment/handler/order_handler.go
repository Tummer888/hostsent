package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/payment/dto"
	"hostsent/backend/internal/modules/admin/payment/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// OrderHandler 支付单管理入口。
type OrderHandler struct {
	svc service.OrderService
}

// NewOrderHandler 创建支付单处理入口。
func NewOrderHandler(svc service.OrderService) *OrderHandler {
	return &OrderHandler{svc: svc}
}

// List 支付单列表。
// @Router /api/v1/admin/payment/orders [get]
func (h *OrderHandler) List(c *gin.Context) {
	var q dto.OrderListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.List(c.Request.Context(), q)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Get 支付单详情。
// @Router /api/v1/admin/payment/orders/{id} [get]
func (h *OrderHandler) Get(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Confirm 人工确认到账（线下渠道）。
// @Router /api/v1/admin/payment/orders/{id}/confirm [post]
func (h *OrderHandler) Confirm(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.OrderConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, _ := operatorFromContext(c)
	resp, err := h.svc.Confirm(c.Request.Context(), id, req, operatorID)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Close 关闭支付单。
// @Router /api/v1/admin/payment/orders/{id}/close [post]
func (h *OrderHandler) Close(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.svc.Close(c.Request.Context(), id)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Sync 主动查单补偿。
// @Router /api/v1/admin/payment/orders/{id}/sync [post]
func (h *OrderHandler) Sync(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.svc.Sync(c.Request.Context(), id)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}
