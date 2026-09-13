package handler

import (
	"github.com/gin-gonic/gin"

	notifydto "hostsent/backend/internal/modules/admin/notification/dto"
	notifyservice "hostsent/backend/internal/modules/admin/notification/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// BroadcastHandler 消息群发入口。
type BroadcastHandler struct {
	svc notifyservice.BroadcastService
}

// NewBroadcastHandler 创建群发处理入口。
func NewBroadcastHandler(svc notifyservice.BroadcastService) *BroadcastHandler {
	return &BroadcastHandler{svc: svc}
}

// Targets 目标检索（用户组列表 / 用户检索 / 条件命中数）。
// @Router /api/v1/admin/notification/broadcast/targets [get]
func (h *BroadcastHandler) Targets(c *gin.Context) {
	var q notifydto.BroadcastTargetQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Targets(c.Request.Context(), q)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

// Preview 群发预览（命中人数 + 样例 + 渲染文案 + 费用估算）。
// @Router /api/v1/admin/notification/broadcast/preview [post]
func (h *BroadcastHandler) Preview(c *gin.Context) {
	var req notifydto.BroadcastPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Preview(c.Request.Context(), req)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

// Send 执行群发（求用户集合 → 分批写投递记录 → 交 worker）。
// @Router /api/v1/admin/notification/broadcast [post]
func (h *BroadcastHandler) Send(c *gin.Context) {
	var req notifydto.BroadcastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.Send(c.Request.Context(), req)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}

// DeliveryHandler 发送日志（投递记录）入口。
type DeliveryHandler struct {
	svc notifyservice.DeliveryService
}

// NewDeliveryHandler 创建发送日志处理入口。
func NewDeliveryHandler(svc notifyservice.DeliveryService) *DeliveryHandler {
	return &DeliveryHandler{svc: svc}
}

// List 投递记录列表。
// @Router /api/v1/admin/notification/deliveries [get]
func (h *DeliveryHandler) List(c *gin.Context) {
	var q notifydto.DeliveryListQuery
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

// Get 投递详情（渲染后内容 + provider 原始码 + 变量 JSON）。
// @Router /api/v1/admin/notification/deliveries/{id} [get]
func (h *DeliveryHandler) Get(c *gin.Context) {
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

// Retry 单条重投（仅 failed / dead / skipped）。
// @Router /api/v1/admin/notification/deliveries/{id}/retry [post]
func (h *DeliveryHandler) Retry(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.svc.Retry(c.Request.Context(), id); err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.SuccessMessage(c, "requeued")
}

// BatchRetry 批量重投（逐条校验状态）。
// @Router /api/v1/admin/notification/deliveries/batch-retry [post]
func (h *DeliveryHandler) BatchRetry(c *gin.Context) {
	var req notifydto.DeliveryRetryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.BatchRetry(c.Request.Context(), req.IDs)
	if err != nil {
		response.Error(c, writeNotifyError(err))
		return
	}
	response.Success(c, resp)
}
