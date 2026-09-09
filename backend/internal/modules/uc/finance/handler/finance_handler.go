// Package handler 提供用户中心财务模块的 HTTP 接口。
// 用户自助只能访问自身账户数据，user_id 一律取自鉴权上下文，不能由前端指定。
//
// 本处理器不直接依赖 admin 的 finance service 层：账务/充值/账单能力与错误码映射
// 由装配层以最小接口与闭包注入（见 NewFinanceHandler），保持 uc 与 admin 服务层解耦。
package handler

import (
	"context"

	"github.com/gin-gonic/gin"

	accountdto "hostsent/backend/internal/modules/admin/finance/account/dto"
	billdto "hostsent/backend/internal/modules/admin/finance/bill/dto"
	finrechdto "hostsent/backend/internal/modules/admin/finance/recharge/dto"
	finrechmodel "hostsent/backend/internal/modules/admin/finance/recharge/model"
	transdto "hostsent/backend/internal/modules/admin/finance/transaction/dto"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// 以下为装配层注入的最小财务能力接口（仅声明本模块真正用到的方法）。
type (
	walletPort interface {
		Balance(ctx context.Context, userID uint64) (*accountdto.WalletInfo, error)
		ListTransactions(ctx context.Context, query transdto.TransactionListQuery) (*transdto.TransactionListResponse, error)
	}
	rechargePort interface {
		Create(ctx context.Context, req finrechdto.RechargeCreateRequest, operatorID uint64) (*finrechdto.RechargeInfo, error)
		ApproveByNo(ctx context.Context, rechargeNo string, req finrechdto.RechargeApproveRequest, operatorID uint64) (*finrechdto.RechargeInfo, error)
	}
	billPort interface {
		List(ctx context.Context, query billdto.BillListQuery) (*billdto.BillListResponse, error)
	}
)

// FinanceHandler 用户中心财务 HTTP 处理器。
type FinanceHandler struct {
	wallet   walletPort
	recharge rechargePort
	bill     billPort
	mapErr   func(error) *apperrors.AppError // 财务域错误映射（由装配层提供，避免依赖 admin 错误变量）
}

// NewFinanceHandler 创建用户中心财务处理器。
func NewFinanceHandler(wallet walletPort, recharge rechargePort, bill billPort, mapErr func(error) *apperrors.AppError) *FinanceHandler {
	return &FinanceHandler{wallet: wallet, recharge: recharge, bill: bill, mapErr: mapErr}
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
	resp, err := h.wallet.Balance(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, h.mapErr(err))
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
	resp, err := h.wallet.ListTransactions(c.Request.Context(), query)
	if err != nil {
		response.Error(c, h.mapErr(err))
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
	resp, err := h.recharge.Create(c.Request.Context(), req, userID)

	if err != nil {
		response.Error(c, h.mapErr(err))
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
	resp, err := h.bill.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, h.mapErr(err))
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
	_, err := h.recharge.ApproveByNo(c.Request.Context(), req.RechargeNo, finrechdto.RechargeApproveRequest{
		ChannelTx: req.ChannelTx,
	}, 0)
	if err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.Success(c, gin.H{"handled": true, "status": finrechmodel.RechargeStatusSuccess})
}
