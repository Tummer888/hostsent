// Package handler 提供对账差异台账的 HTTP 入口。
package handler

import (
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	reconmodel "hostsent/backend/internal/modules/admin/resource/reconcile/model"
	"hostsent/backend/internal/modules/admin/resource/reconcile/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// FindingHandler 差异台账处理入口。
type FindingHandler struct {
	scanner service.FindingScanner
}

// NewFindingHandler 创建差异台账处理入口。
func NewFindingHandler(scanner service.FindingScanner) *FindingHandler {
	return &FindingHandler{scanner: scanner}
}

// ListFindings godoc
// @Summary 查询对账差异台账（未处理/已确认的差异）
// @Description 持久化的差异记录：售价低于成本、毛利过薄、到期错位、状态不同步；
// @Description 含首次检出时间（"已持续 N 天"）与处置状态。
// @Tags 资源管理-运维
// @Security BearerAuth
// @Param provider_id query int false "渠道 ID"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/resource/reconcile/findings [get]
func (h *FindingHandler) ListFindings(c *gin.Context) {
	var q struct {
		ProviderID uint64 `form:"provider_id"`
		// ProviderKind 链路筛选（upstream/compute）。
		ProviderKind string `form:"provider_kind"`
		Page         int    `form:"page"`
		PageSize     int    `form:"page_size"`
	}
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	items, total, err := h.scanner.ListOpen(c.Request.Context(), q.ProviderID, q.ProviderKind, q.Page, q.PageSize)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	list := make([]gin.H, 0, len(items))
	for i := range items {
		it := items[i]
		row := gin.H{
			"id":             it.ID,
			"provider_id":    it.ProviderID,
			"instance_id":    it.InstanceID,
			"kind":           it.Kind,
			"kind_name":      reconmodel.KindLabel(it.Kind),
			"severity":       it.Severity,
			"local_value":    it.LocalValue,
			"upstream_value": it.UpstreamValue,
			"status":         it.Status,
			"note":           it.Note,
			"first_seen_at":  it.FirstSeenAt,
			"last_seen_at":   it.LastSeenAt,
			"notified_at":    it.NotifiedAt,
			"days_persisted": daysBetween(it.FirstSeenAt, it.LastSeenAt),
		}
		if it.Delta != nil {
			row["delta"] = *it.Delta
		}
		list = append(list, row)
	}
	response.Page(c, total, list, normPage(q.Page), normPageSize(q.PageSize))
}

// UpdateFinding godoc
// @Summary 处置一条对账差异（确认/忽略/重新打开）
// @Tags 资源管理-运维
// @Security BearerAuth
// @Param id path int true "差异 ID"
// @Param body body object true "{status, note}"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/resource/reconcile/findings/{id} [put]
func (h *FindingHandler) UpdateFinding(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, apperrors.New(20001, "invalid id"))
		return
	}
	var body struct {
		Status string `json:"status" binding:"required"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	var adminID uint64
	if claims, ok := middleware.GetAdminClaims(c); ok {
		adminID = claims.AdminID
	}
	if err := h.scanner.UpdateStatus(c.Request.Context(), id, body.Status, adminID, body.Note); err != nil {
		if errors.Is(err, service.ErrInvalidFindingStatus) {
			response.Error(c, apperrors.New(20001, err.Error()))
			return
		}
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.SuccessMessage(c, "已更新")
}

func normPage(p int) int {
	if p < 1 {
		return 1
	}
	return p
}

func normPageSize(p int) int {
	if p <= 0 {
		return 20
	}
	if p > 100 {
		return 100
	}
	return p
}

// daysBetween 已持续天数（保留一位小数，便于"不到一天"也看得出）。
// 用 LastSeenAt 而不是 now：扫描是周期的，用最后检出时间才能反映台账自身的数据新鲜度。
func daysBetween(first, last time.Time) float64 {
	if first.IsZero() || last.Before(first) {
		return 0
	}
	return math.Round(last.Sub(first).Hours()/24*10) / 10
}
