package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/finance/stats/dto"
	"hostsent/backend/internal/modules/admin/finance/stats/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// StatsHandler 财务统计处理入口。
type StatsHandler struct {
	statsService service.StatsService
}

// NewStatsHandler 创建财务统计处理入口。
func NewStatsHandler(statsService service.StatsService) *StatsHandler {
	return &StatsHandler{statsService: statsService}
}

// Stats godoc
// @Summary 财务统计聚合（总览与报表共用）
// @Description 期间 KPI、趋势、类型分布、账单口径、待办与钱包口径，全部由数据库侧全量聚合。
// @Tags 财务管理-统计
// @Security BearerAuth
// @Param start_time query string false "开始日期 YYYY-MM-DD（默认近 30 天）"
// @Param end_time query string false "结束日期 YYYY-MM-DD（默认今天）"
// @Param granularity query string false "粒度 day/month（默认 day，跨度 > 366 天自动按月）"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/finance/stats [get]
func (h *StatsHandler) Stats(c *gin.Context) {
	var query dto.StatsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.statsService.Stats(c.Request.Context(), query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// writeError 将财务统计子域错误映射为统一错误码。
func writeError(err error) *apperrors.AppError {
	return apperrors.New(50001, err.Error())
}
