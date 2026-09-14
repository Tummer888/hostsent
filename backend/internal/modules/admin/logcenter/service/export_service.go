package service

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/modules/admin/logcenter/catalog"
	"hostsent/backend/internal/modules/admin/logcenter/dto"
	logmodel "hostsent/backend/internal/modules/admin/logcenter/model"
	logrepo "hostsent/backend/internal/modules/admin/logcenter/repository"
	"hostsent/backend/internal/pkg/storage"
)

// 导出参数默认值。
const (
	// DefaultMaxExportRows 单次导出上限（防误操作把磁盘写满）；超限按水位线切分成多个文件。
	DefaultMaxExportRows = int64(5_000_000)
	// DefaultMaxExportFileBytes 单文件滚动阈值。
	DefaultMaxExportFileBytes = int64(512 << 20)
	// exportCursor 游标分页批量（避免 OFFSET 深分页）。
	exportCursor = 5000
)

// ErrExportUnavailable 存储不可用。
var ErrExportUnavailable = fmt.Errorf("%w: 导出存储不可用", ErrPolicyRejected)

// ExportService 导出服务（doc92 §5）。
type ExportService interface {
	// Export 人工导出（列表页触发），返回 file_id。
	Export(ctx context.Context, req dto.ExportRequest, operatorID uint64, operatorName string) (*dto.ExportResult, error)
	// CleanupExport 清理任务的备份导出（固定 jsonl，无损可回灌）。
	CleanupExport(ctx context.Context, src *catalog.Source, f logrepo.Filter, jobID uint64, operatorID uint64, operatorName string) (*logmodel.LogExportFile, error)
	// Verify 复核导出文件：重算 sha256 + 行数与 DB 比对（doc92 §6.1 verifying 阶段）。
	Verify(ctx context.Context, fileID uint64, src *catalog.Source, f logrepo.Filter) error
	// Open 打开导出文件供下载（调用方负责 Close）。
	Open(relPath string) (*os.File, error)
	// OpenByID 按记录 ID 打开导出文件（调用方负责 Close）。
	OpenByID(ctx context.Context, id uint64) (*os.File, error)
	// List 导出文件列表。
	List(ctx context.Context, q logrepo.ExportListQuery) (*dto.ExportFileListResponse, error)
	// Get 取单个文件元数据。
	Get(ctx context.Context, id uint64) (*dto.ExportFileInfo, error)
	// Delete 删除导出文件（清理任务产生的备份拒绝删除）。
	Delete(ctx context.Context, id uint64) error
	// PurgeExpired 清理过期的人工导出文件（只改状态，行永久保留）。
	PurgeExpired(ctx context.Context, limit int) (int, error)
}

type exportService struct {
	queryRepo    logrepo.LogQueryRepository
	exportRepo   logrepo.ExportRepository
	cleanupRepo  logrepo.CleanupRepository
	store        *storage.LocalStore
	exportDays   int
	maxRows      int64
	maxFileBytes int64
	logger       *zap.Logger
}

// ExportServiceOptions 导出服务参数。
type ExportServiceOptions struct {
	// ExportRootDir 存储子目录（默认 log-exports）。
	ExportRootDir string
	// RetentionDays 导出文件自身保留期（log_export_retention_days，默认 365）。
	RetentionDays int
	// MaxRows 单次导出上限。
	MaxRows int64
	// MaxFileBytes 单文件滚动阈值。
	MaxFileBytes int64
}

// NewExportService 创建导出服务。
func NewExportService(
	queryRepo logrepo.LogQueryRepository,
	exportRepo logrepo.ExportRepository,
	cleanupRepo logrepo.CleanupRepository,
	store *storage.LocalStore,
	opts ExportServiceOptions,
	logger *zap.Logger,
) ExportService {
	if opts.RetentionDays <= 0 {
		opts.RetentionDays = 365
	}
	if opts.MaxRows <= 0 {
		opts.MaxRows = DefaultMaxExportRows
	}
	if opts.MaxFileBytes <= 0 {
		opts.MaxFileBytes = DefaultMaxExportFileBytes
	}
	return &exportService{
		queryRepo: queryRepo, exportRepo: exportRepo, cleanupRepo: cleanupRepo,
		store: store, exportDays: opts.RetentionDays, maxRows: opts.MaxRows,
		maxFileBytes: opts.MaxFileBytes, logger: logger,
	}
}

