package service

import (
	"context"
	"math"
	"strconv"
	"strings"

	accountrepo "hostsent/backend/internal/modules/admin/finance/account/repository"
	"hostsent/backend/internal/modules/admin/finance/bill/dto"
	settingsmodel "hostsent/backend/internal/modules/admin/finance/settings/model"
	transrepo "hostsent/backend/internal/modules/admin/finance/transaction/repository"
	"hostsent/backend/internal/pkg/money"
	"hostsent/backend/internal/pkg/observability"
)

// ConfigReader 读取 system_configs 原文（键不存在返回 ok=false，不报错，沿用装配层注入约定）。
type ConfigReader func(ctx context.Context, key string) (string, bool, error)

// ConfigKeyReconTolerance 对账差异容差（元）：键定义见 settings/model，唯一定义处。
const ConfigKeyReconTolerance = settingsmodel.ConfigKeyReconTolerance

// defaultReconTolerance 容差默认值：1 分钱。历史实现硬编码 0.005，
// 现由财务配置接管（口径可调，且页面上能看到当前取值）。
const defaultReconTolerance = 0.01

// ReconService 对账能力：比对账务流水与钱包余额，检查账实相符。
type ReconService interface {
	Reconcile(ctx context.Context, period string) (*dto.ReconcileResponse, error)
	// SetConfigReader 注入配置读取器（finance.recon_tolerance）：装配层接线。
	SetConfigReader(reader ConfigReader)
}

type reconService struct {
	walletRepo accountrepo.WalletRepository
	txRepo     transrepo.TransactionRepository
	// configReader 读财务配置；未注入时用默认容差。
	configReader ConfigReader
}

// NewReconService 创建对账服务。
func NewReconService(walletRepo accountrepo.WalletRepository, txRepo transrepo.TransactionRepository) ReconService {
	return &reconService{walletRepo: walletRepo, txRepo: txRepo}
}

// SetConfigReader 注入配置读取器（装配层提供，避免账单子域 import 系统配置模块）。
func (s *reconService) SetConfigReader(reader ConfigReader) {
	s.configReader = reader
}

// Reconcile 汇总流水净变动与钱包余额进行核对。账实差异 = (收入 - 支出) - 当前钱包余额。
// 注意：程序化结转产生的流水与人工调账都会改变净变动，差异非 0 不必然代表丢账，
// 判定阈值由财务配置（finance.recon_tolerance）决定。
func (s *reconService) Reconcile(ctx context.Context, period string) (*dto.ReconcileResponse, error) {
	income, expense, count, err := s.txRepo.Stats(ctx)
	if err != nil {
		return nil, err
	}
	walletBalance, err := s.walletRepo.SumBalance(ctx)
	if err != nil {
		return nil, err
	}
	tolerance := s.tolerance(ctx)
	diff := money.Round2((income - expense) - walletBalance)
	status := "ok"
	if math.Abs(diff) > tolerance {
		status = "suspicious"
	}
	// 观测：账务对账（T5.3）——正常/可疑计数。
	observability.Inc("finance_reconcile_total", 1)
	if status == "suspicious" {
		observability.Inc("finance_reconcile_suspicious_total", 1)
	}
	return &dto.ReconcileResponse{
		Period:        period,
		IncomeTotal:   money.Round2(income),
		ExpenseTotal:  money.Round2(expense),
		TxCount:       count,
		WalletBalance: money.Round2(walletBalance),
		Diff:          diff,
		Status:        status,
		Tolerance:     tolerance,
	}, nil
}

// tolerance 对账容差：读财务配置，缺失/非法时回默认值。
func (s *reconService) tolerance(ctx context.Context) float64 {
	if s.configReader == nil {
		return defaultReconTolerance
	}
	raw, ok, err := s.configReader(ctx, ConfigKeyReconTolerance)
	if err != nil || !ok {
		return defaultReconTolerance
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || value < 0 {
		return defaultReconTolerance
	}
	return money.Round2(value)
}
