package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/modules/admin/logcenter/catalog"
	"hostsent/backend/internal/modules/admin/logcenter/dto"
	logmodel "hostsent/backend/internal/modules/admin/logcenter/model"
	logrepo "hostsent/backend/internal/modules/admin/logcenter/repository"
)

// CleanupService 清理引擎（doc92 §6）—— 本模块的核心。
//
// 状态机：pending → exporting → verifying → deleting → done，
// 任意非终态可 → cancelled，任意阶段出错 → failed（已删批次不回滚，但有备份可回灌）。
type CleanupService interface {
	Preview(ctx context.Context, req dto.CleanupPreviewRequest) (*dto.CleanupPreviewResponse, error)
	// Start 创建任务并异步执行；返回任务信息。
	Start(ctx context.Context, req dto.CleanupRunRequest, triggerType string, operatorID uint64, operatorName string, exportBeforeDelete bool) (*dto.CleanupJobInfo, error)
	// RunScheduled 调度入口（当日只跑一次，由调用方判定）。
	RunScheduled(ctx context.Context, exportBeforeDelete bool) error
	List(ctx context.Context, q CleanupListQuery) (*dto.CleanupJobListResponse, error)
	Detail(ctx context.Context, id uint64) (*dto.CleanupJobDetailResponse, error)
	Cancel(ctx context.Context, id uint64) error
	// HasRunToday 当天是否已有调度任务。
	HasRunToday(ctx context.Context) (bool, error)
}

// CleanupListQuery 清理任务列表查询参数。
type CleanupListQuery struct {
	Status    string
	Trigger   string
	StartTime string
	EndTime   string
	Page      int
	PageSize  int
}

// CleanupServiceOptions 清理服务参数。
type CleanupServiceOptions struct {
	// ExportBeforeDelete 删除前必须导出（log_export_before_delete，默认 true）。
	ExportBeforeDelete bool
	// BatchSize 默认批次（策略行未配置时用）。
	BatchSize int
	// ArchiveCronOverride 归档节奏由策略行决定，这里不用。
	// Locker 可选分布式锁（Redis 可用时装配）；nil 时仅靠部分唯一索引互斥。
	Locker Locker
}

// Locker 分布式锁端口（Redis 实现由装配层注入；不可用时传 nil）。
type Locker interface {
	// TryLock 尝试获取锁；成功返回释放函数与 true。
	TryLock(ctx context.Context, key string, ttl time.Duration) (func(), bool)
}

type cleanupService struct {
	queryRepo   logrepo.LogQueryRepository
	cleanupRepo logrepo.CleanupRepository
	exportRepo  logrepo.ExportRepository
	exportSvc   ExportService
	policySvc   PolicyService
	jobRepo     logrepo.JobRunRepository
	opts        CleanupServiceOptions
	logger      *zap.Logger

	// cancel 支持「清理过程中点取消」（doc92 场景 24）。
	mu      sync.Mutex
	cancels map[uint64]context.CancelFunc
}

// NewCleanupService 创建清理引擎。
func NewCleanupService(
	queryRepo logrepo.LogQueryRepository,
	cleanupRepo logrepo.CleanupRepository,
	exportRepo logrepo.ExportRepository,
	exportSvc ExportService,
	policySvc PolicyService,
	jobRepo logrepo.JobRunRepository,
	opts CleanupServiceOptions,
	logger *zap.Logger,
) CleanupService {
	if opts.BatchSize <= 0 {
		opts.BatchSize = 5000
	}
	return &cleanupService{
		queryRepo: queryRepo, cleanupRepo: cleanupRepo, exportRepo: exportRepo,
		exportSvc: exportSvc, policySvc: policySvc, jobRepo: jobRepo,
		opts: opts, logger: logger, cancels: map[uint64]context.CancelFunc{},
	}
}

// errConfirmRequired 高危操作确认文本不符。
var errConfirmRequired = fmt.Errorf("%w: 需输入 DELETE 确认执行清理", ErrPolicyRejected)