// Export 人工导出：默认 csv（给人看），可选 jsonl（给机器看）。
//
// 流式写出（io.Pipe + goroutine 边查边写），千万行也不会把内存吃满；
// 边写边算 sha256，完成后回填 row_count/file_size/sha256。
func (s *exportService) Export(ctx context.Context, req dto.ExportRequest, operatorID uint64, operatorName string) (*dto.ExportResult, error) {
	if s.store == nil {
		return nil, ErrExportUnavailable
	}
	src := catalog.Get(req.Source)
	if src == nil {
		return nil, fmt.Errorf("%w: 未知日志源 %q", ErrPolicyRejected, req.Source)
	}
	format := normalizeFormat(req.Format)
	f := logrepo.Filter{Keyword: req.Keyword}
	if t, err := logrepo.ParseTime(req.From); err == nil && req.From != "" {
		f.From = t
	}
	if t, err := logrepo.ParseTime(req.To); err == nil && req.To != "" {
		f.To = t
	}
	if len(req.Filters) > 0 {
		allowed := map[string]bool{}
		for _, c := range src.SelectColumns {
			allowed[c.Key] = true
		}
		equals := map[string]string{}
		for param, value := range req.Filters {
			column, ok := src.FilterColumns[param]
			if ok && allowed[column] && strings.TrimSpace(value) != "" {
				equals[column] = strings.TrimSpace(value)
			}
		}
		f.Equals = equals
	}

	logicalName := buildFileName(src.Key, f.From, f.To, format)
	file, err := s.streamExport(ctx, src, f, format, logicalName, operatorID, operatorName, 0)
	if err != nil {
		return nil, err
	}
	return &dto.ExportResult{
		FileID: file.ID, FileName: file.FileName, RowCount: file.RowCount,
		FileSize: file.FileSize, SHA256: file.SHA256,
	}, nil
}

// CleanupExport 清理备份导出：固定 jsonl（无损、可回灌）。
//
// 备份范围必须与删除候选逐行一致，否则会出现「备份了删不到的」或更糟的
// 「删了没备份的」。范围归一化统一走 cleanupScope，Verify 用同一函数，
// 两边的 count 才可能相等（doc92 §10 场景 13：校验阶段比对行数）。
//
// 备份不完整就不允许进入删除阶段，所以这里任何错误都要向上抛（调用方整体失败）。
func (s *exportService) CleanupExport(ctx context.Context, src *catalog.Source, f logrepo.Filter, jobID uint64, operatorID uint64, operatorName string) (*logmodel.LogExportFile, error) {
	if s.store == nil {
		return nil, ErrExportUnavailable
	}
	logicalName := buildFileName(src.Key, time.Time{}, time.Time{}, "jsonl")
	return s.streamExport(ctx, src, cleanupScope(f), "jsonl", logicalName, operatorID, operatorName, jobID)
}

// cleanupScope 把筛选条件归一化成「删除候选范围」，与 DeleteBatch 的
// `time_column < watermark AND (delete_guard)` 逐行一致：
//
//   - 上界改开区间（`<`）—— 恰好等于水位线的那行删除不算，备份也不能算；
//   - 带上源的 DeleteGuard —— 清理只删终态行（pending/running 受保护）；
//   - 清空下界 —— 删除没有下界，备份必须覆盖整段历史。
func cleanupScope(f logrepo.Filter) logrepo.Filter {
	f.From = time.Time{}
	f.ToExclusive = true
	f.WithGuard = true
	return f
}

