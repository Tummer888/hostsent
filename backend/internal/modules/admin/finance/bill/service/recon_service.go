package service

import (
	"context"
	"math"

	accountrepo "hostsent/backend/internal/modules/admin/finance/account/repository"
	"hostsent/backend/internal/modules/admin/finance/bill/dto"
	transrepo "hostsent/backend/internal/modules/admin/finance/transaction/repository"
	"hostsent/backend/internal/pkg/money"
)

// ReconService 对账能力：比对账务流水与钱包余额，检查账实相符。
type ReconService interface {
	Reconcile(ctx context.Context, period string) (*dto.ReconcileResponse, error)
}

type reconService struct {
	walletRepo accountrepo.WalletRepository
	txRepo     transrepo.TransactionRepository
}

// NewReconService 创建对账服务。
func NewReconService(walletRepo accountrepo.WalletRepository, txRepo transrepo.TransactionRepository) ReconService {
	return &reconService{walletRepo: walletRepo, txRepo: txRepo}
}

// Reconcile 汇总流水净变动与钱包余额进行核对。账实差异 = (收入 - 支出) - 当前钱包余额。
func (s *reconService) Reconcile(ctx context.Context, period string) (*dto.ReconcileResponse, error) {
	income, expense, count, err := s.txRepo.Stats(ctx)
	if err != nil {
		return nil, err
	}
	walletBalance, err := s.walletRepo.SumBalance(ctx)
	if err != nil {
		return nil, err
	}
	diff := money.Round2((income - expense) - walletBalance)
	status := "ok"
	if math.Abs(diff) > 0.005 {
		status = "suspicious"
	}
	return &dto.ReconcileResponse{
		Period:        period,
		IncomeTotal:   money.Round2(income),
		ExpenseTotal:  money.Round2(expense),
		TxCount:       count,
		WalletBalance: money.Round2(walletBalance),
		Diff:          diff,
		Status:        status,
	}, nil
}
