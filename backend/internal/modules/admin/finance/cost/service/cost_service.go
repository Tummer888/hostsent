// Package service 提供成本管理与利润核算能力。
//
// 口径（doc111 §3，页面页脚同文展示，代码与文档必须一致）：
//
//	收入  服务收入 = 期间消费 − 期间退款（营业收入）；资金口径收入仅作参考。
//	成本  ① 上游余额消耗 = 期初余额 + 期间充值 − 期末余额（余额快照推算，多退少补在充值记录里冲正）；
//	      ② 成本项配置（母机月费 / 员工工资 / 机房带宽…按月计入，一次性项计入发生月）；
//	      ③ 用户佣金入账（资金流水 type=commission 收入合计）；
//	      ④ 推广返现计提（返现台账 cashback + renewal_cashback − refund_clawback）。
//	利润  利润 = 服务收入 − 成本合计；利润率 = 利润 / 服务收入。
//	周期  自然月 [月初 00:00, 次月月初)；当月为「至今」视图，固定成本另按天摊到截至日。
package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/finance/cost/dto"
	"hostsent/backend/internal/modules/admin/finance/cost/model"
	"hostsent/backend/internal/modules/admin/finance/cost/repository"
	statsdto "hostsent/backend/internal/modules/admin/finance/stats/dto"
	"hostsent/backend/internal/pkg/money"
	"hostsent/backend/internal/pkg/upstream"
)

// 业务错误。
var (
	// ErrItemNotFound 成本项不存在。
	ErrItemNotFound = errors.New("成本项不存在")
	// ErrInvalidItem 成本项参数非法。
	ErrInvalidItem = errors.New("成本项参数非法")
	// ErrSnapshotNotFound 余额快照不存在。
	ErrSnapshotNotFound = errors.New("余额快照不存在")
	// ErrTopupNotFound 充值记录不存在。
	ErrTopupNotFound = errors.New("上游充值记录不存在")
	// ErrProviderNotFound 渠道不存在或不可用。
	ErrProviderNotFound = errors.New("渠道不存在")
	// ErrBalanceUnsupported 该渠道不支持自动查询余额（适配器未实现 AccountReader）。
	ErrBalanceUnsupported = errors.New("该渠道类型暂不支持自动查询余额，请手工录入快照")
	// ErrLedgerUnsupported 该渠道不支持读取上游账本（适配器未实现 FinanceLedgerReader）。
	ErrLedgerUnsupported = errors.New("该渠道类型暂不支持读取上游账本（消费/充值流水），请手工录入")
)

// ConfigReader 读取 system_configs 原文（键不存在返回 ok=false）。
type ConfigReader func(ctx context.Context, key string) (string, bool, error)

// ConfigKeySnapshotHour 每日自动快照执行小时（0-23，财务配置页维护）。
const ConfigKeySnapshotHour = "finance.cost_snapshot_hour"

// ConfigKeyBalanceWarning 上游余额低水位阈值（元；0=不启用固定阈值，只按到期金额判断）。
const ConfigKeyBalanceWarning = "finance.upstream_balance_warning"

const (
	defaultSnapshotHour = 3
	// ledgerPageSize 账本同步每页条数（上游实测 limit=60 稳定）。
	ledgerPageSize = 60
	// maxLedgerPages 单次同步单边最多翻页数（6000 条；超出只记警告，下次续传）。
	maxLedgerPages = 100
	// dueCacheTTL 上游到期清单缓存时长（页面刷新不重复打上游）。
	dueCacheTTL = 10 * time.Minute
	// dueWindowDays 余额水位告警的到期窗口（余额 < 未来 30 天到期金额 → critical）。
	dueWindowDays = 30
)

// RevenueReader 收入口径读取（由财务统计服务实现，避免口径漂移）。
type RevenueReader interface {
	RevenueOf(ctx context.Context, start, end time.Time) (*statsdto.Revenue, error)
}

// ReferralAccrualReader 返现计提合计（由返现仓储实现，避免成本模块直读他域台账表）。
type ReferralAccrualReader interface {
	// SumAccrual 期间计提净额 = cashback + renewal_cashback − refund_clawback。
	SumAccrual(ctx context.Context, start, end time.Time) (float64, error)
}

// UpstreamRef 需要跟踪余额的上游渠道。
type UpstreamRef struct {
	ID           uint64
	Name         string
	ProviderType string
	// BalanceSupported 该渠道适配器是否实现账户余额读取（AccountReader）。
	// 由装配层在列举时判定（纯类型断言，无副作用）；不支持时页面只提供手工录入。
	BalanceSupported bool
	// LedgerSupported 该渠道适配器是否实现上游账本读取（FinanceLedgerReader）：可同步消费/充值流水。
	LedgerSupported bool
}

// UpstreamAccountPort 上游账户余额能力（装配层注入，避免成本模块 import 资源渠道模块）。
type UpstreamAccountPort interface {
	// ListAccountProviders 需要跟踪余额的渠道（启用中的上游转售渠道，含能力标记）。
	ListAccountProviders(ctx context.Context) ([]UpstreamRef, error)
	// FetchBalance 抓取渠道账户余额（有副作用：由调用方决定是否落快照）；
	// 渠道不支持时返回 ErrBalanceUnsupported。
	FetchBalance(ctx context.Context, providerID uint64) (amount float64, currency string, err error)
}

// UpstreamLedgerPort 上游账本能力（装配层注入；成本模块不 import 渠道模块）。
//
// 与 UpstreamAccountPort（只给当前余额）互补：账本给的是可归集的流水明细，
// 月度成本因此不必再靠余额差推算（doc111 §5.2）。
type UpstreamLedgerPort interface {
	// LedgerPage 分页读取账本：kind 取 upstream.LedgerKindConsume / LedgerKindTopup。
	LedgerPage(ctx context.Context, providerID uint64, kind string, page, limit int) ([]upstream.LedgerEntry, int, error)
	// DueHosts 上游主机的续费信息（到期金额/到期日），用于余额水位告警。
	DueHosts(ctx context.Context, providerID uint64) ([]upstream.DueHost, error)
}