// Preview 预演：算候选行数、最早/最晚时间、预估文件大小，不导出不删除。
func (s *cleanupService) Preview(ctx context.Context, req dto.CleanupPreviewRequest) (*dto.CleanupPreviewResponse, error) {
	sources, err := s.resolveSources(req.Sources)
	if err != nil {
		return nil, err
	}
	requested, err := parseOptionalTime(req.BeforeTime)
	if err != nil {
		return nil, err
	}
	resp := &dto.CleanupPreviewResponse{
		ExportBeforeDelete: s.opts.ExportBeforeDelete,
		Items:              make([]dto.CleanupPreviewItem, 0, len(sources)),
	}
	for _, src := range sources {
		policy, err := s.policySvc.Get(ctx, src.Key)
		if err != nil {
			return nil, err
		}
		before := s.policySvc.ResolveWatermark(ctx, src, requested)
		if policy.Action != catalog.ActionClean {
			// 只读备份的源不参与删除，但仍在预演里列出来（让运营看清「谁不会被清」）。
			continue
		}
		total, err := s.queryRepo.CountBefore(ctx, src, before, false)
		if err != nil {
			return nil, err
		}
		eligible, err := s.queryRepo.CountBefore(ctx, src, before, true)
		if err != nil {
			return nil, err
		}
		from, to, err := s.queryRepo.RangeBefore(ctx, src, before)
		if err != nil {
			return nil, err
		}
		item := dto.CleanupPreviewItem{
			SourceKey: src.Key, DisplayName: src.DisplayName, Action: policy.Action,
			RetentionDays: policy.RetentionDays, BeforeTime: fmtTime(before),
			TotalRows: total, EligibleRows: eligible, GuardedRows: total - eligible,
			EarliestTime: fmtTimePtr(from), LatestTime: fmtTimePtr(to),
			EstimatedBytes: estimateBytes(src, eligible),
			Enabled:        policy.Enabled,
		}
		resp.Items = append(resp.Items, item)
		resp.EligibleRows += eligible
		resp.GuardedRows += item.GuardedRows
	}
	if resp.EligibleRows == 0 {
		resp.Message = "无满足条件的记录，无需清理"
	}
	return resp, nil
}

// estimateBytes 预估 jsonl 备份体积（保守按每行 ~512B 估）。
func estimateBytes(src *catalog.Source, rows int64) int64 {
	perRow := int64(256)
	for _, c := range src.SelectColumns {
		switch c.Type {
		case "longtext", "json":
			perRow += 256
		default:
			perRow += 32
		}
	}
	return rows * perRow
}

// Start 创建任务并异步执行。
//
// 三重防护（doc92 §0.3）：
//  1. export_before_delete=false 时直接拒绝执行（不导出就不允许删）；
//  2. 必须输入 DELETE 确认；
//  3. 已有非终态任务时拒绝（互斥，Redis 锁 + 部分唯一索引双保险）。
func (s *cleanupService) Start(ctx context.Context, req dto.CleanupRunRequest, triggerType string, operatorID uint64, operatorName string, exportBeforeDelete bool) (*dto.CleanupJobInfo, error) {
	if strings.TrimSpace(strings.ToUpper(req.ConfirmText)) != "DELETE" && triggerType == logmodel.TriggerManual {
		return nil, errConfirmRequired
	}
	if !exportBeforeDelete {
		return nil, fmt.Errorf("%w: 已关闭删除前导出（log_export_before_delete=false），为安全起见拒绝执行清理", ErrPolicyRejected)
	}
	sources, err := s.resolveSources(req.Sources)
	if err != nil {
		return nil, err
	}
	requested, err := parseOptionalTime(req.BeforeTime)
	if err != nil {
		return nil, err
	}
	// 互斥：先试 Redis 锁（多实例），失败再靠 DB 部分唯一索引兜底。
	var release func()
	if s.opts.Locker != nil {
		release, ok := s.opts.Locker.TryLock(ctx, "log:cleanup:lock", 30*time.Minute)
		if !ok {
			return nil, fmt.Errorf("%w: 已有清理任务在执行", ErrPolicyRejected)
		}
		defer func() {
			if release != nil {
				release()
			}
		}()
	}
	_ = release
	if active, err := s.cleanupRepo.HasActive(ctx); err != nil {
		return nil, err
	} else if active {
		return nil, fmt.Errorf("%w: 已有清理任务在执行", ErrPolicyRejected)
	}

	// 水位线取所有参与源中最保守（最早）的那个，写入任务供页面展示。
	watermark := requested
	if watermark.IsZero() {
		for _, src := range sources {
			w := s.policySvc.ResolveWatermark(ctx, src, requested)
			if watermark.IsZero() || w.Before(watermark) {
				watermark = w
			}
		}
	}
	keys := make([]string, 0, len(sources))
	for _, src := range sources {
		keys = append(keys, src.Key)
	}
	row := &logmodel.LogCleanupJob{
		JobNo: jobNo(time.Now()), TriggerType: triggerType, Status: logmodel.CleanupStatusPending,
		Sources: encodeStrings(keys), BeforeTime: watermark, DryRun: false,
		ExportRequired: exportBeforeDelete, OperatorID: operatorID, OperatorName: operatorName,
	}
	if err := s.cleanupRepo.Create(ctx, row); err != nil {
		if errors.Is(err, logrepo.ErrJobRunning) {
			return nil, fmt.Errorf("%w: 已有清理任务在执行", ErrPolicyRejected)
		}
		return nil, err
	}

	// 异步执行：请求返回后任务继续跑，页面轮询进度。
	runCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancels[row.ID] = cancel
	s.mu.Unlock()
	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.cancels, row.ID)
			s.mu.Unlock()
			cancel()
		}()
		if err := s.run(runCtx, row.ID); err != nil {
			s.logger.Warn("logcenter cleanup failed", zap.Uint64("job_id", row.ID), zap.Error(err))
		}
	}()
	info := toCleanupJobInfo(row)
	return &info, nil
}

