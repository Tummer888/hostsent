// Package handler 用户中心续费管理 HTTP 处理器。
// 用户自助只能操作自身实例与记录，user_id 一律取自鉴权上下文，不能由前端指定。
package handler

import (
	"github.com/gin-gonic/gin"

	lifecycledto "hostsent/backend/internal/modules/admin/lifecycle/dto"
	lifecycleservice "hostsent/backend/internal/modules/admin/lifecycle/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// LifecycleUserHandler 用户中心续费管理处理器。
type LifecycleUserHandler struct {
	lifecycleSvc lifecycleservice.LifecycleService
	renewalSvc   lifecycleservice.RenewalService
}

// NewLifecycleUserHandler 创建用户中心续费管理处理器。
func NewLifecycleUserHandler(lifecycleSvc lifecycleservice.LifecycleService, renewalSvc lifecycleservice.RenewalService) *LifecycleUserHandler {
	return &LifecycleUserHandler{lifecycleSvc: lifecycleSvc, renewalSvc: renewalSvc}
}

// currentUserID 返回数据归属账号 ID（P4-04）：续费/到期视图一律取主账号。
func currentUserID(c *gin.Context) (uint64, bool) {
	userID := middleware.EffectiveUserID(c)
	if userID == 0 {
		return 0, false
	}
	return userID, true
}

// unauthorized 用户未登录。
func unauthorized(c *gin.Context) {
	response.Error(c, apperrors.New(10001, "unauthorized"))
}

// RenewalsView 我的续费管理聚合视图（我的实例到期情况 + 策略摘要）。
// @Summary 我的续费管理视图
// @Tags 用户中心-续费管理
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/cloud/renewals/view [get]
func (h *LifecycleUserHandler) RenewalsView(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	resp, err := h.renewalSvc.UserRenewalsView(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, writeLifecycleError(err))
		return
	}
	response.Success(c, resp)
}

// Renew 用户发起续费（生成待支付订单并尝试余额支付）。
// @Summary 用户发起续费
// @Tags 用户中心-续费管理
// @Security BearerAuth
// @Param id path int true "实例 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/cloud/instances/{id}/renew [post]
func (h *LifecycleUserHandler) Renew(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req lifecycledto.UserRenewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许空 body，默认续费 1 个周期
		req.PeriodCount = 0
	}
	resp, err := h.renewalSvc.UserRenew(c.Request.Context(), userID, id, &req)
	if err != nil {
		response.Error(c, writeLifecycleError(err))
		return
	}
	response.Success(c, resp)
}

// ToggleAutoRenew 设置实例自动续费开关。
// @Summary 设置自动续费开关
// @Tags 用户中心-续费管理
// @Security BearerAuth
// @Param id path int true "实例 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/cloud/instances/{id}/auto-renew [put]
func (h *LifecycleUserHandler) ToggleAutoRenew(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req lifecycledto.AutoRenewToggleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	if err := h.renewalSvc.ToggleAutoRenew(c.Request.Context(), userID, id, &req); err != nil {
		response.Error(c, writeLifecycleError(err))
		return
	}
	response.Success(c, gin.H{"status": "ok"})
}

// Records 我的续费记录分页。
// @Summary 我的续费记录
// @Tags 用户中心-续费管理
// @Security BearerAuth
// @Param status query string false "状态：pending/success/failed/cancelled"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/cloud/renewals [get]
func (h *LifecycleUserHandler) Records(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	var query lifecycledto.UserRenewalListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.renewalSvc.UserRenewalRecords(c.Request.Context(), userID, &query)
	if err != nil {
		response.Error(c, writeLifecycleError(err))
		return
	}
	response.Success(c, resp)
}

// Detail 我的续费记录详情（校验归属）。
// @Summary 我的续费记录详情
// @Tags 用户中心-续费管理
// @Security BearerAuth
// @Param id path int true "续费记录 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/cloud/renewals/{id} [get]
func (h *LifecycleUserHandler) Detail(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	resp, err := h.renewalSvc.UserRenewalDetail(c.Request.Context(), userID, id)
	if err != nil {
		response.Error(c, writeLifecycleError(err))
		return
	}
	response.Success(c, resp)
}
