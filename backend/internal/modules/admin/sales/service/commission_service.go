package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/sales/dto"
	"hostsent/backend/internal/modules/admin/sales/model"
	"hostsent/backend/internal/modules/admin/sales/repository"
	"hostsent/backend/internal/pkg/money"
)

// CommissionService 提成计提、冲减、解冻与查询。
//
// 计提与冲减由装配层通过订单钩子调用（admin order service 的开通成功/退款通过钩子），
// 因此这些方法必须「不返回致命错误」地影响主流程：它们返回 error 由装配层记日志吞掉。
type CommissionService interface {
	// AccrueForOrder 订单成交后按归属快照计提提成（进入解冻期）。
	AccrueForOrder(ctx context.Context, in AccrualInput) error
	// ClawbackForRefund 退款通过后按退款占比冲减该订单已计提提成。
	ClawbackForRefund(ctx context.Context, in ClawbackInput) error
	// ReleaseDue 把已到解冻期的计提转入可用余额（调度器与手动触发共用，幂等）。
	ReleaseDue(ctx context.Context, limit int) (int, error)
	// Summary 提成账户概览（销售本人或主管代查）。
	Summary(ctx context.Context, adminID uint64) (*dto.CommissionSummary, error)
	// List 提成台账（按数据范围过滤）。
	List(ctx context.Context, scope dto.Scope, q dto.CommissionQuery) (*dto.CommissionListResponse, error)
}

// AccrualInput 订单成交后的提成计提入参。
type AccrualInput struct {
	OrderID     uint64
	OrderNo     string
	BuyerUserID uint64
	BaseAmount  float64 // 计提基数 = 订单实付金额
	IsRenewal   bool
}

// ClawbackInput 退款通过后的提成冲减入参。
type ClawbackInput struct {
	OrderID      uint64
	OrderNo      string
	BuyerUserID  uint64
	PaidAmount   float64 // 订单实付（冲减比例分母）
	RefundAmount float64
	RefundNo     string
}

type commissionService struct {
	db           *gorm.DB
	commRepo     repository.CommissionRepository
	relRepo      repository.RelationRepository
	withdrawRepo repository.WithdrawalRepository // 可选：概览里的待审核提现额
	logger       *zap.Logger
}

// NewCommissionService 创建提成服务。withdrawRepo 仅用于概览统计，可为 nil。
func NewCommissionService(db *gorm.DB, commRepo repository.CommissionRepository, relRepo repository.RelationRepository, withdrawRepo repository.WithdrawalRepository, logger *zap.Logger) *commissionService {
	return &commissionService{db: db, commRepo: commRepo, relRepo: relRepo, withdrawRepo: withdrawRepo, logger: logger}
}

