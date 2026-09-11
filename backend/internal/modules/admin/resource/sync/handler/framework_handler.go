package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/resource/sync/dto"
	"hostsent/backend/internal/modules/admin/resource/sync/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// FrameworkHandler 同步框架（调度 / 调价 / 差异）处理入口。
type FrameworkHandler struct {
	frameworkService service.FrameworkService
}

// NewFrameworkHandler 创建同步框架处理入口。
func NewFrameworkHandler(frameworkService service.FrameworkService) *FrameworkHandler {
	return &FrameworkHandler{frameworkService: frameworkService}
}

// ListSchedules godoc
// @Summary 查询渠道 × scope 调度配置
// @Tags 资源管理-同步
// @Security BearerAuth
// @Router /api/v1/admin/resource/sync/schedules [get]
func (h *FrameworkHandler) ListSchedules(c *gin.Context) {
	var query dto.SyncScheduleListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.frameworkService.ListSchedules(c.Request.Context(), query)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// UpdateSchedule godoc
// @Summary 更新调度配置（节奏 / 启停 / 优先级 / 时间窗）
// @Tags 资源管理-同步
// @Security BearerAuth
// @Router /api/v1/admin/resource/sync/schedules/{id} [put]
func (h *FrameworkHandler) UpdateSchedule(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.SyncScheduleUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.frameworkService.UpdateSchedule(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// ScopeMeta godoc
// @Summary 查询已支持的同步 scope 元数据
// @Tags 资源管理-同步
// @Security BearerAuth
// @Router /api/v1/admin/resource/sync/scopes [get]
func (h *FrameworkHandler) ScopeMeta(c *gin.Context) {
	response.Success(c, h.frameworkService.SyncScopeMeta(c.Request.Context()))
}

// ListPriceChanges godoc
// @Summary 查询上游调价事件（待确认 / 已处置）
// @Tags 资源管理-同步
// @Security BearerAuth
// @Router /api/v1/admin/resource/sync/price-changes [get]
func (h *FrameworkHandler) ListPriceChanges(c *gin.Context) {
	var query dto.PriceChangeListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.frameworkService.ListPriceChanges(c.Request.Context(), query)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// HandlePriceChanges godoc
// @Summary 批量确认 / 驳回上游调价
// @Tags 资源管理-同步
// @Security BearerAuth
// @Router /api/v1/admin/resource/sync/price-changes/handle [post]
func (h *FrameworkHandler) HandlePriceChanges(c *gin.Context) {
	var req dto.PriceChangeHandleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	n, err := h.frameworkService.HandlePriceChanges(c.Request.Context(), req, currentAdminID(c), currentAdminName(c))
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, gin.H{"handled": n})
}

// ListDiffs godoc
// @Summary 查询同步差异记录
// @Tags 资源管理-同步
// @Security BearerAuth
// @Router /api/v1/admin/resource/sync/diffs [get]
func (h *FrameworkHandler) ListDiffs(c *gin.Context) {
	var query dto.SyncDiffListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.frameworkService.ListDiffs(c.Request.Context(), query)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// DiffSummary godoc
// @Summary 按渠道汇总同步差异（对账页数据源）
// @Tags 资源管理-同步
// @Security BearerAuth
// @Router /api/v1/admin/resource/sync/diffs/summary [get]
func (h *FrameworkHandler) DiffSummary(c *gin.Context) {
	providerID, _ := strconv.ParseUint(c.Query("provider_id"), 10, 64)
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	resp, err := h.frameworkService.DiffSummary(c.Request.Context(), providerID, days)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

func parseID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, apperrors.New(20001, "invalid id"))
		return 0, false
	}
	return id, true
}

// currentAdminID 从鉴权上下文取管理员 ID，缺失返回 0。
func currentAdminID(c *gin.Context) uint64 {
	if claims, ok := middleware.GetAdminClaims(c); ok {
		return claims.AdminID
	}
	return 0
}

// currentAdminName 从鉴权上下文取管理员名称，缺失返回空串。
func currentAdminName(c *gin.Context) string {
	if claims, ok := middleware.GetAdminClaims(c); ok {
		return claims.Username
	}
	return ""
}
