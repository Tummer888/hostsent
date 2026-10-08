package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/pkg/jobrun"
)

// LifecycleScheduler 生命周期调度器：按固定周期执行到期提醒与自动续费扫描。
type LifecycleScheduler struct {
	lifecycleSvc LifecycleService
	logger       *zap.Logger
	interval     time.Duration // 扫描周期
}

// NewLifecycleScheduler 创建调度器，默认 10 分钟扫描一轮。
func NewLifecycleScheduler(lifecycleSvc LifecycleService, logger *zap.Logger) *LifecycleScheduler {
	return &LifecycleScheduler{
		lifecycleSvc: lifecycleSvc,
		logger:       logger,
		interval:     10 * time.Minute,
	}
}

// Start 启动调度器 goroutine；ctx 取消时退出。
func (s *LifecycleScheduler) Start(ctx context.Context) {
	go func() {
		t := time.NewTicker(s.interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("lifecycle scheduler stopped")
				return
			case <-t.C:
				s.runOnce(ctx)
			}
		}
	}()
	s.logger.Info("lifecycle scheduler started", zap.Duration("interval", s.interval))
}

// runOnce 单轮扫描：到期提醒 → 自动续费；失败仅记录，不中断调度。
func (s *LifecycleScheduler) runOnce(ctx context.Context) {
	// 包一层任务运行留痕（doc92 §7.2）；留痕端点未注入时等价于直接调用。
	jobrun.RunErr(ctx, "lifecycle_scan", "lifecycle", jobrun.TriggerScheduled,
		func(ctx context.Context) (int, int, map[string]any, error) {
			if err := s.lifecycleSvc.RunScanOnce(ctx); err != nil {
				s.logger.Error("lifecycle scan failed", zap.Error(err))
				return 0, 0, nil, err
			}
			return 0, 0, map[string]any{"result": "ok"}, nil
		})
}
