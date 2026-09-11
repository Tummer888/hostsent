package service

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
	syncrepo "hostsent/backend/internal/modules/admin/resource/sync/repository"
)

// Scheduler 定时调度器：按「渠道 × scope」多节奏扫描到期调度行并触发对应同步（T3.2）。
//
// 与 P0 的区别：旧实现按 resource_providers.sync_interval 一个间隔触发固定三类
// 同步；现在每个 (provider_id, scope) 独立节奏（目录小时级、实例分钟级），
// 支持优先级与允许执行时间窗。调度行由 EnsureSchedules 依能力描述符懒创建。
type Scheduler struct {
	engine    *SyncEngine
	syncRepo  syncrepo.SyncRepository
	fwRepo    syncrepo.FrameworkRepository
	logger    *zap.Logger
	interval  time.Duration // 扫描周期
	retention time.Duration // 同步任务/日志保留期（T0.5）
}

// NewScheduler 创建定时调度器，默认 60 秒扫描一次，同步数据保留 30 天。
func NewScheduler(engine *SyncEngine, syncRepo syncrepo.SyncRepository, fwRepo syncrepo.FrameworkRepository, logger *zap.Logger) *Scheduler {
	return &Scheduler{
		engine:    engine,
		syncRepo:  syncRepo,
		fwRepo:    fwRepo,
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
		// 启动即：确保调度行存在、回收僵死任务、清理过保留期数据，
		// 避免重启后渠道被历史任务长期阻塞。
		s.ensureSchedulesOnce(ctx)
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
				s.ensureSchedulesOnce(ctx)
				s.purgeOnce(ctx)
			}
		}
	}()
	s.logger.Info("scheduler started", zap.Duration("interval", s.interval))
}

// runOnce 扫描到期调度行并逐个触发。同一渠道多个 scope 到期时按 priority 降序
// 触发（高优先级先跑）；单条失败不影响其他。
func (s *Scheduler) runOnce(ctx context.Context) {
	due, err := s.fwRepo.ListDueSchedules(ctx, time.Now())
	if err != nil {
		s.logger.Error("scheduler find due schedules failed", zap.Error(err))
		return
	}
	// 同一轮里同一渠道只做一次适配器就绪校验，避免重复构建。
	ready := map[uint64]bool{}
	for _, item := range due {
		if !withinWindow(item, time.Now()) {
			// 不在允许执行的时间窗内：把下次到期推到窗口起点，不再每小时重试。
			s.deferToWindow(ctx, item)
			continue
		}
		if ok, checked := ready[item.ProviderID]; checked {
			if !ok {
				continue
			}
		} else {
			ok := s.engine.CheckProviderReady(ctx, item.ProviderID)
			ready[item.ProviderID] = ok
			if !ok {
				continue
			}
		}
		if err := s.trigger(ctx, item); err != nil {
			s.logger.Warn("scheduler trigger sync failed",
				zap.Uint64("provider_id", item.ProviderID),
				zap.String("scope", item.Scope), zap.Error(err))
		}
	}
}

// trigger 触发单条调度：创建任务并立刻推进 next_run_at，
// 避免任务尚在排队时被 60 秒扫描重复触发（任务完成时 recordScheduleResult 会以实际完成时间重算）。
func (s *Scheduler) trigger(ctx context.Context, item syncmodel.SyncSchedule) error {
	_, err := s.engine.StartSync(ctx, item.ProviderID, item.Scope)
	if err != nil && !errors.Is(err, ErrTaskRunning) {
		// 触发失败也要推进，否则会陷入每轮重试同一坏调度。
		next := time.Now().Add(time.Duration(safeInterval(item.IntervalSeconds)) * time.Second)
		_ = s.fwRepo.RecordScheduleRun(ctx, item.ID, syncmodel.ScheduleStatusFailed, err.Error(), time.Now(), next)
		return err
	}
	next := time.Now().Add(time.Duration(safeInterval(item.IntervalSeconds)) * time.Second)
	if item.WindowStart != nil {
		next = nextWindowStart(next, item.WindowStart, item.WindowEnd)
	}
	if err := s.fwRepo.RecordScheduleRun(ctx, item.ID, syncmodel.ScheduleStatusSuccess, "", time.Now(), next); err != nil {
		return err
	}
	s.logger.Info("scheduler trigger sync",
		zap.Uint64("provider_id", item.ProviderID), zap.String("scope", item.Scope))
	return nil
}

// ensureSchedulesOnce 依各渠道当前能力描述符补齐缺失的调度行（幂等，实现见 engine）。
func (s *Scheduler) ensureSchedulesOnce(ctx context.Context) {
	s.engine.EnsureAllSchedules(ctx)
}

// deferToWindow 把不在时间窗内的调度推到下一个窗口起点。
func (s *Scheduler) deferToWindow(ctx context.Context, item syncmodel.SyncSchedule) {
	if item.WindowStart == nil {
		return
	}
	next := nextWindowStart(time.Now(), item.WindowStart, item.WindowEnd)
	lastRun := time.Now()
	if item.LastRunAt != nil {
		lastRun = *item.LastRunAt
	}
	if err := s.fwRepo.RecordScheduleRun(ctx, item.ID, syncmodel.ScheduleStatusSkipped, "不在允许执行的时间窗内", lastRun, next); err != nil {
		s.logger.Warn("scheduler defer to window failed", zap.Uint64("provider_id", item.ProviderID), zap.Error(err))
	}
}

// purgeOnce 按保留期分批清理历史同步任务、日志与差异记录（T0.5/T3.5）。
func (s *Scheduler) purgeOnce(ctx context.Context) {
	before := time.Now().Add(-s.retention)
	n, err := s.syncRepo.PurgeSyncDataBefore(ctx, before, 5000)
	if err != nil {
		s.logger.Error("scheduler purge sync data failed", zap.Error(err))
	}
	if m, ferr := s.fwRepo.PurgeFrameworkBefore(ctx, before, 5000); ferr != nil {
		s.logger.Error("scheduler purge framework data failed", zap.Error(ferr))
	} else {
		n += m
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

// ============================================================================
// 时间窗与节流工具
// ============================================================================

// withinWindow 判断当前时刻是否落在允许执行的小时区间内。
// start/end 为 0-23 的小时；start==end 视为"该小时整点"而非全天，避免误配成不限；
// 任一为 nil 表示不限。支持跨夜（如 22→6）。
func withinWindow(item syncmodel.SyncSchedule, now time.Time) bool {
	if item.WindowStart == nil || item.WindowEnd == nil {
		return true
	}
	hour := int16(now.Hour())
	start, end := *item.WindowStart, *item.WindowEnd
	if start == end {
		return hour == start
	}
	if start < end {
		return hour >= start && hour < end
	}
	// 跨夜：22 点到次日 6 点
	return hour >= start || hour < end
}

// nextWindowStart 返回不早于 from 的下一个窗口开启时刻。
func nextWindowStart(from time.Time, start, end *int16) time.Time {
	if start == nil {
		return from
	}
	target := time.Date(from.Year(), from.Month(), from.Day(), int(*start), 0, 0, 0, from.Location())
	if !target.After(from) {
		target = target.Add(24 * time.Hour)
	}
	_ = end
	return target
}

// safeInterval 校正非法间隔，避免 0/负数导致每轮都到期。
func safeInterval(seconds int) int {
	if seconds <= 0 {
		return 3600
	}
	return seconds
}