// CostService 成本管理能力。
type CostService interface {
	// Overview 月度成本总览（成本构成 + 上游台账 + 近 12 月趋势 + 利润/利润率）。
	Overview(ctx context.Context, month string) (*dto.CostOverviewResponse, error)

	// ListItems 成本项分页列表（含「按月计入合计」，不受分页影响）。
	ListItems(ctx context.Context, q dto.CostItemListQuery) (*dto.CostItemListResponse, error)
	CreateItem(ctx context.Context, req dto.CostItemRequest, operatorID uint64) (*dto.CostItemInfo, error)
	UpdateItem(ctx context.Context, id uint64, req dto.CostItemRequest, operatorID uint64) (*dto.CostItemInfo, error)
	DeleteItem(ctx context.Context, id uint64) error

	// Ledger 上游余额台账（各渠道期初/充值/消耗/期末 + 最近快照与充值）。
	Ledger(ctx context.Context, month string) (*dto.UpstreamLedgerResponse, error)
	SaveSnapshot(ctx context.Context, req dto.SnapshotRequest) (*dto.SnapshotInfo, error)
	DeleteSnapshot(ctx context.Context, id uint64) error
	SaveTopup(ctx context.Context, req dto.TopupRequest, operatorID uint64) (*dto.TopupInfo, error)
	DeleteTopup(ctx context.Context, id uint64) error
	// ListSnapshots/ListTopups 快照与充值记录明细（分页，用于核对与纠错）。
	ListSnapshots(ctx context.Context, q dto.BalanceRecordQuery) (*dto.SnapshotListResponse, error)
	ListTopups(ctx context.Context, q dto.BalanceRecordQuery) (*dto.TopupListResponse, error)
	// FetchBalance 抓取渠道余额并落当日快照（source=auto）。
	FetchBalance(ctx context.Context, providerID uint64) (*dto.SnapshotInfo, error)
	// SnapshotHour 每日自动快照执行小时（财务配置 finance.cost_snapshot_hour）。
	SnapshotHour(ctx context.Context) int

	// —— 上游账本（doc111 §5.2）——
	// SyncLedger 同步上游账本：full=false 增量（按上游自增 ID 断点）、true 全量回填；
	// providerID=0 表示同步全部支持账本的渠道。单个渠道失败不影响其它渠道。
	SyncLedger(ctx context.Context, providerID uint64, full bool) (*dto.LedgerSyncResponse, error)
	// ListLedgerEntries 账本流水分页（消费/充值，页面「上游流水」页签）。
	ListLedgerEntries(ctx context.Context, q dto.LedgerEntryQuery) (*dto.LedgerEntryListResponse, error)
	// BalanceAlerts 上游余额水位告警（余额 < 未来 30 天到期金额，或 < 配置阈值）。
	BalanceAlerts(ctx context.Context) ([]dto.BalanceAlert, error)
	// nowForSchedule 供调度器取「现在」（便于测试注入时间）。
	nowForSchedule() time.Time
	// RunAutoSnapshots 执行一轮自动抓取（每日定时任务调用；不支持的渠道跳过）。
	RunAutoSnapshots(ctx context.Context) (saved int, skipped int, failed int)

	SetConfigReader(reader ConfigReader)
	SetUpstreamPort(port UpstreamAccountPort)
	SetUpstreamLedgerPort(port UpstreamLedgerPort)
	SetReferralReader(reader ReferralAccrualReader)
}

type costService struct {
	repo         repository.CostRepository
	revenue      RevenueReader
	upstream     UpstreamAccountPort
	ledgerPort   UpstreamLedgerPort
	referral     ReferralAccrualReader
	configReader ConfigReader
	now          func() time.Time

	// dueMu/dueCache 上游到期清单缓存：台账页与总览页都会算告警，避免每次刷新都打上游。
	dueMu    sync.Mutex
	dueCache map[uint64]dueCacheEntry
}

// dueCacheEntry 到期清单缓存项。
type dueCacheEntry struct {
	at   time.Time
	dues []upstream.DueHost
	err  error
}

// NewCostService 创建成本管理服务。
func NewCostService(repo repository.CostRepository, revenue RevenueReader) CostService {
	return &costService{repo: repo, revenue: revenue, now: time.Now}
}

// SetConfigReader 注入配置读取器（finance.cost_snapshot_hour）。
func (s *costService) SetConfigReader(reader ConfigReader) { s.configReader = reader }

// SetUpstreamPort 注入上游账户余额端口（装配层实现）。
func (s *costService) SetUpstreamPort(port UpstreamAccountPort) { s.upstream = port }

// SetUpstreamLedgerPort 注入上游账本端口（消费/充值流水 + 到期清单）。
func (s *costService) SetUpstreamLedgerPort(port UpstreamLedgerPort) { s.ledgerPort = port }

// SetReferralReader 注入返现计提读取器（装配层接线，避免成本模块 import 返现模块）。
func (s *costService) SetReferralReader(reader ReferralAccrualReader) { s.referral = reader }

// nowForSchedule 供调度器读取当前时间（实现 CostService，保持时间来源单一）。
func (s *costService) nowForSchedule() time.Time { return s.now() }

// SnapshotHour 自动快照小时：读财务配置，缺失/非法时回默认 3 点。
func (s *costService) SnapshotHour(ctx context.Context) int {
	if s.configReader == nil {
		return defaultSnapshotHour
	}
	raw, ok, err := s.configReader(ctx, ConfigKeySnapshotHour)
	if err != nil || !ok {
		return defaultSnapshotHour
	}
	hour, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || hour < 0 || hour > 23 {
		return defaultSnapshotHour
	}
	return hour
}

// —— 月度总览 ——

func (s *costService) Overview(ctx context.Context, month string) (*dto.CostOverviewResponse, error) {
	start, end, isCurrent, asOfEnd := resolveMonth(month, s.now())
	refs, err := s.accountProviders(ctx)
	if err != nil {
		return nil, err
	}

	// 收入（当月按至今；历史月按整月）。
	revenue, err := s.revenue.RevenueOf(ctx, start, asOfEnd)
	if err != nil {
		return nil, err
	}
	referralCost := 0.0
	if s.referral != nil {
		if referralCost, err = s.referral.SumAccrual(ctx, start, asOfEnd); err != nil {
			return nil, err
		}
	}

	// 上游台账（当月 + 近 12 月趋势共用同一份历史，避免逐月重复查询）。
	histories, err := s.loadHistories(ctx, refs, trendStart(start), asOfEnd)
	if err != nil {
		return nil, err
	}
	rows, snapshotTotal := buildLedger(histories, start, end, asOfEnd)

	// 上游账本（流水口径）：一次装载窗口内全部消费条目，当月合计与逐月趋势共用。
	ledgerEntries, err := s.repo.ListLedgerInRange(ctx, 0, model.LedgerKindConsume, trendStart(start), asOfEnd)
	if err != nil {
		return nil, err
	}
	// 双口径合并：已同步账本的渠道用流水（主口径），其余用快照推算（兜底）。
	ledgerAuthority, err := s.ledgerAuthority(ctx, refs)
	if err != nil {
		return nil, err
	}
	upstreamTotal, ledgerTotal, ledgerDiff, costSource := applyLedgerToRows(rows, aggregateLedger(ledgerEntries, start, end), ledgerAuthority)

	// 成本项：整月口径 + 按天摊到截至日（当月才有意义）。
	items, err := s.repo.ListAllActiveItems(ctx)
	if err != nil {
		return nil, err
	}
	fixedCost := monthItemsTotal(items, start)
	fixedToDate := prorate(fixedCost, start, end, asOfEnd)

	costTotal := money.Round2(upstreamTotal + fixedCost + revenue.CommissionTotal + referralCost)
	costToDate := money.Round2(upstreamTotal + fixedToDate + revenue.CommissionTotal + referralCost)
	profit := money.Round2(revenue.ServiceRevenue - costTotal)
	profitToDate := money.Round2(revenue.ServiceRevenue - costToDate)

	upstreamLabel, upstreamUsage := upstreamLineCopy(costSource)
	lines := []dto.CostLine{
		{
			Key: "upstream", Label: upstreamLabel, Amount: money.Round2(upstreamTotal), Auto: true,
			Usage: upstreamUsage,
		},
		{
			Key: "fixed", Label: "成本项配置合计", Amount: money.Round2(fixedCost), Auto: false,
			Usage: "成本项配置页维护（母机月费 / 人力 / 机房带宽…），按月计入",
		},
		{
			Key: "commission", Label: "用户佣金入账", Amount: revenue.CommissionTotal, Auto: true,
			Usage: "资金流水 type=commission 的收入合计（平台付给分销商）",
		},
		{
			Key: "referral", Label: "推广返现计提", Amount: money.Round2(referralCost), Auto: true,
			Usage: "返现台账计提：cashback + renewal_cashback − refund_clawback",
		},
	}

	trend, err := s.buildTrend(ctx, histories, items, start, aggregateLedgerByMonth(ledgerEntries), ledgerAuthority)
	if err != nil {
		return nil, err
	}

	// 余额水位告警：辅助信息，取不到不影响成本出数（失败会以 level=unknown 显式返回）。
	alerts, _ := s.BalanceAlerts(ctx)
	syncedAt := ""
	if latest, lerr := s.repo.LatestLedgerSyncedAt(ctx, 0); lerr == nil && latest != nil {
		syncedAt = latest.Format(time.RFC3339)
	}

	resp := &dto.CostOverviewResponse{
		Month:            start.Format("2006-01"),
		AsOf:             asOfEnd.AddDate(0, 0, -1).Format("2006-01-02"),
		Current:          isCurrent,
		ServiceRevenue:   revenue.ServiceRevenue,
		FundIncome:       revenue.FundIncome,
		ConsumeTotal:     revenue.ConsumeTotal,
		RefundTotal:      revenue.RefundTotal,
		CostTotal:        costTotal,
		CostToDate:       costToDate,
		UpstreamCost:     money.Round2(upstreamTotal),
		FixedCost:        money.Round2(fixedCost),
		FixedCostToDate:  money.Round2(fixedToDate),
		CommissionCost:   revenue.CommissionTotal,
		ReferralCost:     money.Round2(referralCost),
		Profit:           profit,
		ProfitRate:       rate(profit, revenue.ServiceRevenue),
		ProfitToDate:     profitToDate,
		ProfitRateToDate: rate(profitToDate, revenue.ServiceRevenue),
		CostLines:        lines,
		UpstreamRows:     rows,
		Trend:            trend,
		Caliber:          caliberText(),

		UpstreamCostLedger:   money.Round2(ledgerTotal),
		UpstreamCostSnapshot: money.Round2(snapshotTotal),
		UpstreamCostSource:   costSource,
		UpstreamLedgerDiff:   money.Round2(ledgerDiff),
		LedgerSyncedAt:       syncedAt,
		BalanceAlerts:        alerts,
	}
	if len(items) == 0 {
		resp.UnconfiguredHint = "尚未配置任何成本项：母机月费、人力、机房带宽等固定成本请到「成本项配置」添加，否则成本只含上游消耗与佣金/返现自动项。"
	}
	return resp, nil
}

