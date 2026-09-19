package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/pkg/jobrun"
)

// PurgeScheduler 留存期清理调度器（doc104 §4.6）。
//
// 结构对齐 sales/service/scheduler.go 与 lifecycle/service/scheduler.go：
// ticker goroutine，ctx 取消即退出，启动时先跑一轮。
//
// 间隔取 24 小时：留存期以「天」为粒度（默认 180 天），分钟级轮询只是徒增
// 一次全表扫 deleted_at 的开销。到期用户晚几小时被清理没有任何业务影响。
type PurgeScheduler struct {
	svc      DeletionService
	logger   *zap.Logger
	interval time.Duration
	// batch 单轮上限，避免一次手工/定时触发就把大批行删掉锁库。
	batch int
}

// NewPurgeScheduler 创建清理调度器。
func NewPurgeScheduler(svc DeletionService, logger *zap.Logger) *PurgeScheduler {
	return &PurgeScheduler{svc: svc, logger: logger, interval: 24 * time.Hour, batch: defaultPurgeBatch}
}

// Start 启动调度器 goroutine；ctx 取消时退出。
func (s *PurgeScheduler) Start(ctx context.Context) {
	go func() {
		s.runOnce(ctx)
		t := time.NewTicker(s.interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("user purge scheduler stopped")
				return
			case <-t.C:
				s.runOnce(ctx)
			}
		}
	}()
	s.logger.Info("user purge scheduler started", zap.Duration("interval", s.interval), zap.Int("batch", s.batch))
}

// runOnce 单轮清理：失败仅记录，不中断调度。
//
// 落库策略：本任务一天一轮，不属于「高频任务」，因此空轮也写 job_run_logs ——
// 运营需要从日志中心看出「留存期清理每天都在正常跑、当天 0 条」，
// 这恰恰是合规审计关心的证据，不能像高频任务那样把空轮静默掉。
func (s *PurgeScheduler) runOnce(ctx context.Context) {
	jobrun.RunErr(ctx, "user_purge", "user", jobrun.TriggerScheduled,
		func(ctx context.Context) (int, int, map[string]any, error) {
			resp, err := s.svc.Purge(ctx, false, s.batch)
			if err != nil {
				s.logger.Error("user purge failed", zap.Error(err))
				return 0, 0, nil, err
			}
			if resp.Purged > 0 || len(resp.Skipped) > 0 {
				s.logger.Info("user purge finished",
					zap.Int("purged", resp.Purged),
					zap.Int("skipped", len(resp.Skipped)),
					zap.Bool("has_more", resp.HasMore),
					zap.Int("retention_days", resp.RetentionDays),
				)
			}
			return len(resp.Candidates), resp.Purged, map[string]any{
				"purged":         resp.Purged,
				"skipped":        len(resp.Skipped),
				"has_more":       resp.HasMore,
				"retention_days": resp.RetentionDays,
			}, nil
		})
}
