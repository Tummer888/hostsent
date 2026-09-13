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

// PayoutPort 打款能力最小接口（由支付中心实现，装配层注入）。
// 与 finance/withdraw 的 PayoutPort 同形，bizType 固定传 sales_withdraw（doc86 §2.5）。
type PayoutPort interface {
	CreatePayout(ctx context.Context, bizType string, withdrawID uint64, withdrawNo string, userID uint64, amount float64, mode string) (payoutID uint64, payoutNo string, actualMode string, err error)
	StatusByNo(ctx context.Context, payoutNo string) (string, error)
}

// WithdrawalService 提成提现：申请 → 冻结 → 审核 → 打款 → 结算。
//
// 与财务提现的差异（doc86 §3.6）：
//   - 主体是 admins（admin_id），提现单独立表 sales_withdrawals；
//   - 没有「转入钱包」出口：本模块不含任何钱包调用，提成只能提现。
type WithdrawalService interface {
	// Apply 销售自助申请提现（可用余额 → 冻结）。
	Apply(ctx context.Context, adminID uint64, req dto.WithdrawApplyRequest) (*dto.WithdrawalInfo, error)
	// List 提现单列表（按数据范围过滤）。
	List(ctx context.Context, scope dto.Scope, q dto.WithdrawalQuery) (*dto.WithdrawalListResponse, error)
	// Approve 审核通过 → 建打款单（biz_type=sales_withdraw）。
	Approve(ctx context.Context, scope dto.Scope, id, operatorID uint64, operatorName, remark string) (*dto.WithdrawalInfo, error)
	// Reject 审核驳回 → 解冻回可用。
	Reject(ctx context.Context, scope dto.Scope, id, operatorID uint64, operatorName, remark string) (*dto.WithdrawalInfo, error)
	// MarkPaid 登记打款 → 冻结出账（幂等）。
	MarkPaid(ctx context.Context, id uint64, channelTx, receiptURL, remark string, operatorID uint64) error
	// MarkFailed 打款失败 → 解冻回可用。
	MarkFailed(ctx context.Context, id uint64, reason string, operatorID uint64) error
}

type withdrawalService struct {
	db       *gorm.DB
	commRepo repository.CommissionRepository
	wdRepo   repository.WithdrawalRepository
	relRepo  repository.RelationRepository
	payout   PayoutPort
	logger   *zap.Logger
}

// NewWithdrawalService 创建提成提现服务。payout 可为 nil（未装配支付中心时仅支持线下处理）。
func NewWithdrawalService(db *gorm.DB, commRepo repository.CommissionRepository, wdRepo repository.WithdrawalRepository, relRepo repository.RelationRepository, logger *zap.Logger) *withdrawalService {
	return &withdrawalService{db: db, commRepo: commRepo, wdRepo: wdRepo, relRepo: relRepo, logger: logger}
}

// SetPayoutPort 注入打款能力（装配层调用）。
func (s *withdrawalService) SetPayoutPort(port PayoutPort) { s.payout = port }

