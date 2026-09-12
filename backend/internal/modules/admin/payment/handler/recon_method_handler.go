package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/payment/dto"
	"hostsent/backend/internal/modules/admin/payment/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// ReconHandler 渠道对账入口。
type ReconHandler struct {
	svc service.ReconService
}

// NewReconHandler 创建对账处理入口。
func NewReconHandler(svc service.ReconService) *ReconHandler {
	return &ReconHandler{svc: svc}
}

// Reconcile 执行渠道对账。
// @Router /api/v1/admin/payment/recon [post]
func (h *ReconHandler) Reconcile(c *gin.Context) {
	var req dto.ReconRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Reconcile(c.Request.Context(), req)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// List 对账记录列表。
// @Router /api/v1/admin/payment/recon [get]
func (h *ReconHandler) List(c *gin.Context) {
	page, pageSize := pageParams(c)
	resp, err := h.svc.List(c.Request.Context(), page, pageSize)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// MethodHandler 支付方式路由（场景规则与优先级）入口。
type MethodHandler struct {
	svc service.PreferenceService
	ch  service.ChannelService
}

// NewMethodHandler 创建支付方式处理入口。
func NewMethodHandler(svc service.PreferenceService, ch service.ChannelService) *MethodHandler {
	return &MethodHandler{svc: svc, ch: ch}
}

// Options 管理端查看某场景可用方式（渠道 + 优先级）。
// @Router /api/v1/admin/payment/methods [get]
func (h *MethodHandler) Options(c *gin.Context) {
	scene := c.DefaultQuery("scene", "native")
	resp, err := h.svc.MethodOptions(c.Request.Context(), 0, scene, 0)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}
