package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/sales/dto"
	"hostsent/backend/internal/modules/admin/sales/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
)

// CommissionHandler 提成台账与提现管理端处理器。
type CommissionHandler struct {
	commission service.CommissionService
	withdraw   service.WithdrawalService
	// customer 仅用于主管代查时的本部门销售校验（候选列表来源）。
	customer service.CustomerService
}

// NewCommissionHandler 创建提成处理器。
func NewCommissionHandler(commission service.CommissionService, withdraw service.WithdrawalService, customer service.CustomerService) *CommissionHandler {
	return &CommissionHandler{commission: commission, withdraw: withdraw, customer: customer}
}

// List 提成台账。
// @Summary 提成台账
// @Tags 销售-提成
// @Security BearerAuth
// @Param admin_id query int false "指定销售（主管/超管）"
// @Param type query string false "台账类型"
// @Param order_no query string false "订单号模糊"
// @Param start_at query string false "开始日期 YYYY-MM-DD"
// @Param end_at query string false "结束日期 YYYY-MM-DD"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/sales/commissions [get]
func (h *CommissionHandler) List(c *gin.Context) {
	var query dto.CommissionQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.commission.List(c.Request.Context(), resolveScope(c), query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Summary 提成账户概览。
// @Summary 提成账户概览
// @Tags 销售-提成
// @Security BearerAuth
// @Param admin_id query int false "指定销售（主管/超管），默认本人"
// @Router /api/v1/admin/sales/commissions/summary [get]
func (h *CommissionHandler) Summary(c *gin.Context) {
	scope := resolveScope(c)
	adminID := uint64(0)
	if v, err := strconvParseUint(c.Query("admin_id")); err == nil {
		adminID = v
	}
	// 范围收敛：超管可代查任意人；主管限本部门；销售只能查自己。
	switch {
	case scope.All:
		if adminID == 0 {
			adminID, _ = operatorFromContext(c)
		}
	case scope.AdminID > 0:
		adminID = scope.AdminID
	default:
		if adminID == 0 {
			adminID, _ = operatorFromContext(c)
		} else if err := h.checkDeptForAdmin(c, adminID); err != nil {
			response.Error(c, writeError(err))
			return
		}
	}
	resp, err := h.commission.Summary(c.Request.Context(), adminID)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// checkDeptForAdmin 主管代查时校验目标销售属于本部门。
func (h *CommissionHandler) checkDeptForAdmin(c *gin.Context, adminID uint64) error {
	grant, ok := middleware.GetAdminGrant(c)
	if !ok || grant.IsSuper() {
		return nil
	}
	if grant.DepartmentID == 0 {
		return service.ErrOutOfScope
	}
	resp, err := h.customer.Candidates(c.Request.Context(), grant.DepartmentID)
	if err != nil {
		return err
	}
	for _, item := range resp.Items {
		if item.AdminID == adminID {
			return nil
		}
	}
	return service.ErrOutOfScope
}

// ApplyWithdrawal 销售自助申请提现。
// @Summary 申请提成提现
// @Tags 销售-提成
// @Security BearerAuth
// @Router /api/v1/admin/sales/withdrawals [post]
func (h *CommissionHandler) ApplyWithdrawal(c *gin.Context) {
	var req dto.WithdrawApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	adminID, _ := operatorFromContext(c)
	resp, err := h.withdraw.Apply(c.Request.Context(), adminID, req)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// Withdrawals 提现单列表。
// @Summary 提成提现单列表
// @Tags 销售-提成
// @Security BearerAuth
// @Router /api/v1/admin/sales/withdrawals [get]
func (h *CommissionHandler) Withdrawals(c *gin.Context) {
	var query dto.WithdrawalQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.withdraw.List(c.Request.Context(), resolveScope(c), query)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// ApproveWithdrawal 审核通过。
// @Summary 提成提现审核通过
// @Tags 销售-提成
// @Security BearerAuth
// @Param id path int true "提现单 ID"
// @Router /api/v1/admin/sales/withdrawals/{id}/approve [post]
func (h *CommissionHandler) ApproveWithdrawal(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.WithdrawAuditRequest
	_ = c.ShouldBindJSON(&req)
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.withdraw.Approve(c.Request.Context(), resolveScope(c), id, operatorID, operatorName, req.Remark)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// RejectWithdrawal 审核驳回。
// @Summary 提成提现审核驳回
// @Tags 销售-提成
// @Security BearerAuth
// @Param id path int true "提现单 ID"
// @Router /api/v1/admin/sales/withdrawals/{id}/reject [post]
func (h *CommissionHandler) RejectWithdrawal(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.WithdrawAuditRequest
	_ = c.ShouldBindJSON(&req)
	operatorID, operatorName := operatorFromContext(c)
	resp, err := h.withdraw.Reject(c.Request.Context(), resolveScope(c), id, operatorID, operatorName, req.Remark)
	if err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, resp)
}

// PayWithdrawal 登记打款（结算冻结资金）。
// @Summary 登记提成打款
// @Tags 销售-提成
// @Security BearerAuth
// @Param id path int true "提现单 ID"
// @Router /api/v1/admin/sales/withdrawals/{id}/pay [post]
func (h *CommissionHandler) PayWithdrawal(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.WithdrawPayRequest
	_ = c.ShouldBindJSON(&req)
	if req.ChannelTx == "" && req.ReceiptURL == "" {
		response.Error(c, writeError(service.ErrPayoutReceiptRequired))
		return
	}
	operatorID, _ := operatorFromContext(c)
	if err := h.withdraw.MarkPaid(c.Request.Context(), id, req.ChannelTx, req.ReceiptURL, req.Remark, operatorID); err != nil {
		response.Error(c, writeError(err))
		return
	}
	response.Success(c, gin.H{"ok": true})
}
