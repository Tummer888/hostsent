package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/payment/dto"
	"hostsent/backend/internal/modules/admin/payment/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// ChannelHandler 支付渠道管理入口。
type ChannelHandler struct {
	svc service.ChannelService
}

// NewChannelHandler 创建渠道处理入口。
func NewChannelHandler(svc service.ChannelService) *ChannelHandler {
	return &ChannelHandler{svc: svc}
}

// ListTypes 渠道类型列表（驱动动态凭证表单与能力矩阵）。
// @Router /api/v1/admin/payment/channel-types [get]
func (h *ChannelHandler) ListTypes(c *gin.Context) {
	items, err := h.svc.ListTypes(c.Request.Context())
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, gin.H{"items": items})
}

// List 渠道列表。
// @Router /api/v1/admin/payment/channels [get]
func (h *ChannelHandler) List(c *gin.Context) {
	var q dto.ChannelListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.List(c.Request.Context(), q)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Get 渠道详情。
// @Router /api/v1/admin/payment/channels/{id} [get]
func (h *ChannelHandler) Get(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Create 新建渠道。
// @Router /api/v1/admin/payment/channels [post]
func (h *ChannelHandler) Create(c *gin.Context) {
	var req dto.ChannelCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Update 更新渠道。
// @Router /api/v1/admin/payment/channels/{id} [put]
func (h *ChannelHandler) Update(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.ChannelUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Update(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// UpdateStatus 启停渠道。
// @Router /api/v1/admin/payment/channels/{id}/status [patch]
func (h *ChannelHandler) UpdateStatus(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.ChannelStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	if err := h.svc.UpdateStatus(c.Request.Context(), id, req.Status); err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.SuccessMessage(c, "success")
}

// Test 渠道连通性测试。
// @Router /api/v1/admin/payment/channels/{id}/test [post]
func (h *ChannelHandler) Test(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.svc.Test(c.Request.Context(), id)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}
