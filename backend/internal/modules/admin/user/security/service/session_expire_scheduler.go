package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/pkg/jobrun"
)

// SessionExpireScheduler 会话过期状态回写调度器。
//
// 解决的问题：user_sessions.expired_at 到点后没有任何东西把 status 从 active
// 改掉。在线统计虽然已按 expired_at 过滤（口径正确），但 status 列本身仍在骗人 ——
// 安全页按状态筛选/导出会话、用户详情读 active 计数时，拿到的都是虚高的数。
//
// 间隔取 10 分钟：会话有效期以小时计（JWT TTL），分钟级轮询足够让 status 追上
// expired_at；再密只是徒增一次带索引的扫描。
// 不属于 jobrun.HighFrequencyJobs（> 60 秒），因此空轮也留痕 ——
// 运营需要看出「这个回写任务在正常跑」。
type SessionExpireScheduler struct {
	svc      SecurityService
	logger   *zap.Logger
	interval time.Duration
	// batch 单轮上限，避免一次全表回写锁库。
	batch int
}

// NewSessionExpireScheduler 创建调度器。
func NewSessionExpireScheduler(svc SecurityService, logger *zap.Logger) *SessionExpireScheduler {
	return &SessionExpireScheduler{svc: svc, logger: logger, interval: 10 * time.Minute, batch: 500}
}

// Start 启动调度器 goroutine；ctx 取消时退出。
func (s *SessionExpireScheduler) Start(ctx context.Context) {
	go func() {
		s.runOnce(ctx)
		t := time.NewTicker(s.interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("session expire scheduler stopped")
				return
			case <-t.C:
				s.runOnce(ctx)
			}
		}
	}()
	s.logger.Info("session expire scheduler started", zap.Duration("interval", s.interval), zap.Int("batch", s.batch))
}

// runOnce 单轮回写：失败仅记录，不中断调度。
func (s *SessionExpireScheduler) runOnce(ctx context.Context) {
	jobrun.RunErr(ctx, "session_expire", "security", jobrun.TriggerScheduled,
		func(ctx context.Context) (int, int, map[string]any, error) {
			// 循环取直到不足一批：积压（比如进程停机很久后启动）一次跑完，
			// 但每批都受 batch 限制，不会一条 UPDATE 锁住整表。
			total := 0
			for {
				n, err := s.svc.ExpireStaleSessions(ctx, s.batch)
				if err != nil {
					s.logger.Error("session expire failed", zap.Error(err))
					return total, total, nil, err
				}
				total += n
				if n < s.batch {
					break
				}
			}
			if total > 0 {
				s.logger.Info("session expire finished", zap.Int("expired", total))
			}
			return total, total, map[string]any{"expired": total}, nil
		})
}
