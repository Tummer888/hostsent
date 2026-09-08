package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/finance/account/dto"
	"hostsent/backend/internal/modules/admin/finance/account/service"
	transdto "hostsent/backend/internal/modules/admin/finance/transaction/dto"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// WalletHandler 钱包/流水处理入口。
type WalletHandler struct {
	walletService service.WalletService
}

// NewWalletHandler 创建钱包处理入口。
func NewWalletHandler(walletService service.WalletService) *WalletHandler {
	return &WalletHandler{walletService: walletService}
}

// Balance godoc
// @Summary 查询用户钱包
// @Tags 财务管理-钱包
// @Security BearerAuth
// @Param user_id path int true "用户 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/finance/wallets/{user_id} [get]
func (h *WalletHandler) Balance(c *gin.Context) {
	userID, ok := pathID(c, "user_id")
	if !ok {
		return
	}
	resp, err := h.walletService.Balance(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// ListTransactions godoc
// @Summary 资金流水列表（多条件）
// @Tags 财务管理-流水
// @Security BearerAuth
// @Param user_id query int false "用户 ID"
// @Param type query string false "流水类型"
// @Param direction query int false "方向：1收入 -1支出"
// @Param start_time query string false "开始时间"
// @Param end_time query string false "结束时间"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/finance/transactions [get]
func (h *WalletHandler) ListTransactions(c *gin.Context) {
	var query transdto.TransactionListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.walletService.ListTransactions(c.Request.Context(), query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Adjust godoc
// @Summary 人工调账（赠送/扣减）
// @Tags 财务管理-流水
// @Security BearerAuth
// @Param request body dto.AdjustRequest true "调账参数"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/finance/transactions/adjust [post]
func (h *WalletHandler) Adjust(c *gin.Context) {
	var req dto.AdjustRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, _ := operatorFromContext(c)
	resp, err := h.walletService.Adjust(c.Request.Context(), req, operatorID)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}
