package service

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/modules/admin/logcenter/dto"
	logmodel "hostsent/backend/internal/modules/admin/logcenter/model"
	logrepo "hostsent/backend/internal/modules/admin/logcenter/repository"
)

// fakeCleanupSvc 只记录调度相关调用，其余方法退回零值。
type fakeCleanupSvc struct {
	ranToday    bool
	runSchedule int
	lastExport  *bool
	reconciled  int
}

func (f *fakeCleanupSvc) Preview(context.Context, dto.CleanupPreviewRequest) (*dto.CleanupPreviewResponse, error) {
	return nil, nil
}
func (f *fakeCleanupSvc) Start(context.Context, dto.CleanupRunRequest, string, uint64, string, bool) (*dto.CleanupJobInfo, error) {
	return nil, nil
}
func (f *fakeCleanupSvc) RunScheduled(_ context.Context, exportBeforeDelete bool) error {
	f.runSchedule++
	f.lastExport = &exportBeforeDelete
	return nil
}
func (f *fakeCleanupSvc) List(context.Context, CleanupListQuery) (*dto.CleanupJobListResponse, error) {
	return nil, nil
}
func (f *fakeCleanupSvc) Detail(context.Context, uint64) (*dto.CleanupJobDetailResponse, error) {
	return nil, nil
}
func (f *fakeCleanupSvc) Cancel(context.Context, uint64) error { return nil }
func (f *fakeCleanupSvc) HasRunToday(context.Context) (bool, error) {
	return f.ranToday, nil
}
func (f *fakeCleanupSvc) ReconcileStale(context.Context) (int64, error) {
	f.reconciled++
	return 0, nil
}

// 场景 30：log_cleanup_enabled=false 时调度器不创建任务。
//
// 这条开关必须「每轮重读」才成立：配置读取器返回 false 后，即便到了执行小时
// 也不能创建任务；关掉开关不需要重启进程（doc89 §9.1）。
func TestSchedulerSkipsWhenDisabled(t *testing.T) {
	svc := &fakeCleanupSvc{}
	s := NewScheduler(svc, SchedulerOptions{Enabled: true, CleanupHour: time.Now().Hour()}, zap.NewNop())
	s.SetFlagReader(func(context.Context) SchedulerFlags {
		return SchedulerFlags{Enabled: false, CleanupHour: time.Now().Hour()}
	})

	s.tick(context.Background())

	if svc.runSchedule != 0 {
		t.Fatalf("disabled scheduler must not create a cleanup job, calls=%d", svc.runSchedule)
	}
}

// 到了执行小时且当天没跑过 → 创建一次调度任务；已跑过则跳过。
func TestSchedulerRunsOnceAtHour(t *testing.T) {
	svc := &fakeCleanupSvc{}
	s := NewScheduler(svc, SchedulerOptions{Enabled: true, CleanupHour: time.Now().Hour(), ExportBeforeDelete: true}, zap.NewNop())

	s.tick(context.Background())
	if svc.runSchedule != 1 {
		t.Fatalf("expected 1 scheduled cleanup, got %d", svc.runSchedule)
	}
	if svc.lastExport == nil || !*svc.lastExport {
		t.Error("scheduled run must honor the export-before-delete flag")
	}

	// 当天已跑过（daily 语义）→ 不再创建。
	svc.ranToday = true
	s.tick(context.Background())
	if svc.runSchedule != 1 {
		t.Fatalf("already ran today must not create another job, calls=%d", svc.runSchedule)
	}
}

// 未到执行小时 → 不创建（避免 10 分钟一轮的 ticker 在错误时刻动手）。
func TestSchedulerSkipsOutsideHour(t *testing.T) {
	svc := &fakeCleanupSvc{}
	other := (time.Now().Hour() + 5) % 24
	s := NewScheduler(svc, SchedulerOptions{Enabled: true, CleanupHour: other}, zap.NewNop())

	s.tick(context.Background())

	if svc.runSchedule != 0 {
		t.Fatalf("outside cleanup hour must not create a job, calls=%d", svc.runSchedule)
	}
}

// 停滞任务收尾必须独立于 Enabled：关掉调度不影响「手工清理可用」，
// 而此时被残行挡死的恰恰是手工清理。
func TestSchedulerReconcilesStaleEvenWhenDisabled(t *testing.T) {
	svc := &fakeCleanupSvc{}
	s := NewScheduler(svc, SchedulerOptions{Enabled: false}, zap.NewNop())
	s.SetFlagReader(func(context.Context) SchedulerFlags {
		return SchedulerFlags{Enabled: false, CleanupHour: time.Now().Hour()}
	})
	reconciled := 0
	s.SetStaleReconciler(func(context.Context) (int64, error) {
		reconciled++
		return 0, nil
	})

	s.reconcileStaleOnce(context.Background())

	if reconciled != 1 {
		t.Fatalf("stale reconcile must run regardless of the enabled flag, calls=%d", reconciled)
	}
}

// ---------------------------------------------------------------------------
// 任务运行留痕（doc92 §7.2 / 验收场景 9、10）
// ---------------------------------------------------------------------------

type fakeJobRunRepo struct {
	rows        []*logmodel.JobRunLog
	startErr    error
	finishErr   error
	todayCount  int64
	startCalls  int
	finishCalls int
}

