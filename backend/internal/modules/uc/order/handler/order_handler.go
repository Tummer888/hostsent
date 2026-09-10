// Package handler 提供用户中心订单模块 HTTP 接口。
package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/uc/order/dto"
	"hostsent/backend/internal/modules/uc/order/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// OrderHandler 用户中心订单 HTTP 处理器。
type OrderHandler struct {
	orderService service.OrderService
}

// NewOrderHandler 创建用户中心订单处理器。
func NewOrderHandler(orderService service.OrderService) *OrderHandler {
	return &OrderHandler{orderService: orderService}
}

// currentUserID 返回数据归属账号 ID（P4-04）：子账号下单/查单都落在主账号名下。
func currentUserID(c *gin.Context) (uint64, bool) {
	userID := middleware.EffectiveUserID(c)
	if userID == 0 {
		return 0, false
	}
	return userID, true
}

// currentActorID 返回真实操作人 ID：子账号下单时用于订单「操作人」列（P4-09）。
func currentActorID(c *gin.Context) uint64 {
	if actorID := middleware.ActorUserID(c); actorID > 0 {
		return actorID
	}
	return middleware.EffectiveUserID(c)
}

// Create godoc
// @Summary 下单（余额支付，自动开通）
// @Tags 用户中心-订单
// @Security BearerAuth
// @Param request body dto.CreateRequest true "下单参数"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/orders [post]
func (h *OrderHandler) Create(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Error(c, apperrors.New(10001, "unauthorized"))
		return
	}
	var req dto.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	info, err := h.orderService.Create(c.Request.Context(), userID, currentActorID(c), req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, info)
}

// List godoc
// @Summary 我的订单
// @Tags 用户中心-订单
// @Security BearerAuth
// @Param status query string false "状态"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/orders [get]
func (h *OrderHandler) List(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Error(c, apperrors.New(10001, "unauthorized"))
		return
	}
	var query dto.ListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	resp, err := h.orderService.List(c.Request.Context(), userID, query)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// Quote godoc
// @Summary 预结算价格明细
// @Description 返回商品原价、优惠、实付与折扣来源，不落库、不扣款（P5-05）
// @Tags 用户中心-订单
// @Security BearerAuth
// @Param request body dto.QuoteRequest true "预结算参数"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/orders/quote [post]
func (h *OrderHandler) Quote(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Error(c, apperrors.New(10001, "unauthorized"))
		return
	}
	var req dto.QuoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	resp, err := h.orderService.Quote(c.Request.Context(), userID, req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}
