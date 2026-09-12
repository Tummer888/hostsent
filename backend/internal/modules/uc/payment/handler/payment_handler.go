// Package handler 提供用户中心支付模块的 HTTP 接口。
// 用户自助只能操作自身数据：user_id 一律取自鉴权上下文，不能由前端指定。
package handler

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"

	withdrawdto "hostsent/backend/internal/modules/admin/finance/withdraw/dto"
	paydto "hostsent/backend/internal/modules/admin/payment/dto"
	"hostsent/backend/internal/modules/uc/payment/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// 装配层注入的最小能力接口（仅声明本模块真正用到的方法）。
type (
	// methodPort 收银台可用支付方式（按用户偏好排序）。
	methodPort interface {
		MethodOptions(ctx context.Context, userID uint64, scene string, amountFen int64) (*paydto.MethodOptionsResponse, error)
	}
	// preferencePort 支付方式偏好读写。
	preferencePort interface {
		Preferences(ctx context.Context, userID uint64) ([]paydto.PreferenceItem, error)
		SavePreferences(ctx context.Context, userID uint64, req paydto.PreferenceSaveRequest) ([]paydto.PreferenceItem, error)
	}
	// accountPort 用户收款账户管理。
	accountPort interface {
		ListAccounts(ctx context.Context, userID uint64) ([]paydto.PayoutAccountInfo, error)
		CreateAccount(ctx context.Context, userID uint64, req paydto.PayoutAccountCreateRequest) (*paydto.PayoutAccountInfo, error)
		SetDefaultAccount(ctx context.Context, userID, id uint64) error
	}
	// orderPort 支付单查询（收银台轮询支付结果）。
	orderPort interface {
		GetByNo(ctx context.Context, paymentNo string) (*paydto.OrderInfo, error)
	}
	// cashierPort 收银台编排（充值/账单支付/提现）。
	cashierPort interface {
		StartRecharge(ctx context.Context, userID uint64, amount float64, channelCode, scene string) (*paydto.OrderInfo, error)
		StartBillPay(ctx context.Context, userID uint64, billID uint64, billNo string, amount float64, channelCode, scene string) (*paydto.OrderInfo, error)
		StartWithdraw(ctx context.Context, userID uint64, req service.WithdrawInput) (*withdrawdto.WithdrawInfo, error)
		// ListWithdrawals 我的提现记录（含所用收款账户与打款方式）。
		ListWithdrawals(ctx context.Context, userID uint64, page, pageSize int) (*withdrawdto.WithdrawListResponse, error)
	}
)

// PaymentHandler 用户中心支付 HTTP 处理器。
type PaymentHandler struct {
	methods     methodPort
	preferences preferencePort
	accounts    accountPort
	orders      orderPort
	cashier     cashierPort
	mapErr      func(error) *apperrors.AppError
}

// NewPaymentHandler 创建用户中心支付处理器。
func NewPaymentHandler(methods methodPort, preferences preferencePort, accounts accountPort, orders orderPort, cashier cashierPort, mapErr func(error) *apperrors.AppError) *PaymentHandler {
	return &PaymentHandler{methods: methods, preferences: preferences, accounts: accounts, orders: orders, cashier: cashier, mapErr: mapErr}
}

func currentUserID(c *gin.Context) (uint64, bool) {
	userID := middleware.EffectiveUserID(c)
	if userID == 0 {
		return 0, false
	}
	return userID, true
}

func unauthorized(c *gin.Context) {
	response.Error(c, apperrors.New(10001, "unauthorized"))
}

// Methods godoc
// @Summary 收银台可用支付方式（按我的偏好排序）
// @Tags 用户中心-支付
// @Security BearerAuth
// @Param scene query string false "支付场景：native/h5/jsapi/scan"
// @Param amount query number false "金额（元），用于过滤渠道限额"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/payment/methods [get]
func (h *PaymentHandler) Methods(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	scene := c.DefaultQuery("scene", "native")
	amountFen := int64(0)
	if raw := c.Query("amount"); raw != "" {
		amount, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			response.Error(c, apperrors.New(20001, "金额格式不正确"))
			return
		}
		amountFen = int64(amount*100 + 0.5)
	}
	resp, err := h.methods.MethodOptions(c.Request.Context(), userID, scene, amountFen)
	if err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.Success(c, resp)
}