func caliberText() string {
	return "口径：收入=服务收入（期间消费 − 期间退款）；上游成本优先取上游账本消费流水（余额支付的开通/续费 − 对应退款，逐笔可查），" +
		"未接入账本的渠道用余额快照推算（期初 + 充值 − 期末），两者差额列在台账供核查；另含成本项配置 + 用户佣金入账 + 推广返现计提；" +
		"利润=收入−成本，利润率=利润/收入。周期按自然月；当月为「至今」视图，固定成本另给按天摊分后的利润与利润率。"
}

// —— 成本项 ——

func (s *costService) ListItems(ctx context.Context, q dto.CostItemListQuery) (*dto.CostItemListResponse, error) {
	page, pageSize := normalizePage(q.Page), normalizePageSize(q.PageSize)
	items, total, err := s.repo.ListItems(ctx, strings.TrimSpace(q.Keyword), q.Category, q.Status, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	monthStart, _ := monthRange(s.now())
	respItems := make([]dto.CostItemInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, buildItemInfo(item, monthStart))
	}
	// 合计按「当前筛选条件的全量」计算，不受分页影响（与列表口径一致）。
	all, _, err := s.repo.ListItems(ctx, strings.TrimSpace(q.Keyword), q.Category, q.Status, 0, maxItemScan)
	if err != nil {
		return nil, err
	}
	monthlyTotal := monthItemsTotal(all, monthStart)
	return &dto.CostItemListResponse{
		Items:        respItems,
		Meta:         dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
		MonthlyTotal: money.Round2(monthlyTotal),
	}, nil
}

func (s *costService) CreateItem(ctx context.Context, req dto.CostItemRequest, operatorID uint64) (*dto.CostItemInfo, error) {
	item, err := buildItemFromRequest(req)
	if err != nil {
		return nil, err
	}
	item.OperatorID = operatorID
	if item.Status == "" {
		item.Status = model.StatusActive
	}
	if err := s.repo.CreateItem(ctx, item); err != nil {
		return nil, err
	}
	now := s.now()
	info := buildItemInfo(*item, firstOfMonth(now))
	return &info, nil
}

func (s *costService) UpdateItem(ctx context.Context, id uint64, req dto.CostItemRequest, operatorID uint64) (*dto.CostItemInfo, error) {
	existing, err := s.repo.FindItem(ctx, id)
	if err != nil {
		return nil, ErrItemNotFound
	}
	updated, err := buildItemFromRequest(req)
	if err != nil {
		return nil, err
	}
	existing.Name = updated.Name
	existing.Category = updated.Category
	existing.Amount = updated.Amount
	existing.Cycle = updated.Cycle
	existing.OccurredOn = updated.OccurredOn
	existing.EffectiveFrom = updated.EffectiveFrom
	existing.EffectiveTo = updated.EffectiveTo
	existing.Subject = updated.Subject
	existing.Remark = updated.Remark
	existing.Status = updated.Status
	existing.OperatorID = operatorID
	if err := s.repo.UpdateItem(ctx, existing); err != nil {
		return nil, err
	}
	now := s.now()
	info := buildItemInfo(*existing, firstOfMonth(now))
	return &info, nil
}

func (s *costService) DeleteItem(ctx context.Context, id uint64) error {
	if _, err := s.repo.FindItem(ctx, id); err != nil {
		return ErrItemNotFound
	}
	return s.repo.DeleteItem(ctx, id)
}

// —— 上游余额台账 ——

func (s *costService) Ledger(ctx context.Context, month string) (*dto.UpstreamLedgerResponse, error) {
	start, end, _, _ := resolveMonth(month, s.now())
	refs, err := s.accountProviders(ctx)
	if err != nil {
		return nil, err
	}
	// 台账按整月展示（不受「至今」影响），当月靠 Estimated 标注截至日。
	asOfEnd := end
	histories, err := s.loadHistories(ctx, refs, start, asOfEnd)
	if err != nil {
		return nil, err
	}
	rows, _ := buildLedger(histories, start, end, asOfEnd)
	// 账本（流水口径）：页面消耗以流水为主口径，快照推算同时给出用于核对。
	ledgerEntries, err := s.repo.ListLedgerInRange(ctx, 0, model.LedgerKindConsume, start, end)
	if err != nil {
		return nil, err
	}
	ledgerAuthority, err := s.ledgerAuthority(ctx, refs)
	if err != nil {
		return nil, err
	}
	total, _, _, _ := applyLedgerToRows(rows, aggregateLedger(ledgerEntries, start, end), ledgerAuthority)

	supported := make([]uint64, 0, len(refs))
	ledgerSupported := make([]uint64, 0, len(refs))
	for _, history := range histories {
		if history.ref.BalanceSupported {
			supported = append(supported, history.ref.ID)
		}
		if history.ref.LedgerSupported {
			ledgerSupported = append(ledgerSupported, history.ref.ID)
		}
	}
	// 余额水位告警是辅助信息：取不到不影响台账出数（失败会以 level=unknown 显式返回）。
	alerts, _ := s.BalanceAlerts(ctx)
	syncedAt := ""
	if latest, lerr := s.repo.LatestLedgerSyncedAt(ctx, 0); lerr == nil && latest != nil {
		syncedAt = latest.Format(time.RFC3339)
	}
	return &dto.UpstreamLedgerResponse{
		Month:                      start.Format("2006-01"),
		Rows:                       rows,
		TotalConsumption:           money.Round2(total),
		SnapshotSupportedProviders: supported,
		LedgerSupportedProviders:   ledgerSupported,
		LedgerSyncedAt:             syncedAt,
		Alerts:                     alerts,
	}, nil
}

