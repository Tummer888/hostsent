package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/order/dto"
	"hostsent/backend/internal/modules/admin/order/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// OrderHandler 订单处理入口
type OrderHandler struct {
	orderService service.OrderService
}

// NewOrderHandler 创建订单处理入口
func NewOrderHandler(orderService service.OrderService) *OrderHandler {
	return &OrderHandler{orderService: orderService}
}

// List godoc
// @Summary 查询订单分页列表
// @Tags 订单管理-订单
// @Security BearerAuth
// @Param keyword query string false "订单号/产品名"
// @Param user_id query int false "用户 ID"
// @Param product_id query int false "产品 ID"
// @Param status query string false "订单状态"
// @Param pay_method query string false "支付方式"
// @Param start_time query string false "开始时间"
// @Param end_time query string false "结束时间"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/orders [get]
func (h *OrderHandler) List(c *gin.Context) {
	var query dto.OrderListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.orderService.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Get godoc
// @Summary 查询订单详情（含明细项与退款记录）
// @Tags 订单管理-订单
// @Security BearerAuth
// @Param id path int true "订单 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/orders/{id} [get]
func (h *OrderHandler) Get(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.orderService.FindByID(c.Request.Context(), id)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Cancel godoc
// @Summary 取消订单
// @Tags 订单管理-订单
// @Security BearerAuth
// @Param id path int true "订单 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/orders/{id}/cancel [post]
func (h *OrderHandler) Cancel(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	if err := h.orderService.Cancel(c.Request.Context(), id, operatorID, operatorName); err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.SuccessMessage(c, "success")
}

// UpdateRemark godoc
// @Summary 更新订单备注
// @Tags 订单管理-订单
// @Security BearerAuth
// @Param id path int true "订单 ID"
// @Param request body dto.OrderRemarkUpdateRequest true "备注"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/orders/{id}/remark [put]
func (h *OrderHandler) UpdateRemark(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.OrderRemarkUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, _ := operatorFromContext(c)
	if err := h.orderService.UpdateRemark(c.Request.Context(), id, req.Remark, operatorID); err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.SuccessMessage(c, "success")
}

// CreateRefund godoc
// @Summary 创建退款单
// @Tags 订单管理-订单
// @Security BearerAuth
// @Param id path int true "订单 ID"
// @Param request body dto.RefundCreateRequest true "退款参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/orders/{id}/refund [post]
func (h *OrderHandler) CreateRefund(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.RefundCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.orderService.CreateRefund(c.Request.Context(), id, req, operatorID, operatorName)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Activate godoc
// @Summary 重新触发开通实例（幂等）
// @Tags 订单管理-订单
// @Security BearerAuth
// @Param id path int true "订单 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/orders/{id}/activate [post]
func (h *OrderHandler) Activate(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := h.orderService.Activate(c.Request.Context(), id); err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.SuccessMessage(c, "success")
}

// Stats godoc
// @Summary 查询订单统计概览
// @Tags 订单管理-订单
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/orders/stats [get]
func (h *OrderHandler) Stats(c *gin.Context) {
	resp, err := h.orderService.Stats(c.Request.Context())
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}