// Preferences godoc
// @Summary 我的支付方式偏好
// @Tags 用户中心-支付
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/payment/preferences [get]
func (h *PaymentHandler) Preferences(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	items, err := h.preferences.Preferences(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.Success(c, gin.H{"items": items})
}

// SavePreferences godoc
// @Summary 保存我的支付方式偏好（默认方式 + 优先级）
// @Tags 用户中心-支付
// @Security BearerAuth
// @Param request body dto.PreferenceSaveRequest true "偏好参数"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/payment/preferences [put]
func (h *PaymentHandler) SavePreferences(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	var req paydto.PreferenceSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	items, err := h.preferences.SavePreferences(c.Request.Context(), userID, req)
	if err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.Success(c, gin.H{"items": items})
}

// Accounts godoc
// @Summary 我的收款账户列表
// @Tags 用户中心-支付
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/payment/accounts [get]
func (h *PaymentHandler) Accounts(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	items, err := h.accounts.ListAccounts(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.Success(c, gin.H{"items": items})
}

// CreateAccount godoc
// @Summary 新增收款账户
// @Tags 用户中心-支付
// @Security BearerAuth
// @Param request body dto.PayoutAccountCreateRequest true "账户参数"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/payment/accounts [post]
func (h *PaymentHandler) CreateAccount(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	var req paydto.PayoutAccountCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	info, err := h.accounts.CreateAccount(c.Request.Context(), userID, req)
	if err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.Success(c, info)
}

// SetDefaultAccount godoc
// @Summary 设置默认收款账户
// @Tags 用户中心-支付
// @Security BearerAuth
// @Param id path int true "账户 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/payment/accounts/{id}/default [put]
func (h *PaymentHandler) SetDefaultAccount(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	id, err := parseUint64(c.Param("id"))
	if err != nil || id == 0 {
		response.Error(c, apperrors.New(20001, "账户 ID 不合法"))
		return
	}
	if err := h.accounts.SetDefaultAccount(c.Request.Context(), userID, id); err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.SuccessMessage(c, "success")
}

// RechargeRequest 在线充值请求。
type RechargeRequest struct {
	Amount      float64 `json:"amount" binding:"required"`
	ChannelCode string  `json:"channel_code"`
	Scene       string  `json:"scene"`
}

// Recharge godoc
// @Summary 在线充值（创建充值单并发起渠道支付）
// @Tags 用户中心-支付
// @Security BearerAuth
// @Param request body RechargeRequest true "充值参数"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/payment/recharge [post]
func (h *PaymentHandler) Recharge(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	var req RechargeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	info, err := h.cashier.StartRecharge(c.Request.Context(), userID, req.Amount, req.ChannelCode, req.Scene)
	if err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.Success(c, info)
}

// BillPayRequest 账单支付请求。
type BillPayRequest struct {
	BillID      uint64  `json:"bill_id" binding:"required"`
	BillNo      string  `json:"bill_no"`
	Amount      float64 `json:"amount"`
	ChannelCode string  `json:"channel_code"`
	Scene       string  `json:"scene"`
}

// PayBill godoc
// @Summary 支付账单（发起渠道支付）
// @Tags 用户中心-支付
// @Security BearerAuth
// @Param request body BillPayRequest true "账单支付参数"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/payment/bills/pay [post]
func (h *PaymentHandler) PayBill(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	var req BillPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	info, err := h.cashier.StartBillPay(c.Request.Context(), userID, req.BillID, req.BillNo, req.Amount, req.ChannelCode, req.Scene)
	if err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.Success(c, info)
}

// WithdrawRequest 提现申请请求。
type WithdrawRequest struct {
	Amount        float64 `json:"amount" binding:"required"`
	AccountID     uint64  `json:"account_id" binding:"required"`
	PayoutChannel string  `json:"payout_channel"`
	Remark        string  `json:"remark"`
}

// Withdraw godoc
// @Summary 申请提现（冻结余额，等待审核打款）
// @Tags 用户中心-支付
// @Security BearerAuth
// @Param request body WithdrawRequest true "提现参数"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/payment/withdrawals [post]
func (h *PaymentHandler) Withdraw(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	var req WithdrawRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	info, err := h.cashier.StartWithdraw(c.Request.Context(), userID, service.WithdrawInput{
		Amount:        req.Amount,
		AccountID:     req.AccountID,
		PayoutChannel: req.PayoutChannel,
		Remark:        req.Remark,
	})
	if err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.Success(c, info)
}

// Order godoc
// @Summary 查询支付单（收银台轮询支付结果）
// @Tags 用户中心-支付
// @Security BearerAuth
// @Param payment_no path string true "支付单号"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/payment/orders/{payment_no} [get]
func (h *PaymentHandler) Order(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	info, err := h.orders.GetByNo(c.Request.Context(), c.Param("payment_no"))
	if err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	// 只能查看自己的支付单（不存在的单号同样按无权限处理，避免枚举）。
	if info.UserID != userID {
		response.Error(c, apperrors.New(10003, "forbidden"))
		return
	}
	response.Success(c, info)
}

// Withdrawals godoc
// @Summary 我的提现记录（含收款账户与打款方式）
// @Tags 用户中心-支付
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页条数"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/payment/withdrawals [get]
func (h *PaymentHandler) Withdrawals(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	resp, err := h.cashier.ListWithdrawals(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.Success(c, resp)
}
