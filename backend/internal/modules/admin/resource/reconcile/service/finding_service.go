// Package service 提供对账差异扫描：把「当前时刻的异常行」落成台账，并按
// 「新开 / 等级升级」触发一次预警通知。
//
// 为什么通知要有节流：扫描是周期性的（默认 10 分钟），一条长期存在的差异如果
// 每轮都发，管理员的通知中心会被同一条刷屏，最后谁也看不见。策略是——
//   - 新出现的差异：立即发一次；
//   - 等级升级（warning → danger）：再发一次；
//   - 持续存在但未升级：只刷新 last_seen_at，不再发（页面上的"已持续 N 天"承担提醒）；
//   - 被 resolve 后复现：视为新开，重新发一次。
//
// 这套规则由仓储的 UpsertFindings 判定「哪些算新开」，通知层无状态。
package service

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	reconmodel "hostsent/backend/internal/modules/admin/resource/reconcile/model"
	"hostsent/backend/internal/modules/admin/resource/reconcile/repository"
)

// FindingNotifier 预警通知能力（窄接口，由装配层桥接到通知中心）。
type FindingNotifier interface {
	NotifyReconcileFinding(ctx context.Context, finding *reconmodel.ReconcileFinding, label repository.InstanceLabel, seq int) error
}

// ScanResult 一轮扫描的结果汇总（日志与测试断言用）。
type ScanResult struct {
	Scanned  int // 本次扫到的异常实例数
	Findings int // 本次命中的差异条数
	Opened   int // 新开/升级的差异条数（= 发出预警的条数）
	Resolved int64
}

// FindingScanner 差异扫描服务。
type FindingScanner interface {
	Scan(ctx context.Context) (*ScanResult, error)
	// ListOpen 供控制台「差异台账」页读取。
	ListOpen(ctx context.Context, providerID uint64, page, pageSize int) ([]reconmodel.ReconcileFinding, int64, error)
	// UpdateStatus 处置一条差异。
	UpdateStatus(ctx context.Context, id uint64, status string, adminID uint64, note string) error
	// FindingsForInstances 实例详情用：取该实例的全部差异行。
	FindingsForInstances(ctx context.Context, instanceIDs []uint64) (map[uint64][]reconmodel.ReconcileFinding, error)
	// SetNotifier 延迟注入通知能力（通知服务在装配期晚于本服务创建）。
	SetNotifier(n FindingNotifier)
}

type findingScanner struct {
	repo     repository.Repository
	findings repository.FindingRepository
	notifier FindingNotifier
	logger   *zap.Logger
}

// NewFindingScanner 创建差异扫描服务。
func NewFindingScanner(repo repository.Repository, findings repository.FindingRepository, logger *zap.Logger) FindingScanner {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &findingScanner{repo: repo, findings: findings, logger: logger}
}

// SetNotifier 延迟注入通知能力。
func (s *findingScanner) SetNotifier(n FindingNotifier) {
	if n != nil {
		s.notifier = n
	}
}

// Scan 跑一轮：取全量异常行 → 转成差异项 → upsert → 收敛消失项 → 发预警。
func (s *findingScanner) Scan(ctx context.Context) (*ScanResult, error) {
	rows, err := s.findings.ListAnomalies(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]reconmodel.ReconcileFinding, 0, len(rows)*2)
	seenKeys := make([]string, 0, len(rows)*2)
	for i := range rows {
		for _, f := range findingsFromRow(rows[i]) {
			items = append(items, f)
			seenKeys = append(seenKeys, fmt.Sprintf("%d:%s", f.InstanceID, f.Kind))
		}
	}

	opened, err := s.findings.UpsertFindings(ctx, items)
	if err != nil {
		return nil, err
	}
	resolved, err := s.findings.ResolveAbsent(ctx, seenKeys)
	if err != nil {
		return nil, err
	}

	result := &ScanResult{Scanned: len(rows), Findings: len(items), Opened: len(opened), Resolved: resolved}
	if len(opened) == 0 {
		return result, nil
	}
	s.publish(ctx, opened)
	return result, nil
}

