// Package handler 提供推广邀请返现的 HTTP 接口（管理端：台账 / 邀请关系 / 提现审核）。
package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/referral/dto"
	"hostsent/backend/internal/modules/admin/referral/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// ReferralHandler 推广返现管理端处理器。
type ReferralHandler struct {
	invite   service.InviteService
	withdraw service.WithdrawalService
}

// NewReferralHandler 创建推广返现处理器。
func NewReferralHandler(invite service.InviteService, withdraw service.WithdrawalService) *ReferralHandler {
	return &ReferralHandler{invite: invite, withdraw: withdraw}
}

// Cashbacks 返现台账列表。
// @Summary 返现台账
// @Tags 推广返现
// @Security BearerAuth
// @Param user_id query int false "返现归属人（邀请人）"
// @Param type query string false "台账类型"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/referral/cashbacks [get]
func (h *ReferralHandler) Cashbacks(c *gin.Context) {
	var query dto.LedgerQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.invite.Cashbacks(c.Request.Context(), query.UserID, query.Type, query.Page, query.PageSize)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// inviteeQuery 邀请关系查询参数。
type inviteeQuery struct {
	InviterUserID uint64 `form:"inviter_user_id" binding:"required"`
	Page          int    `form:"page"`
	PageSize      int    `form:"page_size"`
}

// Invitees 按邀请人查询其邀请关系与贡献。
// @Summary 邀请关系
// @Tags 推广返现
// @Security BearerAuth
// @Param inviter_user_id query int true "邀请人用户 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/referral/invitees [get]
func (h *ReferralHandler) Invitees(c *gin.Context) {
	var query inviteeQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.invite.Invitees(c.Request.Context(), query.InviterUserID, query.Page, query.PageSize)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Withdrawals 返现提现单列表。
// @Summary 返现提现单列表
// @Tags 推广返现
// @Security BearerAuth
// @Param user_id query int false "用户 ID"
// @Param status query string false "状态"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/referral/withdrawals [get]
func (h *ReferralHandler) Withdrawals(c *gin.Context) {
	var query dto.WithdrawalQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.withdraw.AdminList(c.Request.Context(), query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Approve 提现审核通过。
// @Summary 返现提现审核通过
// @Tags 推广返现
// @Security BearerAuth
// @Param id path int true "提现单 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/referral/withdrawals/{id}/approve [post]
func (h *ReferralHandler) Approve(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.WithdrawalAuditRequest
	_ = c.ShouldBindJSON(&req)
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.withdraw.Approve(c.Request.Context(), id, operatorID, operatorName, req.Remark)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Reject 提现驳回。
// @Summary 返现提现驳回
// @Tags 推广返现
// @Security BearerAuth
// @Param id path int true "提现单 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/referral/withdrawals/{id}/reject [post]
func (h *ReferralHandler) Reject(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.WithdrawalAuditRequest
	_ = c.ShouldBindJSON(&req)
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.withdraw.Reject(c.Request.Context(), id, operatorID, operatorName, req.Remark)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}
