package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/order/dto"
	"hostsent/backend/internal/modules/admin/order/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// RefundPolicyHandler 退款策略处理入口。
type RefundPolicyHandler struct {
	policyService service.RefundPolicyService
}

// NewRefundPolicyHandler 创建退款策略处理入口。
func NewRefundPolicyHandler(policyService service.RefundPolicyService) *RefundPolicyHandler {
	return &RefundPolicyHandler{policyService: policyService}
}

// List godoc
// @Summary 查询退款策略列表
// @Tags 订单管理-退款策略
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/admin/orders/refund-policies [get]
func (h *RefundPolicyHandler) List(c *gin.Context) {
	resp, err := h.policyService.List(c.Request.Context())
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Create godoc
// @Summary 新建退款策略
// @Tags 订单管理-退款策略
// @Security BearerAuth
// @Param request body dto.RefundPolicyRequest true "策略参数"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/orders/refund-policies [post]
func (h *RefundPolicyHandler) Create(c *gin.Context) {
	var req dto.RefundPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.policyService.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Update godoc
// @Summary 更新退款策略
// @Tags 订单管理-退款策略
// @Security BearerAuth
// @Param id path int true "策略 ID"
// @Param request body dto.RefundPolicyRequest true "策略参数"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/orders/refund-policies/{id} [put]
func (h *RefundPolicyHandler) Update(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.RefundPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.policyService.Update(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Delete godoc
// @Summary 删除退款策略
// @Tags 订单管理-退款策略
// @Security BearerAuth
// @Param id path int true "策略 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/orders/refund-policies/{id} [delete]
func (h *RefundPolicyHandler) Delete(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := h.policyService.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.SuccessMessage(c, "success")
}
