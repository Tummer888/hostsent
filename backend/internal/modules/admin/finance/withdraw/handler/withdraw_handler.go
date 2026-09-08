package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/finance/withdraw/dto"
	"hostsent/backend/internal/modules/admin/finance/withdraw/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// WithdrawHandler 提现处理入口。
type WithdrawHandler struct {
	withdrawService service.WithdrawService
}

// NewWithdrawHandler 创建提现处理入口。
func NewWithdrawHandler(withdrawService service.WithdrawService) *WithdrawHandler {
	return &WithdrawHandler{withdrawService: withdrawService}
}

// List godoc
// @Summary 提现单列表
// @Tags 财务管理-提现
// @Security BearerAuth
// @Param user_id query int false "用户 ID"
// @Param status query string false "状态"
// @Param start_time query string false "开始时间"
// @Param end_time query string false "结束时间"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/finance/withdrawals [get]
func (h *WithdrawHandler) List(c *gin.Context) {
	var query dto.WithdrawListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.withdrawService.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Approve godoc
// @Summary 提现审核通过（出账）
// @Tags 财务管理-提现
// @Security BearerAuth
// @Param id path int true "提现单 ID"
// @Param request body dto.WithdrawAuditRequest true "审核说明"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/finance/withdrawals/{id}/approve [post]
func (h *WithdrawHandler) Approve(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.WithdrawAuditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.withdrawService.Approve(c.Request.Context(), id, req, operatorID, operatorName)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Reject godoc
// @Summary 提现驳回
// @Tags 财务管理-提现
// @Security BearerAuth
// @Param id path int true "提现单 ID"
// @Param request body dto.WithdrawAuditRequest true "审核说明"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/finance/withdrawals/{id}/reject [post]
func (h *WithdrawHandler) Reject(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.WithdrawAuditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.withdrawService.Reject(c.Request.Context(), id, req, operatorID, operatorName)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}
