package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	syncrepo "hostsent/backend/internal/modules/admin/resource/sync/repository"
)

// Scheduler 定时调度器：按各提供商 sync_interval 周期扫描到期提供商并触发三类同步。
type Scheduler struct {
	engine   *SyncEngine
	syncRepo syncrepo.SyncRepository
	logger   *zap.Logger
	interval time.Duration // 扫描周期
}

// NewScheduler 创建定时调度器，默认 60 秒扫描一次。
func NewScheduler(engine *SyncEngine, syncRepo syncrepo.SyncRepository, logger *zap.Logger) *Scheduler {
	return &Scheduler{
		engine:   engine,
		syncRepo: syncRepo,
		logger:   logger,
		interval: 60 * time.Second,
	}
}

// Start 启动调度器 goroutine；ctx 取消时退出。
func (s *Scheduler) Start(ctx context.Context) {
	go func() {
		t := time.NewTicker(s.interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("scheduler stopped")
				return
			case <-t.C:
				s.runOnce(ctx)
			}
		}
	}()
	s.logger.Info("scheduler started", zap.Duration("interval", s.interval))
}

// runOnce 扫描到期提供商并逐个异步触发同步；单个提供商失败不影响其他提供商。
func (s *Scheduler) runOnce(ctx context.Context) {
	due, err := s.syncRepo.FindDueProviders(ctx, time.Now())
	if err != nil {
		s.logger.Error("scheduler find due providers failed", zap.Error(err))
		return
	}
	for _, pid := range due {
		s.logger.Info("scheduler trigger sync", zap.Uint64("provider_id", pid))
		go s.engine.SyncAll(context.Background(), pid)
	}
}