// streamExport 流式导出并落库元数据。
//
// 用 io.Pipe：导出 goroutine 边查边写，写文件那一侧用 MultiWriter 同时喂给
// sha256，全程不把结果集读进内存。
func (s *exportService) streamExport(
	ctx context.Context,
	src *catalog.Source,
	f logrepo.Filter,
	format, logicalName string,
	operatorID uint64,
	operatorName string,
	jobID uint64,
) (*logmodel.LogExportFile, error) {
	dir := "log-exports/" + src.Key
	pr, pw := io.Pipe()
	hasher := sha256.New()

	type writeResult struct {
		rel  string
		size int64
		rows int64
		err  error
	}
	done := make(chan writeResult, 1)
	go func() {
		// Save 在读端：它一旦提前返回（目录不可写、磁盘满、建目录失败），
		// 下面 writeRows 仍会往管道里写，写满管道缓冲后永久阻塞 —— 任务就会
		// 卡在 exporting 且连取消都无效（写操作不监听 ctx）。收尾时关掉读端，
		// 让挂起的写立刻收到 ErrClosedPipe，导出失败得以正常上抛。
		defer pr.Close()
		rel, size, err := s.store.Save(dir, logicalName, io.TeeReader(pr, hasher), 0)
		done <- writeResult{rel: rel, size: size, err: err}
	}()

	rows, writeErr := s.writeRows(ctx, src, f, format, pw)
	// 关闭写端让 Save 收尾；若写行已有错误也要关，避免 Save 永久阻塞。
	_ = pw.CloseWithError(writeErr)
	res := <-done
	// 存储侧错误优先：它才是根因，写侧拿到的往往只是被连累的 broken pipe。
	if res.err != nil {
		if res.rel != "" {
			_ = s.store.Remove(res.rel)
		}
		return nil, fmt.Errorf("logcenter: 写入导出文件失败: %w", res.err)
	}
	if writeErr != nil {
		if res.rel != "" {
			_ = s.store.Remove(res.rel)
		}
		return nil, writeErr
	}

	expires := time.Now().AddDate(0, 0, s.exportDays)
	row := &logmodel.LogExportFile{
		SourceKey: src.Key, JobID: jobID, Format: format,
		StoragePath: res.rel, FileName: logicalName,
		FileSize: res.size, RowCount: rows,
		SHA256:     hex.EncodeToString(hasher.Sum(nil)),
		OperatorID: operatorID, OperatorName: operatorName,
		Status: logmodel.ExportStatusSuccess, ExpiresAt: &expires,
	}
	row.PeriodTo = periodPtr(f.To)
	row.PeriodFrom = periodPtr(f.From)
	if err := s.exportRepo.Create(ctx, row); err != nil {
		// 元数据落库失败 → 文件成为孤儿，删掉避免磁盘泄漏。
		_ = s.store.Remove(res.rel)
		return nil, err
	}
	return row, nil
}

// writeRows 游标分页读行并写进 pipe（csv 或 jsonl）。
func (s *exportService) writeRows(ctx context.Context, src *catalog.Source, f logrepo.Filter, format string, w io.Writer) (int64, error) {
	var (
		lastID uint64
		total  int64
	)
	csvWriter := csv.NewWriter(w)
	// UTF-8 BOM：Excel 打开中文 CSV 不乱码（doc92 §5.1）。
	if format == "csv" {
		if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
			return 0, err
		}
		header := make([]string, 0, len(src.SelectColumns))
		for _, c := range src.SelectColumns {
			header = append(header, c.Label)
		}
		if err := csvWriter.Write(header); err != nil {
			return 0, err
		}
		csvWriter.Flush()
		if err := csvWriter.Error(); err != nil {
			return 0, err
		}
	}

	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		batch, err := s.queryRepo.ExportCursor(ctx, src, f, lastID, exportCursor)
		if err != nil {
			return total, err
		}
		if len(batch) == 0 {
			break
		}
		for _, row := range batch {
			if err := writeOne(w, csvWriter, src, row, format); err != nil {
				return total, err
			}
			total++
			if total > s.maxRows {
				return total, fmt.Errorf("logcenter: 导出超过单次上限 %d 行，请缩小时间范围后重试", s.maxRows)
			}
		}
		if format == "csv" {
			csvWriter.Flush()
			if err := csvWriter.Error(); err != nil {
				return total, err
			}
		}
		if id, ok := toUint64(batch[len(batch)-1]["id"]); ok {
			lastID = id
		} else {
			break // 没有 id 无法续游标，直接结束（避免死循环）
		}
		if len(batch) < exportCursor {
			break
		}
	}
	return total, nil
}

