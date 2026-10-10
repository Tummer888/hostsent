package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/finance/cost/dto"
	"hostsent/backend/internal/modules/admin/finance/cost/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// CostHandler 成本管理处理入口。
type CostHandler struct {
	costService service.CostService
}

// NewCostHandler 创建成本管理处理入口。
func NewCostHandler(costService service.CostService) *CostHandler {
	return &CostHandler{costService: costService}
}

// Overview godoc
// @Summary 成本总览（月度成本、利润与利润率、近 12 月趋势）
// @Tags 财务管理-成本
// @Security BearerAuth
// @Param month query string false "月份 YYYY-MM（默认当月）"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/cost/overview [get]
func (h *CostHandler) Overview(c *gin.Context) {
	resp, err := h.costService.Overview(c.Request.Context(), c.Query("month"))
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// ListItems godoc
// @Summary 成本项列表（含按月计入合计）
// @Tags 财务管理-成本
// @Security BearerAuth
// @Param keyword query string false "名称/对象/备注模糊"
// @Param category query string false "分类"
// @Param status query string false "状态 active/disabled"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/cost/items [get]
func (h *CostHandler) ListItems(c *gin.Context) {
	var query dto.CostItemListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.costService.ListItems(c.Request.Context(), query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// CreateItem godoc
// @Summary 新增成本项
// @Tags 财务管理-成本
// @Security BearerAuth
// @Param request body dto.CostItemRequest true "成本项"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/cost/items [post]
func (h *CostHandler) CreateItem(c *gin.Context) {
	var req dto.CostItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, _ := operatorFromContext(c)
	resp, err := h.costService.CreateItem(c.Request.Context(), req, operatorID)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// UpdateItem godoc
// @Summary 编辑成本项
// @Tags 财务管理-成本
// @Security BearerAuth
// @Param id path int true "成本项 ID"
// @Param request body dto.CostItemRequest true "成本项"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/cost/items/{id} [put]
func (h *CostHandler) UpdateItem(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.CostItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	operatorID, _ := operatorFromContext(c)
	resp, err := h.costService.UpdateItem(c.Request.Context(), id, req, operatorID)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// DeleteItem godoc
// @Summary 删除成本项
// @Tags 财务管理-成本
// @Security BearerAuth
// @Param id path int true "成本项 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/cost/items/{id} [delete]
func (h *CostHandler) DeleteItem(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := h.costService.DeleteItem(c.Request.Context(), id); err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.SuccessMessage(c, "success")
}

// Ledger godoc
// @Summary 上游余额台账（各渠道期初/充值/消耗/期末）
// @Tags 财务管理-成本
// @Security BearerAuth
// @Param month query string false "月份 YYYY-MM（默认当月）"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/cost/balances [get]
func (h *CostHandler) Ledger(c *gin.Context) {
	resp, err := h.costService.Ledger(c.Request.Context(), c.Query("month"))
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// SyncLedger godoc
// @Summary 同步上游账本（消费/充值流水）
// @Tags 财务管理-成本
// @Security BearerAuth
// @Param request body dto.LedgerSyncRequest true "provider_id=0 同步全部支持的渠道；full=true 全量回填"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/cost/balances/sync-ledger [post]
func (h *CostHandler) SyncLedger(c *gin.Context) {
	var req dto.LedgerSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.costService.SyncLedger(c.Request.Context(), req.ProviderID, req.Full)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// ListLedgerEntries godoc
// @Summary 上游账本流水（消费/充值明细，分页）
// @Tags 财务管理-成本
// @Security BearerAuth
// @Param kind query string false "consume=消费（默认）/ topup=充值"
// @Param provider_id query int false "渠道 ID（0=全部）"
// @Param month query string false "月份 YYYY-MM（空=不限）"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/cost/balances/entries [get]
func (h *CostHandler) ListLedgerEntries(c *gin.Context) {
	var query dto.LedgerEntryQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.costService.ListLedgerEntries(c.Request.Context(), query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// BalanceAlerts godoc
// @Summary 上游余额水位告警（余额 < 未来 30 天到期金额）
// @Tags 财务管理-成本
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/admin/cost/balances/alerts [get]
func (h *CostHandler) BalanceAlerts(c *gin.Context) {
	resp, err := h.costService.BalanceAlerts(c.Request.Context())
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// FetchBalance godoc
// @Summary 抓取渠道余额并落当日快照
// @Tags 财务管理-成本
// @Security BearerAuth
// @Param request body dto.BalanceFetchRequest true "provider_id 必填"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/cost/balances/fetch [post]
func (h *CostHandler) FetchBalance(c *gin.Context) {
	var req dto.BalanceFetchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.costService.FetchBalance(c.Request.Context(), req.ProviderID)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
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

// operatorFromContext 从鉴权上下文提取操作者 ID 与名称。
func operatorFromContext(c *gin.Context) (uint64, string) {
	if claims, ok := middleware.GetAdminClaims(c); ok {
		return claims.AdminID, claims.Username
	}
	return 0, ""
}

// writeError 将成本子域业务错误映射为统一错误码。
func writeError(err error) *apperrors.AppError {
	switch {
	case errors.Is(err, service.ErrItemNotFound), errors.Is(err, service.ErrProviderNotFound):
		return apperrors.New(20002, err.Error())
	case errors.Is(err, service.ErrInvalidItem):
		return apperrors.New(20001, err.Error())
	case errors.Is(err, service.ErrBalanceUnsupported), errors.Is(err, service.ErrLedgerUnsupported):
		// 30008：能力不支持（渠道适配器未实现余额/账本读取），与参数错误区分开，
		// 前端据此把对应按钮置灰而不是报「参数错误」。
		return apperrors.New(30008, err.Error())
	default:
		return apperrors.New(50001, err.Error())
	}
}
