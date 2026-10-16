package service

import (
	"context"
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

// Reconcile 账实核对：逐户校验账户记账值与台账推出值，并核对业务单据与台账。
//
// 为什么不再用「(收入 - 支出) - 钱包余额」这一个总数：
//
//	这个总数把口径错误和真丢账混成一个数。口径错的典型是它把冻结/解冻算进收付
//	（可用↔冻结的内部划转），提现一次就凭空虚增支出；真正的丢账则被这类噪声掩盖
//	在对不上号的差额里。实测全平台差额 3070 元，逐户拆开才看出是三个账户各自的
//	历史问题（一个 -5000、一个 +1000、一个 +500），总数只是它们的代数和。
//
// 现在改为按恒等式逐户核，任何一户对不上都能直接定位到 user_id 与差多少，
// 而不是给财务一个「差 3070」的数字让他们自己去猜。
func (s *reconService) Reconcile(ctx context.Context, period string) (*dto.ReconcileResponse, error) {
	_, _, count, err := s.txRepo.Stats(ctx)
	if err != nil {
		return nil, err
	}
	tolerance := s.tolerance(ctx)

	// 账户恒等式：余额/冻结/累计收支。
	accountRows, err := s.txRepo.ReconOffenders(ctx, tolerance)
	if err != nil {
		return nil, err
	}
	// 单据与台账：充值到账、退款回补。
	docRows, err := s.txRepo.ReconDocOffenders(ctx, tolerance)
	if err != nil {
		return nil, err
	}

	issues := make([]dto.ReconcileIssue, 0, len(accountRows)+len(docRows))
	for _, row := range accountRows {
		issues = append(issues, buildReconcileIssue(row))
	}
	for _, row := range docRows {
		issues = append(issues, buildReconcileIssue(row))
	}

	// 额度口径汇总：按真实收付（剔除冻结/解冻内部划转）算收入支出，
	// 与账户侧的可用+冻结对账。旧的 Stats 直接按方向求和，把 freeze 当支出，
	// 于是提现一次就被多算一笔，这个数在页面上长期对不上余额。
	totals, err := s.txRepo.ReconTotals(ctx)
	if err != nil {
		return nil, err
	}
	walletAssets := money.Round2(totals.Balance + totals.Frozen)
	netDiff := money.Round2(totals.Income - totals.Expense - walletAssets)

	status := "ok"
	if len(issues) > 0 {
		status = "suspicious"
	}
	// 观测：账务对账（T5.3）——正常/可疑计数与差异规模。
	observability.Inc("finance_reconcile_total", 1)
	if status == "suspicious" {
		observability.Inc("finance_reconcile_suspicious_total", 1)
	}
	return &dto.ReconcileResponse{
		Period:        period,
		IncomeTotal:   money.Round2(totals.Income),
		ExpenseTotal:  money.Round2(totals.Expense),
		TxCount:       count,
		WalletBalance: money.Round2(totals.Balance),
		WalletFrozen:  money.Round2(totals.Frozen),
		Diff:          netDiff,
		Status:        status,
		Tolerance:     tolerance,
		IssueCount:    len(issues),
		Issues:        issues,
	}, nil
}

// buildReconcileIssue 把仓储差异行转成响应结构，附上人话说明。
func buildReconcileIssue(row transrepo.ReconOffender) dto.ReconcileIssue {
	item := dto.ReconcileIssue{
		UserID:       row.UserID,
		Username:     row.Username,
		Check:        row.Check,
		Label:        reconcileCheckLabel(row.Check),
		AccountValue: money.Round2(row.Account),
		LedgerValue:  money.Round2(row.Ledger),
		Diff:         money.Round2(row.Diff),
	}
	item.Remark = reconcileRemark(row.Check, item.Diff)
	return item
}

// reconcileCheckLabel 检查项中文名（前端直接展示，避免前端再维护一份映射）。
func reconcileCheckLabel(check string) string {
	switch check {
	case "balance":
		return "可用余额"
	case "frozen":
		return "冻结金额"
	case "income":
		return "累计收入"
	case "expense":
		return "累计支出"
	case "recharge":
		return "充值到账"
	case "refund":
		return "退款回补"
	case "orphan_ledger":
		return "台账有流水但无单据"
	default:
		return check
	}
}

// reconcileRemark 差异说明：解释差在哪边，方便直接判断是数据缺失还是口径问题。
func reconcileRemark(check string, diff float64) string {
	switch check {
	case "balance":
		if diff > 0 {
			return "账户余额高于台账，可能漏记支出或存在外部直改余额"
		}
		return "账户余额低于台账，可能漏记收入或扣款未回写余额"
	case "frozen":
		if diff > 0 {
			return "冻结余额高于台账，可能存在未留痕的冻结或解冻漏记"
		}
		return "冻结余额低于台账，可能漏记冻结、或结算/解冻多扣了冻结"
	case "income", "expense":
		if diff > 0 {
			return "账户累计值高于台账，可能统计口径重复计数"
		}
		return "账户累计值低于台账，可能漏累加（历史数据或新增链路未接线）"
	case "recharge":
		return "充值单已到账但台账无对应流水，或金额不一致"
	case "refund":
		return "退款单已通过但台账无回补流水，或金额不一致"
	case "orphan_ledger":
		return "台账有该笔流水但业务单据不存在（单据被删或号对不上）"
	default:
		return "账户记账值与台账推出值不一致"
	}
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