// RunScheduled 调度入口：按当前策略对所有 action=clean/archive_only 的源跑一轮。
func (s *cleanupService) RunScheduled(ctx context.Context, exportBeforeDelete bool) error {
	cleanKeys := catalog.CleanableKeys()
	archiveKeys := make([]string, 0, 8)
	for _, src := range catalog.All() {
		if src.Cleanable {
			continue
		}
		policy, err := s.policySvc.Get(ctx, src.Key)
		if err == nil && policy.Action == catalog.ActionArchiveOnly && policy.Enabled {
			archiveKeys = append(archiveKeys, src.Key)
		}
	}
	// archive_only 的源先做冷备（导出但不删），失败不阻断 clean 阶段。
	if len(archiveKeys) > 0 {
		if err := s.archiveOnly(ctx, archiveKeys); err != nil {
			s.logger.Warn("logcenter archive-only failed", zap.Error(err))
		}
	}
	if len(cleanKeys) == 0 {
		return nil
	}
	_, err := s.Start(ctx, dto.CleanupRunRequest{Sources: cleanKeys, ConfirmText: "DELETE"},
		logmodel.TriggerScheduled, 0, "scheduler", exportBeforeDelete)
	return err
}

// archiveOnly 对 audit 类源做「导出但不删」的冷备（doc92 §4.2 例外 2）。
func (s *cleanupService) archiveOnly(ctx context.Context, keys []string) error {
	for _, key := range keys {
		src := catalog.Get(key)
		if src == nil {
			continue
		}
		policy, err := s.policySvc.Get(ctx, key)
		if err != nil {
			continue
		}
		// 归档节奏：monthly 表示每月一次，已有当月归档文件则跳过。
		if policy.ArchiveCron == "monthly" && s.archivedThisMonth(ctx, key) {
			continue
		}
		before := time.Now().AddDate(0, 0, -policy.RetentionDays)
		rows, err := s.queryRepo.CountBefore(ctx, src, before, false)
		if err != nil || rows == 0 {
			continue
		}
		file, err := s.exportSvc.CleanupExport(ctx, src, logrepo.Filter{To: before}, 0, 0, "scheduler")
		if err != nil {
			s.logger.Warn("logcenter archive export failed", zap.String("source", key), zap.Error(err))
			continue
		}
		// 注意：这里刻意不删任何行 —— audit 类是合规基线。
		s.logger.Info("logcenter archived audit source",
			zap.String("source", key), zap.Uint64("file_id", file.ID), zap.Int64("rows", file.RowCount))
		if err := s.policySvc.(*policyService).repo.MarkRun(ctx, key, time.Now(), 0); err != nil {
			s.logger.Warn("logcenter mark archive run failed", zap.String("source", key), zap.Error(err))
		}
	}
	return nil
}