func (s *costService) SaveSnapshot(ctx context.Context, req dto.SnapshotRequest) (*dto.SnapshotInfo, error) {
	// 渠道必须存在（避免给不存在的 channel 记余额）。
	refs, err := s.accountProviders(ctx)
	if err != nil {
		return nil, err
	}
	ref, ok := findRef(refs, req.ProviderID)
	if !ok {
		return nil, ErrProviderNotFound
	}
	date, err := parseDate(req.SnapshotDate)
	if err != nil {
		return nil, ErrInvalidItem
	}
	snapshot := &model.UpstreamBalanceSnapshot{
		ProviderID:   req.ProviderID,
		SnapshotDate: date,
		Balance:      money.Round2(req.Balance),
		Currency:     "CNY",
		Source:       model.SnapshotSourceManual,
		Remark:       strings.TrimSpace(req.Remark),
	}
	if err := s.repo.UpsertSnapshot(ctx, snapshot); err != nil {
		return nil, err
	}
	info := buildSnapshotInfo(*snapshot, ref.Name)
	return &info, nil
}

func (s *costService) DeleteSnapshot(ctx context.Context, id uint64) error {
	if _, err := s.repo.FindSnapshot(ctx, id); err != nil {
		return ErrSnapshotNotFound
	}
	return s.repo.DeleteSnapshot(ctx, id)
}

func (s *costService) SaveTopup(ctx context.Context, req dto.TopupRequest, operatorID uint64) (*dto.TopupInfo, error) {
	refs, err := s.accountProviders(ctx)
	if err != nil {
		return nil, err
	}
	ref, ok := findRef(refs, req.ProviderID)
	if !ok {
		return nil, ErrProviderNotFound
	}
	if req.Amount == 0 {
		return nil, ErrInvalidItem
	}
	date, err := parseDate(req.OccurredOn)
	if err != nil {
		return nil, ErrInvalidItem
	}
	topup := &model.UpstreamBalanceTopup{
		ProviderID: req.ProviderID,
		OccurredOn: date,
		Amount:     money.Round2(req.Amount),
		Remark:     strings.TrimSpace(req.Remark),
		OperatorID: operatorID,
	}
	if err := s.repo.CreateTopup(ctx, topup); err != nil {
		return nil, err
	}
	info := buildTopupInfo(*topup, ref.Name)
	return &info, nil
}

func (s *costService) DeleteTopup(ctx context.Context, id uint64) error {
	if _, err := s.repo.FindTopup(ctx, id); err != nil {
		return ErrTopupNotFound
	}
	return s.repo.DeleteTopup(ctx, id)
}

