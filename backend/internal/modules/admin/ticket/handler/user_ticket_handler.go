package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/ticket/dto"
	"hostsent/backend/internal/modules/admin/ticket/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// UserTicketHandler 用户中心工单 HTTP 处理器。
// 用户自助只能访问自身工单，user_id 一律取自鉴权上下文，不能由前端指定。
type UserTicketHandler struct {
	ticketService   service.TicketService
	categoryService service.CategoryService
}

// NewUserTicketHandler 创建用户中心工单处理器。
func NewUserTicketHandler(ticketService service.TicketService, categoryService service.CategoryService) *UserTicketHandler {
	return &UserTicketHandler{ticketService: ticketService, categoryService: categoryService}
}

// userTicketPageQuery 用户端列表分页参数。
type userTicketPageQuery struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}

// List godoc
// @Summary 查询我的工单列表
// @Tags 用户中心-工单
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/support/tickets [get]
func (h *UserTicketHandler) List(c *gin.Context) {
	userID, _, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	var query userTicketPageQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.ticketService.UserList(c.Request.Context(), userID, query.Page, query.PageSize)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Get godoc
// @Summary 查询我的工单详情（含回复记录）
// @Tags 用户中心-工单
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/support/tickets/{id} [get]
func (h *UserTicketHandler) Get(c *gin.Context) {
	userID, _, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	id, valid := pathID(c, "id")
	if !valid {
		return
	}
	resp, err := h.ticketService.UserFindByID(c.Request.Context(), userID, id)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Create godoc
// @Summary 提交工单
// @Tags 用户中心-工单
// @Security BearerAuth
// @Param request body dto.TicketCreateRequest true "工单参数"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/support/tickets [post]
func (h *UserTicketHandler) Create(c *gin.Context) {
	// 提交人 = 真实操作人（P4-05）：子账号提交的工单记在其本人名下，主账号按归属可查。
	actorID, username, ok := currentActorID(c)
	if !ok {
		unauthorized(c)
		return
	}
	var req dto.TicketCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.ticketService.Create(c.Request.Context(), actorID, username, req)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Reply godoc
// @Summary 追加工单回复
// @Tags 用户中心-工单
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Param request body dto.TicketReplyRequest true "回复内容"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/support/tickets/{id}/replies [post]
func (h *UserTicketHandler) Reply(c *gin.Context) {
	// 归属账号用于权限校验，操作人用于回复落库（P4-05）。
	accountID, _, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	actorID, username, ok := currentActorID(c)
	if !ok {
		unauthorized(c)
		return
	}
	id, valid := pathID(c, "id")
	if !valid {
		return
	}
	var req dto.TicketReplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.ticketService.UserReply(c.Request.Context(), accountID, actorID, username, id, req.Content)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Cancel godoc
// @Summary 取消工单（仅待响应可取消）
// @Tags 用户中心-工单
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/support/tickets/{id}/cancel [post]
func (h *UserTicketHandler) Cancel(c *gin.Context) {
	userID, _, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	id, valid := pathID(c, "id")
	if !valid {
		return
	}
	resp, err := h.ticketService.Cancel(c.Request.Context(), userID, id)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// ListCategories godoc
// @Summary 获取可用工单分类列表
// @Tags 用户中心-工单
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/support/ticket-categories [get]
func (h *UserTicketHandler) ListCategories(c *gin.Context) {
	_, _, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	resp, err := h.categoryService.List(c.Request.Context(), false)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}
