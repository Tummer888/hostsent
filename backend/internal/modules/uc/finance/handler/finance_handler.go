// Package handler 提供用户中心财务模块的 HTTP 接口。
// 用户自助只能访问自身账户数据，user_id 一律取自鉴权上下文，不能由前端指定。
//
// 本处理器不直接依赖 admin 的 finance service 层：账务/充值/账单能力与错误码映射
// 由装配层以最小接口与闭包注入（见 NewFinanceHandler），保持 uc 与 admin 服务层解耦。
package handler

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"

	accountdto "hostsent/backend/internal/modules/admin/finance/account/dto"
	billdto "hostsent/backend/internal/modules/admin/finance/bill/dto"
	finrechdto "hostsent/backend/internal/modules/admin/finance/recharge/dto"
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
		List(ctx context.Context, query finrechdto.RechargeListQuery) (*finrechdto.RechargeListResponse, error)
	}
	billPort interface {
		List(ctx context.Context, query billdto.BillListQuery) (*billdto.BillListResponse, error)
		// doc36 §3.3：用户自助开票与查询本人发票申请。
		RequestInvoice(ctx context.Context, userID uint64, req billdto.InvoiceApplyRequest) (*billdto.InvoiceInfo, error)
		MyInvoices(ctx context.Context, userID uint64, query billdto.InvoiceListQuery) (*billdto.InvoiceListResponse, error)
		// doc36 §3.3 预埋：下载取件与邮件下发。
		InvoiceFile(ctx context.Context, userID, requestID uint64) (*billdto.InvoiceFileInfo, error)
		EmailInvoice(ctx context.Context, userID, requestID uint64) error
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

// currentUserID 返回数据归属账号 ID（P4-04）：余额/账单/流水一律取主账号。
func currentUserID(c *gin.Context) (uint64, bool) {
	userID := middleware.EffectiveUserID(c)
	if userID == 0 {
		return 0, false
	}
	return userID, true
}

// unauthorized 用户未登录。
func unauthorized(c *gin.Context) {
	response.Error(c, apperrors.New(10001, "unauthorized"))
}

// pathID 解析路径参数指定的数字 ID，解析失败时直接写入错误响应。
func pathID(c *gin.Context, param string) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param(param), 10, 64)
	if err != nil {
		response.Error(c, apperrors.New(20001, "invalid "+param))
		return 0, false
	}
	return id, true
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

// Recharges 查询我的充值单（余额页「我的充值单」列表）。
// 此前前端以资金流水冒充满值单（`as unknown as RechargeInfo[]`），充值单号/状态必然空白
// （doc34 F-07）；本接口直接返回本人充值单，含支付中心渠道与支付单号。
// @Summary 我的充值单
// @Tags 用户中心-财务
// @Security BearerAuth
// @Param status query string false "状态"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/finance/recharges [get]
func (h *FinanceHandler) Recharges(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	var query finrechdto.RechargeListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	query.UserID = userID // 仅能查自己的充值单
	resp, err := h.recharge.List(c.Request.Context(), query)
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

// ApplyInvoice 申请开票（doc36 §3.3）。
// 只能对本人已结清且未开票的账单发起；同一账单仅允许一笔待处理申请。
// @Summary 申请开票
// @Tags 用户中心-财务
// @Security BearerAuth
// @Param request body dto.InvoiceApplyRequest true "开票信息"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/finance/invoices [post]
func (h *FinanceHandler) ApplyInvoice(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	var req billdto.InvoiceApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.bill.RequestInvoice(c.Request.Context(), userID, req)
	if err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.Success(c, resp)
}

// MyInvoices 我的发票申请列表（doc36 §3.3）。
// @Summary 我的发票
// @Tags 用户中心-财务
// @Security BearerAuth
// @Param status query string false "申请状态"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/finance/invoices [get]
func (h *FinanceHandler) MyInvoices(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	var query billdto.InvoiceListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	query.UserID = userID // 仅能查自己的发票申请
	resp, err := h.bill.MyInvoices(c.Request.Context(), userID, query)
	if err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.Success(c, resp)
}

// InvoiceDownload 取本人发票文件地址（doc36 §3.3 预埋）。
// 已开票但开票时未回填文件地址返回「尚未就绪」；文件地址就绪后由前端直接打开下载。
// @Summary 下载发票
// @Tags 用户中心-财务
// @Security BearerAuth
// @Param id path int true "申请单 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/finance/invoices/{id}/download [get]
func (h *FinanceHandler) InvoiceDownload(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.bill.InvoiceFile(c.Request.Context(), userID, id)
	if err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.Success(c, resp)
}

// InvoiceEmail 发票邮件下发（doc36 §3.3 预埋，本轮返回未接入）。
// @Summary 邮件发送发票
// @Tags 用户中心-财务
// @Security BearerAuth
// @Param id path int true "申请单 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/finance/invoices/{id}/email [post]
func (h *FinanceHandler) InvoiceEmail(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		unauthorized(c)
		return
	}
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := h.bill.EmailInvoice(c.Request.Context(), userID, id); err != nil {
		response.Error(c, h.mapErr(err))
		return
	}
	response.SuccessMessage(c, "发票已发送至接收邮箱")
}
