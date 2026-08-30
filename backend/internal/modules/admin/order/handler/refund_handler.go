package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/order/dto"
	"hostsent/backend/internal/modules/admin/order/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// RefundHandler 退款处理入口
type RefundHandler struct {
	orderService service.OrderService
}

// NewRefundHandler 创建退款处理入口
func NewRefundHandler(orderService service.OrderService) *RefundHandler {
	return &RefundHandler{orderService: orderService}
}

// List godoc
// @Summary 查询退款单分页列表
// @Tags 订单管理-退款
// @Security BearerAuth
// @Param keyword query string false "退款单号/原因"
// @Param order_id query int false "关联订单 ID"
// @Param status query string false "退款状态"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/refunds [get]
func (h *RefundHandler) List(c *gin.Context) {
	var query dto.RefundListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.orderService.ListRefunds(c.Request.Context(), query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Get godoc
// @Summary 查询退款单详情
// @Tags 订单管理-退款
// @Security BearerAuth
// @Param id path int true "退款单 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/refunds/{id} [get]
func (h *RefundHandler) Get(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.orderService.FindRefund(c.Request.Context(), id)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Approve godoc
// @Summary 审核通过退款
// @Tags 订单管理-退款
// @Security BearerAuth
// @Param id path int true "退款单 ID"
// @Param request body dto.RefundApproveRequest false "审核备注"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/refunds/{id}/approve [post]
func (h *RefundHandler) Approve(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.RefundApproveRequest
	_ = c.ShouldBindJSON(&req)
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.orderService.ApproveRefund(c.Request.Context(), id, operatorID, operatorName, req.Remark)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Reject godoc
// @Summary 驳回退款
// @Tags 订单管理-退款
// @Security BearerAuth
// @Param id path int true "退款单 ID"
// @Param request body dto.RefundApproveRequest false "驳回备注"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/refunds/{id}/reject [post]
func (h *RefundHandler) Reject(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.RefundApproveRequest
	_ = c.ShouldBindJSON(&req)
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.orderService.RejectRefund(c.Request.Context(), id, operatorID, operatorName, req.Remark)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}
