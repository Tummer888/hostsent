// Package handler 提供用户中心推广邀请返现的 HTTP 接口。
// 返现归属人一律取自鉴权上下文（子账号取主账号），不能由前端指定。
package handler

import (
	"context"

	"github.com/gin-gonic/gin"

	referraldto "hostsent/backend/internal/modules/admin/referral/dto"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// 装配层注入的最小能力接口（仅声明本模块用到的方法，避免 uc 依赖 admin 服务层）。
type (
	invitePort interface {
		Profile(ctx context.Context, userID uint64) (*referraldto.Profile, error)
		Invitees(ctx context.Context, userID uint64, page, pageSize int) (*referraldto.InviteeListResponse, error)
		Cashbacks(ctx context.Context, userID uint64, typeFilter string, page, pageSize int) (*referraldto.CashbackListResponse, error)
	}
	withdrawPort interface {
		Apply(ctx context.Context, userID uint64, req referraldto.WithdrawRequest) (*referraldto.WithdrawalInfo, error)
		TransferToWallet(ctx context.Context, userID uint64, amount float64) error
		ListByUser(ctx context.Context, userID uint64, status string, page, pageSize int) (*referraldto.WithdrawalListResponse, error)
	}
)

// ReferralHandler 用户中心推广返现处理器。
type ReferralHandler struct {
	invite   invitePort
	withdraw withdrawPort
	mapErr   func(error) *apperrors.AppError // 返现域错误映射（装配层注入）
}

// NewReferralHandler 创建用户中心推广返现处理器。
func NewReferralHandler(invite invitePort, withdraw withdrawPort, mapErr func(error) *apperrors.AppError) *ReferralHandler {
	return &ReferralHandler{invite: invite, withdraw: withdraw, mapErr: mapErr}
}

// currentUserID 返回数据归属账号 ID（子账号归主账号）。
func currentUserID(c *gin.Context) (uint64, bool) {
	userID := middleware.EffectiveUserID(c)
	if userID == 0 {
		return 0, false
	}
	return userID, true
}

// Profile 我的推广概览（邀请码/返现余额/邀请人数）。
// @Summary 我的推广概览
// @Tags 用户中心-推广返现
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/referral/profile [get]
func (h *ReferralHandler) Profile(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Error(c, apperrors.New(10001, "unauthorized"))
		return
	}
	resp, err := h.invite.Profile(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.Success(c, resp)
}

// Invitees 我的邀请列表。
// @Summary 我的邀请列表
// @Tags 用户中心-推广返现
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/referral/invitees [get]
func (h *ReferralHandler) Invitees(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Error(c, apperrors.New(10001, "unauthorized"))
		return
	}
	var query referraldto.Query
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.invite.Invitees(c.Request.Context(), userID, query.Page, query.PageSize)
	if err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.Success(c, resp)
}

// Cashbacks 我的返现明细。
// @Summary 我的返现明细
// @Tags 用户中心-推广返现
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/referral/cashbacks [get]
func (h *ReferralHandler) Cashbacks(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Error(c, apperrors.New(10001, "unauthorized"))
		return
	}
	var query struct {
		referraldto.Query
		Type string `form:"type"`
	}
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.invite.Cashbacks(c.Request.Context(), userID, query.Type, query.Page, query.PageSize)
	if err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.Success(c, resp)
}

// Withdrawals 我的提现记录。
// @Summary 我的返现提现记录
// @Tags 用户中心-推广返现
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/referral/withdrawals [get]
func (h *ReferralHandler) Withdrawals(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Error(c, apperrors.New(10001, "unauthorized"))
		return
	}
	var query struct {
		referraldto.Query
		Status string `form:"status"`
	}
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.withdraw.ListByUser(c.Request.Context(), userID, query.Status, query.Page, query.PageSize)
	if err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.Success(c, resp)
}

// ApplyWithdrawal 申请提现（冻结返现余额，待后台审核）。
// @Summary 申请返现提现
// @Tags 用户中心-推广返现
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/referral/withdrawals [post]
func (h *ReferralHandler) ApplyWithdrawal(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Error(c, apperrors.New(10001, "unauthorized"))
		return
	}
	var req referraldto.WithdrawRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.withdraw.Apply(c.Request.Context(), userID, req)
	if err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.Success(c, resp)
}

// Transfer 返现转入现金余额（即时到账）。
// @Summary 返现转入余额
// @Tags 用户中心-推广返现
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/referral/transfer [post]
func (h *ReferralHandler) Transfer(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Error(c, apperrors.New(10001, "unauthorized"))
		return
	}
	var req referraldto.TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	if err := h.withdraw.TransferToWallet(c.Request.Context(), userID, req.Amount); err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.SuccessMessage(c, "已转入现金余额")
}
