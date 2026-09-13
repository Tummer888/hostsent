package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/sales/dto"
	"hostsent/backend/internal/modules/admin/sales/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// PerformanceHandler 业绩排行与目标管理端处理器。
type PerformanceHandler struct {
	performance service.PerformanceService
}

// NewPerformanceHandler 创建业绩处理器。
func NewPerformanceHandler(performance service.PerformanceService) *PerformanceHandler {
	return &PerformanceHandler{performance: performance}
}

// Ranking 业绩排行。
// @Summary 业绩排行
// @Tags 销售-业绩
// @Security BearerAuth
// @Param period query string false "周期 YYYY-MM，默认当月"
// @Param department_id query int false "按部门过滤"
// @Param sort_by query string false "排序：amount/orders"
// @Router /api/v1/admin/sales/performance/ranking [get]
func (h *PerformanceHandler) Ranking(c *gin.Context) {
	var query dto.RankingQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.performance.Ranking(c.Request.Context(), resolveScope(c), query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Targets 目标列表（含实时达成）。
// @Summary 业绩目标列表
// @Tags 销售-业绩
// @Security BearerAuth
// @Param period query string false "周期 YYYY-MM，默认当月"
// @Param department_id query int false "按部门过滤"
// @Router /api/v1/admin/sales/performance/targets [get]
func (h *PerformanceHandler) Targets(c *gin.Context) {
	var query dto.RankingQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.performance.ListTargets(c.Request.Context(), resolveScope(c), query.Period, query.DepartmentID)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// upsertTargetsRequest 目标批量下发入参。
type upsertTargetsRequest struct {
	Items []dto.TargetUpsertRequest `json:"items" binding:"required"`
}

// UpsertTargets 目标下发（按行 upsert）。
// @Summary 下发业绩目标
// @Tags 销售-业绩
// @Security BearerAuth
// @Router /api/v1/admin/sales/performance/targets [put]
func (h *PerformanceHandler) UpsertTargets(c *gin.Context) {
	var req upsertTargetsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, _ := operatorFromContext(c)
	if err := h.performance.UpsertTargets(c.Request.Context(), resolveScope(c), operatorID, req.Items); err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// Me 我的业绩与目标达成。
// @Summary 我的业绩
// @Tags 销售-业绩
// @Security BearerAuth
// @Param period query string false "周期 YYYY-MM，默认当月"
// @Param admin_id query int false "指定销售（主管/超管）"
// @Router /api/v1/admin/sales/performance/me [get]
func (h *PerformanceHandler) Me(c *gin.Context) {
	scope := resolveScope(c)
	adminID, _ := operatorFromContext(c)
	if scope.All {
		if v, err := strconvParseUint(c.Query("admin_id")); err == nil && v > 0 {
			adminID = v
		}
	}
	resp, err := h.performance.MyPerformance(c.Request.Context(), adminID, c.Query("period"))
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}
