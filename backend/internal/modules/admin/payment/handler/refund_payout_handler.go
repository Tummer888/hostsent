package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/payment/dto"
	"hostsent/backend/internal/modules/admin/payment/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// RefundHandler 渠道退款单入口。
type RefundHandler struct {
	svc service.RefundService
}

// NewRefundHandler 创建退款处理入口。
func NewRefundHandler(svc service.RefundService) *RefundHandler {
	return &RefundHandler{svc: svc}
}

// List 退款单列表。
// @Router /api/v1/admin/payment/refunds [get]
func (h *RefundHandler) List(c *gin.Context) {
	var q dto.RefundListQuery
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

// PayoutHandler 打款单入口。
type PayoutHandler struct {
	svc service.PayoutService
}

// NewPayoutHandler 创建打款处理入口。
func NewPayoutHandler(svc service.PayoutService) *PayoutHandler {
	return &PayoutHandler{svc: svc}
}

// List 打款单列表。
// @Router /api/v1/admin/payment/payouts [get]
func (h *PayoutHandler) List(c *gin.Context) {
	var q dto.PayoutListQuery
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

// MarkPaid 人工打款登记。
// @Router /api/v1/admin/payment/payouts/{id}/mark-paid [post]
func (h *PayoutHandler) MarkPaid(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.PayoutMarkPaidRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, _ := operatorFromContext(c)
	resp, err := h.svc.AdminMarkPaid(c.Request.Context(), id, req, operatorID)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Retry 重试打款。
// @Router /api/v1/admin/payment/payouts/{id}/retry [post]
func (h *PayoutHandler) Retry(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	operatorID, _ := operatorFromContext(c)
	resp, err := h.svc.AdminRetry(c.Request.Context(), id, operatorID)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}
