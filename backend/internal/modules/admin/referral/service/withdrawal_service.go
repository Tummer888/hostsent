package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/referral/dto"
	"hostsent/backend/internal/modules/admin/referral/model"
	"hostsent/backend/internal/modules/admin/referral/repository"
	"hostsent/backend/internal/pkg/money"
)

// WalletTransfer 把返现转入现金余额。装配层桥接到 WalletService.Change：
// 入账方向为收入，biz_type=referral_transfer，ref_no=转账号（钱包侧据此幂等）。
type WalletTransfer func(ctx context.Context, userID uint64, amount float64, bizType, refNo, remark string) error

// WithdrawalService 返现提现与转出。
type WithdrawalService interface {
	// Apply 申请提现：冻结可用余额，生成待审核提现单。
	Apply(ctx context.Context, userID uint64, req dto.WithdrawRequest) (*dto.WithdrawalInfo, error)
	// TransferToWallet 返现转入现金余额（即时到账，不需审核）。
	TransferToWallet(ctx context.Context, userID uint64, amount float64) error
	// ListByUser 我的提现记录。
	ListByUser(ctx context.Context, userID uint64, status string, page, pageSize int) (*dto.WithdrawalListResponse, error)
	// AdminList 管理端提现单列表。
	AdminList(ctx context.Context, f dto.WithdrawalQuery) (*dto.WithdrawalListResponse, error)
	// Approve 审核通过：扣减冻结并计入累计出账。
	Approve(ctx context.Context, id, operatorID uint64, operatorName, remark string) (*dto.WithdrawalInfo, error)
	// Reject 审核驳回：解冻退回可用余额。
	Reject(ctx context.Context, id, operatorID uint64, operatorName, remark string) (*dto.WithdrawalInfo, error)
}

type withdrawalService struct {
	db       *gorm.DB
	repo     repository.ReferralRepository
	transfer WalletTransfer
	logger   *zap.Logger
}

// NewWithdrawalService 创建返现提现服务。transfer 为 nil 时禁用转入余额。
func NewWithdrawalService(db *gorm.DB, repo repository.ReferralRepository, transfer WalletTransfer, logger *zap.Logger) *withdrawalService {
	return &withdrawalService{db: db, repo: repo, transfer: transfer, logger: logger}
}

func (s *withdrawalService) Apply(ctx context.Context, userID uint64, req dto.WithdrawRequest) (*dto.WithdrawalInfo, error) {
	rates, err := s.repo.Rates(ctx)
	if err != nil {
		return nil, err
	}
	if !rates.Enabled {
		return nil, ErrReferralDisabled
	}
	amount := money.Round2(req.Amount)
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}
	if amount < rates.MinWithdraw {
		return nil, ErrBelowMinWithdraw
	}

	var created *model.ReferralWithdrawal
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		acc, err := s.repo.EnsureAccount(tx, userID)
		if err != nil {
			return err
		}
		// 余额为负（退款冲减形成欠款）时禁止提现。
		if acc.Balance < amount {
			return ErrInsufficientBalance
		}
		before := acc.Balance
		after := money.Round2(before - amount)
		acc.Balance = after
		acc.Frozen = money.Round2(acc.Frozen + amount)
		acc.Version++
		if err := s.repo.UpdateAccount(tx, acc); err != nil {
			return err
		}

		record := &model.ReferralWithdrawal{
			WithdrawNo: genWithdrawNo(),
			UserID:     userID,
			Amount:     amount,
			Channel:    req.Channel,
			Account:    req.Account,
			Status:     model.WithdrawStatusPending,
		}
		if err := s.repo.CreateWithdrawal(tx, record); err != nil {
			return err
		}
		created = record
		return s.repo.CreateTx(tx, &model.ReferralTransaction{
			TxNo:          genTxNo(),
			UserID:        userID,
			Type:          model.TxTypeWithdrawFreeze,
			Direction:     model.DirectionExpense,
			Amount:        amount,
			BalanceBefore: before,
			BalanceAfter:  after,
			BizType:       model.TxTypeWithdrawFreeze,
			RefNo:         record.WithdrawNo,
			Remark:        "提现申请冻结返现余额",
		})
	})
	if err != nil {
		return nil, err
	}
	return withdrawalInfo(*created, ""), nil
}

