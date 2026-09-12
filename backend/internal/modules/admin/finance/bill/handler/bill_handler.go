package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/finance/bill/dto"
	"hostsent/backend/internal/modules/admin/finance/bill/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// BillHandler 账单处理入口。
type BillHandler struct {
	billService service.BillService
}

// NewBillHandler 创建账单处理入口。
func NewBillHandler(billService service.BillService) *BillHandler {
	return &BillHandler{billService: billService}
}

// List godoc
// @Summary 账单列表
// @Tags 财务管理-账单
// @Security BearerAuth
// @Param user_id query int false "用户 ID"
// @Param user_keyword query string false "用户账号（用户名/邮箱）"
// @Param bill_no query string false "账单号（精确）"
// @Param keyword query string false "账单号（模糊）"
// @Param period query string false "账期"
// @Param status query string false "状态"
// @Param bill_type query string false "账单分类"
// @Param invoice_status query string false "发票状态"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/finance/bills [get]
func (h *BillHandler) List(c *gin.Context) {
	var query dto.BillListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.billService.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Generate godoc
// @Summary 生成账单（按用户 + 账期归集消费/退款与分类）
// @Tags 财务管理-账单
// @Security BearerAuth
// @Param request body dto.GenerateRequest true "用户与账期"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/finance/bills/generate [post]
func (h *BillHandler) Generate(c *gin.Context) {
	var req dto.GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	bill, err := h.billService.GenerateForUser(c.Request.Context(), req.UserID, req.Period)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, bill)
}

// Invoices godoc
// @Summary 发票申请列表
// @Tags 财务管理-发票
// @Security BearerAuth
// @Param user_id query int false "用户 ID"
// @Param status query string false "申请状态"
// @Param bill_no query string false "账单号"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/finance/invoices [get]
func (h *BillHandler) Invoices(c *gin.Context) {
	var query dto.InvoiceListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.billService.Invoices(c.Request.Context(), query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// IssueInvoice godoc
// @Summary 开票（回填发票号）
// @Tags 财务管理-发票
// @Security BearerAuth
// @Param id path int true "申请单 ID"
// @Param request body dto.InvoiceIssueRequest true "发票号"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/finance/invoices/{id}/issue [post]
func (h *BillHandler) IssueInvoice(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.InvoiceIssueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, _ := operatorFromContext(c)
	resp, err := h.billService.IssueInvoice(c.Request.Context(), id, req, operatorID)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// RejectInvoice godoc
// @Summary 驳回开票申请
// @Tags 财务管理-发票
// @Security BearerAuth
// @Param id path int true "申请单 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/finance/invoices/{id}/reject [post]
func (h *BillHandler) RejectInvoice(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	operatorID, _ := operatorFromContext(c)
	resp, err := h.billService.RejectInvoice(c.Request.Context(), id, req.Reason, operatorID)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Close godoc
// @Summary 关账
// @Tags 财务管理-账单
// @Security BearerAuth
// @Param id path int true "账单 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/finance/bills/{id}/close [post]
func (h *BillHandler) Close(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := h.billService.Close(c.Request.Context(), id); err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.SuccessMessage(c, "success")
}
