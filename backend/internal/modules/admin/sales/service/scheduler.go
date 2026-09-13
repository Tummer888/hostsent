package service

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// ReleaseScheduler 提成解冻调度器：按固定周期把已过解冻期的计提转入可用余额。
// 结构对齐 lifecycle/service/scheduler.go（ticker goroutine，ctx 取消即退出）。
type ReleaseScheduler struct {
	svc      CommissionService
	logger   *zap.Logger
	interval time.Duration
}

// NewReleaseScheduler 创建解冻调度器。解冻期以「天」为粒度，分钟级扫描足够及时。
func NewReleaseScheduler(svc CommissionService, logger *zap.Logger) *ReleaseScheduler {
	return &ReleaseScheduler{svc: svc, logger: logger, interval: time.Minute}
}

// Start 启动调度器 goroutine；ctx 取消时退出。
// 启动时先跑一轮：进程重启后积压的到期计提不必等下一个 tick。
func (s *ReleaseScheduler) Start(ctx context.Context) {
	go func() {
		s.runOnce(ctx)
		t := time.NewTicker(s.interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("sales commission release scheduler stopped")
				return
			case <-t.C:
				s.runOnce(ctx)
			}
		}
	}()
	s.logger.Info("sales commission release scheduler started", zap.Duration("interval", s.interval))
}

// runOnce 单轮解冻：失败仅记录，不中断调度。
func (s *ReleaseScheduler) runOnce(ctx context.Context) {
	n, err := s.svc.ReleaseDue(ctx, 200)
	if err != nil {
		s.logger.Error("sales commission release failed", zap.Error(err))
		return
	}
	if n > 0 {
		s.logger.Info("sales commission released", zap.Int("count", n))
	}
}