// archivedThisMonth 当月是否已有该源的归档文件。
func (s *cleanupService) archivedThisMonth(ctx context.Context, sourceKey string) bool {
	files, _, err := s.exportRepo.List(ctx, logrepo.ExportListQuery{
		SourceKey: sourceKey, Page: 1, PageSize: 5,
	})
	if err != nil || len(files) == 0 {
		return false
	}
	now := time.Now()
	latest := files[0].CreatedAt
	return latest.Year() == now.Year() && latest.Month() == now.Month()
}

// run 逐源串行执行（doc92 §6.3）。
func (s *cleanupService) run(ctx context.Context, jobID uint64) error {
	job, err := s.cleanupRepo.FindByID(ctx, jobID)
	if err != nil {
		return err
	}
	if job == nil {
		return fmt.Errorf("logcenter: 清理任务 %d 不存在", jobID)
	}
	started := time.Now()
	job.StartedAt = &started
	job.Status = logmodel.CleanupStatusPending
	if err := s.cleanupRepo.Update(ctx, job); err != nil {
		return err
	}

	keys := decodeStrings(job.Sources)
	total := len(keys)
	for i, key := range keys {
		if err := ctx.Err(); err != nil {
			return s.finishCancelled(ctx, job)
		}
		src := catalog.Get(key)
		if src == nil {
			return s.fail(ctx, job, fmt.Errorf("未知日志源 %q", key))
		}
		// 双保险：API 层已拦一次，执行期再拦一次（防手工改库塞进不可清源）。
		if !src.Cleanable {
			return s.fail(ctx, job, fmt.Errorf("源 %s 属于 %s 类，不允许清理", key, src.Class))
		}
		policy, err := s.policySvc.Get(ctx, key)
		if err != nil {
			return s.fail(ctx, job, err)
		}
		job.CurrentSource = key
		job.Progress = progressOf(i, total)
		_ = s.cleanupRepo.Update(ctx, job)
		if !policy.Enabled || policy.Action != catalog.ActionClean {
			continue
		}

		before := s.policySvc.ResolveWatermark(ctx, src, job.BeforeTime)
		eligible, err := s.queryRepo.CountBefore(ctx, src, before, true)
		if err != nil {
			return s.fail(ctx, job, err)
		}
		if eligible == 0 {
			continue
		}
		job.ScannedRows += eligible

		// ① 导出 + 校验：导不出来就整体失败，一行都不删（硬约束 3）。
		if job.ExportRequired && !job.DryRun {
			job.Status = logmodel.CleanupStatusExporting
			_ = s.cleanupRepo.Update(ctx, job)
			file, err := s.exportSvc.CleanupExport(ctx, src, logrepo.Filter{To: before}, job.ID, job.OperatorID, job.OperatorName)
			if err != nil {
				return s.fail(ctx, job, fmt.Errorf("导出 %s 失败：%w", key, err))
			}
			job.Status = logmodel.CleanupStatusVerifying
			_ = s.cleanupRepo.Update(ctx, job)
			if err := s.exportSvc.Verify(ctx, file.ID, src, logrepo.Filter{To: before}); err != nil {
				return s.fail(ctx, job, fmt.Errorf("校验 %s 备份失败：%w", key, err))
			}
			job.ExportFileIDs = encodeUint64s(append(decodeUint64s(job.ExportFileIDs), file.ID))
			_ = s.cleanupRepo.Update(ctx, job)
		} else if !job.DryRun {
			// export_required=false 只能来自「配置被明确关掉」，此时必须拒绝执行。
			return s.fail(ctx, job, fmt.Errorf("已关闭删除前导出，为安全起见拒绝执行清理"))
		}

		if job.DryRun {
			continue
		}

		// ② 分批删除（每批独立事务，批间可取消）。
		job.Status = logmodel.CleanupStatusDeleting
		_ = s.cleanupRepo.Update(ctx, job)
		batch := policy.BatchSize
		if batch <= 0 {
			batch = s.opts.BatchSize
		}
		var deleted int64
		for {
			if err := ctx.Err(); err != nil {
				job.DeletedRows += deleted
				return s.finishCancelled(ctx, job)
			}
			n, err := s.queryRepo.DeleteBatch(ctx, src, before, batch)
			if err != nil {
				job.DeletedRows += deleted
				return s.fail(ctx, job, fmt.Errorf("删除 %s 失败：%w", key, err))
			}
			deleted += n
			job.DeletedRows += n
			_ = s.cleanupRepo.Update(ctx, job)
			if n == 0 {
				break
			}
		}
		if deleted != eligible {
			// 行数对不上：可能被并发写入了新行，或水位线算错 —— 要留痕告警。
			job.ExpiredRows += eligible - deleted
			s.logger.Warn("logcenter cleanup row mismatch",
				zap.String("source", key), zap.Int64("eligible", eligible), zap.Int64("deleted", deleted))
		}
		if err := policyMarkRun(s.policySvc, ctx, key, deleted); err != nil {
			s.logger.Warn("logcenter mark policy run failed", zap.String("source", key), zap.Error(err))
		}
	}

	finished := time.Now()
	job.Status = logmodel.CleanupStatusDone
	job.Progress = 100
	job.CurrentSource = ""
	job.FinishedAt = &finished
	return s.cleanupRepo.Update(ctx, job)
}

