package server

// 日志中心装配（doc92 L1–L7）：
//
//   - 上游接口日志采集（纯适配器经 upstream.SetRecorder 端点外接，适配器零 DB 依赖）
//   - 任务运行日志（10 个调度点经 Runner 留痕）
//   - 26 个日志源的统一浏览 / 导出 / 保留策略 / 统一清理引擎
//
// 依赖方向：logcenter 模块不 import 任何业务模块；业务侧调度点只 import
// internal/pkg/upstream（采集端点）与 logcenter/service 的 Runner（任务留痕），
// 不反向依赖日志中心的仓储与处理器。

import (
	"context"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	logcatalog "hostsent/backend/internal/modules/admin/logcenter/catalog"
	loghandler "hostsent/backend/internal/modules/admin/logcenter/handler"
	logrepo "hostsent/backend/internal/modules/admin/logcenter/repository"
	logservice "hostsent/backend/internal/modules/admin/logcenter/service"
	"hostsent/backend/internal/pkg/storage"
	"hostsent/backend/internal/pkg/upstream"
)

// logcenterBundle 日志中心处理器与后台组件集合。
type logcenterBundle struct {
	handler *loghandler.Handler
	policy  logservice.PolicyService
	// recorder 上游接口日志写入器（Server.Run 中启动消费 goroutine）。
	recorder *logservice.UpstreamRecorder
	// scheduler 统一清理调度器（Server.Run 中启动）。
	scheduler *logservice.Scheduler
	// Runner 任务运行留痕器：业务调度点注入使用（doc92 §7.2）。
	Runner *logservice.Runner
}

// logcenterBundleDeps 装配依赖。
type logcenterBundleDeps struct {
	DB *gorm.DB
	// ConfigReader 逐键读取 system_configs（键不存在返回 ok=false，不报错）。
	ConfigReader func(ctx context.Context, key string) (string, bool, error)
	// ConfigIntOr 读整数配置（缺失/非法时返回 fallback）。
	ConfigIntOr func(ctx context.Context, key string, fallback int) int
	// ConfigBoolOr 读布尔配置（缺失/非法时返回 fallback）。
	ConfigBoolOr func(ctx context.Context, key string, fallback bool) bool
	// StorageRoot 导出文件落盘根目录（与工单附件共用）。
	StorageRoot string
	// Locker 可选 Redis 分布式锁（缓存不可用时传 nil）。
	Locker logservice.Locker
	Logger *zap.Logger
}

// buildLogcenterBundle 装配日志中心。
func buildLogcenterBundle(deps logcenterBundleDeps) *logcenterBundle {
	logger := deps.Logger
	queryRepo := logrepo.NewLogQueryRepository(deps.DB)
	upstreamRepo := logrepo.NewUpstreamLogRepository(deps.DB)
	jobRunRepo := logrepo.NewJobRunRepository(deps.DB)
	policyRepo := logrepo.NewPolicyRepository(deps.DB)
	exportRepo := logrepo.NewExportRepository(deps.DB)
	cleanupRepo := logrepo.NewCleanupRepository(deps.DB)

	// 采集侧写入器：日志表不可用绝不能拖垮业务调用（丢弃 + 计数 + 告警）。
	recorder := logservice.NewUpstreamRecorder(
		upstreamRepo, logger, logservice.DefaultUpstreamRecorderOptions(),
	)
	upstream.SetRecorder(recorder)
	upstream.SetCaptureOptions(upstream.CaptureOptions{
		Enabled:      deps.ConfigBoolOr(context.Background(), "log_upstream_capture", true),
		SampleRate:   deps.ConfigIntOr(context.Background(), "log_upstream_sample_rate", 100),
		BodyMaxBytes: deps.ConfigIntOr(context.Background(), "log_upstream_body_max_bytes", 4096),
	})

	// 保留策略：全局默认值可被逐源策略行覆盖（doc92 §6.2）。
	policySvc := logservice.NewPolicyService(
		policyRepo, deps.ConfigIntOr(context.Background(), "log_retention_days", logcatalog.DefaultRetentionDays), logger,
	)
	if err := policySvc.Seed(context.Background()); err != nil {
		logger.Warn("logcenter: seed retention policies failed", zap.Error(err))
	}

	// 导出存储：与工单附件共用同一 LocalStore 根目录，子目录隔离（log-exports/）。
	var store *storage.LocalStore
	if deps.StorageRoot != "" {
		s, err := storage.NewLocalStore(deps.StorageRoot)
		if err != nil {
			logger.Error("logcenter: init export storage failed", zap.Error(err))
			store = nil
		} else {
			store = s
		}
	}
	exportSvc := logservice.NewExportService(queryRepo, exportRepo, cleanupRepo, store, logservice.ExportServiceOptions{
		ExportRootDir: "log-exports",
		RetentionDays: deps.ConfigIntOr(context.Background(), "log_export_retention_days", 365),
	}, logger)

	cleanupSvc := logservice.NewCleanupService(
		queryRepo, cleanupRepo, exportRepo, exportSvc, policySvc, jobRunRepo,
		logservice.CleanupServiceOptions{
			ExportBeforeDelete: deps.ConfigBoolOr(context.Background(), "log_export_before_delete", true),
			BatchSize:          5000,
			Locker:             deps.Locker,
		},
		logger,
	)

	querySvc := logservice.NewQueryService(queryRepo, policySvc, exportRepo, cleanupRepo, jobRunRepo, recorder, logger)

	scheduler := logservice.NewScheduler(cleanupSvc, logservice.SchedulerOptions{
		Enabled:            deps.ConfigBoolOr(context.Background(), "log_cleanup_enabled", true),
		CleanupHour:        deps.ConfigIntOr(context.Background(), "log_cleanup_hour", 3),
		ExportBeforeDelete: deps.ConfigBoolOr(context.Background(), "log_export_before_delete", true),
	}, logger)
	scheduler.SetExportPurger(func(ctx context.Context) (int, error) {
		return exportSvc.PurgeExpired(ctx, 200)
	})

	handler := loghandler.NewHandler(loghandler.HandlerDeps{
		Query:   querySvc,
		Export:  exportSvc,
		Cleanup: cleanupSvc,
		Policy:  policySvc,
		// 关闭删除前导出时页面对执行按钮置灰；服务端另有硬拒绝兜底。
		ExportBeforeDelete: func() bool {
			return deps.ConfigBoolOr(context.Background(), "log_export_before_delete", true)
		},
	})

	// 任务运行留痕器（doc92 §7.2）：单任务每日软上限 500 次。
	// 空轮跳过由 per-call 开关决定（高频任务才跳），故这里不全局打开。
	runner := logservice.NewRunner(jobRunRepo, logger, logservice.RunnerOptions{
		SkipEmptyRuns: false,
		MaxRunsPerDay: deps.ConfigIntOr(context.Background(), "log_job_max_runs_per_day", 500),
	})
	// 注入中性端点：业务调度器只写 jobrun.Run(...)，不依赖日志中心模块。
	jobrun.SetRunner(&logcenterJobRunner{runner: runner})

	return &logcenterBundle{
		handler: handler, policy: policySvc, recorder: recorder,
		scheduler: scheduler, Runner: runner,
	}
}