func (s *withdrawalService) Apply(ctx context.Context, adminID uint64, req dto.WithdrawApplyRequest) (*dto.WithdrawalInfo, error) {
	rates, err := s.commRepo.Rates(ctx)
	if err != nil {
		return nil, err
	}
	if !rates.Enabled {
		return nil, ErrSalesDisabled
	}
	amount := money.Round2(req.Amount)
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}
	if amount < rates.MinWithdraw {
		return nil, ErrBelowMinWithdraw
	}
	var created *model.SalesWithdrawal
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		acc, err := s.commRepo.EnsureAccount(tx, adminID)
		if err != nil {
			return err
		}
		// 欠款（余额为负）时一律拒绝提现，必须先由后续提成把欠款填平。
		if acc.Balance < amount {
			return ErrInsufficientCommission
		}
		before := acc.Balance
		after := money.Round2(before - amount)
		acc.Balance = after
		acc.Frozen = money.Round2(acc.Frozen + amount)
		acc.Version++
		if err := s.commRepo.UpdateAccount(tx, acc); err != nil {
			return err
		}
		record := &model.SalesWithdrawal{
			WithdrawNo:  genSalesWithdrawNo(),
			AdminID:     adminID,
			Amount:      amount,
			Channel:     strings.TrimSpace(req.Channel),
			Account:     maskTail(req.Account),
			AccountName: strings.TrimSpace(req.AccountName),
			BankName:    strings.TrimSpace(req.BankName),
			PayoutMode:  "manual",
			Status:      model.WithdrawStatusPending,
			Remark:      strings.TrimSpace(req.Remark),
		}
		if err := s.wdRepo.CreateWithdrawal(tx, record); err != nil {
			return err
		}
		created = record
		return s.commRepo.CreateTx(tx, &model.CommissionTransaction{
			TxNo: genTxNo(), AdminID: adminID, Type: model.TxTypeWithdrawFreeze, Direction: model.DirectionExpense,
			Amount: amount, BalanceBefore: before, BalanceAfter: after,
			BizType: model.TxTypeWithdrawFreeze, RefNo: record.WithdrawNo,
			Remark: "提成提现申请冻结",
		})
	})
	if err != nil {
		return nil, err
	}
	return buildWithdrawalInfo(*created, "", "", ""), nil
}

func (s *withdrawalService) List(ctx context.Context, scope dto.Scope, q dto.WithdrawalQuery) (*dto.WithdrawalListResponse, error) {
	f := repository.WithdrawFilter{Status: q.Status, Page: q.Page, PageSize: q.PageSize}
	switch {
	case scope.All:
		f.AdminID = q.AdminID
		f.DepartmentID = q.DepartmentID
	case scope.AdminID > 0:
		// 销售只看自己的提现单，忽略前端 admin_id 传参。
		f.AdminID = scope.AdminID
	case scope.DepartmentID > 0:
		f.AdminID = q.AdminID
		f.DepartmentID = scope.DepartmentID
	default:
		return &dto.WithdrawalListResponse{Items: []dto.WithdrawalInfo{}, Meta: dto.ListMeta{}}, nil
	}
	rows, total, err := s.wdRepo.ListWithdrawalRows(ctx, f)
	if err != nil {
		return nil, err
	}
	items := make([]dto.WithdrawalInfo, 0, len(rows))
	for _, row := range rows {
		items = append(items, *buildWithdrawalInfo(row.SalesWithdrawal, row.AdminName, row.AdminRealName, row.DepartmentName))
	}
	p, ps := normalizePage(q.Page, q.PageSize)
	return &dto.WithdrawalListResponse{Items: items, Meta: dto.ListMeta{Page: p, PageSize: ps, Total: total}}, nil
}

