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
	attachmentSvc   service.AttachmentService
}

// NewUserTicketHandler 创建用户中心工单处理器（attachmentSvc 可为 nil）。
func NewUserTicketHandler(
	ticketService service.TicketService,
	categoryService service.CategoryService,
	attachmentSvc service.AttachmentService,
) *UserTicketHandler {
	return &UserTicketHandler{ticketService: ticketService, categoryService: categoryService, attachmentSvc: attachmentSvc}
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
	// 用户侧不支持内部备注：服务端强制置 false，不信任请求体。
	req.IsInternal = false
	resp, err := h.ticketService.UserReply(c.Request.Context(), accountID, actorID, username, id, req)
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
// @Summary 获取可用工单分类列表（含提交前置条件与实名状态）
// @Tags 用户中心-工单
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/support/ticket-categories [get]
func (h *UserTicketHandler) ListCategories(c *gin.Context) {
	accountID, _, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	actorID := accountID
	if id, _, ok := currentActorID(c); ok {
		actorID = id
	}
	resp, err := h.categoryService.ListForUser(c.Request.Context(), accountID, actorID)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// UploadAttachment godoc
// @Summary 上传我的工单附件
// @Tags 用户中心-工单
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/support/tickets/{id}/attachments [post]
func (h *UserTicketHandler) UploadAttachment(c *gin.Context) {
	accountID, _, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	if h.attachmentSvc == nil {
		response.Error(c, apperrors.New(50001, "附件服务未启用"))
		return
	}
	ticketID, valid := pathID(c, "id")
	if !valid {
		return
	}
	// 归属校验：只能往自己账号家族的工单上传附件，防止借上传接口探测他人工单。
	// 待挂载（ticket_id=0）时不校验，此时附件尚未关联任何工单。
	if ticketID > 0 {
		if _, err := h.ticketService.UserFindByID(c.Request.Context(), accountID, ticketID); err != nil {
			response.Error(c, writeError(err))
			return
		}
	}
	actorID, _, ok := currentActorID(c)
	if !ok {
		unauthorized(c)
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Error(c, apperrors.New(20001, "请选择要上传的文件"))
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		response.Error(c, apperrors.New(20001, "无法读取上传文件"))
		return
	}
	defer f.Close()
	// 用户侧不能上传内部附件：服务端强制 false。
	resp, err := h.attachmentSvc.Upload(c.Request.Context(), ticketID, actorID, fileHeader.Filename, f, false)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// DownloadAttachment godoc
// @Summary 下载我的工单附件
// @Tags 用户中心-工单
// @Security BearerAuth
// @Param id path int true "附件 ID"
// @Success 200 {file} file
// @Router /api/v1/uc/support/attachments/{id}/download [get]
func (h *UserTicketHandler) DownloadAttachment(c *gin.Context) {
	accountID, _, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	if h.attachmentSvc == nil {
		response.Error(c, apperrors.New(50001, "附件服务未启用"))
		return
	}
	id, valid := pathID(c, "id")
	if !valid {
		return
	}
	userIDs := h.ticketService.AccountUserIDs(c.Request.Context(), accountID)
	if len(userIDs) == 0 {
		unauthorized(c)
		return
	}
	item, reader, err := h.attachmentSvc.Open(c.Request.Context(), id, false, userIDs)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	defer reader.Close()
	writeAttachment(c, item.FileName, item.FileType, item.FileSize, reader)
}
