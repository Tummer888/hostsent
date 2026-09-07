// Package handler 提供生命周期管理模块（到期管理/续费记录/生命周期策略）的 HTTP 接口。
package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	lifecycledto "hostsent/backend/internal/modules/admin/lifecycle/dto"
	lifecycleservice "hostsent/backend/internal/modules/admin/lifecycle/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// pathID 解析路径参数指定的数字 ID，解析失败时直接写入错误响应。
func pathID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, apperrors.New(20001, "invalid id"))
		return 0, false
	}
	return id, true
}

// adminIDFromContext 从鉴权上下文提取管理员 ID。
func adminIDFromContext(c *gin.Context) (uint64, bool) {
	if claims, ok := middleware.GetAdminClaims(c); ok {
		return claims.AdminID, true
	}
	return 0, false
}

// writeLifecycleError 将生命周期域业务错误映射为统一错误码。
func writeLifecycleError(err error) *apperrors.AppError {
	switch {
	case errors.Is(err, lifecycleservice.ErrInstanceNotFound),
		errors.Is(err, lifecycleservice.ErrRenewalNotFound):
		return apperrors.New(20002, err.Error())
	case errors.Is(err, lifecycleservice.ErrStatusNotAllowed),
		errors.Is(err, lifecycleservice.ErrPolicyInvalid):
		return apperrors.New(20003, err.Error())
	case errors.Is(err, lifecycleservice.ErrInvalidPeriod):
		return apperrors.New(20001, err.Error())
	case errors.Is(err, lifecycleservice.ErrPermissionDenied):
		return apperrors.New(10003, err.Error())
	case errors.Is(err, lifecycleservice.ErrInsufficientBalance):
		return apperrors.New(30001, err.Error())
	default:
		return apperrors.New(50001, err.Error())
	}
}

// ExpiringHandler 到期管理（派生阶段视图）HTTP 处理器。
type ExpiringHandler struct {
	lifecycleSvc lifecycleservice.LifecycleService
}

// NewExpiringHandler 创建到期管理处理器。
func NewExpiringHandler(lifecycleSvc lifecycleservice.LifecycleService) *ExpiringHandler {
	return &ExpiringHandler{lifecycleSvc: lifecycleSvc}
}

// List 到期实例分页列表。
// @Summary 到期实例分页列表（按生命周期阶段筛选）
// @Tags 生命周期管理-到期管理
// @Security BearerAuth
// @Param stage query string false "阶段：expiring/grace/suspended/destroyed/active，空=全部"
// @Param keyword query string false "实例标识/实例名/用户名"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/instances/expiring [get]
func (h *ExpiringHandler) List(c *gin.Context) {
	var query lifecycledto.ExpiringListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.lifecycleSvc.ListExpiring(c.Request.Context(), &query)
	if err != nil {
		response.Error(c, writeLifecycleError(err))
		return
	}
	response.Success(c, resp)
}

// LifecycleAdminHandler 管理端生命周期处理器：续费记录 + 代续费 + 策略 + 手动扫描。
type LifecycleAdminHandler struct {
	lifecycleSvc lifecycleservice.LifecycleService
	renewalSvc   lifecycleservice.RenewalService
}

// NewLifecycleAdminHandler 创建管理端生命周期处理器。
func NewLifecycleAdminHandler(lifecycleSvc lifecycleservice.LifecycleService, renewalSvc lifecycleservice.RenewalService) *LifecycleAdminHandler {
	return &LifecycleAdminHandler{lifecycleSvc: lifecycleSvc, renewalSvc: renewalSvc}
}

// ListRenewals 续费记录分页。
// @Summary 续费记录分页列表
// @Tags 生命周期管理-续费记录
// @Security BearerAuth
// @Param keyword query string false "续费单号/订单号/实例标识"
// @Param user_id query int false "用户 ID"
// @Param status query string false "状态：pending/success/failed/cancelled"
// @Param source query string false "来源：manual/auto/admin"
// @Param start_time query string false "开始日期"
// @Param end_time query string false "结束日期"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/lifecycle/renewals [get]
func (h *LifecycleAdminHandler) ListRenewals(c *gin.Context) {
	var query lifecycledto.RenewalListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.renewalSvc.ListRenewals(c.Request.Context(), &query)
	if err != nil {
		response.Error(c, writeLifecycleError(err))
		return
	}
	response.Success(c, resp)
}

// GetRenewal 续费记录详情。
// @Summary 续费记录详情
// @Tags 生命周期管理-续费记录
// @Security BearerAuth
// @Param id path int true "续费记录 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/lifecycle/renewals/{id} [get]
func (h *LifecycleAdminHandler) GetRenewal(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	resp, err := h.renewalSvc.GetRenewal(c.Request.Context(), id)
	if err != nil {
		response.Error(c, writeLifecycleError(err))
		return
	}
	response.Success(c, resp)
}

// RenewAdmin 管理员代续费。
// @Summary 管理员代续费
// @Tags 生命周期管理-到期管理
// @Security BearerAuth
// @Param id path int true "实例 ID"
// @Param body body dto.AdminRenewRequest true "续费请求"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/instances/{id}/renew [post]
func (h *LifecycleAdminHandler) RenewAdmin(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req lifecycledto.AdminRenewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	adminID, _ := adminIDFromContext(c)
	resp, err := h.renewalSvc.AdminRenew(c.Request.Context(), adminID, id, &req)
	if err != nil {
		response.Error(c, writeLifecycleError(err))
		return
	}
	response.Success(c, resp)
}

// GetPolicy 查询生命周期策略。
// @Summary 查询生命周期策略
// @Tags 生命周期管理-策略
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/admin/lifecycle/policy [get]
func (h *LifecycleAdminHandler) GetPolicy(c *gin.Context) {
	resp, err := h.lifecycleSvc.GetPolicy(c.Request.Context())
	if err != nil {
		response.Error(c, writeLifecycleError(err))
		return
	}
	response.Success(c, resp)
}

// UpdatePolicy 更新生命周期策略。
// @Summary 更新生命周期策略
// @Tags 生命周期管理-策略
// @Security BearerAuth
// @Param body body dto.PolicyUpdateRequest true "策略更新请求"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/lifecycle/policy [put]
func (h *LifecycleAdminHandler) UpdatePolicy(c *gin.Context) {
	var req lifecycledto.PolicyUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.lifecycleSvc.UpdatePolicy(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, writeLifecycleError(err))
		return
	}
	response.Success(c, resp)
}

// ScanOnce 手动触发一轮扫描（提醒+自动续费）。
// @Summary 手动触发生命周期扫描
// @Tags 生命周期管理-策略
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/admin/lifecycle/scan [post]
func (h *LifecycleAdminHandler) ScanOnce(c *gin.Context) {
	if err := h.lifecycleSvc.RunScanOnce(c.Request.Context()); err != nil {
		response.Error(c, writeLifecycleError(err))
		return
	}
	response.Success(c, gin.H{"status": "ok"})
}
