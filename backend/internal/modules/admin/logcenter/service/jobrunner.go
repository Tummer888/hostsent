package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	logmodel "hostsent/backend/internal/modules/admin/logcenter/model"
	logrepo "hostsent/backend/internal/modules/admin/logcenter/repository"
)

// Runner 统一的定时任务执行包装（doc92 §7.1）：记录开始/结束/耗时/结果/异常，
// 并用 uk_job_run_logs_running 做重入保护。
//
// 任一任务在调度循环里包一层 Run 即可获得 job_run_logs 记录，
// 任务本身不需要知道日志中心的存在。
type Runner struct {
	repo   logrepo.JobRunRepository
	logger *zap.Logger
	// skipEmptyRuns true 时「本轮什么都没处理」不落库（高频轮询任务用，
	// 否则 notify_delivery 15 秒一轮会在 180 天里积累百万行「扫了 0 条」）。
	skipEmptyRuns bool
	// maxRunsPerDay 软上限：某任务单日写入超过该值后本轮不写（防 bug 导致的狂写）。
	maxRunsPerDay int
}

// RunnerOptions 构造参数。
type RunnerOptions struct {
	SkipEmptyRuns bool
	MaxRunsPerDay int
}

// NewRunner 创建包装器。maxRunsPerDay <= 0 时用默认值 500。
func NewRunner(repo logrepo.JobRunRepository, logger *zap.Logger, opts RunnerOptions) *Runner {
	if opts.MaxRunsPerDay <= 0 {
		opts.MaxRunsPerDay = 500
	}
	return &Runner{repo: repo, logger: logger, skipEmptyRuns: opts.SkipEmptyRuns, maxRunsPerDay: opts.MaxRunsPerDay}
}

// JobFunc 任务体：返回 (扫描数, 处理数, 摘要, 错误)。
type JobFunc func(ctx context.Context) (scanned, affected int, summary map[string]any, err error)

// Run 执行一次任务并落 job_run_logs（空轮跳过按构造参数决定）。
//
// 任何失败都只记录日志，绝不 panic/阻断调度：日志中心的可用性不能反过来成为
// 业务调度的单点。日志中心不可用时任务照常执行（日志丢失）。
func (r *Runner) Run(ctx context.Context, jobName, jobGroup, triggerType string, fn JobFunc) {
	r.RunSkipEmpty(ctx, jobName, jobGroup, triggerType, r.skipEmptyRuns, fn)
}

// RunSkipEmpty 执行一次任务，按调用方给的 skipEmpty 决定「本轮无处理对象时不落库」。
//
// per-call 开关让装配层能按任务名判定高频任务（doc92 §7.2 的 4 个 ≤60 秒轮询），
// 而不必为同一份仓储构造两个 Runner。
func (r *Runner) RunSkipEmpty(ctx context.Context, jobName, jobGroup, triggerType string, skipEmpty bool, fn JobFunc) {
	if r == nil || fn == nil {
		return
	}
	started := time.Now()
	scanned, affected, summary, err := fn(ctx)
	if skipEmpty && scanned == 0 && affected == 0 && err == nil {
		return
	}
	if r.repo == nil {
		return
	}
	if !r.writeAllowed(ctx, jobName) {
		return
	}

	row := &logmodel.JobRunLog{
		JobName:       jobName,
		JobGroup:      jobGroup,
		TriggerType:   triggerType,
		Status:        logmodel.JobStatusRunning,
		StartedAt:     started,
		ItemsScanned:  scanned,
		ItemsAffected: affected,
		Summary:       encodeSummary(summary),
	}
	if err := r.repo.Start(ctx, row); err != nil {
		if err == logrepo.ErrJobRunning {
			// 上一轮还没结束：跳过本轮（这就是重入保护）。
			r.logger.Warn("job skipped: previous run still active", zap.String("job", jobName))
			return
		}
		r.logger.Warn("job run log start failed", zap.String("job", jobName), zap.Error(err))
		return
	}

	finished := time.Now()
	row.Status = logmodel.JobStatusSuccess
	if err != nil {
		row.Status = logmodel.JobStatusFailed
		row.ErrorMessage = truncate(err.Error(), 1000)
	}
	row.FinishedAt = &finished
	row.DurationMS = int(finished.Sub(started).Milliseconds())
	if err := r.repo.Finish(ctx, row); err != nil {
		// 收尾失败会让该 job 的 running 行残留，下一轮会被判为重入而跳过；
		// 这属于「宁可漏跑一轮也不要并行两轮」的取舍，但要打日志让人看见。
		r.logger.Warn("job run log finish failed", zap.String("job", jobName), zap.Error(err))
	}
}

// writeAllowed 软上限检查：单日写入过多说明任务可能陷入异常循环。
func (r *Runner) writeAllowed(ctx context.Context, jobName string) bool {
	n, err := r.repo.CountToday(ctx, jobName)
	if err != nil {
		return true // 查不出来就不拦，宁可多写也不要漏记
	}
	if n >= int64(r.maxRunsPerDay) {
		r.logger.Warn("job run log suppressed: daily cap reached",
			zap.String("job", jobName), zap.Int64("today", n), zap.Int("cap", r.maxRunsPerDay))
		return false
	}
	return true
}

// staleRunningTimeout 多久没结束的 running 行算「进程中断遗留」。
//
// 取 6 小时：任何一轮任务的正常耗时都远小于它（最慢的生命周期扫描是 10 分钟一轮），
// 因此不会误伤真正在跑的长任务，又能让重启后静默停摆的 job 尽快恢复。
const staleRunningTimeout = 6 * time.Hour

// ReconcileStaleRunning 收尾进程中断遗留的 running 行（启动时调用一次）。
//
// 不这样做的后果：uk_job_run_logs_running 保证「同一 job 至多一条 running」，
// 而这条 running 只在 Finish 成功时才转终态。进程在任务执行中途被 kill，该行
// 永远停在 running，之后每一轮 Start 都命中唯一索引 → 被当成「上一轮还在跑」
// 跳过 → 该 job 永久静默停摆，日志里只留一条 warn（doc92 场景 8/9 的另一面）。
func (r *Runner) ReconcileStaleRunning(ctx context.Context) (int64, error) {
	if r == nil || r.repo == nil {
		return 0, nil
	}
	n, err := r.repo.FailStaleRunning(ctx, time.Now().Add(-staleRunningTimeout),
		"进程中断，任务未正常收尾")
	if err != nil {
		return 0, err
	}
	if n > 0 {
		r.logger.Warn("job run logs: reconciled stale running rows", zap.Int64("rows", n))
	}
	return n, nil
}
