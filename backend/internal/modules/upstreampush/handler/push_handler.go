// Package handler 提供上游推送接收端的 HTTP 入口。
//
// 这两个端点**不走任何鉴权中间件**：调用方是上游服务器，没有我方令牌体系；
// 信任来源是 body 内的 signature（用 orders.downstream_token 验签）。
// 因此路由注册时必须挂限流，且响应体只用统一结构，不泄漏内部错误细节。
package handler

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"hostsent/backend/internal/modules/upstreampush/dto"
	"hostsent/backend/internal/modules/upstreampush/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// Handler 上游推送接收入口。
type Handler struct {
	svc    service.Service
	logger *zap.Logger
}

// NewHandler 创建上游推送接收入口。
func NewHandler(svc service.Service, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{svc: svc, logger: logger}
}

// HostSync 接收主机信息推送（上游 pushHostInfo）。
//
// 注意：上游 commonCurl 把非 200 一律当作失败并计入 zjmf_pushhost 重试队列
// （至多 5 次）。因此这里对"业务上无法处理但请求本身合法"的情况（如
// 本地尚无实例行、推送早于落库）返回 200 + applied=false，只在验签失败时
// 返回业务错误码，让上游停止重试同一份伪造/错配请求。
//
// @Summary 接收上游主机信息推送
// @Tags 上游推送
// @Accept x-www-form-urlencoded
// @Success 200 {object} response.Body
// @Router /api/host/sync [post]
func (h *Handler) HostSync(c *gin.Context) {
	var req dto.HostSyncRequest
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, apperrors.New(40001, "参数解析失败"))
		return
	}
	result, err := h.svc.HandleHostSync(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, service.ErrSignature) || errors.Is(err, service.ErrNotFound) {
			// 验签失败与目标不存在对外同码同文案，避免枚举哪些订单有下游令牌。
			response.Error(c, apperrors.New(40002, "签名验证失败"))
			return
		}
		response.Error(c, apperrors.New(40001, "推送参数不合法"))
		return
	}
	response.Success(c, result)
}

// TicketReplySync 接收上游工单回复推送（上游 pushTicketReply）。
//
// @Summary 接收上游工单回复推送
// @Tags 上游推送
// @Accept x-www-form-urlencoded
// @Success 200 {object} response.Body
// @Router /api/ticket_reply/sync [post]
func (h *Handler) TicketReplySync(c *gin.Context) {
	var req dto.TicketReplySyncRequest
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, apperrors.New(40001, "参数解析失败"))
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		response.Error(c, apperrors.New(40001, "回复内容为空"))
		return
	}
	result, err := h.svc.HandleTicketReplySync(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, service.ErrSignature) || errors.Is(err, service.ErrNotFound) {
			response.Error(c, apperrors.New(40002, "签名验证失败"))
			return
		}
		response.Error(c, apperrors.New(40001, "推送参数不合法"))
		return
	}
	response.Success(c, result)
}
