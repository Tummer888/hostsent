package handler

import (
	"github.com/gin-gonic/gin"

	notifydto "hostsent/backend/internal/modules/admin/notification/dto"
	notifyservice "hostsent/backend/internal/modules/admin/notification/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// ChannelHandler 通知渠道管理入口。
type ChannelHandler struct {
	svc     notifyservice.ChannelService
	testSvc notifyservice.TestSendService
}

// NewChannelHandler 创建渠道处理入口。
func NewChannelHandler(svc notifyservice.ChannelService, testSvc notifyservice.TestSendService) *ChannelHandler {
	return &ChannelHandler{svc: svc, testSvc: testSvc}
}

// ListTypes 渠道类型列表（驱动动态凭证表单）。
// @Router /api/v1/admin/notification/channel-types [get]
func (h *ChannelHandler) ListTypes(c *gin.Context) {
	items, err := h.svc.ListTypes(c.Request.Context(), c.Query("category"))
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, gin.H{"items": items})
}

// List 渠道列表。
// @Router /api/v1/admin/notification/channels [get]
func (h *ChannelHandler) List(c *gin.Context) {
	var q notifydto.ChannelListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.List(c.Request.Context(), q)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

// Get 渠道详情（凭证脱敏回显）。
// @Router /api/v1/admin/notification/channels/{id} [get]
func (h *ChannelHandler) Get(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	resp, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

// Create 新建渠道。
// @Router /api/v1/admin/notification/channels [post]
func (h *ChannelHandler) Create(c *gin.Context) {
	var req notifydto.ChannelCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

// Update 更新渠道。
// @Router /api/v1/admin/notification/channels/{id} [put]
func (h *ChannelHandler) Update(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req notifydto.ChannelUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Update(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

// UpdateStatus 启停渠道。
// @Router /api/v1/admin/notification/channels/{id}/status [patch]
func (h *ChannelHandler) UpdateStatus(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req notifydto.ChannelStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	if err := h.svc.UpdateStatus(c.Request.Context(), id, req.Status); err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.SuccessMessage(c, "success")
}

// Test 渠道连通性测试（占位 provider 返回 pending=true，HTTP 200 不是 500）。
// @Router /api/v1/admin/notification/channels/{id}/test [post]
func (h *ChannelHandler) Test(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req notifydto.ChannelTestRequest
	_ = c.ShouldBindJSON(&req)
	resp, err := h.svc.Test(c.Request.Context(), id, req.Target)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

// TestSend 短信/邮箱测试发送统一入口。
// @Router /api/v1/admin/notification/test-send [post]
func (h *ChannelHandler) TestSend(c *gin.Context) {
	var req notifydto.TestSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	adminID, _ := adminIDFromContext(c)
	resp, err := h.testSvc.Send(c.Request.Context(), adminID, req)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}
