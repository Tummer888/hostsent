// Package service 提供财务统计聚合服务（总览与报表共用）。
package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	settingsmodel "hostsent/backend/internal/modules/admin/finance/settings/model"
	"hostsent/backend/internal/modules/admin/finance/stats/dto"
	"hostsent/backend/internal/modules/admin/finance/stats/repository"
	transmodel "hostsent/backend/internal/modules/admin/finance/transaction/model"
	withdrawmodel "hostsent/backend/internal/modules/admin/finance/referral/walletwithdraw/model"
	"hostsent/backend/internal/pkg/money"
)

// ConfigReader 读取 system_configs 原文（键不存在返回 ok=false，不报错，沿用装配层注入约定）。
type ConfigReader func(ctx context.Context, key string) (string, bool, error)

// ConfigKeyBalanceWarning 余额预警阈值（元）：键定义见 settings/model，唯一定义处。
const ConfigKeyBalanceWarning = settingsmodel.ConfigKeyBalanceWarning

const (
	defaultBalanceWarning = 50.0
	defaultRangeDays      = 30
	maxRangeDays          = 1096 // 3 年，超出则收敛起点
	maxDayGranularityDays = 366  // 超过则自动升为按月
)

// StatsService 财务统计能力。
type StatsService interface {
	Stats(ctx context.Context, q dto.StatsQuery) (*dto.StatsResponse, error)
	// RevenueOf 期间收入口径（服务收入 = 消费 − 退款，另附资金口径与佣金）。
	// 成本/利润模块复用本方法取收入，保证与财务总览/报表同口径。
	RevenueOf(ctx context.Context, start, end time.Time) (*dto.Revenue, error)
	// SetConfigReader 注入配置读取器（finance.balance_warning）：装配层接线。
	SetConfigReader(reader ConfigReader)
}

type statsService struct {
	repo         repository.StatsRepository
	configReader ConfigReader
}

// NewStatsService 创建财务统计服务。
func NewStatsService(repo repository.StatsRepository) StatsService {
	return &statsService{repo: repo}
}

// SetConfigReader 注入配置读取器（装配层提供，避免本模块 import 系统配置模块）。
func (s *statsService) SetConfigReader(reader ConfigReader) {
	s.configReader = reader
}

