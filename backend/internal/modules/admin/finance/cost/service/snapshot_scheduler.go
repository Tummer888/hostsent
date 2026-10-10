package service

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// SnapshotScheduler 上游同步的每日定时任务：余额快照 + 账本流水 + 水位告警（doc111 §5）。
//
// 为什么需要它：
//   - 月度成本以「上游账本消费流水」为唯一自动口径 → 每天增量同步一次流水（新单/续费当天就进成本）；
//   - 台账页的「最新余额」与告警依赖余额快照 → 每天自动抓一条（上游只给当前余额，不抓就断档）；
//   - 余额水位（余额 < 未来 30 天到期金额）也需要每天重算，否则等到停机才知道钱不够。
//
// 执行规则：
//   - 每 tickEvery 检查一次；本地时间到达财务配置的 snapshot_hour（默认 3 点）后当天首次触发；
//   - 进程重启不重复执行（用当天日期去重）；配置改到已过时点则次日生效（不回溯补跑）；
//   - 账本同步默认增量（按上游自增 ID 断点续传）；单渠道失败只记日志，不影响其它渠道与业务流程。
type SnapshotScheduler struct {
	service   CostService
	logger    *zap.Logger
	tickEvery time.Duration
	lastRun   string // 已执行日期 YYYY-MM-DD（内存态，重启后由「今天还没跑」自然补跑）
}

// NewSnapshotScheduler 创建每日快照调度器。
func NewSnapshotScheduler(service CostService, logger *zap.Logger) *SnapshotScheduler {
	return &SnapshotScheduler{service: service, logger: logger, tickEvery: 10 * time.Minute}
}

// Start 启动调度循环（阻塞在独立 goroutine 中，随 ctx 结束退出）。
func (s *SnapshotScheduler) Start(ctx context.Context) {
	if s.service == nil {
		return
	}
	go func() {
		// 启动后先等一个 tick，避免与启动期其它初始化抢资源。
		timer := time.NewTimer(time.Minute)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				s.maybeRun(ctx)
				timer.Reset(s.tickEvery)
			}
		}
	}()
}

func (s *SnapshotScheduler) maybeRun(ctx context.Context) {
	now := s.service.nowForSchedule()
	today := now.Format("2006-01-02")
	if s.lastRun == today {
		return
	}
	hour := s.service.SnapshotHour(ctx)
	if now.Hour() < hour {
		return
	}
	s.lastRun = today

	// ① 余额快照：只有具备余额读取能力的渠道会被抓取，其余计入 skipped。
	saved, skipped, failed := s.service.RunAutoSnapshots(ctx)
	if s.logger != nil {
		s.logger.Info("上游余额快照自动抓取完成",
			zap.Int("saved", saved), zap.Int("skipped_unsupported", skipped), zap.Int("failed", failed))
	}

	// ② 账本流水：增量同步（消费 + 充值），单渠道失败不影响其它渠道。
	if result, err := s.service.SyncLedger(ctx, 0, false); err == nil {
		for _, item := range result.Results {
			if item.Failed != "" {
				if s.logger != nil {
					s.logger.Warn("上游账本同步失败",
						zap.Uint64("provider_id", item.ProviderID), zap.String("provider", item.ProviderName),
						zap.String("reason", item.Failed))
				}
				continue
			}
			if s.logger != nil {
				s.logger.Info("上游账本同步完成",
					zap.Uint64("provider_id", item.ProviderID), zap.String("provider", item.ProviderName),
					zap.Int("consumption_written", item.Consumption), zap.Int("topup_written", item.Topup),
					zap.Int("consumption_total", item.ConsumptionTotal), zap.Int("topup_total", item.TopupTotal))
			}
		}
	} else if s.logger != nil {
		s.logger.Warn("上游账本同步跳过", zap.Error(err))
	}

	// ③ 余额水位告警：余额是否够付未来 30 天的续费（不够就大声记一条）。
	if alerts, err := s.service.BalanceAlerts(ctx); err == nil {
		for _, alert := range alerts {
			if alert.Level == "ok" {
				continue
			}
			if s.logger != nil {
				s.logger.Warn("上游余额水位告警",
					zap.Uint64("provider_id", alert.ProviderID), zap.String("provider", alert.ProviderName),
					zap.String("level", alert.Level), zap.Float64("balance", alert.Balance),
					zap.Float64("due_within_30d", alert.DueWithin30d), zap.String("message", alert.Message))
			}
		}
	}
}