// fail 记录失败原因并落终态（已删批次不回滚，但备份仍在，可回灌）。
func (s *cleanupService) fail(ctx context.Context, job *logmodel.LogCleanupJob, cause error) error {
	finished := time.Now()
	job.Status = logmodel.CleanupStatusFailed
	job.ErrorMessage = truncate(cause.Error(), 1000)
	job.FinishedAt = &finished
	// 失败也要落终态，否则部分唯一索引会把后续任务永久挡住。
	if updateErr := s.cleanupRepo.Update(ctx, job); updateErr != nil {
		s.logger.Error("logcenter: persist failed cleanup job failed",
			zap.Uint64("job_id", job.ID), zap.Error(updateErr))
	}
	return cause
}

// finishCancelled 取消：停在当前批次，已删批次不回滚。
func (s *cleanupService) finishCancelled(ctx context.Context, job *logmodel.LogCleanupJob) error {
	finished := time.Now()
	job.Status = logmodel.CleanupStatusCancelled
	job.FinishedAt = &finished
	job.ErrorMessage = truncate("任务已取消", 1000)
	return s.cleanupRepo.Update(context.WithoutCancel(ctx), job)
}

// List 任务列表。
func (s *cleanupService) List(ctx context.Context, q CleanupListQuery) (*dto.CleanupJobListResponse, error) {
	page, pageSize := normalizePage(q.Page, q.PageSize)
	rows, total, err := s.cleanupRepo.List(ctx, logrepo.CleanupListQuery{
		Status: q.Status, Trigger: q.Trigger, StartTime: q.StartTime, EndTime: q.EndTime,
		Page: page, PageSize: pageSize,
	})
	if err != nil {
		return nil, err
	}
	items := make([]dto.CleanupJobInfo, 0, len(rows))
	for i := range rows {
		items = append(items, toCleanupJobInfo(&rows[i]))
	}
	return &dto.CleanupJobListResponse{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// Detail 任务详情（含导出文件列表）。
func (s *cleanupService) Detail(ctx context.Context, id uint64) (*dto.CleanupJobDetailResponse, error) {
	row, err := s.cleanupRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	resp := &dto.CleanupJobDetailResponse{Job: toCleanupJobInfo(row)}
	fileIDs := decodeUint64s(row.ExportFileIDs)
	for _, fid := range fileIDs {
		info, err := s.exportSvc.Get(ctx, fid)
		if err != nil || info == nil {
			continue
		}
		resp.ExportFiles = append(resp.ExportFiles, *info)
	}
	return resp, nil
}

// Cancel 取消非终态任务。
func (s *cleanupService) Cancel(ctx context.Context, id uint64) error {
	row, err := s.cleanupRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if row == nil {
		return fmt.Errorf("%w: 清理任务不存在", ErrPolicyRejected)
	}
	if !logmodel.IsActiveStatus(row.Status) {
		return fmt.Errorf("%w: 任务已结束，无法取消", ErrPolicyRejected)
	}
	s.mu.Lock()
	cancel := s.cancels[id]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		return nil
	}
	// 进程重启后内存里没有 cancel：直接把状态置为 cancelled（执行 goroutine 早已不存在）。
	finished := time.Now()
	row.Status = logmodel.CleanupStatusCancelled
	row.FinishedAt = &finished
	row.ErrorMessage = "任务已取消"
	return s.cleanupRepo.Update(ctx, row)
}

// HasRunToday 当天是否已有调度任务（doc92 §6.6 的每日语义）。
func (s *cleanupService) HasRunToday(ctx context.Context) (bool, error) {
	return s.cleanupRepo.HasScheduledToday(ctx)
}

// resolveSources 解析请求里的源列表；为空时取全部可清理源。
func (s *cleanupService) resolveSources(keys []string) ([]*catalog.Source, error) {
	if len(keys) == 0 {
		keys = catalog.CleanableKeys()
	}
	out := make([]*catalog.Source, 0, len(keys))
	for _, key := range keys {
		src := catalog.Get(key)
		if src == nil {
			return nil, fmt.Errorf("%w: 未知日志源 %q", ErrPolicyRejected, key)
		}
		if !src.Cleanable {
			return nil, fmt.Errorf("%w: 源 %s 属于 %s 类，不允许清理", ErrPolicyRejected, key, src.Class)
		}
		out = append(out, src)
	}
	return out, nil
}

// policyMarkRun 回填策略行的上次执行信息。
func policyMarkRun(svc PolicyService, ctx context.Context, sourceKey string, deleted int64) error {
	impl, ok := svc.(*policyService)
	if !ok || impl == nil {
		return nil
	}
	return impl.repo.MarkRun(ctx, sourceKey, time.Now(), deleted)
}

// parseOptionalTime 解析可空时间参数。
func parseOptionalTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	return logrepo.ParseTime(raw)
}