func (s *statsService) Stats(ctx context.Context, q dto.StatsQuery) (*dto.StatsResponse, error) {
	start, end, clamped := normalizeRange(q.StartTime, q.EndTime)
	granularity := normalizeGranularity(q.Granularity, start, end)

	txSummary, err := s.repo.TxSummary(ctx, start, end)
	if err != nil {
		return nil, err
	}
	internalCount, err := s.repo.TxInternalCount(ctx, start, end)
	if err != nil {
		return nil, err
	}
	trendRows, err := s.repo.TxTrend(ctx, start, end, granularity)
	if err != nil {
		return nil, err
	}
	typeRows, err := s.repo.TxByType(ctx, start, end)
	if err != nil {
		return nil, err
	}
	billRange, err := s.repo.BillRange(ctx, start, end)
	if err != nil {
		return nil, err
	}
	billStatus, err := s.repo.BillByStatus(ctx, start, end)
	if err != nil {
		return nil, err
	}
	unpaidCount, unpaidAmount, err := s.repo.BillUnpaid(ctx)
	if err != nil {
		return nil, err
	}
	rechargePendingCount, rechargePendingAmount, err := s.repo.RechargeByStatus(ctx, "pending")
	if err != nil {
		return nil, err
	}
	withdrawPendingCount, withdrawPendingAmount, err := s.repo.WithdrawByStatuses(ctx, []string{withdrawmodel.WithdrawStatusPending})
	if err != nil {
		return nil, err
	}
	withdrawPayingCount, withdrawPayingAmount, err := s.repo.WithdrawByStatuses(ctx, []string{
		withdrawmodel.WithdrawStatusApproved,
		withdrawmodel.WithdrawStatusPaying,
	})
	if err != nil {
		return nil, err
	}
	invoicePending, err := s.repo.InvoicePending(ctx)
	if err != nil {
		return nil, err
	}
	balanceTotal, frozenTotal, walletCount, err := s.repo.WalletTotals(ctx)
	if err != nil {
		return nil, err
	}
	threshold := s.balanceWarningThreshold(ctx)
	lowCount, lowAmount, err := s.repo.WalletBelow(ctx, threshold)
	if err != nil {
		return nil, err
	}

	income := money.Round2(txSummary.Income)
	expense := money.Round2(txSummary.Expense)
	net := money.Round2(income - expense)
	avg := 0.0
	if txSummary.Count > 0 {
		avg = money.Round2((income + expense) / float64(txSummary.Count))
	}

	billTotal := money.Round2(billRange.TotalAmount)
	billRefund := money.Round2(billRange.RefundAmount)

	return &dto.StatsResponse{
		Range: dto.StatsRange{
			StartTime:   start.Format("2006-01-02"),
			EndTime:     end.AddDate(0, 0, -1).Format("2006-01-02"),
			Granularity: granularity,
		},
		Summary: dto.StatsSummary{
			IncomeTotal:   income,
			ExpenseTotal:  expense,
			NetTotal:      net,
			TxCount:       txSummary.Count,
			AvgAmount:     avg,
			InternalCount: internalCount,
		},
		Trend:         fillTrend(trendRows, start, end, granularity),
		TypeBreakdown: toTypeBreakdown(typeRows),
		Bills: dto.StatsBillSummary{
			Count:          billRange.Count,
			TotalAmount:    billTotal,
			RefundAmount:   billRefund,
			NetAmount:      money.Round2(billTotal - billRefund),
			RechargeCount:  billRange.RechargeCount,
			RechargeAmount: money.Round2(billRange.RechargeAmount),
			ByStatus:       toBillStatus(billStatus),
			UnpaidCount:    unpaidCount,
			UnpaidAmount:   money.Round2(unpaidAmount),
		},
		Pending: dto.StatsPendingSummary{
			RechargePendingCount:  rechargePendingCount,
			RechargePendingAmount: money.Round2(rechargePendingAmount),
			WithdrawPendingCount:  withdrawPendingCount,
			WithdrawPendingAmount: money.Round2(withdrawPendingAmount),
			WithdrawPayingCount:   withdrawPayingCount,
			WithdrawPayingAmount:  money.Round2(withdrawPayingAmount),
			InvoicePendingCount:   invoicePending,
		},
		Wallet: dto.StatsWalletSummary{
			BalanceTotal:        money.Round2(balanceTotal),
			FrozenTotal:         money.Round2(frozenTotal),
			Count:               walletCount,
			LowBalanceThreshold: threshold,
			LowBalanceCount:     lowCount,
			LowBalanceAmount:    money.Round2(lowAmount),
		},
		Caliber: caliberText(clamped),
	}, nil
}

// RevenueOf 期间收入口径：一次类型分布聚合 + 一次总额聚合，取消费/退款/佣金与资金口径收入。
func (s *statsService) RevenueOf(ctx context.Context, start, end time.Time) (*dto.Revenue, error) {
	summary, err := s.repo.TxSummary(ctx, start, end)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.TxByType(ctx, start, end)
	if err != nil {
		return nil, err
	}
	var consume, refund, commission float64
	for _, row := range rows {
		switch row.Type {
		case transmodel.TxTypeConsume:
			consume += row.Expense
		case transmodel.TxTypeRefund:
			refund += row.Income
		case transmodel.TxTypeCommission:
			commission += row.Income
		}
	}
	consume = money.Round2(consume)
	refund = money.Round2(refund)
	return &dto.Revenue{
		ServiceRevenue:  money.Round2(consume - refund),
		FundIncome:      money.Round2(summary.Income),
		ConsumeTotal:    consume,
		RefundTotal:     refund,
		CommissionTotal: money.Round2(commission),
	}, nil
}

func caliberText(clamped bool) string {
	text := "汇总口径：收入/支出按资金流水方向全量聚合，剔除冻结/解冻内部划转（可用↔冻结之间划转，非真实收付）；" +
		"提现出账按结算流水计支出。账单与待办为独立口径：账单按生成时间、未结为全量。"
	if clamped {
		text += "请求区间超过 3 年，起点已自动收敛。"
	}
	return text
}