func (s *costService) ListSnapshots(ctx context.Context, q dto.BalanceRecordQuery) (*dto.SnapshotListResponse, error) {
	page, pageSize := normalizePage(q.Page), normalizePageSize(q.PageSize)
	start, end := monthWindow(q.Month)
	items, total, err := s.repo.ListSnapshots(ctx, q.ProviderID, start, end, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	names := s.providerNames(ctx)
	respItems := make([]dto.SnapshotInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, buildSnapshotInfo(item, names[item.ProviderID]))
	}
	return &dto.SnapshotListResponse{
		Items: respItems,
		Meta:  dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}

func (s *costService) ListTopups(ctx context.Context, q dto.BalanceRecordQuery) (*dto.TopupListResponse, error) {
	page, pageSize := normalizePage(q.Page), normalizePageSize(q.PageSize)
	start, end := monthWindow(q.Month)
	items, total, err := s.repo.ListTopups(ctx, q.ProviderID, start, end, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	names := s.providerNames(ctx)
	respItems := make([]dto.TopupInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, buildTopupInfo(item, names[item.ProviderID]))
	}
	return &dto.TopupListResponse{
		Items: respItems,
		Meta:  dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}

// FetchBalance 抓取渠道余额并落当日快照；适配器未实现 AccountReader 时给出可读提示。
func (s *costService) FetchBalance(ctx context.Context, providerID uint64) (*dto.SnapshotInfo, error) {
	if s.upstream == nil {
		return nil, ErrBalanceUnsupported
	}
	refs, err := s.accountProviders(ctx)
	if err != nil {
		return nil, err
	}
	ref, ok := findRef(refs, providerID)
	if !ok {
		return nil, ErrProviderNotFound
	}
	balance, currency, err := s.upstream.FetchBalance(ctx, providerID)
	if err != nil {
		return nil, err
	}
	if currency == "" {
		currency = "CNY"
	}
	snapshot := &model.UpstreamBalanceSnapshot{
		ProviderID:   providerID,
		SnapshotDate: dayStart(s.now()),
		Balance:      money.Round2(balance),
		Currency:     currency,
		Source:       model.SnapshotSourceAuto,
		Remark:       "接口抓取",
	}
	if err := s.repo.UpsertSnapshot(ctx, snapshot); err != nil {
		return nil, err
	}
	info := buildSnapshotInfo(*snapshot, ref.Name)
	return &info, nil
}

// RunAutoSnapshots 每日自动快照：只抓「具备余额读取能力」的渠道（BalanceSupported），
// 其余跳过；单渠道失败只计数，不影响其它渠道。
func (s *costService) RunAutoSnapshots(ctx context.Context) (saved, skipped, failed int) {
	if s.upstream == nil {
		return 0, 0, 0
	}
	refs, err := s.upstream.ListAccountProviders(ctx)
	if err != nil {
		return 0, 0, 1
	}
	for _, ref := range refs {
		// 能力标记在列举时已判定（纯类型断言）：适配器未实现余额读取、或该渠道类型
		// 在当前版本根本没有适配器时都不必发起调用 —— 直接跳过，只把「声称支持却失败」
		// 计入 failed，避免每天的日志里堆一串明知不可为的失败。
		if !ref.BalanceSupported {
			skipped++
			continue
		}
		balance, currency, err := s.upstream.FetchBalance(ctx, ref.ID)
		if err != nil {
			if errors.Is(err, ErrBalanceUnsupported) {
				skipped++
				continue
			}
			failed++
			continue
		}
		if currency == "" {
			currency = "CNY"
		}
		snapshot := &model.UpstreamBalanceSnapshot{
			ProviderID:   ref.ID,
			SnapshotDate: dayStart(s.now()),
			Balance:      money.Round2(balance),
			Currency:     currency,
			Source:       model.SnapshotSourceAuto,
			Remark:       "每日自动抓取",
		}
		if err := s.repo.UpsertSnapshot(ctx, snapshot); err != nil {
			failed++
			continue
		}
		saved++
	}
	return saved, skipped, failed
}

// —— 内部：历史装载与月度推算 ——

type upstreamHistory struct {
	ref          UpstreamRef
	snapshots    []model.UpstreamBalanceSnapshot // 窗口内，日期升序
	beforeWindow *model.UpstreamBalanceSnapshot  // 窗口前最近一条（第一个月的期初）
	topups       []model.UpstreamBalanceTopup    // 窗口内，日期升序
}

func (s *costService) accountProviders(ctx context.Context) ([]UpstreamRef, error) {
	if s.upstream == nil {
		return nil, nil
	}
	return s.upstream.ListAccountProviders(ctx)
}

// loadHistories 一次装载窗口内全部快照与充值（月度台账与 12 月趋势共用，避免逐月查询）。
func (s *costService) loadHistories(ctx context.Context, refs []UpstreamRef, start, end time.Time) ([]upstreamHistory, error) {
	histories := make([]upstreamHistory, 0, len(refs))
	for _, ref := range refs {
		snapshots, err := s.repo.ListSnapshotsInRange(ctx, ref.ID, start, end)
		if err != nil {
			return nil, err
		}
		before, err := s.repo.LatestSnapshotBefore(ctx, ref.ID, start)
		if err != nil && !isNotFound(err) {
			return nil, err
		}
		topups, err := s.repo.ListTopupsInRange(ctx, ref.ID, start, end)
		if err != nil {
			return nil, err
		}
		histories = append(histories, upstreamHistory{
			ref:          ref,
			snapshots:    snapshots,
			beforeWindow: before,
			topups:       topups,
		})
	}
	return histories, nil
}

// ledgerOf 单渠道的单月推算：期初 = 上月最后一条；期末 = 本月最后一条。
// 本月无快照时不推算消耗（consumption=nil），由页面提示录入。
func ledgerOf(history upstreamHistory, start, end, asOfEnd time.Time) dto.UpstreamLedgerRow {
	row := dto.UpstreamLedgerRow{
		ProviderID:   history.ref.ID,
		ProviderName: history.ref.Name,
		ProviderType: history.ref.ProviderType,
		Currency:     "CNY",
	}
	// 期初：窗口内第一个月之前的快照，或窗口内该月之前的最后一条。
	var opening *model.UpstreamBalanceSnapshot
	if history.beforeWindow != nil {
		opening = history.beforeWindow
	}
	for i := range history.snapshots {
		snap := history.snapshots[i]
		if snap.SnapshotDate.Before(start) {
			opening = &history.snapshots[i]
		}
	}
	// 期末：本月内最后一条（升序装载，取最后一个落在月内的）。
	var closing *model.UpstreamBalanceSnapshot
	for i := range history.snapshots {
		snap := history.snapshots[i]
		if !snap.SnapshotDate.Before(start) && snap.SnapshotDate.Before(end) {
			closing = &history.snapshots[i]
		}
	}
	// 最新一条（不限月份）：窗口内最后一条快照。
	if len(history.snapshots) > 0 {
		latest := history.snapshots[len(history.snapshots)-1]
		value := latest.Balance
		row.LatestBalance = &value
		row.LatestDate = latest.SnapshotDate.Format("2006-01-02")
		row.Currency = latest.Currency
	}
	if opening != nil {
		value := opening.Balance
		row.OpeningBalance = &value
		row.OpeningDate = opening.SnapshotDate.Format("2006-01-02")
	}
	// 充值合计：本月内。
	topupTotal := 0.0
	for _, topup := range history.topups {
		if !topup.OccurredOn.Before(start) && topup.OccurredOn.Before(end) {
			topupTotal += topup.Amount
		}
	}
	row.TopupTotal = money.Round2(topupTotal)
	if closing != nil {
		value := closing.Balance
		row.ClosingBalance = &value
		row.ClosingDate = closing.SnapshotDate.Format("2006-01-02")
		row.Currency = closing.Currency
		// 快照日期早于截至日 → 消耗是「截至该日」的估算值。
		if closing.SnapshotDate.Before(asOfEnd.AddDate(0, 0, -1)) {
			row.Estimated = true
		}
		if opening != nil {
			consumption := money.Round2(opening.Balance + topupTotal - closing.Balance)
			row.Consumption = &consumption
		}
	} else {
		row.MissingSnapshot = true
		row.Estimated = true
	}
	return row
}

func buildLedger(histories []upstreamHistory, start, end, asOfEnd time.Time) ([]dto.UpstreamLedgerRow, float64) {
	rows := make([]dto.UpstreamLedgerRow, 0, len(histories))
	total := 0.0
	for _, history := range histories {
		row := ledgerOf(history, start, end, asOfEnd)
		if row.Consumption != nil {
			total += *row.Consumption
		}
		rows = append(rows, row)
	}
	return rows, money.Round2(total)
}

// buildTrend 近 12 月趋势：收入来自统计服务，上游成本按「流水优先、快照兜底」逐月取数
// （与总览同一规则，见 applyLedgerToRows）。
// 收入窗口：历史月按整月，仅「当前自然月」截到今天（否则历史月会把今天的收入算进去）。
func (s *costService) buildTrend(ctx context.Context, histories []upstreamHistory, items []model.CostItem, endMonthStart time.Time, monthlyLedger map[uint64]map[string]ledgerAgg, ledgerAuthority map[uint64]bool) ([]dto.CostTrendPoint, error) {
	now := s.now()
	currentMonthStart, _ := monthRange(now)
	points := make([]dto.CostTrendPoint, 0, trendMonths)
	for i := trendMonths - 1; i >= 0; i-- {
		monthStart := endMonthStart.AddDate(0, -i, 0)
		monthEnd := monthStart.AddDate(0, 1, 0)
		revenueEnd := monthEnd
		if monthStart.Equal(currentMonthStart) {
			revenueEnd = dayStart(now).AddDate(0, 0, 1)
		}
		revenue, err := s.revenue.RevenueOf(ctx, monthStart, revenueEnd)
		if err != nil {
			return nil, err
		}
		referralCost := 0.0
		if s.referral != nil {
			if referralCost, err = s.referral.SumAccrual(ctx, monthStart, revenueEnd); err != nil {
				return nil, err
			}
		}
		monthRows, _ := buildLedger(histories, monthStart, monthEnd, monthEnd)
		monthKey := monthStart.Format("2006-01")
		monthAgg := make(map[uint64]ledgerAgg, len(histories))
		for _, history := range histories {
			if agg, ok := monthlyLedger[history.ref.ID][monthKey]; ok {
				monthAgg[history.ref.ID] = agg
			}
		}
		upstreamTotal, _, _, _ := applyLedgerToRows(monthRows, monthAgg, ledgerAuthority)
		fixed := monthItemsTotal(items, monthStart)
		costTotal := money.Round2(upstreamTotal + fixed + revenue.CommissionTotal + referralCost)
		profit := money.Round2(revenue.ServiceRevenue - costTotal)
		points = append(points, dto.CostTrendPoint{
			Month:          monthStart.Format("2006-01"),
			ServiceRevenue: revenue.ServiceRevenue,
			CostTotal:      costTotal,
			UpstreamCost:   money.Round2(upstreamTotal),
			FixedCost:      money.Round2(fixed),
			OtherCost:      money.Round2(revenue.CommissionTotal + referralCost),
			Profit:         profit,
			ProfitRate:     rate(profit, revenue.ServiceRevenue),
		})
	}
	return points, nil
}

// —— 上游账本（doc111 §5.2）——

// SyncLedger 同步上游账本：providerID=0 表示全部支持账本的渠道。
func (s *costService) SyncLedger(ctx context.Context, providerID uint64, full bool) (*dto.LedgerSyncResponse, error) {
	if s.ledgerPort == nil {
		return nil, ErrLedgerUnsupported
	}
	refs, err := s.accountProviders(ctx)
	if err != nil {
		return nil, err
	}
	targets := make([]UpstreamRef, 0, len(refs))
	for _, ref := range refs {
		if providerID > 0 && ref.ID != providerID {
			continue
		}
		if !ref.LedgerSupported {
			continue
		}
		targets = append(targets, ref)
	}
	if len(targets) == 0 {
		// 指定了渠道却没有账本能力 → 明确报「不支持」，而不是返回空结果让人以为同步成功。
		if providerID > 0 {
			return nil, ErrLedgerUnsupported
		}
		return &dto.LedgerSyncResponse{Results: []dto.LedgerSyncProviderResult{}}, nil
	}
	resp := &dto.LedgerSyncResponse{Results: make([]dto.LedgerSyncProviderResult, 0, len(targets))}
	for _, ref := range targets {
		resp.Results = append(resp.Results, s.syncProviderLedger(ctx, ref, full))
	}
	return resp, nil
}

// syncProviderLedger 单渠道同步：消费与充值各翻页一次；单边失败记录原因，不影响另一边。
func (s *costService) syncProviderLedger(ctx context.Context, ref UpstreamRef, full bool) dto.LedgerSyncProviderResult {
	result := dto.LedgerSyncProviderResult{ProviderID: ref.ID, ProviderName: ref.Name}
	sides := []struct {
		kind     string
		written  *int
		upstream *int
	}{
		{model.LedgerKindConsume, &result.Consumption, &result.ConsumptionTotal},
		{model.LedgerKindTopup, &result.Topup, &result.TopupTotal},
	}
	for _, side := range sides {
		written, total, err := s.syncLedgerKind(ctx, ref.ID, side.kind, full)
		if err != nil {
			result.Failed = err.Error()
			return result
		}
		*side.written, *side.upstream = written, total
	}
	return result
}

// syncLedgerKind 单渠道单边同步：逐页 upsert；增量模式遇到已同步过的上游 ID 即停
// （上游按 ID 倒序返回，「追平」= 页面里出现 <= 本地最大 ID 的记录）。
func (s *costService) syncLedgerKind(ctx context.Context, providerID uint64, kind string, full bool) (written, upstreamTotal int, err error) {
	minID := int64(0)
	if !full {
		if minID, err = s.repo.MaxLedgerExternalID(ctx, providerID, kind); err != nil {
			return 0, 0, err
		}
	}
	for page := 1; page <= maxLedgerPages; page++ {
		entries, total, err := s.ledgerPort.LedgerPage(ctx, providerID, kind, page, ledgerPageSize)
		if err != nil {
			return written, upstreamTotal, err
		}
		upstreamTotal = total
		if len(entries) == 0 {
			break
		}
		batch := make([]model.UpstreamLedgerEntry, 0, len(entries))
		reachedSynced := false
		for _, entry := range entries {
			if id, perr := strconv.ParseInt(entry.ExternalID, 10, 64); perr == nil && !full && id > 0 && id <= minID {
				reachedSynced = true
				continue
			}
			batch = append(batch, model.UpstreamLedgerEntry{
				ProviderID:   providerID,
				Kind:         kind,
				ExternalID:   entry.ExternalID,
				OccurredAt:   entry.OccurredAt,
				Amount:       money.Round2(entry.Amount),
				RefundAmount: money.Round2(entry.RefundAmount),
				Category:     truncateRunes(entry.Category, 60),
				RefNo:        truncateRunes(entry.RefNo, 60),
				Description:  truncateRunes(entry.Description, 240),
				Currency:     "CNY",
			})
		}
		if len(batch) > 0 {
			n, uerr := s.repo.UpsertLedgerEntries(ctx, batch)
			if uerr != nil {
				return written, upstreamTotal, uerr
			}
			written += n
		}
		if reachedSynced || len(entries) < ledgerPageSize {
			break
		}
	}
	return written, upstreamTotal, nil
}

// ListLedgerEntries 账本流水分页（消费/充值）。
func (s *costService) ListLedgerEntries(ctx context.Context, q dto.LedgerEntryQuery) (*dto.LedgerEntryListResponse, error) {
	page, pageSize := normalizePage(q.Page), normalizePageSize(q.PageSize)
	start, end := monthWindow(q.Month)
	kind := strings.TrimSpace(q.Kind)
	if kind != model.LedgerKindTopup {
		kind = model.LedgerKindConsume
	}
	items, total, err := s.repo.ListLedger(ctx, q.ProviderID, kind, start, end, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	names := s.providerNames(ctx)
	respItems := make([]dto.LedgerEntryInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, dto.LedgerEntryInfo{
			ID:           item.ID,
			ProviderID:   item.ProviderID,
			ProviderName: names[item.ProviderID],
			Kind:         item.Kind,
			OccurredAt:   item.OccurredAt.Format(time.RFC3339),
			Amount:       item.Amount,
			RefundAmount: item.RefundAmount,
			NetAmount:    money.Round2(item.Amount - item.RefundAmount),
			Category:     item.Category,
			RefNo:        item.RefNo,
			Description:  item.Description,
			Currency:     item.Currency,
		})
	}
	// 合计与列表同一筛选口径（渠道 + 月份），不随分页变化。
	net, err := s.repo.SumLedgerAmount(ctx, q.ProviderID, model.LedgerKindConsume, start, end)
	if err != nil {
		return nil, err
	}
	topup, err := s.repo.SumLedgerAmount(ctx, q.ProviderID, model.LedgerKindTopup, start, end)
	if err != nil {
		return nil, err
	}
	return &dto.LedgerEntryListResponse{
		Items:            respItems,
		Meta:             dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
		ConsumptionTotal: money.Round2(net),
		TopupTotal:       money.Round2(topup),
	}, nil
}

// BalanceAlerts 上游余额水位告警：余额 < 未来 30 天到期金额（critical）或 < 配置阈值（warning）。
// 没有余额来源（未实现 AccountReader）的渠道不参与；上游取数失败时显式返回 level=unknown，
// 而不是静默当成「余额充足」。
func (s *costService) BalanceAlerts(ctx context.Context) ([]dto.BalanceAlert, error) {
	refs, err := s.accountProviders(ctx)
	if err != nil {
		return nil, err
	}
	threshold := s.balanceWarningThreshold(ctx)
	alerts := make([]dto.BalanceAlert, 0, len(refs))
	for _, ref := range refs {
		if !ref.BalanceSupported {
			continue
		}
		snap, serr := s.repo.LatestSnapshot(ctx, ref.ID)
		if serr != nil {
			if isNotFound(serr) {
				continue // 还没有任何余额快照：页面以「缺快照」引导录入，不重复告警
			}
			return nil, serr
		}
		alert := dto.BalanceAlert{
			ProviderID:   ref.ID,
			ProviderName: ref.Name,
			Balance:      money.Round2(snap.Balance),
			Currency:     snap.Currency,
			Threshold:    threshold,
		}
		dues, derr := s.dueHosts(ctx, ref.ID)
		if derr != nil {
			alert.Level = "unknown"
			alert.Message = fmt.Sprintf("到期金额获取失败：%v（当前余额 %.2f）", derr, snap.Balance)
			alerts = append(alerts, alert)
			continue
		}
		due, count := duesWithin(dues, s.now(), dueWindowDays)
		alert.DueWithin30d = money.Round2(due)
		alert.DueCount = count
		switch {
		case due > 0 && snap.Balance < due:
			alert.Level = "critical"
			alert.Message = fmt.Sprintf("余额 %.2f 不足以支付未来 %d 天内的续费 %.2f（%d 台主机，含已到期未付），请尽快充值",
				snap.Balance, dueWindowDays, due, count)
		case threshold > 0 && snap.Balance < threshold:
			alert.Level = "warning"
			alert.Message = fmt.Sprintf("余额 %.2f 低于低水位阈值 %.2f，建议充值", snap.Balance, threshold)
		default:
			alert.Level = "ok"
			alert.Message = fmt.Sprintf("余额 %.2f，未来 %d 天内续费 %.2f（%d 台主机）", snap.Balance, dueWindowDays, due, count)
		}
		alerts = append(alerts, alert)
	}
	return alerts, nil
}

// balanceWarningThreshold 低水位阈值（财务配置 finance.upstream_balance_warning；缺失/非法=0 不启用）。
func (s *costService) balanceWarningThreshold(ctx context.Context) float64 {
	if s.configReader == nil {
		return 0
	}
	raw, ok, err := s.configReader(ctx, ConfigKeyBalanceWarning)
	if err != nil || !ok {
		return 0
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || value < 0 {
		return 0
	}
	return value
}

// dueHosts 上游到期清单（10 分钟缓存：台账页与总览页都会算告警，避免反复打上游）。
func (s *costService) dueHosts(ctx context.Context, providerID uint64) ([]upstream.DueHost, error) {
	if s.ledgerPort == nil {
		return nil, ErrLedgerUnsupported
	}
	s.dueMu.Lock()
	if s.dueCache == nil {
		s.dueCache = make(map[uint64]dueCacheEntry)
	}
	if cached, found := s.dueCache[providerID]; found && s.now().Sub(cached.at) < dueCacheTTL {
		s.dueMu.Unlock()
		return cached.dues, cached.err
	}
	s.dueMu.Unlock()

	dues, err := s.ledgerPort.DueHosts(ctx, providerID)

	s.dueMu.Lock()
	s.dueCache[providerID] = dueCacheEntry{at: s.now(), dues: dues, err: err}
	s.dueMu.Unlock()
	return dues, err
}

// —— 双口径工具 ——

// ledgerAuthority 该渠道的账本是否可作准：适配器支持账本 **且** 已同步过至少一条记录。
// 只有「已同步过」才敢把「本月无流水」当成「本月零成本」；否则退回快照推算。
func (s *costService) ledgerAuthority(ctx context.Context, refs []UpstreamRef) (map[uint64]bool, error) {
	out := make(map[uint64]bool, len(refs))
	for _, ref := range refs {
		if !ref.LedgerSupported {
			continue
		}
		maxID, err := s.repo.MaxLedgerExternalID(ctx, ref.ID, model.LedgerKindConsume)
		if err != nil {
			return nil, err
		}
		if maxID > 0 {
			out[ref.ID] = true
		}
	}
	return out, nil
}

// ledgerAgg 账本聚合（渠道维度）。
type ledgerAgg struct {
	Sum   float64
	Count int
}

// aggregateLedger 区间内按渠道聚合消费净额。
func aggregateLedger(entries []model.UpstreamLedgerEntry, start, end time.Time) map[uint64]ledgerAgg {
	out := make(map[uint64]ledgerAgg, 8)
	for _, entry := range entries {
		if entry.OccurredAt.Before(start) || !entry.OccurredAt.Before(end) {
			continue
		}
		agg := out[entry.ProviderID]
		agg.Sum += entry.Amount - entry.RefundAmount
		agg.Count++
		out[entry.ProviderID] = agg
	}
	return out
}

// aggregateLedgerByMonth 按「渠道 × 自然月」聚合（趋势用）。
func aggregateLedgerByMonth(entries []model.UpstreamLedgerEntry) map[uint64]map[string]ledgerAgg {
	out := make(map[uint64]map[string]ledgerAgg, 8)
	for _, entry := range entries {
		month := entry.OccurredAt.Format("2006-01")
		if out[entry.ProviderID] == nil {
			out[entry.ProviderID] = make(map[string]ledgerAgg, trendMonths)
		}
		agg := out[entry.ProviderID][month]
		agg.Sum += entry.Amount - entry.RefundAmount
		agg.Count++
		out[entry.ProviderID][month] = agg
	}
	return out
}

// applyLedgerToRows 把账本口径落到台账行并算出上游成本合计（页面与文档同文规则）：
//
//	① 渠道已同步过账本（authoritative）→ 本月成本一律以流水为准：有流水取流水净额，
//	   没有条目就是 0——这是「本月确实没消费」的真相，不能退回快照推算
//	   （否则漏记充值的月份会被推算成负数成本）；
//	② 渠道未接入/尚未同步账本 → 用快照推算兜底（期初 + 充值 − 期末）；
//	③ 两者都没有 → none（页面提示「缺快照/待同步」）。
//
// 返回值：cost=合并成本；ledgerTotal=流水合计；diff=流水−快照（只对「两种口径都有」的渠道累计，才可比）；
// source=ledger / snapshot / mixed / none（页面据此标注口径来源）。
func applyLedgerToRows(rows []dto.UpstreamLedgerRow, ledger map[uint64]ledgerAgg, authoritative map[uint64]bool) (cost, ledgerTotal, diff float64, source string) {
	ledgerOnly, snapshotOnly := 0, 0
	for i := range rows {
		if authoritative[rows[i].ProviderID] {
			agg := ledger[rows[i].ProviderID] // 本月没有条目时为零值：流水口径下的 0
			sum := money.Round2(agg.Sum)
			rows[i].LedgerConsumption = &sum
			rows[i].LedgerEntries = agg.Count
			rows[i].CostSource = "ledger"
			cost += sum
			ledgerTotal += sum
			ledgerOnly++
			if rows[i].Consumption != nil {
				diff += sum - *rows[i].Consumption
			}
			continue
		}
		if rows[i].Consumption != nil {
			rows[i].CostSource = "snapshot"
			cost += *rows[i].Consumption
			snapshotOnly++
			continue
		}
		rows[i].CostSource = "none"
	}
	switch {
	case ledgerOnly > 0 && snapshotOnly > 0:
		source = "mixed"
	case ledgerOnly > 0:
		source = "ledger"
	case snapshotOnly > 0:
		source = "snapshot"
	default:
		source = "none"
	}
	return money.Round2(cost), money.Round2(ledgerTotal), money.Round2(diff), source
}

// upstreamLineCopy 上游成本行的标签与来源说明（口径变化时页面与文档一致）。
func upstreamLineCopy(source string) (label, usage string) {
	switch source {
	case "ledger":
		return "上游余额消耗（流水）",
			"上游账本消费流水净额（余额支付的开通/续费 − 对应退款），可在「上游余额台账 → 上游流水」逐笔核对"
	case "mixed":
		return "上游余额消耗（流水 + 快照推算）",
			"有账本的渠道取流水净额，其余渠道取快照推算（期初 + 充值 − 期末）；两类渠道见台账「取数来源」列"
	case "snapshot":
		return "上游余额消耗（快照推算）",
			"各渠道余额快照推算：期初 + 期间充值 − 期末（无本月快照的渠道不计入）；该渠道未接入上游账本"
	default:
		return "上游余额消耗",
			"暂无数据：既没有上游账本流水，也没有可推算的余额快照"
	}
}

// duesWithin 未来 days 天内到期的金额合计与主机台数。
func duesWithin(dues []upstream.DueHost, now time.Time, days int) (float64, int) {
	limit := now.AddDate(0, 0, days)
	total, count := 0.0, 0
	for _, due := range dues {
		if !due.NextDueAt.Before(limit) {
			continue
		}
		total += due.Amount
		count++
	}
	return total, count
}

// truncateRunes 按字符截断（上游描述可能带 HTML/超长文本，列宽有限）。
func truncateRunes(s string, max int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max])
}

// —— 工具 ——

const (
	trendMonths = 12
	// maxItemScan 列表合计的扫描上限：成本项是人工维护的配置数据（量级在几十条），
	// 用它一次算全量合计，避免再写一个 SUM 查询。
	maxItemScan = 10000
)

// monthItemsTotal 某成本项对指定自然月的计入金额。
func monthItemsTotal(items []model.CostItem, monthStart time.Time) float64 {
	monthEnd := monthStart.AddDate(0, 1, 0)
	lastDay := monthEnd.AddDate(0, 0, -1)
	total := 0.0
	for _, item := range items {
		if item.Status != model.StatusActive {
			continue
		}
		if item.Cycle == model.CycleOnce {
			if item.OccurredOn != nil && !item.OccurredOn.Before(monthStart) && item.OccurredOn.Before(monthEnd) {
				total += item.Amount
			}
			continue
		}
		// 按月项：生效区间与本月相交即整月计入（月中开始/结束不按天折，口径简单可解释）。
		if item.EffectiveFrom.After(lastDay) {
			continue
		}
		if item.EffectiveTo != nil && item.EffectiveTo.Before(monthStart) {
			continue
		}
		total += item.Amount
	}
	return money.Round2(total)
}

// prorate 按月成本按天摊到截至日：固定成本 × 已过天数 / 当月天数（历史月为整月）。
func prorate(fixed float64, monthStart, monthEnd, asOfEnd time.Time) float64 {
	totalDays := int(monthEnd.Sub(monthStart).Hours() / 24)
	elapsed := int(asOfEnd.Sub(monthStart).Hours() / 24)
	if totalDays <= 0 {
		return fixed
	}
	if elapsed > totalDays {
		elapsed = totalDays
	}
	if elapsed < 0 {
		elapsed = 0
	}
	return money.Round2(fixed * float64(elapsed) / float64(totalDays))
}

// rate 利润率（百分比，保留 2 位）；收入为 0 时返回 0（页面同时标注「无收入」）。
func rate(profit, revenue float64) float64 {
	if revenue == 0 {
		return 0
	}
	return math.Round(profit/revenue*10000) / 100
}

// resolveMonth 解析月份参数，返回 [start, end) 与「是否当月」，asOfEnd 为收入/计提的截止（右开）。
func resolveMonth(month string, now time.Time) (start, end time.Time, isCurrent bool, asOfEnd time.Time) {
	monthStart, monthEnd := monthRange(now)
	if parsed, ok := parseMonth(strings.TrimSpace(month)); ok {
		monthStart = parsed
		monthEnd = parsed.AddDate(0, 1, 0)
	}
	isCurrent = monthStart.Equal(monthRangeStart(now))
	if isCurrent {
		return monthStart, monthEnd, true, dayStart(now).AddDate(0, 0, 1)
	}
	return monthStart, monthEnd, false, monthEnd
}

// trendStart 趋势窗口起点 = 结束月往前 11 个月的月初。
func trendStart(endMonthStart time.Time) time.Time {
	return endMonthStart.AddDate(0, -(trendMonths - 1), 0)
}

func buildItemFromRequest(req dto.CostItemRequest) (*model.CostItem, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrInvalidItem
	}
	category := strings.TrimSpace(req.Category)
	if _, ok := model.CategoryLabels[category]; !ok {
		return nil, ErrInvalidItem
	}
	cycle := strings.TrimSpace(req.Cycle)
	if cycle == "" {
		cycle = model.CycleMonthly
	}
	if cycle != model.CycleMonthly && cycle != model.CycleOnce {
		return nil, ErrInvalidItem
	}
	if req.Amount < 0 {
		return nil, ErrInvalidItem
	}
	status := strings.TrimSpace(req.Status)
	if status != "" && status != model.StatusActive && status != model.StatusDisabled {
		return nil, ErrInvalidItem
	}
	item := &model.CostItem{
		Name:     name,
		Category: category,
		Amount:   money.Round2(req.Amount),
		Cycle:    cycle,
		Subject:  strings.TrimSpace(req.Subject),
		Remark:   strings.TrimSpace(req.Remark),
		Status:   status,
	}
	if item.Status == "" {
		item.Status = model.StatusActive
	}
	if cycle == model.CycleOnce {
		date, err := parseDate(req.OccurredOn)
		if err != nil {
			return nil, ErrInvalidItem
		}
		item.OccurredOn = &date
		item.EffectiveFrom = date
	} else {
		from, err := parseDate(req.EffectiveFrom)
		if err != nil {
			// 未填生效起始日：默认本月 1 日（当月即开始计入，符合直觉）。
			from = firstOfMonth(time.Now())
		}
		item.EffectiveFrom = from
		if strings.TrimSpace(req.EffectiveTo) != "" {
			to, err := parseDate(req.EffectiveTo)
			if err != nil {
				return nil, ErrInvalidItem
			}
			if to.Before(from) {
				return nil, ErrInvalidItem
			}
			item.EffectiveTo = &to
		}
	}
	return item, nil
}