func (f *fakeJobRunRepo) Start(_ context.Context, row *logmodel.JobRunLog) error {
	f.startCalls++
	if f.startErr != nil {
		return f.startErr
	}
	row.ID = uint64(len(f.rows) + 1)
	cp := *row
	f.rows = append(f.rows, &cp)
	return nil
}
func (f *fakeJobRunRepo) Finish(_ context.Context, row *logmodel.JobRunLog) error {
	f.finishCalls++
	if f.finishErr != nil {
		return f.finishErr
	}
	for i := range f.rows {
		if f.rows[i].ID == row.ID {
			f.rows[i].Status = row.Status
			f.rows[i].ErrorMessage = row.ErrorMessage
		}
	}
	return nil
}
func (f *fakeJobRunRepo) CountToday(context.Context, string) (int64, error) {
	return f.todayCount, nil
}
func (f *fakeJobRunRepo) FailStaleRunning(context.Context, time.Time, string) (int64, error) {
	return 0, nil
}

// 场景 10：高频任务空轮不落库（否则 notify_delivery 15 秒一轮会攒上百万行）。
func TestRunnerSkipEmptyForHighFrequencyJob(t *testing.T) {
	repo := &fakeJobRunRepo{}
	r := NewRunner(repo, zap.NewNop(), RunnerOptions{SkipEmptyRuns: true})

	called := 0
	for i := 0; i < 10; i++ {
		r.Run(context.Background(), "notify_delivery", "notify", logmodel.TriggerScheduled,
			func(context.Context) (int, int, map[string]any, error) {
				called++
				return 0, 0, nil, nil
			})
	}
	if called != 10 {
		t.Fatalf("job body must still run every round, ran %d", called)
	}
	if len(repo.rows) != 0 {
		t.Fatalf("empty rounds must not write job_run_logs, wrote %d", len(repo.rows))
	}

	// 有处理对象时照常落库。
	r.Run(context.Background(), "notify_delivery", "notify", logmodel.TriggerScheduled,
		func(context.Context) (int, int, map[string]any, error) {
			return 3, 2, map[string]any{"sent": 2}, nil
		})
	if len(repo.rows) != 1 || repo.rows[0].Status != logmodel.JobStatusSuccess {
		t.Fatalf("non-empty round must be recorded, rows=%+v", repo.rows)
	}
}

// 失败轮次也落库（且带 error_message）—— 否则最难排查的情况反而没有日志。
func TestRunnerRecordsFailureForNonHighFrequencyJob(t *testing.T) {
	repo := &fakeJobRunRepo{}
	r := NewRunner(repo, zap.NewNop(), RunnerOptions{SkipEmptyRuns: false})

	r.Run(context.Background(), "sync_scan", "sync", logmodel.TriggerScheduled,
		func(context.Context) (int, int, map[string]any, error) {
			return 0, 0, nil, context.DeadlineExceeded
		})

	if len(repo.rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(repo.rows))
	}
	row := repo.rows[0]
	if row.Status != logmodel.JobStatusFailed || row.ErrorMessage == "" {
		t.Fatalf("failed run must be recorded with a message, got %+v", row)
	}
}

// 场景 9：上一轮仍在 running（唯一索引挡住）→ 本轮跳过，不产生第二条 running 行。
func TestRunnerSkipsWhenPreviousRunStillActive(t *testing.T) {
	repo := &fakeJobRunRepo{startErr: logrepo.ErrJobRunning}
	r := NewRunner(repo, zap.NewNop(), RunnerOptions{SkipEmptyRuns: false})

	r.Run(context.Background(), "sync_scan", "sync", logmodel.TriggerScheduled,
		func(context.Context) (int, int, map[string]any, error) {
			return 1, 1, nil, nil
		})

	if len(repo.rows) != 0 {
		t.Fatalf("reentrant round must not write a second running row, rows=%d", len(repo.rows))
	}
	if repo.finishCalls != 0 {
		t.Error("skipped round must not try to finish a row it never created")
	}
}

// 单日软上限：超过上限后本轮不写，防 bug 导致的死循环狂写。
func TestRunnerDailyCapSuppressesWrites(t *testing.T) {
	repo := &fakeJobRunRepo{todayCount: 500}
	r := NewRunner(repo, zap.NewNop(), RunnerOptions{SkipEmptyRuns: false, MaxRunsPerDay: 500})

	r.Run(context.Background(), "sync_scan", "sync", logmodel.TriggerScheduled,
		func(context.Context) (int, int, map[string]any, error) {
			return 1, 1, nil, nil
		})

	if repo.startCalls != 0 {
		t.Fatalf("daily cap must suppress the write, start calls=%d", repo.startCalls)
	}
}

// ---------------------------------------------------------------------------
// 场景 8/9 的另一面：进程中断遗留的 running 行必须能在启动时收尾，
// 否则该 job 之后每一轮都被判重入而永久静默停摆。
// ---------------------------------------------------------------------------

type staleJobRunRepo struct {
	fakeJobRunRepo
	lastStaleBefore time.Time
	staleRows       int64
}

func (s *staleJobRunRepo) FailStaleRunning(_ context.Context, before time.Time, _ string) (int64, error) {
	s.lastStaleBefore = before
	return s.staleRows, nil
}

func TestRunnerReconcileStaleRunning(t *testing.T) {
	repo := &staleJobRunRepo{staleRows: 2}
	r := NewRunner(repo, zap.NewNop(), RunnerOptions{})

	n, err := r.ReconcileStaleRunning(context.Background())
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 reconciled rows, got %d", n)
	}
	// 阈值必须是「约 6 小时前」，既不会误伤正在跑的任务，也能尽快恢复停摆的 job。
	age := time.Since(repo.lastStaleBefore)
	if age < 5*time.Hour || age > 7*time.Hour {
		t.Errorf("stale threshold should be ~6h, got %v", age)
	}
}