// TransferToWallet 返现 → 现金余额。
// 先扣返现余额并落台账（防并发重复转出），再调钱包入账；
// 钱包入账失败时做补偿回滚，账实仍然一致。进程在两步之间崩溃的极端场景，
// 钱包侧可用同一 transferNo 幂等重放（见 docs 84）。
func (s *withdrawalService) TransferToWallet(ctx context.Context, userID uint64, amount float64) error {
	if s.transfer == nil {
		return ErrReferralDisabled
	}
	amount = money.Round2(amount)
	if amount <= 0 {
		return ErrInvalidAmount
	}
	rates, err := s.repo.Rates(ctx)
	if err != nil {
		return err
	}
	if !rates.Enabled {
		return ErrReferralDisabled
	}

	transferNo := genTransferNo()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		acc, err := s.repo.EnsureAccount(tx, userID)
		if err != nil {
			return err
		}
		if acc.Balance < amount {
			return ErrInsufficientBalance
		}
		before := acc.Balance
		after := money.Round2(before - amount)
		acc.Balance = after
		acc.TotalOut = money.Round2(acc.TotalOut + amount)
		acc.Version++
		if err := s.repo.UpdateAccount(tx, acc); err != nil {
			return err
		}
		return s.repo.CreateTx(tx, &model.ReferralTransaction{
			TxNo:          genTxNo(),
			UserID:        userID,
			Type:          model.TxTypeTransferOut,
			Direction:     model.DirectionExpense,
			Amount:        amount,
			BalanceBefore: before,
			BalanceAfter:  after,
			BizType:       model.TxTypeTransferOut,
			RefNo:         transferNo,
			Remark:        "返现转入现金余额",
		})
	})
	if err != nil {
		return err
	}

	if err := s.transfer(ctx, userID, amount, "referral_transfer", transferNo, "返现转入余额"); err != nil {
		// 补偿：把已扣返现余额加回，保证账实一致。
		if cerr := s.compensateTransfer(ctx, userID, amount, transferNo); cerr != nil {
			s.logger.Error("referral transfer compensate failed",
				zap.Uint64("user_id", userID), zap.Float64("amount", amount),
				zap.String("transfer_no", transferNo), zap.Error(cerr))
		}
		return err
	}
	return nil
}

// compensateTransfer 回滚一次失败的转入（加回可用余额与累计出账，并留补偿台账）。
func (s *withdrawalService) compensateTransfer(ctx context.Context, userID uint64, amount float64, transferNo string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		acc, err := s.repo.EnsureAccount(tx, userID)
		if err != nil {
			return err
		}
		if _, err := s.repo.FindTxByBiz(tx, userID, model.TxTypeTransferReturn, transferNo); err == nil {
			return nil // 已补偿
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		before := acc.Balance
		after := money.Round2(before + amount)
		acc.Balance = after
		acc.TotalOut = money.Round2(acc.TotalOut - amount)
		acc.Version++
		if err := s.repo.UpdateAccount(tx, acc); err != nil {
			return err
		}
		return s.repo.CreateTx(tx, &model.ReferralTransaction{
			TxNo:          genTxNo(),
			UserID:        userID,
			Type:          model.TxTypeTransferReturn,
			Direction:     model.DirectionIncome,
			Amount:        amount,
			BalanceBefore: before,
			BalanceAfter:  after,
			BizType:       model.TxTypeTransferReturn,
			RefNo:         transferNo,
			Remark:        "转入余额失败，返现回滚",
		})
	})
}

func (s *withdrawalService) ListByUser(ctx context.Context, userID uint64, status string, page, pageSize int) (*dto.WithdrawalListResponse, error) {
	rows, total, err := s.repo.ListWithdrawalRows(ctx, repository.WithdrawFilter{UserID: userID, Status: status, Page: page, PageSize: pageSize})
	if err != nil {
		return nil, err
	}
	return buildWithdrawalList(rows, total, page, pageSize), nil
}

func (s *withdrawalService) AdminList(ctx context.Context, f dto.WithdrawalQuery) (*dto.WithdrawalListResponse, error) {
	rows, total, err := s.repo.ListWithdrawalRows(ctx, repository.WithdrawFilter{
		UserID: f.UserID, Status: f.Status, Page: f.Page, PageSize: f.PageSize,
	})
	if err != nil {
		return nil, err
	}
	return buildWithdrawalList(rows, total, f.Page, f.PageSize), nil
}

