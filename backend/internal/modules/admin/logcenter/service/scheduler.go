package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/modules/admin/logcenter/catalog"
	"hostsent/backend/internal/pkg/jobrun"
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
	// reconcileStale 收尾停滞的非终态清理任务（可选；nil 时跳过）。
	//
	// 独立于 Enabled：log_cleanup_enabled=false 只关「调度创建」，手工清理
	// 仍可用 —— 而停滞残行恰恰会把手工清理挡死（互斥索引）。
	reconcileStale func(ctx context.Context) (int64, error)
	// ReconcileEvery 停滞任务核对间隔（默认 10 分钟）。
	ReconcileEvery time.Duration
	// StaleAfter 多久没推进算停滞（默认 30 分钟，与互斥锁 TTL 同量级）。
	StaleAfter time.Duration
	// flags 每次 tick 重新读配置（运营改完不必重启进程，doc89 §9.1「改配置不需要发版」）。
	// nil 时退回构造时的静态字段，保持单测可直接构造。
	flags func(ctx context.Context) SchedulerFlags
}

// SchedulerFlags 调度运行时开关（每次 tick 读取）。
type SchedulerFlags struct {
	Enabled            bool
	CleanupHour        int
	ExportBeforeDelete bool
}

// currentFlags 取本轮生效的开关。
func (s *Scheduler) currentFlags(ctx context.Context) SchedulerFlags {
	if s.flags != nil {
		f := s.flags(ctx)
		if f.CleanupHour < 0 || f.CleanupHour > 23 {
			f.CleanupHour = s.CleanupHour
		}
		return f
	}
	return SchedulerFlags{Enabled: s.Enabled, CleanupHour: s.CleanupHour, ExportBeforeDelete: s.ExportBeforeDelete}
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

// SetStaleReconciler 注入停滞清理任务收尾函数。
func (s *Scheduler) SetStaleReconciler(fn func(ctx context.Context) (int64, error)) {
	s.reconcileStale = fn
}

// SetFlagReader 注入运行时开关读取器（装配层从 system_configs 读）。
//
// 不注入时用构造时的静态值：这样单测可以只给 SchedulerOptions 就驱动 tick。
func (s *Scheduler) SetFlagReader(fn func(ctx context.Context) SchedulerFlags) {
	s.flags = fn
}

// Start 启动调度 goroutine。
func (s *Scheduler) Start(ctx context.Context) {
	if s == nil || s.svc == nil {
		return
	}
	// 启动即收尾停滞任务：进程被 kill 留下的非终态行会永久挡住后续清理。
	s.reconcileStaleOnce(ctx)
	// 启动即清一次过期导出文件：6 小时一轮的 ticker 在「刚重启」时不会触发，
	// 而滞留的过期文件白占磁盘（人工导出，删文件不删行，可安全重跑）。
	s.purgeExpiredExports(ctx)
	go func() {
		t := time.NewTicker(s.CheckEvery)
		defer t.Stop()
		purgeT := time.NewTicker(s.ExportPurgeEvery)
		defer purgeT.Stop()
		reconcileT := time.NewTicker(s.reconcileInterval())
		defer reconcileT.Stop()
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("logcenter scheduler stopped")
				return
			case <-t.C:
				s.tick(ctx)
			case <-purgeT.C:
				s.purgeExpiredExports(ctx)
			case <-reconcileT.C:
				s.reconcileStaleOnce(ctx)
			}
		}
	}()
	s.logger.Info("logcenter scheduler started",
		zap.Bool("enabled", s.Enabled), zap.Int("hour", s.CleanupHour))
}

func (s *Scheduler) reconcileInterval() time.Duration {
	if s.ReconcileEvery <= 0 {
		return 10 * time.Minute
	}
	return s.ReconcileEvery
}

func (s *Scheduler) staleAfter() time.Duration {
	if s.StaleAfter <= 0 {
		return 30 * time.Minute
	}
	return s.StaleAfter
}

// reconcileStaleOnce 把停滞的非终态清理任务收成 failed（不依赖 Enabled）。
func (s *Scheduler) reconcileStaleOnce(ctx context.Context) {
	if s.reconcileStale == nil {
		return
	}
	if _, err := s.reconcileStale(ctx); err != nil {
		s.logger.Warn("logcenter: reconcile stale cleanup jobs failed", zap.Error(err))
	}
}

// tick 一轮检查：到点 + 当天没跑过 → 创建调度任务。
//
// 创建动作本身留痕 job_run_logs（doc92 §7.2 第 10 行 job_name=log_cleanup）。
// HasRunToday 在包装外判定，避免「到点但当天已跑过」的 10 分钟轮询也写日志：
// 每天最多一条 log_cleanup 记录，正对应每天最多一个清理任务。
func (s *Scheduler) tick(ctx context.Context) {
	flags := s.currentFlags(ctx)
	if !flags.Enabled {
		return
	}
	if time.Now().Hour() != flags.CleanupHour {
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
	jobrun.RunErr(ctx, "log_cleanup", catalog.GroupJob, jobrun.TriggerScheduled,
		func(ctx context.Context) (int, int, map[string]any, error) {
			if err := s.svc.RunScheduled(ctx, flags.ExportBeforeDelete); err != nil {
				// 互斥冲突或已关闭导出都属于正常拒绝，不刷错误日志。
				s.logger.Warn("logcenter scheduled cleanup skipped", zap.Error(err))
				return 0, 0, nil, err
			}
			s.logger.Info("logcenter scheduled cleanup created")
			sources := len(catalog.CleanableKeys())
			return sources, sources, map[string]any{"sources": sources}, nil
		})
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