func (s *withdrawalService) Approve(ctx context.Context, scope dto.Scope, id, operatorID uint64, operatorName, remark string) (*dto.WithdrawalInfo, error) {
	var approved *model.SalesWithdrawal
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		w, err := s.wdRepo.FindWithdrawalByID(tx, id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrWithdrawNotFound
			}
			return err
		}
		if err := s.checkScope(ctx, scope, w.AdminID); err != nil {
			return err
		}
		if w.Status != model.WithdrawStatusPending {
			return ErrWithdrawStatus
		}
		now := time.Now()
		w.Status = model.WithdrawStatusApproved
		w.AuditBy = operatorID
		w.AuditByName = operatorName
		w.AuditedAt = &now
		if remark != "" {
			w.Remark = remark
		}
		if err := s.wdRepo.UpdateWithdrawal(tx, w); err != nil {
			return err
		}
		approved = w
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 打款任务在事务外创建：渠道回调较慢，不占着提现单行锁。
	// 并发重复审核已被状态机挡住（只有一条事务能把 pending 推到 approved）。
	if s.payout != nil {
		payoutID, payoutNo, actualMode, perr := s.payout.CreatePayout(ctx, payoutBizSalesWithdraw, approved.ID, approved.WithdrawNo, 0, approved.Amount, "manual")
		if perr != nil {
			return nil, perr
		}
		approved.PayoutID = payoutID
		approved.PayoutNo = payoutNo
		if actualMode != "" {
			approved.PayoutMode = actualMode
		}
		// 渠道接口打款已成功时直接结算；人工模式等待登记打款。
		if actualMode == "api" {
			if st, serr := s.payout.StatusByNo(ctx, payoutNo); serr == nil {
				switch st {
				case "paid":
					if err := s.MarkPaid(ctx, approved.ID, "", "", "渠道接口打款成功", operatorID); err != nil {
						s.logger.Warn("sales withdrawal auto settle failed",
							zap.Uint64("withdrawal_id", approved.ID), zap.Error(err))
					}
				case "failed":
					if err := s.MarkFailed(ctx, approved.ID, "渠道接口打款失败", operatorID); err != nil {
						s.logger.Warn("sales withdrawal auto fail-back failed",
							zap.Uint64("withdrawal_id", approved.ID), zap.Error(err))
					}
				}
			}
		}
		if err := s.db.WithContext(ctx).Model(&model.SalesWithdrawal{}).
			Where("id = ?", approved.ID).
			Updates(map[string]any{"payout_id": approved.PayoutID, "payout_no": approved.PayoutNo, "payout_mode": approved.PayoutMode}).
			Error; err != nil {
			return nil, err
		}
	}
	return s.info(ctx, approved.ID)
}

func (s *withdrawalService) Reject(ctx context.Context, scope dto.Scope, id, operatorID uint64, operatorName, remark string) (*dto.WithdrawalInfo, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		w, err := s.wdRepo.FindWithdrawalByID(tx, id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrWithdrawNotFound
			}
			return err
		}
		if err := s.checkScope(ctx, scope, w.AdminID); err != nil {
			return err
		}
		if w.Status != model.WithdrawStatusPending {
			return ErrWithdrawStatus
		}
		acc, err := s.commRepo.EnsureAccount(tx, w.AdminID)
		if err != nil {
			return err
		}
		before := acc.Balance
		after := money.Round2(before + w.Amount)
		acc.Balance = after
		acc.Frozen = money.Round2(maxFloat(acc.Frozen-w.Amount, 0))
		acc.Version++
		if err := s.commRepo.UpdateAccount(tx, acc); err != nil {
			return err
		}
		if err := s.commRepo.CreateTx(tx, &model.CommissionTransaction{
			TxNo: genTxNo(), AdminID: w.AdminID, Type: model.TxTypeWithdrawReturn, Direction: model.DirectionIncome,
			Amount: w.Amount, BalanceBefore: before, BalanceAfter: after,
			BizType: model.TxTypeWithdrawReturn, RefNo: w.WithdrawNo, Remark: "提现驳回，提成解冻退回",
		}); err != nil {
			return err
		}
		now := time.Now()
		w.Status = model.WithdrawStatusRejected
		w.AuditBy = operatorID
		w.AuditByName = operatorName
		w.AuditedAt = &now
		if remark != "" {
			w.Remark = remark
		}
		return s.wdRepo.UpdateWithdrawal(tx, w)
	})
	if err != nil {
		return nil, err
	}
	return s.info(ctx, id)
}

