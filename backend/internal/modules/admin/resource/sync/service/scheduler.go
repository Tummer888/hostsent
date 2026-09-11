package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	syncrepo "hostsent/backend/internal/modules/admin/resource/sync/repository"
)

// Scheduler 定时调度器：按各提供商 sync_interval 周期扫描到期提供商并触发三类同步。
type Scheduler struct {
	engine    *SyncEngine
	syncRepo  syncrepo.SyncRepository
	logger    *zap.Logger
	interval  time.Duration // 扫描周期
	retention time.Duration // 同步任务/日志保留期（T0.5）
}

// NewScheduler 创建定时调度器，默认 60 秒扫描一次，同步数据保留 30 天。
func NewScheduler(engine *SyncEngine, syncRepo syncrepo.SyncRepository, logger *zap.Logger) *Scheduler {
	return &Scheduler{
		engine:    engine,
		syncRepo:  syncRepo,
		logger:    logger,
		interval:  60 * time.Second,
		retention: 30 * 24 * time.Hour,
	}
}

// Start 启动调度器 goroutine；ctx 取消时退出。
func (s *Scheduler) Start(ctx context.Context) {
	go func() {
		t := time.NewTicker(s.interval)
		defer t.Stop()
		retentionT := time.NewTicker(24 * time.Hour)
		defer retentionT.Stop()
		staleT := time.NewTicker(10 * time.Minute)
		defer staleT.Stop()
		// 启动即回收僵死任务与过保留期数据，避免重启后渠道被历史任务长期阻塞。
		s.reapStaleOnce(ctx)
		s.purgeOnce(ctx)
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("scheduler stopped")
				return
			case <-t.C:
				s.runOnce(ctx)
			case <-staleT.C:
				s.reapStaleOnce(ctx)
			case <-retentionT.C:
				s.purgeOnce(ctx)
			}
		}
	}()
	s.logger.Info("scheduler started", zap.Duration("interval", s.interval))
}

// runOnce 扫描到期提供商并逐个异步触发同步；单个提供商失败不影响其他提供商。
// 触发前做前置校验：适配器不可用的渠道直接熔断跳过，不产生注定失败的任务。
func (s *Scheduler) runOnce(ctx context.Context) {
	due, err := s.syncRepo.FindDueProviders(ctx, time.Now())
	if err != nil {
		s.logger.Error("scheduler find due providers failed", zap.Error(err))
		return
	}
	for _, pid := range due {
		if !s.engine.CheckProviderReady(ctx, pid) {
			continue
		}
		s.logger.Info("scheduler trigger sync", zap.Uint64("provider_id", pid))
		go s.engine.SyncAll(context.Background(), pid)
	}
}

// purgeOnce 按保留期分批清理历史同步任务与日志（T0.5）。
func (s *Scheduler) purgeOnce(ctx context.Context) {
	before := time.Now().Add(-s.retention)
	n, err := s.syncRepo.PurgeSyncDataBefore(ctx, before, 5000)
	if err != nil {
		s.logger.Error("scheduler purge sync data failed", zap.Error(err))
		return
	}
	if n > 0 {
		s.logger.Info("scheduler purged sync data",
			zap.Int64("rows", n), zap.Time("before", before))
	}
}

// staleTaskTimeout 超过该时长仍在 pending/running 的任务视为僵死。
const staleTaskTimeout = time.Hour

// reapStaleOnce 回收僵死任务，解除其对渠道调度的永久阻塞。
func (s *Scheduler) reapStaleOnce(ctx context.Context) {
	n, err := s.syncRepo.FailStaleTasks(ctx, time.Now().Add(-staleTaskTimeout))
	if err != nil {
		s.logger.Error("scheduler reap stale sync tasks failed", zap.Error(err))
		return
	}
	if n > 0 {
		s.logger.Warn("scheduler reaped stale sync tasks", zap.Int64("rows", n))
	}
}