func buildItemInfo(item model.CostItem, monthStart time.Time) dto.CostItemInfo {
	info := dto.CostItemInfo{
		ID:            item.ID,
		Name:          item.Name,
		Category:      item.Category,
		CategoryLabel: model.CategoryLabels[item.Category],
		Amount:        item.Amount,
		Cycle:         item.Cycle,
		Subject:       item.Subject,
		Remark:        item.Remark,
		Status:        item.Status,
		OperatorID:    item.OperatorID,
		EffectiveFrom: item.EffectiveFrom.Format("2006-01-02"),
		CreatedAt:     item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     item.UpdatedAt.Format(time.RFC3339),
		MonthlyAmount: monthItemsTotal([]model.CostItem{item}, monthStart),
	}
	if item.OccurredOn != nil {
		info.OccurredOn = item.OccurredOn.Format("2006-01-02")
	}
	if item.EffectiveTo != nil {
		info.EffectiveTo = item.EffectiveTo.Format("2006-01-02")
	}
	return info
}

func buildSnapshotInfo(item model.UpstreamBalanceSnapshot, providerName string) dto.SnapshotInfo {
	return dto.SnapshotInfo{
		ID:           item.ID,
		ProviderID:   item.ProviderID,
		ProviderName: providerName,
		SnapshotDate: item.SnapshotDate.Format("2006-01-02"),
		Balance:      item.Balance,
		Currency:     item.Currency,
		Source:       item.Source,
		Remark:       item.Remark,
		CreatedAt:    item.CreatedAt.Format(time.RFC3339),
	}
}

