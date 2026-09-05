// Package handler 提供工单支持模块的 HTTP 接口（管理端）。
package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/ticket/dto"
	"hostsent/backend/internal/modules/admin/ticket/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// TicketHandler 管理员工单处理入口。
type TicketHandler struct {
	ticketService service.TicketService
}

// NewTicketHandler 创建管理员工单处理入口。
func NewTicketHandler(ticketService service.TicketService) *TicketHandler {
	return &TicketHandler{ticketService: ticketService}
}

// List godoc
// @Summary 查询工单分页列表
// @Tags 工单支持-工单
// @Security BearerAuth
// @Param keyword query string false "工单号/标题"
// @Param user_keyword query string false "用户账号"
// @Param user_id query int false "用户 ID"
// @Param category query string false "分类编码"
// @Param priority query string false "优先级"
// @Param status query string false "工单状态"
// @Param assigned_to query int false "处理人 ID"
// @Param start_time query string false "开始时间"
// @Param end_time query string false "结束时间"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/tickets [get]
func (h *TicketHandler) List(c *gin.Context) {
	var query dto.TicketListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.ticketService.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Get godoc
// @Summary 查询工单详情（含回复记录）
// @Tags 工单支持-工单
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/tickets/{id} [get]
func (h *TicketHandler) Get(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.ticketService.FindByID(c.Request.Context(), id)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Reply godoc
// @Summary 管理员回复工单
// @Tags 工单支持-工单
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Param request body dto.TicketReplyRequest true "回复内容"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/tickets/{id}/reply [post]
func (h *TicketHandler) Reply(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.TicketReplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	senderID, senderName := operatorFromContext(c)
	resp, err := h.ticketService.Reply(c.Request.Context(), id, senderID, senderName, req.Content)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Assign godoc
// @Summary 分配工单给指定管理员
// @Tags 工单支持-工单
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Param request body dto.TicketAssignRequest true "分配参数"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/tickets/{id}/assign [put]
func (h *TicketHandler) Assign(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.TicketAssignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.ticketService.Assign(c.Request.Context(), id, req.AssignedTo)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// UpdateStatus godoc
// @Summary 更新工单状态（状态机校验）
// @Tags 工单支持-工单
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Param request body dto.TicketStatusRequest true "目标状态"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/tickets/{id}/status [put]
func (h *TicketHandler) UpdateStatus(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.TicketStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.ticketService.UpdateStatus(c.Request.Context(), id, req.Status)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Close godoc
// @Summary 关闭工单
// @Tags 工单支持-工单
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/tickets/{id}/close [post]
func (h *TicketHandler) Close(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.ticketService.Close(c.Request.Context(), id, operatorID, operatorName)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Stats godoc
// @Summary 查询工单统计概览
// @Tags 工单支持-工单
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/admin/tickets/stats [get]
func (h *TicketHandler) Stats(c *gin.Context) {
	resp, err := h.ticketService.Stats(c.Request.Context())
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}
