package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/order/dto"
	"hostsent/backend/internal/modules/admin/order/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// RefundHandler 退款处理入口。
//
// attachmentService 可为 nil（附件存储未启用）：上传/下载接口返回明确错误，
// 退款主流程不受影响。
type RefundHandler struct {
	orderService      service.OrderService
	attachmentService service.RefundAttachmentService
}

// NewRefundHandler 创建退款处理入口
func NewRefundHandler(orderService service.OrderService) *RefundHandler {
	return &RefundHandler{orderService: orderService}
}

// SetAttachmentService 注入凭据附件服务（装配层调用）。
func (h *RefundHandler) SetAttachmentService(svc service.RefundAttachmentService) {
	h.attachmentService = svc
}

// List godoc
// @Summary 查询退款单分页列表
// @Tags 订单管理-退款
// @Security BearerAuth
// @Param keyword query string false "退款单号/原因"
// @Param order_id query int false "关联订单 ID"
// @Param status query string false "退款状态"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/refunds [get]
func (h *RefundHandler) List(c *gin.Context) {
	var query dto.RefundListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.orderService.ListRefunds(c.Request.Context(), query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Get godoc
// @Summary 查询退款单详情
// @Tags 订单管理-退款
// @Security BearerAuth
// @Param id path int true "退款单 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/refunds/{id} [get]
func (h *RefundHandler) Get(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.orderService.FindRefund(c.Request.Context(), id)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Approve godoc
// @Summary 审核通过退款
// @Tags 订单管理-退款
// @Security BearerAuth
// @Param id path int true "退款单 ID"
// @Param request body dto.RefundApproveRequest false "审核备注"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/refunds/{id}/approve [post]
func (h *RefundHandler) Approve(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.RefundApproveRequest
	_ = c.ShouldBindJSON(&req)
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.orderService.ApproveRefund(c.Request.Context(), id, operatorID, operatorName, req)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Reject godoc
// @Summary 驳回退款
// @Tags 订单管理-退款
// @Security BearerAuth
// @Param id path int true "退款单 ID"
// @Param request body dto.RefundApproveRequest false "驳回备注"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/refunds/{id}/reject [post]
func (h *RefundHandler) Reject(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.RefundApproveRequest
	_ = c.ShouldBindJSON(&req)
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.orderService.RejectRefund(c.Request.Context(), id, operatorID, operatorName, req)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// BatchApprove godoc
// @Summary 批量审核通过退款单
// @Description 逐条复用单条审批（含渠道退款/余额回补/实例处置钩子），返回成功/跳过/失败明细
// @Tags 订单管理-退款
// @Security BearerAuth
// @Param request body dto.BatchRefundAuditRequest true "退款单 ID 集合与审核备注"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/refunds/batch-approve [post]
func (h *RefundHandler) BatchApprove(c *gin.Context) {
	var req dto.BatchRefundAuditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.orderService.BatchApproveRefunds(c.Request.Context(), req.IDs, operatorID, operatorName, req.Remark)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// BatchReject godoc
// @Summary 批量驳回退款单
// @Tags 订单管理-退款
// @Security BearerAuth
// @Param request body dto.BatchRefundAuditRequest true "退款单 ID 集合与审核备注"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/refunds/batch-reject [post]
func (h *RefundHandler) BatchReject(c *gin.Context) {
	var req dto.BatchRefundAuditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.orderService.BatchRejectRefunds(c.Request.Context(), req.IDs, operatorID, operatorName, req.Remark)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Review godoc
// @Summary 事后复核退款单（免审核自动退款后的最后一道人工确认）
// @Tags 订单管理-退款
// @Security BearerAuth
// @Param id path int true "退款单 ID"
// @Param request body dto.RefundReviewRequest false "复核结论"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/refunds/{id}/review [post]
func (h *RefundHandler) Review(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.RefundReviewRequest
	_ = c.ShouldBindJSON(&req)
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.orderService.ReviewRefund(c.Request.Context(), id, operatorID, operatorName, req.Remark)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// UploadAttachment godoc
// @Summary 上传退款审核凭据附件
// @Description 单文件上限 10MB，白名单类型（图片/PDF/表格/压缩包）；返回附件 ID 供审核提交时引用
// @Tags 订单管理-退款
// @Security BearerAuth
// @Param id path int true "退款单 ID"
// @Param file formData file true "凭据文件"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/refunds/{id}/attachments [post]
func (h *RefundHandler) UploadAttachment(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if h.attachmentService == nil {
		response.Error(c, apperrors.New(50001, service.ErrRefundAttachmentStoreUnavailable.Error()))
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Error(c, apperrors.New(20001, "缺少上传文件"))
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	defer func() { _ = f.Close() }()
	operatorID, _ := operatorFromContext(c)
	info, err := h.attachmentService.Upload(c.Request.Context(), id, operatorID,
		fileHeader.Filename, f, fileHeader.Size)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, info)
}

// DownloadAttachment godoc
// @Summary 下载退款审核凭据附件（鉴权）
// @Tags 订单管理-退款
// @Security BearerAuth
// @Param id path int true "附件 ID"
// @Success 200 {file} binary
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/refunds/attachments/{id}/download [get]
func (h *RefundHandler) DownloadAttachment(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if h.attachmentService == nil {
		response.Error(c, apperrors.New(50001, service.ErrRefundAttachmentStoreUnavailable.Error()))
		return
	}
	item, rc, err := h.attachmentService.Open(c.Request.Context(), id)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	defer func() { _ = rc.Close() }()
	writeAttachment(c, item.FileName, item.FileType, item.FileSize, rc)
}