// AccrueForOrder 计提提成。
// 幂等键 (归属销售, accrue_*, 订单号)：同一订单重复触发（重试开通/重复回调）只记一次账。
func (s *commissionService) AccrueForOrder(ctx context.Context, in AccrualInput) error {
	if in.OrderNo == "" || in.BaseAmount <= 0 || in.OrderID == 0 {
		return nil
	}
	rates, err := s.commRepo.Rates(ctx)
	if err != nil {
		return err
	}
	if !rates.Enabled {
		return nil
	}
	// 归属以订单快照为准：下单时未归属（0）则本单不参与提成，事后补归属也不追记。
	adminID, err := s.commRepo.OrderSalesOwner(ctx, in.OrderID)
	if err != nil {
		return err
	}
	if adminID == 0 {
		return nil
	}

	bizType := model.TxTypeAccrueSubsequent
	rate := rates.Subsequent
	switch {
	case in.IsRenewal:
		bizType = model.TxTypeAccrueRenewal
		rate = rates.Renewal
	default:
		existed, err := s.commRepo.HasSettledOrder(ctx, in.BuyerUserID, in.OrderID)
		if err != nil {
			return err
		}
		if !existed {
			bizType = model.TxTypeAccrueFirst
			rate = rates.FirstOrder
		}
	}
	amount := money.Round2(in.BaseAmount * rate)
	if amount <= 0 {
		return nil
	}

	var releaseAt *time.Time
	if rates.ReleaseDays > 0 {
		t := time.Now().AddDate(0, 0, rates.ReleaseDays)
		releaseAt = &t
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		acc, err := s.commRepo.EnsureAccount(tx, adminID)
		if err != nil {
			return err
		}
		if _, err := s.commRepo.FindTxByBiz(tx, adminID, bizType, in.OrderNo); err == nil {
			return nil // 已计提
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if releaseAt == nil {
			// 解冻期为 0：直接进可用余额，并记一条解冻流水保持台账完整可读。
			before := acc.Balance
			after := money.Round2(before + amount)
			acc.Balance = after
			acc.TotalIncome = money.Round2(acc.TotalIncome + amount)
			acc.Version++
			if err := s.commRepo.UpdateAccount(tx, acc); err != nil {
				return err
			}
			if err := s.commRepo.CreateTx(tx, &model.CommissionTransaction{
				TxNo: genTxNo(), AdminID: adminID, Type: bizType, Direction: model.DirectionIncome,
				Amount: amount, BalanceBefore: before, BalanceAfter: after,
				BizType: bizType, RefNo: in.OrderNo, OrderID: in.OrderID, OrderNo: in.OrderNo,
				CustomerID: in.BuyerUserID, Remark: commissionRemark(bizType, in.OrderNo),
			}); err != nil {
				return err
			}
			return s.commRepo.CreateTx(tx, &model.CommissionTransaction{
				TxNo: genTxNo(), AdminID: adminID, Type: model.TxTypeRelease, Direction: model.DirectionIncome,
				Amount: amount, BalanceBefore: before, BalanceAfter: after,
				BizType: model.TxTypeRelease, RefNo: in.OrderNo,
				OrderID: in.OrderID, OrderNo: in.OrderNo, CustomerID: in.BuyerUserID,
				Remark: "解冻期为 0，计提即时可用",
			})
		}
		before := acc.Balance
		acc.PendingRelease = money.Round2(acc.PendingRelease + amount)
		acc.TotalIncome = money.Round2(acc.TotalIncome + amount)
		acc.Version++
		if err := s.commRepo.UpdateAccount(tx, acc); err != nil {
			return err
		}
		return s.commRepo.CreateTx(tx, &model.CommissionTransaction{
			TxNo: genTxNo(), AdminID: adminID, Type: bizType, Direction: model.DirectionIncome,
			Amount: amount, BalanceBefore: before, BalanceAfter: before,
			BizType: bizType, RefNo: in.OrderNo, OrderID: in.OrderID, OrderNo: in.OrderNo,
			CustomerID: in.BuyerUserID, ReleaseAt: releaseAt,
			Remark: commissionRemark(bizType, in.OrderNo),
		})
	})
}

// ClawbackForRefund 按比例冲减提成。
// 冲减额 = 该订单已计提 × 退款额 / 实付额，上限 = 已计提 - 已冲减；
// 先从解冻中扣（未到手的钱先还），不足部分扣可用余额（允许为负：已提现/已打款后仍发生退款）。
func (s *commissionService) ClawbackForRefund(ctx context.Context, in ClawbackInput) error {
	if in.RefundNo == "" || in.RefundAmount <= 0 || in.PaidAmount <= 0 || in.OrderID == 0 {
		return nil
	}
	adminID, err := s.commRepo.OrderSalesOwner(ctx, in.OrderID)
	if err != nil {
		return err
	}
	if adminID == 0 {
		return nil
	}
	accrualTypes := []string{model.TxTypeAccrueFirst, model.TxTypeAccrueSubsequent, model.TxTypeAccrueRenewal}
	accrued, err := s.commRepo.SumTxAmount(ctx, adminID, in.OrderID, accrualTypes...)
	if err != nil {
		return err
	}
	if accrued <= 0 {
		return nil
	}
	clawed, err := s.commRepo.SumTxAmount(ctx, adminID, in.OrderID, model.TxTypeRefundClawback)
	if err != nil {
		return err
	}
	remaining := money.Round2(accrued - clawed)
	if remaining <= 0 {
		return nil
	}
	amount := money.Round2(accrued * in.RefundAmount / in.PaidAmount)
	if amount > remaining {
		amount = remaining
	}
	if amount <= 0 {
		return nil
	}
	rates, err := s.commRepo.Rates(ctx)
	if err != nil {
		return err
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		acc, err := s.commRepo.EnsureAccount(tx, adminID)
		if err != nil {
			return err
		}
		if _, err := s.commRepo.FindTxByBiz(tx, adminID, model.TxTypeRefundClawback, in.RefundNo); err == nil {
			return nil // 该退款单已冲减
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		fromPending := amount
		if fromPending > acc.PendingRelease {
			fromPending = acc.PendingRelease
		}
		fromBalance := money.Round2(amount - fromPending)
		if fromBalance > 0 && acc.Balance-fromBalance < 0 && !rates.AllowNegative {
			// 未开启负余额：可用余额扣到 0 为止，不足部分记日志由人工追缴。
			fromBalance = acc.Balance
			if fromBalance < 0 {
				fromBalance = 0
			}
			s.logger.Warn("sales commission clawback truncated by allow_negative=false",
				zap.Uint64("admin_id", adminID), zap.String("refund_no", in.RefundNo),
				zap.Float64("expected", amount), zap.Float64("actual", fromPending+fromBalance))
			amount = money.Round2(fromPending + fromBalance)
		}
		before := acc.Balance
		after := money.Round2(before - fromBalance)
		acc.Balance = after
		acc.PendingRelease = money.Round2(acc.PendingRelease - fromPending)
		acc.TotalOut = money.Round2(acc.TotalOut + amount)
		acc.Version++
		if err := s.commRepo.UpdateAccount(tx, acc); err != nil {
			return err
		}
		return s.commRepo.CreateTx(tx, &model.CommissionTransaction{
			TxNo: genTxNo(), AdminID: adminID, Type: model.TxTypeRefundClawback, Direction: model.DirectionExpense,
			Amount: amount, BalanceBefore: before, BalanceAfter: after,
			BizType: model.TxTypeRefundClawback, RefNo: in.RefundNo,
			OrderID: in.OrderID, OrderNo: in.OrderNo, CustomerID: in.BuyerUserID,
			Remark: fmt.Sprintf("订单 %s 退款 %.2f 冲减提成（解冻中 %.2f / 可用 %.2f）",
				in.OrderNo, in.RefundAmount, fromPending, fromBalance),
		})
	})
}