// logcenterJobRunner 把日志中心的 Runner 适配为中性 jobrun 端点。
//
// 高频任务（≤60 秒轮询）本轮无处理对象时不落库；其余任务即使空跑也留痕
// （每日/每小时任务空跑本身有信息量）。任务名判定见 jobrun.HighFrequencyJobs。
type logcenterJobRunner struct {
	runner *logservice.Runner
}

// Run 见 jobrun.Runner。
func (l *logcenterJobRunner) Run(ctx context.Context, jobName, jobGroup, triggerType string, fn jobrun.TypedFunc) {
	if l == nil || l.runner == nil {
		if fn != nil {
			_, _, _, _ = fn(ctx)
		}
		return
	}
	l.runner.RunSkipEmpty(ctx, jobName, jobGroup, triggerType, jobrun.ShouldSkipEmpty(jobName), fn)
}

// logcenterConfigReader 适配：整数配置读取（缺失或非法返回 fallback）。
func logcenterConfigIntOr(reader func(ctx context.Context, key string) (string, bool, error)) func(ctx context.Context, key string, fallback int) int {
	return func(ctx context.Context, key string, fallback int) int {
		raw, ok, err := reader(ctx, key)
		if err != nil || !ok {
			return fallback
		}
		v, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return fallback
		}
		return v
	}
}

// logcenterConfigBoolOr 适配：布尔配置读取（缺失或非法返回 fallback）。
// 布尔判定与 doc91/doc90 既有开关一致：1/true/on/yes 为真。
func logcenterConfigBoolOr(reader func(ctx context.Context, key string) (string, bool, error)) func(ctx context.Context, key string, fallback bool) bool {
	return func(ctx context.Context, key string, fallback bool) bool {
		raw, ok, err := reader(ctx, key)
		if err != nil || !ok {
			return fallback
		}
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "1", "true", "on", "yes":
			return true
		case "0", "false", "off", "no":
			return false
		default:
			return fallback
		}
	}
}

// logcenterRedisLocker 以缓存客户端实现清理互斥锁。
//
// 缓存不可用时降级为进程内 SETNX（单实例有效），跨实例互斥由
// log_cleanup_jobs 的部分唯一索引兜底（doc92 §6.5）。
type logcenterRedisLocker struct {
	cache interface {
		Lock(ctx context.Context, key string, ttl time.Duration) (func(), bool)
	}
}

// TryLock 尝试加锁；成功返回释放函数。
func (l *logcenterRedisLocker) TryLock(ctx context.Context, key string, ttl time.Duration) (func(), bool) {
	if l == nil || l.cache == nil {
		return nil, false
	}
	return l.cache.Lock(ctx, key, ttl)
}
