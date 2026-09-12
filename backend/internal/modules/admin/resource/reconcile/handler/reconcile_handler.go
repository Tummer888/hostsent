// Package handler 提供实例对账 HTTP 入口。
package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/resource/reconcile/dto"
	"hostsent/backend/internal/modules/admin/resource/reconcile/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// ReconcileHandler 实例对账处理入口。
type ReconcileHandler struct {
	svc service.ReconcileService
}

// NewReconcileHandler 创建实例对账处理入口。
func NewReconcileHandler(svc service.ReconcileService) *ReconcileHandler {
	return &ReconcileHandler{svc: svc}
}

// List godoc
// @Summary 查询实例对账（本地售价/到期 vs 上游成本/到期）
// @Description 面向已开通的上游链路实例，比对本地售价与上游成本、本地到期与上游到期，标注异常等级。
// @Tags 资源管理-运维
// @Security BearerAuth
// @Param provider_id query int false "渠道 ID"
// @Param keyword query string false "实例名/实例号/商品名关键词"
// @Param anomaly query string false "异常过滤 danger/warning/ok/below_cost/thin_margin/expire_early/expire_late/missing"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/resource/reconcile [get]
func (h *ReconcileHandler) List(c *gin.Context) {
	var query dto.ReconcileListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}