// MarkPaid 打款结算：frozen 出账（幂等，以提现单状态为幂等闸门）。
func (s *withdrawalService) MarkPaid(ctx context.Context, id uint64, channelTx, receiptURL, remark string, operatorID uint64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		w, err := s.wdRepo.FindWithdrawalByID(tx, id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrWithdrawNotFound
			}
			return err
		}
		if w.Status == model.WithdrawStatusPaid {
			return nil // 已结算，幂等返回
		}
		if w.Status != model.WithdrawStatusApproved && w.Status != model.WithdrawStatusPaying {
			return ErrWithdrawStatus
		}
		acc, err := s.commRepo.EnsureAccount(tx, w.AdminID)
		if err != nil {
			return err
		}
		acc.Frozen = money.Round2(maxFloat(acc.Frozen-w.Amount, 0))
		acc.TotalOut = money.Round2(acc.TotalOut + w.Amount)
		acc.Version++
		if err := s.commRepo.UpdateAccount(tx, acc); err != nil {
			return err
		}
		if err := s.commRepo.CreateTx(tx, &model.CommissionTransaction{
			TxNo: genTxNo(), AdminID: w.AdminID, Type: model.TxTypeWithdrawPaid, Direction: model.DirectionExpense,
			Amount: w.Amount, BalanceBefore: acc.Balance, BalanceAfter: acc.Balance,
			BizType: model.TxTypeWithdrawPaid, RefNo: w.WithdrawNo,
			Remark: firstNonEmpty(remark, "提成提现已打款"),
		}); err != nil {
			return err
		}
		now := time.Now()
		w.Status = model.WithdrawStatusPaid
		w.PaidAt = &now
		if remark != "" {
			w.Remark = remark
		}
		if err := s.wdRepo.UpdateWithdrawal(tx, w); err != nil {
			return err
		}
		if s.payout != nil && w.PayoutNo != "" && channelTx != "" {
			// 回写渠道流水到打款单；失败不影响资金侧结算（以提现单为准）。
			if err := s.syncPayout(ctx, w.PayoutNo, channelTx, receiptURL, remark, operatorID); err != nil {
				s.logger.Warn("sales payout mark-paid sync failed",
					zap.String("payout_no", w.PayoutNo), zap.Error(err))
			}
		}
		return nil
	})
}

// MarkFailed 打款失败：frozen 解冻回可用余额，状态置 failed（资金不悬空）。
func (s *withdrawalService) MarkFailed(ctx context.Context, id uint64, reason string, operatorID uint64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		w, err := s.wdRepo.FindWithdrawalByID(tx, id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrWithdrawNotFound
			}
			return err
		}
		if w.Status != model.WithdrawStatusApproved && w.Status != model.WithdrawStatusPaying {
			return ErrWithdrawStatus
		}
		acc, err := s.commRepo.EnsureAccount(tx, w.AdminID)
		if err != nil {
			return err
		}
		before := acc.Balance
		after := money.Round2(before + w.Amount)
		acc.Balance = after
		acc.Frozen = money.Round2(maxFloat(acc.Frozen-w.Amount, 0))
		acc.Version++
		if err := s.commRepo.UpdateAccount(tx, acc); err != nil {
			return err
		}
		if err := s.commRepo.CreateTx(tx, &model.CommissionTransaction{
			TxNo: genTxNo(), AdminID: w.AdminID, Type: model.TxTypeWithdrawReturn, Direction: model.DirectionIncome,
			Amount: w.Amount, BalanceBefore: before, BalanceAfter: after,
			BizType: model.TxTypeWithdrawReturn, RefNo: w.WithdrawNo,
			Remark: firstNonEmpty(reason, "打款失败，提成解冻退回"),
		}); err != nil {
			return err
		}
		w.Status = model.WithdrawStatusFailed
		if reason != "" {
			w.Remark = reason
		}
		return s.wdRepo.UpdateWithdrawal(tx, w)
	})
}

// —— 内部工具 ——

// payoutBizSalesWithdraw 销售提成提现域标识，与 payment/model 的同名常量取值一致。
// sales 模块不 import payment（只依赖 PayoutPort 契约），故按字符串对齐。
const payoutBizSalesWithdraw = "sales_withdraw"