// normalizeRange 解析并归一化统计区间：缺省近 30 天（含今天），end 为闭区间次日。
func normalizeRange(startRaw, endRaw string) (start, end time.Time, clamped bool) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end = today.AddDate(0, 0, 1)
	if t, ok := parseDay(endRaw); ok {
		end = t.AddDate(0, 0, 1)
	}
	start = end.AddDate(0, 0, -defaultRangeDays)
	if t, ok := parseDay(startRaw); ok {
		start = t
	}
	if start.After(end) {
		start, end = end.AddDate(0, 0, -1), start
	}
	if end.Sub(start) > maxRangeDays*24*time.Hour {
		start = end.AddDate(0, 0, -maxRangeDays)
		clamped = true
	}
	return start, end, clamped
}

// normalizeGranularity 归一化粒度：默认按日；跨度过长时自动升为按月（响应回显实际粒度）。
func normalizeGranularity(raw string, start, end time.Time) string {
	g := strings.TrimSpace(raw)
	if g != dto.GranularityDay && g != dto.GranularityMonth {
		g = dto.GranularityDay
	}
	if g == dto.GranularityDay && end.Sub(start) > maxDayGranularityDays*24*time.Hour {
		return dto.GranularityMonth
	}
	return g
}

func parseDay(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	if len(raw) > 10 {
		raw = raw[:10]
	}
	t, err := time.ParseInLocation("2006-01-02", raw, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// fillTrend 补齐区间内无流水的周期点，保证图表轴连续。
func fillTrend(rows []repository.TrendRow, start, end time.Time, granularity string) []dto.StatsTrendPoint {
	index := make(map[string]repository.TrendRow, len(rows))
	for _, row := range rows {
		index[row.Period] = row
	}
	points := make([]dto.StatsTrendPoint, 0, len(rows)+1)
	appendPoint := func(key string) {
		row := index[key]
		income := money.Round2(row.Income)
		expense := money.Round2(row.Expense)
		points = append(points, dto.StatsTrendPoint{
			Period:  key,
			Income:  income,
			Expense: expense,
			Net:     money.Round2(income - expense),
			Count:   row.Count,
		})
	}
	if granularity == dto.GranularityMonth {
		for cursor := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location()); cursor.Before(end); cursor = cursor.AddDate(0, 1, 0) {
			appendPoint(cursor.Format("2006-01"))
		}
		return points
	}
	for cursor := start; cursor.Before(end); cursor = cursor.AddDate(0, 0, 1) {
		appendPoint(cursor.Format("2006-01-02"))
	}
	return points
}

func toTypeBreakdown(rows []repository.TxTypeRow) []dto.StatsTypeBreakdown {
	items := make([]dto.StatsTypeBreakdown, 0, len(rows))
	for _, row := range rows {
		income := money.Round2(row.Income)
		expense := money.Round2(row.Expense)
		items = append(items, dto.StatsTypeBreakdown{
			Type:     row.Type,
			Income:   income,
			Expense:  expense,
			Net:      money.Round2(income - expense),
			Count:    row.Count,
			Internal: isInternalType(row.Type),
		})
	}
	return items
}

func isInternalType(txType string) bool {
	return txType == transmodel.TxTypeFreeze || txType == transmodel.TxTypeUnfreeze
}

func toBillStatus(rows []repository.BillStatusRow) []dto.StatsBillStatusRow {
	items := make([]dto.StatsBillStatusRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.StatsBillStatusRow{
			Status: row.Status,
			Count:  row.Count,
			Amount: money.Round2(row.Amount),
		})
	}
	return items
}

// balanceWarningThreshold 余额预警阈值：读取财务配置，缺失或非法时用默认值。
func (s *statsService) balanceWarningThreshold(ctx context.Context) float64 {
	if s.configReader == nil {
		return defaultBalanceWarning
	}
	raw, ok, err := s.configReader(ctx, ConfigKeyBalanceWarning)
	if err != nil || !ok {
		return defaultBalanceWarning
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || value < 0 {
		return defaultBalanceWarning
	}
	return money.Round2(value)
}
