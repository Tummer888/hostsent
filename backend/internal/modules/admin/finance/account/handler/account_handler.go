package handler

import (
	"encoding/csv"
	"strconv"
	"time"

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

// ExportTransactions godoc
// @Summary 导出资金流水明细（CSV，按当前筛选条件）
// @Description 同一组筛选条件、按时间正序，最多 20000 条；冻结/解冻内部划转同样导出，便于账实核对。
// @Tags 财务管理-流水
// @Security BearerAuth
// @Param keyword query string false "关键词：流水号/订单号/关联单号/用户名"
// @Param user_id query int false "用户 ID"
// @Param type query string false "流水类型"
// @Param direction query int false "方向：1收入 -1支出"
// @Param start_time query string false "开始时间"
// @Param end_time query string false "结束时间"
// @Success 200 {file} binary
// @Router /api/v1/admin/finance/transactions/export [get]
func (h *WalletHandler) ExportTransactions(c *gin.Context) {
	var query transdto.TransactionListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	items, err := h.walletService.ExportTransactions(c.Request.Context(), query, exportRowLimit)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}

	filename := "finance-transactions-" + time.Now().Format("20060102150405") + ".csv"
	// UTF-8 BOM：Excel 直接打开中文列名不乱码。
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Writer.WriteString("\xEF\xBB\xBF")
	writer := csv.NewWriter(c.Writer)
	_ = writer.Write([]string{"流水号", "用户ID", "用户名", "类型", "业务标识", "方向", "变动金额", "变动前余额", "变动后余额", "订单号", "关联单号", "备注", "操作人ID", "发生时间"})
	for _, item := range items {
		_ = writer.Write([]string{
			item.TxNo,
			strconv.FormatUint(item.UserID, 10),
			item.Username,
			item.Type,
			item.BizType,
			directionLabel(item.Direction),
			strconv.FormatFloat(item.Amount, 'f', 2, 64),
			strconv.FormatFloat(item.BalanceBefore, 'f', 2, 64),
			strconv.FormatFloat(item.BalanceAfter, 'f', 2, 64),
			item.OrderNo,
			item.RefNo,
			item.Remark,
			strconv.FormatUint(item.OperatorID, 10),
			item.CreatedAt,
		})
	}
	writer.Flush()
}

// exportRowLimit 单次导出行数上限：财年明细量级够用，避免一次导出把内存与响应体撑爆。
const exportRowLimit = 20000

func directionLabel(direction int) string {
	if direction > 0 {
		return "收入"
	}
	if direction < 0 {
		return "支出"
	}
	return ""
}

// Adjust godoc
// @Summary 人工调账（赠送/扣减）
// @Description 财务配置 finance.adjust_enabled=false 时直接拒绝。
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
