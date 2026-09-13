// Package handler 提供工单支持模块的 HTTP 接口（管理端）。
package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/ticket/dto"
	"hostsent/backend/internal/modules/admin/ticket/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// TicketHandler 管理员工单处理入口。
type TicketHandler struct {
	ticketService   service.TicketService
	attachmentSvc   service.AttachmentService
	categoryService service.CategoryService
}

// NewTicketHandler 创建管理员工单处理入口。attachmentSvc 可为 nil（附件未启用）。
func NewTicketHandler(
	ticketService service.TicketService,
	attachmentSvc service.AttachmentService,
	categoryService service.CategoryService,
) *TicketHandler {
	return &TicketHandler{ticketService: ticketService, attachmentSvc: attachmentSvc, categoryService: categoryService}
}

// applyDataScope 按鉴权快照注入工单可见范围（doc86 §2.4）。
//
// 超管不注入 = 全量；其余按「本部门分类」裁剪，客服（无 ticket:review）再并上
// 「指派给我的」，避免客服看不到被跨部门转派过来的工单。
// 范围由服务端从 grant 推导，前端传参一律忽略。
func (h *TicketHandler) applyDataScope(c *gin.Context, query *dto.TicketListQuery) {
	adminID, _ := operatorFromContext(c)
	query.AdminID = adminID

	grant, ok := middleware.GetAdminGrant(c)
	if !ok || grant.IsSuper() {
		return
	}
	categories, err := h.ticketService.VisibleCategoriesByDepartment(c.Request.Context(), grant.DepartmentID)
	if err != nil || len(categories) == 0 {
		// 未归属部门或部门下没有启用分类：退化为「仅指派给我的」，
		// 用不可能命中的哨兵 code 表达「无部门可见分类」，避免退化成全量。
		query.VisibleCategories = []string{"__none__"}
		query.OrAssignee = adminID
		return
	}
	query.VisibleCategories = categories
	if !grant.HasAny("ticket:review") {
		query.OrAssignee = adminID
	}
}

// visibleDepartments 复核中心的部门数据范围：超管返回 nil（全量），其余只含本部门。
func (h *TicketHandler) visibleDepartments(c *gin.Context) []uint64 {
	grant, ok := middleware.GetAdminGrant(c)
	if !ok || grant.IsSuper() {
		return nil
	}
	if grant.DepartmentID == 0 {
		// 无部门归属的主管看不到任何部门的待复核，用 0 表达空集（部门 id 从 1 起）。
		return []uint64{0}
	}
	return []uint64{grant.DepartmentID}
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
// @Param review_status query string false "复核状态"
// @Param department_id query int false "部门 ID"
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
	h.applyDataScope(c, &query)
	resp, err := h.ticketService.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// ListReviews godoc
// @Summary 待复核回复队列
// @Tags 工单支持-工单
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/tickets/reviews [get]
func (h *TicketHandler) ListReviews(c *gin.Context) {
	var query dto.TicketListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	query.VisibleDepartments = h.visibleDepartments(c)
	resp, err := h.ticketService.ListReviews(c.Request.Context(), query)
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
// @Summary 管理员回复工单（支持内部备注）
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
	// 内部备注需权限：无权限时服务端强制回落为普通回复，不能只靠前端隐藏开关。
	isInternal := req.IsInternal && hasAdminPerm(c, "ticket:internal_note")
	resp, err := h.ticketService.Reply(c.Request.Context(), id, service.ReplyInput{
		SenderID:      senderID,
		SenderName:    senderName,
		Content:       req.Content,
		IsInternal:    isInternal,
		AttachmentIDs: req.AttachmentIDs,
	})
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// ReviewReply godoc
// @Summary 复核一条待复核回复（双人复核）
// @Tags 工单支持-工单
// @Security BearerAuth
// @Param replyId path int true "回复 ID"
// @Param request body dto.TicketReviewRequest true "复核参数"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/tickets/replies/{replyId}/review [post]
func (h *TicketHandler) ReviewReply(c *gin.Context) {
	replyID, ok := pathID(c, "replyId")
	if !ok {
		return
	}
	var req dto.TicketReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	reviewerID, reviewerName := operatorFromContext(c)
	resp, err := h.ticketService.ReviewReply(c.Request.Context(), replyID, service.ReviewInput{
		ReviewerID:   reviewerID,
		ReviewerName: reviewerName,
		Action:       req.Action,
		Note:         req.Note,
	})
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
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.ticketService.Assign(c.Request.Context(), id, req.AssignedTo, operatorID, operatorName)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Claim godoc
// @Summary 认领未分配工单
// @Tags 工单支持-工单
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/tickets/{id}/claim [post]
func (h *TicketHandler) Claim(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.ticketService.Claim(c.Request.Context(), id, operatorID, operatorName)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Transfer godoc
// @Summary 转派工单给其他员工
// @Tags 工单支持-工单
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Param request body dto.TicketTransferRequest true "转派参数"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/tickets/{id}/transfer [put]
func (h *TicketHandler) Transfer(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.TicketTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.ticketService.Transfer(c.Request.Context(), id, operatorID, operatorName, req.ToID, req.Note)
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
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.ticketService.UpdateStatus(c.Request.Context(), id, req.Status, operatorID, operatorName)
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

// UploadAttachment godoc
// @Summary 上传工单附件（先上传拿 ID，再随建单/回复提交）
// @Tags 工单支持-工单
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/tickets/{id}/attachments [post]
func (h *TicketHandler) UploadAttachment(c *gin.Context) {
	ticketID, ok := pathID(c, "id")
	if !ok {
		return
	}
	if h.attachmentSvc == nil {
		response.Error(c, apperrors.New(50001, "附件服务未启用"))
		return
	}
	operatorID, _ := operatorFromContext(c)
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
	isInternal := c.PostForm("is_internal") == "true" && hasAdminPerm(c, "ticket:internal_note")
	resp, err := h.attachmentSvc.Upload(c.Request.Context(), ticketID, operatorID, fileHeader.Filename, f, isInternal)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// DownloadAttachment godoc
// @Summary 下载工单附件（管理端，可见内部附件）
// @Tags 工单支持-工单
// @Security BearerAuth
// @Param id path int true "附件 ID"
// @Success 200 {file} file
// @Router /api/v1/admin/tickets/attachments/{id}/download [get]
func (h *TicketHandler) DownloadAttachment(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if h.attachmentSvc == nil {
		response.Error(c, apperrors.New(50001, "附件服务未启用"))
		return
	}
	item, reader, err := h.attachmentSvc.Open(c.Request.Context(), id, true, nil)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	defer reader.Close()
	// 落盘名是随机串，这里用原始文件名回填，避免用户下载到无语义的名字。
	writeAttachment(c, item.FileName, item.FileType, item.FileSize, reader)
}
