// Package handler 提供用户中心代理专区 HTTP 接口（P6-03）。
package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/uc/agent/dto"
	"hostsent/backend/internal/modules/uc/agent/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// AgentHandler 代理专区 HTTP 处理器。
type AgentHandler struct {
	agentService service.AgentService
}

// NewAgentHandler 创建代理专区处理器。
func NewAgentHandler(agentService service.AgentService) *AgentHandler {
	return &AgentHandler{agentService: agentService}
}

// Profile godoc
// @Summary 代理概览（等级/推广码/业绩）
// @Tags 用户中心-代理
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/agent/profile [get]
func (h *AgentHandler) Profile(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	resp, err := h.agentService.Profile(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, agentError(err))
		return
	}
	response.Success(c, resp)
}

// Stats godoc
// @Summary 代理看板汇总
// @Tags 用户中心-代理
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/agent/stats [get]
func (h *AgentHandler) Stats(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	resp, err := h.agentService.Stats(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, agentError(err))
		return
	}
	response.Success(c, resp)
}

// Team godoc
// @Summary 我的团队与下级
// @Tags 用户中心-代理
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/agent/team [get]
func (h *AgentHandler) Team(c *gin.Context) {
	h.list(c, "team")
}

// Commissions godoc
// @Summary 我的佣金记录
// @Tags 用户中心-代理
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/agent/commissions [get]
func (h *AgentHandler) Commissions(c *gin.Context) {
	h.list(c, "commissions")
}

// Settlements godoc
// @Summary 我的结算单
// @Tags 用户中心-代理
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/agent/settlements [get]
func (h *AgentHandler) Settlements(c *gin.Context) {
	h.list(c, "settlements")
}

// list 三个列表接口共用查询解析与错误映射。
func (h *AgentHandler) list(c *gin.Context, kind string) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	var query dto.ListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	ctx := c.Request.Context()
	var (
		data any
		err  error
	)
	switch kind {
	case "team":
		data, err = h.agentService.Team(ctx, userID, query)
	case "commissions":
		data, err = h.agentService.Commissions(ctx, userID, query)
	default:
		data, err = h.agentService.Settlements(ctx, userID, query)
	}
	if err != nil {
		response.Error(c, agentError(err))
		return
	}
	response.Success(c, data)
}

// currentUser 取归属账号 ID（子账号由中间件拒绝，不会走到这里）。
func currentUser(c *gin.Context) (uint64, bool) {
	userID := middleware.EffectiveUserID(c)
	if userID == 0 {
		response.Error(c, apperrors.New(10001, "unauthorized"))
		return 0, false
	}
	return userID, true
}

func agentError(err error) *apperrors.AppError {
	if errors.Is(err, service.ErrNotAgent) {
		return apperrors.New(40301, err.Error())
	}
	return apperrors.New(20001, err.Error())
}