// ReleaseDue 解冻到期计提：pending_release → balance。
// 每条计提流水按其金额解冻，但以账户实际 pending_release 为上限：
// 退款冲减可能已提前扣掉部分解冻中金额，此时后续流水解冻额被压缩，保证总量守恒。
func (s *commissionService) ReleaseDue(ctx context.Context, limit int) (int, error) {
	now := time.Now()
	released := 0
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rows, err := s.commRepo.ListReleasable(tx, now, limit)
		if err != nil {
			return err
		}
		for _, row := range rows {
			acc, err := s.commRepo.EnsureAccount(tx, row.AdminID)
			if err != nil {
				return err
			}
			amount := row.Amount
			if amount > acc.PendingRelease {
				amount = acc.PendingRelease
			}
			// 已被退款冲减吃光的计提：只标记已处理，不再产生资金流动。
			if amount <= 0 {
				if err := s.commRepo.MarkReleased(tx, row.ID); err != nil {
					return err
				}
				continue
			}
			before := acc.Balance
			after := money.Round2(before + amount)
			acc.Balance = after
			acc.PendingRelease = money.Round2(acc.PendingRelease - amount)
			acc.Version++
			if err := s.commRepo.UpdateAccount(tx, acc); err != nil {
				return err
			}
			if err := s.commRepo.CreateTx(tx, &model.CommissionTransaction{
				TxNo: genTxNo(), AdminID: row.AdminID, Type: model.TxTypeRelease, Direction: model.DirectionIncome,
				Amount: amount, BalanceBefore: before, BalanceAfter: after,
				BizType: model.TxTypeRelease, RefNo: row.TxNo,
				OrderID: row.OrderID, OrderNo: row.OrderNo, CustomerID: row.CustomerID,
				Remark: fmt.Sprintf("计提 %s 解冻转可用", row.TxNo),
			}); err != nil {
				return err
			}
			if err := s.commRepo.MarkReleased(tx, row.ID); err != nil {
				return err
			}
			released++
		}
		return nil
	})
	if err != nil {
		return released, err
	}
	return released, nil
}