func buildTopupInfo(item model.UpstreamBalanceTopup, providerName string) dto.TopupInfo {
	return dto.TopupInfo{
		ID:           item.ID,
		ProviderID:   item.ProviderID,
		ProviderName: providerName,
		OccurredOn:   item.OccurredOn.Format("2006-01-02"),
		Amount:       item.Amount,
		Remark:       item.Remark,
		OperatorID:   item.OperatorID,
		CreatedAt:    item.CreatedAt.Format(time.RFC3339),
	}
}

// providerNames 渠道 ID→名称（列表展示用；渠道列举失败时返回 nil，页面退化为只显示 ID）。
func (s *costService) providerNames(ctx context.Context) map[uint64]string {
	refs, err := s.accountProviders(ctx)
	if err != nil {
		return nil
	}
	names := make(map[uint64]string, len(refs))
	for _, ref := range refs {
		names[ref.ID] = ref.Name
	}
	return names
}

func findRef(refs []UpstreamRef, id uint64) (UpstreamRef, bool) {
	for _, ref := range refs {
		if ref.ID == id {
			return ref, true
		}
	}
	return UpstreamRef{}, false
}

func parseDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, ErrInvalidItem
	}
	if len(raw) > 10 {
		raw = raw[:10]
	}
	return time.ParseInLocation("2006-01-02", raw, time.Local)
}

func parseMonth(raw string) (time.Time, bool) {
	if raw == "" {
		return time.Time{}, false
	}
	if len(raw) > 7 {
		raw = raw[:7]
	}
	parsed, err := time.ParseInLocation("2006-01", raw, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

// monthWindow 解析月份筛选：空串返回零值时间（不限时间），非法值同样退化为不限。
func monthWindow(raw string) (time.Time, time.Time) {
	parsed, ok := parseMonth(strings.TrimSpace(raw))
	if !ok {
		return time.Time{}, time.Time{}
	}
	return parsed, parsed.AddDate(0, 1, 0)
}

func monthRange(now time.Time) (time.Time, time.Time) {
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	return start, start.AddDate(0, 1, 0)
}

func firstOfMonth(now time.Time) time.Time { return monthRangeStart(now) }

func monthRangeStart(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
}

func dayStart(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

func normalizePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func normalizePageSize(size int) int {
	if size <= 0 {
		return 10
	}
	if size > 100 {
		return 100
	}
	return size
}

func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