// jobNo 生成任务号：CLN20260913001。
func jobNo(now time.Time) string {
	return fmt.Sprintf("CLN%s%03d", now.Format("20060102"), now.UnixNano()%1000)
}

// progressOf 计算进度百分比。
func progressOf(index, total int) int {
	if total <= 0 {
		return 0
	}
	p := index * 100 / total
	if p > 99 {
		p = 99 // 100 只在全部完成时出现
	}
	return p
}

// toCleanupJobInfo 任务模型 → DTO。
func toCleanupJobInfo(row *logmodel.LogCleanupJob) dto.CleanupJobInfo {
	return dto.CleanupJobInfo{
		ID: row.ID, JobNo: row.JobNo, TriggerType: row.TriggerType,
		Status: row.Status, StatusLabel: StatusLabel(row.Status),
		Sources: decodeStrings(row.Sources), BeforeTime: fmtTime(row.BeforeTime),
		DryRun: row.DryRun, ExportRequired: row.ExportRequired,
		ExportFileIDs: decodeUint64s(row.ExportFileIDs),
		ScannedRows:   row.ScannedRows, DeletedRows: row.DeletedRows,
		ExpiredRows: row.ExpiredRows, Progress: row.Progress,
		CurrentSource: row.CurrentSource, ErrorMessage: row.ErrorMessage,
		OperatorID: row.OperatorID, OperatorName: row.OperatorName,
		StartedAt: fmtTimePtr(row.StartedAt), FinishedAt: fmtTimePtr(row.FinishedAt),
		CreatedAt: fmtTime(row.CreatedAt), Active: logmodel.IsActiveStatus(row.Status),
	}
}

// StatusLabel 清理任务状态中文标签（前端不渲染英文，doc92 §9.3）。
func StatusLabel(status string) string {
	switch status {
	case logmodel.CleanupStatusPending:
		return "待执行"
	case logmodel.CleanupStatusExporting:
		return "导出中"
	case logmodel.CleanupStatusVerifying:
		return "校验中"
	case logmodel.CleanupStatusDeleting:
		return "删除中"
	case logmodel.CleanupStatusDone:
		return "已完成"
	case logmodel.CleanupStatusFailed:
		return "失败"
	case logmodel.CleanupStatusCancelled:
		return "已取消"
	default:
		return status
	}
}
