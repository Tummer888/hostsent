// Package handler 提供用户中心财务模块的 HTTP 接口。
// 用户自助只能访问自身账户数据，user_id 一律取自鉴权上下文，不能由前端指定。
package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	accountservice "hostsent/backend/internal/modules/admin/finance/account/service"
	billdto "hostsent/backend/internal/modules/admin/finance/bill/dto"
	billservice "hostsent/backend/internal/modules/admin/finance/bill/service"
	finrechdto "hostsent/backend/internal/modules/admin/finance/recharge/dto"
	finrechmodel "hostsent/backend/internal/modules/admin/finance/recharge/model"
	rechservice "hostsent/backend/internal/modules/admin/finance/recharge/service"
	transdto "hostsent/backend/internal/modules/admin/finance/transaction/dto"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// FinanceHandler 用户中心财务 HTTP 处理器。
type FinanceHandler struct {
	walletService   accountservice.WalletService
	rechargeService rechservice.RechargeService
	billService     billservice.BillService
}

// NewFinanceHandler 创建用户中心财务处理器。
func NewFinanceHandler(walletService accountservice.WalletService, rechargeService rechservice.RechargeService, billService billservice.BillService) *FinanceHandler {
	return &FinanceHandler{
		walletService:   walletService,
		rechargeService: rechargeService,
		billService:     billService,
	}
}

// rechargeCallbackRequest 充值渠道回调请求。
type rechargeCallbackRequest struct {
	RechargeNo string `json:"recharge_no" binding:"required"` // 充值单号
	ChannelTx  string `json:"channel_tx"`                     // 渠道交易号
	Status     string `json:"status"`                         // 渠道状态：success/failed
}

// currentUserID 从鉴权上下文提取当前登录用户 ID。
func currentUserID(c *gin.Context) (uint64, bool) {
	claims, ok := middleware.GetClaims(c)
	if !ok || claims.UserID == 0 {
		return 0, false
	}
	return claims.UserID, true
}

// mapErr 将财务域错误映射为统一错误码。
func mapErr(err error) *apperrors.AppError {
	switch {
	case errors.Is(err, accountservice.ErrWalletNotFound):
		return apperrors.New(20002, err.Error())
	case errors.Is(err, accountservice.ErrInsufficientBalance):
		return apperrors.New(30001, err.Error())
	case errors.Is(err, accountservice.ErrStatusConflict):
		return apperrors.New(20003, err.Error())
	default:
		return apperrors.New(50001, err.Error())
	}
}

// unauthorized 用户未登录。
func unauthorized(c *gin.Context) {
	response.Error(c, apperrors.New(10001, "unauthorized"))
}

// Balance 查询我的钱包余额。
// @Summary 查询我的余额
// @Tags 用户中心-财务
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/finance/balance [get]
func (h *FinanceHandler) Balance(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	resp, err := h.walletService.Balance(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, mapErr(err))
		return
	}
	response.Success(c, resp)
}

// Transactions 查询我的资金流水。
// @Summary 我的资金流水
// @Tags 用户中心-财务
// @Security BearerAuth
// @Param type query string false "流水类型"
// @Param direction query int false "方向：1收入 -1支出"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/finance/transactions [get]
func (h *FinanceHandler) Transactions(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	var query transdto.TransactionListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	query.UserID = userID // 仅能查自己的流水
	resp, err := h.walletService.ListTransactions(c.Request.Context(), query)
	if err != nil {
		response.Error(c, mapErr(err))
		return
	}
	response.Success(c, resp)
}

// CreateRecharge 发起充值（登记线下充值单）。
// @Summary 发起充值
// @Tags 用户中心-财务
// @Security BearerAuth
// @Param request body dto.RechargeCreateRequest true "充值参数"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/finance/recharge [post]
func (h *FinanceHandler) CreateRecharge(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	var req finrechdto.RechargeCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	req.UserID = userID // 只能给自己充值
	resp, err := h.rechargeService.Create(c.Request.Context(), req, userID)

	if err != nil {
		response.Error(c, mapErr(err))
		return
	}
	response.Success(c, resp)
}

// Bills 查询我的账单。
// @Summary 我的账单
// @Tags 用户中心-财务
// @Security BearerAuth
// @Param period query string false "账期"
// @Param status query string false "状态"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/finance/bills [get]
func (h *FinanceHandler) Bills(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	var query billdto.BillListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	query.UserID = userID // 仅能查自己的账单
	resp, err := h.billService.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, mapErr(err))
		return
	}
	response.Success(c, resp)
}

// RechargeCallback 充值渠道回调（幂等：同单号只到账一次）。
// @Summary 充值回调
// @Tags 用户中心-财务
// @Param request body rechargeCallbackRequest true "回调参数"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/finance/recharge/callback [post]
func (h *FinanceHandler) RechargeCallback(c *gin.Context) {
	var req rechargeCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	// 仅当渠道通知成功时才入账；失败/其他状态忽略
	if req.Status != "" && req.Status != finrechmodel.RechargeStatusSuccess {
		response.Success(c, gin.H{"handled": true, "status": req.Status})
		return
	}
	_, err := h.rechargeService.ApproveByNo(c.Request.Context(), req.RechargeNo, finrechdto.RechargeApproveRequest{
		ChannelTx: req.ChannelTx,
	}, 0)
	if err != nil {
		response.Error(c, mapErr(err))
		return
	}
	response.Success(c, gin.H{"handled": true, "status": finrechmodel.RechargeStatusSuccess})
}