// publish 为每条新开差异发一次预警；逐条容错（一条发失败不影响其余与扫描本身）。
func (s *findingScanner) publish(ctx context.Context, opened []reconmodel.ReconcileFinding) {
	if s.notifier == nil {
		return
	}
	ids := make([]uint64, 0, len(opened))
	seen := map[uint64]struct{}{}
	for i := range opened {
		id := opened[i].InstanceID
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	labels, err := s.findings.InstanceLabels(ctx, ids)
	if err != nil {
		s.logger.Warn("差异预警：取实例标签失败", zap.Error(err))
		labels = map[uint64]repository.InstanceLabel{}
	}
	now := time.Now()
	for i := range opened {
		f := opened[i]
		// 序号从 1 起：0 表示"从未通知过"，配进 source_id 里保证重开能再提醒一次。
		seq := f.NoticeSeq + 1
		if err := s.notifier.NotifyReconcileFinding(ctx, &f, labels[f.InstanceID], seq); err != nil {
			s.logger.Warn("差异预警：发布通知失败",
				zap.Uint64("instance_id", f.InstanceID), zap.String("kind", f.Kind), zap.Error(err))
			continue
		}
		if f.ID > 0 {
			if err := s.findings.MarkNotified(ctx, f.ID, seq, now); err != nil {
				s.logger.Warn("差异预警：记录已通知失败", zap.Uint64("id", f.ID), zap.Error(err))
			}
		}
	}
}

// findingsFromRow 把一行对账结果展开成若干条差异。
//
// 一行可能同时命中价格 + 到期 + 状态三类异常，每类各自成一条台账——
// 分开展示才能回答"定价问题处理完了没有、账期问题还在不在"。
func findingsFromRow(row repository.ReconcileRow) []reconmodel.ReconcileFinding {
	out := make([]reconmodel.ReconcileFinding, 0, 3)
	base := reconmodel.ReconcileFinding{
		ProviderID: row.ProviderID,
		InstanceID: row.ID,
		Status:     reconmodel.StatusOpen,
	}

	switch row.PriceAnomaly {
	case "below_cost":
		f := base
		f.Kind = reconmodel.KindPriceBelowCost
		f.Severity = reconmodel.SeverityOf(f.Kind)
		f.LocalValue = moneyText(row.LocalPrice)
		f.UpstreamValue = moneyText(row.UpstreamCost)
		f.Delta = row.MarginAmount
		out = append(out, f)
	case "thin_margin":
		f := base
		f.Kind = reconmodel.KindPriceThinMargin
		f.Severity = reconmodel.SeverityOf(f.Kind)
		f.LocalValue = moneyText(row.LocalPrice)
		f.UpstreamValue = moneyText(row.UpstreamCost)
		f.Delta = row.MarginRate
		out = append(out, f)
	}

	switch row.ExpireAnomaly {
	case "expire_early":
		f := base
		f.Kind = reconmodel.KindExpireEarly
		f.Severity = reconmodel.SeverityOf(f.Kind)
		f.LocalValue = timeText(row.LocalExpire)
		f.UpstreamValue = timeText(row.UpstreamExpire)
		f.Delta = row.ExpireDiffDays
		out = append(out, f)
	case "expire_late":
		f := base
		f.Kind = reconmodel.KindExpireLate
		f.Severity = reconmodel.SeverityOf(f.Kind)
		f.LocalValue = timeText(row.LocalExpire)
		f.UpstreamValue = timeText(row.UpstreamExpire)
		f.Delta = row.ExpireDiffDays
		out = append(out, f)
	}

	switch row.StatusAnomaly {
	case "status_mismatch":
		f := base
		f.Kind = reconmodel.KindStatusMismatch
		f.Severity = reconmodel.SeverityOf(f.Kind)
		f.LocalValue = row.Status
		f.UpstreamValue = row.UpstreamStatus
		out = append(out, f)
	case "status_stale":
		f := base
		f.Kind = reconmodel.KindStatusStale
		f.Severity = reconmodel.SeverityOf(f.Kind)
		f.LocalValue = row.Status
		f.UpstreamValue = row.UpstreamStatus
		out = append(out, f)
	}
	return out
}

func (s *findingScanner) ListOpen(ctx context.Context, providerID uint64, page, pageSize int) ([]reconmodel.ReconcileFinding, int64, error) {
	return s.findings.ListOpen(ctx, providerID, page, pageSize)
}

func (s *findingScanner) UpdateStatus(ctx context.Context, id uint64, status string, adminID uint64, note string) error {
	switch status {
	case reconmodel.StatusOpen, reconmodel.StatusAck, reconmodel.StatusIgnored:
	default:
		return ErrInvalidFindingStatus
	}
	return s.findings.UpdateStatus(ctx, id, status, adminID, note)
}

func (s *findingScanner) FindingsForInstances(ctx context.Context, instanceIDs []uint64) (map[uint64][]reconmodel.ReconcileFinding, error) {
	return s.findings.FindingsForInstances(ctx, instanceIDs)
}

func moneyText(v *float64) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%.2f", *v)
}

func timeText(v *time.Time) string {
	if v == nil {
		return ""
	}
	return v.Format("2006-01-02")
}
