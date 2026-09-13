package service

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// Scheduler 日志清理调度器（doc92 §6.6）。
//
// 形态照抄既有调度器：ticker goroutine + ctx 取消退出。区别是这里用
// 「每 10 分钟检查一次 + 当天已跑过就跳过」而不是「每天固定时刻一次性触发」，
// 因为 ticker 在重启/时区/DST 下会漏跑或重跑；轮询 + 当日去重是幂等的。
type Scheduler struct {
	svc    CleanupService
	logger *zap.Logger
	// CheckEvery 检查间隔（默认 10 分钟）。
	CheckEvery time.Duration
	// CleanupHour 期望执行的小时（0–23，默认 3）。
	CleanupHour int
	// Enabled 总开关（log_cleanup_enabled）。
	Enabled bool
	// ExportBeforeDelete 删除前导出开关（log_export_before_delete）。
	ExportBeforeDelete bool
	// ExportPurgeEvery 过期导出文件清理间隔（默认 6 小时）。
	ExportPurgeEvery time.Duration
	// purgeExport 过期导出文件清理（可选；nil 时跳过）。
	purgeExport func(ctx context.Context) (int, error)
}

// SchedulerOptions 调度参数。
type SchedulerOptions struct {
	Enabled            bool
	CleanupHour        int
	ExportBeforeDelete bool
	CheckEvery         time.Duration
	ExportPurgeEvery   time.Duration
}

// NewScheduler 创建调度器。
func NewScheduler(svc CleanupService, opts SchedulerOptions, logger *zap.Logger) *Scheduler {
	if opts.CheckEvery <= 0 {
		opts.CheckEvery = 10 * time.Minute
	}
	if opts.ExportPurgeEvery <= 0 {
		opts.ExportPurgeEvery = 6 * time.Hour
	}
	if opts.CleanupHour < 0 || opts.CleanupHour > 23 {
		opts.CleanupHour = 3
	}
	return &Scheduler{
		svc: svc, logger: logger, CheckEvery: opts.CheckEvery,
		CleanupHour: opts.CleanupHour, Enabled: opts.Enabled,
		ExportBeforeDelete: opts.ExportBeforeDelete, ExportPurgeEvery: opts.ExportPurgeEvery,
	}
}

// SetExportPurger 注入过期导出文件清理函数。
func (s *Scheduler) SetExportPurger(fn func(ctx context.Context) (int, error)) {
	s.purgeExport = fn
}

// Start 启动调度 goroutine。
func (s *Scheduler) Start(ctx context.Context) {
	if s == nil || s.svc == nil {
		return
	}
	go func() {
		t := time.NewTicker(s.CheckEvery)
		defer t.Stop()
		purgeT := time.NewTicker(s.ExportPurgeEvery)
		defer purgeT.Stop()
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("logcenter scheduler stopped")
				return
			case <-t.C:
				s.tick(ctx)
			case <-purgeT.C:
				s.purgeExpiredExports(ctx)
			}
		}
	}()
	s.logger.Info("logcenter scheduler started",
		zap.Bool("enabled", s.Enabled), zap.Int("hour", s.CleanupHour))
}

// tick 一轮检查：到点 + 当天没跑过 → 创建调度任务。
func (s *Scheduler) tick(ctx context.Context) {
	if !s.Enabled {
		return
	}
	if time.Now().Hour() != s.CleanupHour {
		return
	}
	ran, err := s.svc.HasRunToday(ctx)
	if err != nil {
		s.logger.Warn("logcenter: check today's cleanup run failed", zap.Error(err))
		return
	}
	if ran {
		return
	}
	if err := s.svc.RunScheduled(ctx, s.ExportBeforeDelete); err != nil {
		// 互斥冲突或已关闭导出都属于正常拒绝，不刷错误日志。
		s.logger.Warn("logcenter scheduled cleanup skipped", zap.Error(err))
		return
	}
	s.logger.Info("logcenter scheduled cleanup created")
}

// purgeExpiredExports 清理过期的人工导出文件。
func (s *Scheduler) purgeExpiredExports(ctx context.Context) {
	if s.purgeExport == nil {
		return
	}
	n, err := s.purgeExport(ctx)
	if err != nil {
		s.logger.Warn("logcenter: purge expired exports failed", zap.Error(err))
		return
	}
	if n > 0 {
		s.logger.Info("logcenter purged expired exports", zap.Int("files", n))
	}
}

// Runner 装配：把调度任务本身也记进 job_run_logs（doc92 §7.2 第 10 行）。
func (s *Scheduler) RunOnceForTest(ctx context.Context) { s.tick(ctx) }