func (s *commissionService) Summary(ctx context.Context, adminID uint64) (*dto.CommissionSummary, error) {
	rates, err := s.commRepo.Rates(ctx)
	if err != nil {
		return nil, err
	}
	summary := &dto.CommissionSummary{
		AdminID:        adminID,
		Enabled:        rates.Enabled,
		MinWithdraw:    rates.MinWithdraw,
		ReleaseDays:    rates.ReleaseDays,
		FirstOrderRate: rates.FirstOrder,
		SubsequentRate: rates.Subsequent,
		RenewalRate:    rates.Renewal,
	}
	acc, err := s.commRepo.FindAccount(ctx, adminID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if acc != nil {
		summary.Balance = acc.Balance
		summary.PendingRelease = acc.PendingRelease
		summary.Frozen = acc.Frozen
		summary.TotalIncome = acc.TotalIncome
		summary.TotalOut = acc.TotalOut
	}
	if brief, err := s.relRepo.AdminBrief(ctx, adminID); err != nil {
		return nil, err
	} else if brief != nil {
		summary.AdminName = brief.Username
		summary.AdminRealName = brief.RealName
		summary.DepartmentName = brief.DepartmentName
	}
	if summary.PendingWithdraw, err = s.pendingWithdrawSum(ctx, adminID); err != nil {
		return nil, err
	}
	if summary.ActiveCustomers, err = s.relRepo.CountActiveByAdmin(ctx, adminID); err != nil {
		return nil, err
	}
	if summary.Balance < 0 {
		summary.Debt = true
		summary.Withdrawable = 0
	} else {
		summary.Withdrawable = summary.Balance
	}
	return summary, nil
}

func (s *commissionService) List(ctx context.Context, scope dto.Scope, q dto.CommissionQuery) (*dto.CommissionListResponse, error) {
	f := repository.TxFilter{
		Type: q.Type, OrderNo: q.OrderNo, CustomerID: q.CustomerID,
		Page: q.Page, PageSize: q.PageSize,
	}
	if start, ok := parseDayStart(q.StartAt); ok {
		f.StartAt = &start
	}
	if end, ok := parseDayEnd(q.EndAt); ok {
		f.EndAt = &end
	}
	switch {
	case scope.All:
		f.AdminID = q.AdminID
		f.DepartmentID = q.DepartmentID
	case scope.AdminID > 0:
		// 销售只看自己的台账，忽略前端 admin_id 传参。
		f.AdminID = scope.AdminID
	case scope.DepartmentID > 0:
		f.AdminID = q.AdminID
		f.DepartmentID = scope.DepartmentID
	default:
		return &dto.CommissionListResponse{Items: []dto.CommissionInfo{}, Meta: dto.ListMeta{}}, nil
	}
	rows, total, err := s.commRepo.ListTxRows(ctx, f)
	if err != nil {
		return nil, err
	}
	labels := model.TxTypeLabels()
	items := make([]dto.CommissionInfo, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.CommissionInfo{
			ID:             row.ID,
			TxNo:           row.TxNo,
			AdminID:        row.AdminID,
			AdminName:      row.AdminName,
			AdminRealName:  row.AdminRealName,
			DepartmentName: row.DepartmentName,
			Type:           row.Type,
			TypeLabel:      labels[row.Type],
			Direction:      row.Direction,
			Amount:         row.Amount,
			BalanceAfter:   row.BalanceAfter,
			OrderID:        row.OrderID,
			OrderNo:        row.OrderNo,
			CustomerUserID: row.CustomerID,
			CustomerName:   row.CustomerName,
			ReleaseAt:      formatTimePtr(row.ReleaseAt),
			Remark:         row.Remark,
			CreatedAt:      row.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	p, ps := normalizePage(q.Page, q.PageSize)
	return &dto.CommissionListResponse{Items: items, Meta: dto.ListMeta{Page: p, PageSize: ps, Total: total}}, nil
}

// —— 内部工具 ——

func commissionRemark(bizType, orderNo string) string {
	label := model.TxTypeLabels()[bizType]
	if label == "" {
		label = "提成"
	}
	return fmt.Sprintf("%s：订单 %s", label, orderNo)
}

// genTxNo 生成提成台账流水号，如 SC2026091012000012345。
func genTxNo() string {
	return fmt.Sprintf("SC%s%05d", time.Now().Format("20060102150405"), rand.Intn(100000))
}

// parseDayStart 解析 YYYY-MM-DD 为当天 00:00:00。
func parseDayStart(v string) (time.Time, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("2006-01-02", v, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// parseDayEnd 解析 YYYY-MM-DD 为次日 00:00:00（右开区间，含当天）。
func parseDayEnd(v string) (time.Time, bool) {
	t, ok := parseDayStart(v)
	if !ok {
		return time.Time{}, false
	}
	return t.AddDate(0, 0, 1), true
}

// pendingWithdrawSum 待审核提现额（概览展示）。提现仓储可为 nil（未装配提现能力时跳过）。
func (s *commissionService) pendingWithdrawSum(ctx context.Context, adminID uint64) (float64, error) {
	if s.withdrawRepo == nil {
		return 0, nil
	}
	return s.withdrawRepo.SumPendingAmount(ctx, adminID)
}