func (s *withdrawalService) Approve(ctx context.Context, id, operatorID uint64, operatorName, remark string) (*dto.WithdrawalInfo, error) {
	var updated *model.ReferralWithdrawal
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		w, err := s.repo.FindWithdrawalByID(tx, id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrWithdrawNotFound
			}
			return err
		}
		if w.Status != model.WithdrawStatusPending {
			return ErrWithdrawStatus
		}
		acc, err := s.repo.EnsureAccount(tx, w.UserID)
		if err != nil {
			return err
		}
		acc.Frozen = money.Round2(maxFloat(acc.Frozen-w.Amount, 0))
		acc.TotalOut = money.Round2(acc.TotalOut + w.Amount)
		acc.Version++
		if err := s.repo.UpdateAccount(tx, acc); err != nil {
			return err
		}
		now := time.Now()
		w.Status = model.WithdrawStatusApproved
		w.AuditBy = operatorID
		w.AuditByName = operatorName
		w.AuditedAt = &now
		if remark != "" {
			w.Remark = remark
		}
		if err := s.repo.UpdateWithdrawal(tx, w); err != nil {
			return err
		}
		updated = w
		return nil
	})
	if err != nil {
		return nil, err
	}
	return withdrawalInfo(*updated, ""), nil
}

func (s *withdrawalService) Reject(ctx context.Context, id, operatorID uint64, operatorName, remark string) (*dto.WithdrawalInfo, error) {
	var updated *model.ReferralWithdrawal
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		w, err := s.repo.FindWithdrawalByID(tx, id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrWithdrawNotFound
			}
			return err
		}
		if w.Status != model.WithdrawStatusPending {
			return ErrWithdrawStatus
		}
		acc, err := s.repo.EnsureAccount(tx, w.UserID)
		if err != nil {
			return err
		}
		before := acc.Balance
		after := money.Round2(before + w.Amount)
		acc.Balance = after
		acc.Frozen = money.Round2(maxFloat(acc.Frozen-w.Amount, 0))
		acc.Version++
		if err := s.repo.UpdateAccount(tx, acc); err != nil {
			return err
		}
		if err := s.repo.CreateTx(tx, &model.ReferralTransaction{
			TxNo:          genTxNo(),
			UserID:        w.UserID,
			Type:          model.TxTypeWithdrawReturn,
			Direction:     model.DirectionIncome,
			Amount:        w.Amount,
			BalanceBefore: before,
			BalanceAfter:  after,
			BizType:       model.TxTypeWithdrawReturn,
			RefNo:         w.WithdrawNo,
			Remark:        "提现驳回，返现解冻退回",
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
		if err := s.repo.UpdateWithdrawal(tx, w); err != nil {
			return err
		}
		updated = w
		return nil
	})
	if err != nil {
		return nil, err
	}
	return withdrawalInfo(*updated, ""), nil
}

// —— DTO 组装 ——

func buildWithdrawalList(rows []repository.WithdrawalRow, total int64, page, pageSize int) *dto.WithdrawalListResponse {
	items := make([]dto.WithdrawalInfo, 0, len(rows))
	for _, row := range rows {
		items = append(items, *withdrawalInfo(row.ReferralWithdrawal, row.Username))
	}
	p, ps := normalizePage(page, pageSize)
	return &dto.WithdrawalListResponse{Items: items, Meta: dto.ListMeta{Page: p, PageSize: ps, Total: total}}
}

func withdrawalInfo(w model.ReferralWithdrawal, username string) *dto.WithdrawalInfo {
	auditedAt := ""
	if w.AuditedAt != nil {
		auditedAt = w.AuditedAt.Format(time.RFC3339)
	}
	return &dto.WithdrawalInfo{
		ID:          w.ID,
		WithdrawNo:  w.WithdrawNo,
		UserID:      w.UserID,
		Username:    username,
		Amount:      w.Amount,
		Channel:     w.Channel,
		Account:     w.Account,
		Status:      w.Status,
		AuditBy:     w.AuditBy,
		AuditByName: w.AuditByName,
		AuditedAt:   auditedAt,
		Remark:      w.Remark,
		CreatedAt:   w.CreatedAt.Format(time.RFC3339),
	}
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// genWithdrawNo 生成提现单号，如 RW2026091012000012345。
func genWithdrawNo() string {
	return fmt.Sprintf("RW%s%05d", time.Now().Format("20060102150405"), rand.Intn(100000))
}

// genTransferNo 生成转账号，如 RT2026091012000012345。
func genTransferNo() string {
	return fmt.Sprintf("RT%s%05d", time.Now().Format("20060102150405"), rand.Intn(100000))
}