// syncPayout 把打款结果回写到支付中心打款单（可选能力，端口未暴露时不报错）。
func (s *withdrawalService) syncPayout(ctx context.Context, payoutNo, channelTx, receiptURL, remark string, operatorID uint64) error {
	if syncer, ok := s.payout.(interface {
		MarkPaid(ctx context.Context, payoutNo, channelTx, receiptURL, remark string, operatorID uint64) error
	}); ok {
		return syncer.MarkPaid(ctx, payoutNo, channelTx, receiptURL, remark, operatorID)
	}
	return nil
}

// checkScope 校验操作者数据范围：主管只能操作本部门销售的提现单，超管不受限。
func (s *withdrawalService) checkScope(ctx context.Context, scope dto.Scope, adminID uint64) error {
	if scope.All || scope.DepartmentID == 0 {
		return nil
	}
	dept, err := s.relRepo.DepartmentOfAdmin(ctx, adminID)
	if err != nil {
		return err
	}
	if dept != scope.DepartmentID {
		return ErrOutOfScope
	}
	return nil
}

func (s *withdrawalService) info(ctx context.Context, id uint64) (*dto.WithdrawalInfo, error) {
	rows, _, err := s.wdRepo.ListWithdrawalRows(ctx, repository.WithdrawFilter{Page: 1, PageSize: 100})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.ID == id {
			return buildWithdrawalInfo(row.SalesWithdrawal, row.AdminName, row.AdminRealName, row.DepartmentName), nil
		}
	}
	w, err := s.wdRepo.FindWithdrawalByID(s.db.WithContext(ctx), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWithdrawNotFound
		}
		return nil, err
	}
	return buildWithdrawalInfo(*w, "", "", ""), nil
}

// buildWithdrawalInfo 组装提现单 DTO（账号已脱敏，不回传完整卡号）。
func buildWithdrawalInfo(w model.SalesWithdrawal, adminName, adminReal, deptName string) *dto.WithdrawalInfo {
	return &dto.WithdrawalInfo{
		ID:             w.ID,
		WithdrawNo:     w.WithdrawNo,
		AdminID:        w.AdminID,
		AdminName:      adminName,
		AdminRealName:  adminReal,
		DepartmentName: deptName,
		Amount:         w.Amount,
		Channel:        w.Channel,
		Account:        w.Account,
		AccountName:    w.AccountName,
		BankName:       w.BankName,
		PayoutMode:     w.PayoutMode,
		PayoutNo:       w.PayoutNo,
		Status:         w.Status,
		StatusLabel:    withdrawStatusLabel(w.Status),
		AuditBy:        w.AuditBy,
		AuditByName:    w.AuditByName,
		AuditedAt:      formatTimePtr(w.AuditedAt),
		PaidAt:         formatTimePtr(w.PaidAt),
		Remark:         w.Remark,
		CreatedAt:      w.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

func withdrawStatusLabel(status string) string {
	switch status {
	case model.WithdrawStatusPending:
		return "待审核"
	case model.WithdrawStatusApproved:
		return "待打款"
	case model.WithdrawStatusPaying:
		return "打款中"
	case model.WithdrawStatusPaid:
		return "已打款"
	case model.WithdrawStatusRejected:
		return "已驳回"
	case model.WithdrawStatusFailed:
		return "打款失败"
	default:
		return status
	}
}

// genSalesWithdrawNo 生成提成提现单号，如 SW2026091012000012345。
func genSalesWithdrawNo() string {
	return fmt.Sprintf("SW%s%05d", time.Now().Format("20060102150405"), rand.Intn(100000))
}

// maskTail 只保留尾 4 位，其余以 * 代替（提现单不落明文卡号）。
func maskTail(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	r := []rune(v)
	if len(r) <= 4 {
		return strings.Repeat("*", len(r))
	}
	return strings.Repeat("*", len(r)-4) + string(r[len(r)-4:])
}

func firstNonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