// writeOne 写一行（csv 按列顺序，jsonl 完整保留原始字段名）。
func writeOne(w io.Writer, csvWriter *csv.Writer, src *catalog.Source, row map[string]any, format string) error {
	if format == "jsonl" {
		// jsonl 给机器看：字段名保持原始（便于程序化回灌/比对）。
		encoded, err := json.Marshal(row)
		if err != nil {
			return err
		}
		if _, err := w.Write(append(encoded, '\n')); err != nil {
			return err
		}
		return nil
	}
	record := make([]string, 0, len(src.SelectColumns))
	for _, c := range src.SelectColumns {
		record = append(record, cellString(row[c.Key]))
	}
	return csvWriter.Write(record)
}

// cellString 把任意值转成 CSV 单元格字符串。
func cellString(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case []byte:
		return string(val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case time.Time:
		return fmtTime(val)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(val, 10)
	default:
		encoded, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(encoded)
	}
}

// Verify 复核导出文件：sha256 重算 + 行数比对。
//
// 这一步是「先导出后删除」的强制点：校验不过 → 任务失败在 verifying，一行都不删。
func (s *exportService) Verify(ctx context.Context, fileID uint64, src *catalog.Source, f logrepo.Filter) error {
	if s.store == nil {
		return ErrExportUnavailable
	}
	row, err := s.exportRepo.FindByID(ctx, fileID)
	if err != nil {
		return err
	}
	if row == nil {
		return fmt.Errorf("logcenter: 导出文件 %d 不存在", fileID)
	}
	file, err := s.store.Open(row.StoragePath)
	if err != nil {
		return fmt.Errorf("logcenter: 打开导出文件失败: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	size, err := io.Copy(hasher, file)
	if err != nil {
		return fmt.Errorf("logcenter: 读取导出文件失败: %w", err)
	}
	if got := hex.EncodeToString(hasher.Sum(nil)); got != row.SHA256 {
		return fmt.Errorf("logcenter: 导出文件 sha256 校验失败（期望 %s，实际 %s）", row.SHA256, got)
	}
	if size != row.FileSize {
		return fmt.Errorf("logcenter: 导出文件大小不符（期望 %d，实际 %d）", row.FileSize, size)
	}
	// 行数交叉校验：与「将要被删的候选行数」比对，确保备份真的覆盖了删除范围。
	if src != nil {
		expected, err := s.queryRepo.Count(ctx, src, cleanupScope(f))
		if err != nil {
			return err
		}
		if expected != row.RowCount {
			return fmt.Errorf("logcenter: 导出行数与候选行数不符（备份 %d，候选 %d）", row.RowCount, expected)
		}
	}
	return nil
}

// Open 打开导出文件（下载用）。
func (s *exportService) Open(relPath string) (*os.File, error) {
	if s.store == nil {
		return nil, ErrExportUnavailable
	}
	return s.store.Open(relPath)
}

// OpenByID 按记录 ID 打开导出文件（下载用）。
func (s *exportService) OpenByID(ctx context.Context, id uint64) (*os.File, error) {
	row, err := s.exportRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, fmt.Errorf("%w: 导出文件不存在", ErrPolicyRejected)
	}
	if row.Status == logmodel.ExportStatusDeleted {
		return nil, fmt.Errorf("%w: 导出文件已删除", ErrPolicyRejected)
	}
	return s.Open(row.StoragePath)
}

// List 导出文件列表。
func (s *exportService) List(ctx context.Context, q logrepo.ExportListQuery) (*dto.ExportFileListResponse, error) {
	page, pageSize := normalizePage(q.Page, q.PageSize)
	q.Page, q.PageSize = page, pageSize
	rows, total, err := s.exportRepo.List(ctx, q)
	if err != nil {
		return nil, err
	}
	items := make([]dto.ExportFileInfo, 0, len(rows))
	for i := range rows {
		items = append(items, toExportFileInfo(&rows[i]))
	}
	return &dto.ExportFileListResponse{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// Get 单个文件元数据。
func (s *exportService) Get(ctx context.Context, id uint64) (*dto.ExportFileInfo, error) {
	row, err := s.exportRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	info := toExportFileInfo(row)
	return &info, nil
}

// Delete 删除导出文件。
//
// 清理任务产生的备份是「删了哪些数据」的唯一凭据，永久保留 —— 这里拒绝删除
// （doc92 §8.3 与硬约束 7）。
func (s *exportService) Delete(ctx context.Context, id uint64) error {
	row, err := s.exportRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if row == nil {
		return fmt.Errorf("%w: 导出文件不存在", ErrPolicyRejected)
	}
	if row.JobID != 0 {
		return fmt.Errorf("%w: 该文件是清理动作的备份凭据，不可删除", ErrPolicyRejected)
	}
	if row.Status == logmodel.ExportStatusDeleted {
		return nil
	}
	if s.store != nil && row.StoragePath != "" {
		if err := s.store.Remove(row.StoragePath); err != nil {
			return err
		}
	}
	// 只改状态，行永不清理。
	return s.exportRepo.MarkDeleted(ctx, id)
}

// PurgeExpired 清理过期的人工导出文件（doc92 §5.4）。
func (s *exportService) PurgeExpired(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.exportRepo.ExpiredUnreferenced(ctx, time.Now(), limit)
	if err != nil {
		return 0, err
	}
	removed := 0
	for i := range rows {
		if s.store != nil && rows[i].StoragePath != "" {
			if err := s.store.Remove(rows[i].StoragePath); err != nil {
				s.logger.Warn("logcenter: remove expired export file failed",
					zap.Uint64("id", rows[i].ID), zap.Error(err))
				continue
			}
		}
		if err := s.exportRepo.MarkDeleted(ctx, rows[i].ID); err != nil {
			s.logger.Warn("logcenter: mark expired export deleted failed",
				zap.Uint64("id", rows[i].ID), zap.Error(err))
			continue
		}
		removed++
	}
	return removed, nil
}

// toExportFileInfo 元数据 → DTO。
func toExportFileInfo(row *logmodel.LogExportFile) dto.ExportFileInfo {
	src := catalog.Get(row.SourceKey)
	name := row.SourceKey
	if src != nil {
		name = src.DisplayName
	}
	return dto.ExportFileInfo{
		ID: row.ID, SourceKey: row.SourceKey, DisplayName: name, JobID: row.JobID,
		Format: row.Format, FileName: row.FileName, FileSize: row.FileSize,
		RowCount: row.RowCount, SHA256: row.SHA256,
		PeriodFrom: fmtTimePtr(row.PeriodFrom), PeriodTo: fmtTimePtr(row.PeriodTo),
		OperatorID: row.OperatorID, OperatorName: row.OperatorName,
		Status: row.Status, FailReason: row.FailReason,
		ExpiresAt: fmtTimePtr(row.ExpiresAt), CreatedAt: fmtTime(row.CreatedAt),
		BoundJob: row.JobID != 0,
	}
}

// normalizeFormat 导出格式收敛（默认 csv）。
func normalizeFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "jsonl", "json":
		return "jsonl"
	default:
		return "csv"
	}
}

// buildFileName 生成逻辑文件名（下载时还原该名字）。
func buildFileName(sourceKey string, from, to time.Time, ext string) string {
	stamp := time.Now().Format("20060102-150405")
	left, right := "all", "now"
	if !from.IsZero() {
		left = from.Format("20060102")
	}
	if !to.IsZero() {
		right = to.Format("20060102")
	}
	return fmt.Sprintf("%s_%s_%s_%s.%s", sourceKey, left, right, stamp, ext)
}

// periodPtr 零值时间转 nil（列可空）。
func periodPtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	v := t
	return &v
}

// toUint64 把任意数值列转 uint64。
func toUint64(v any) (uint64, bool) {
	switch val := v.(type) {
	case uint64:
		return val, true
	case int64:
		if val < 0 {
			return 0, false
		}
		return uint64(val), true
	case int32:
		if val < 0 {
			return 0, false
		}
		return uint64(val), true
	case int:
		if val < 0 {
			return 0, false
		}
		return uint64(val), true
	case string:
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}
