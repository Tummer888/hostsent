// Package repository 提供日志中心的数据访问（doc92 L1–L6）。
//
// 本包最关键的约束：所有 SQL 的表名/列名都来自 catalog 的代码常量，绝不来自请求。
// 每次构造 SQL 前都用 validate 做一次标识符校验（regexp），把「拼错/被污染」的
// 可能性挡在执行之前 —— 这是 doc92 §0.3 硬约束 1 的实现落点。
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hostsent/backend/internal/modules/admin/logcenter/catalog"
	logmodel "hostsent/backend/internal/modules/admin/logcenter/model"
)

// identRe 合法标识符：小写字母/数字/下划线，且不以数字开头。
var identRe = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// ErrUnsafeIdentifier 标识符未通过校验（理论上不可达，因为来源是代码常量）。
var ErrUnsafeIdentifier = errors.New("logcenter: unsafe sql identifier")

// validate 校验源的标识符与列名白名单。
func validate(src *catalog.Source) error {
	if src == nil {
		return fmt.Errorf("logcenter: nil source")
	}
	if !identRe.MatchString(src.Table) {
		return fmt.Errorf("%w: table %q", ErrUnsafeIdentifier, src.Table)
	}
	if !identRe.MatchString(src.TimeColumn) {
		return fmt.Errorf("%w: time column %q", ErrUnsafeIdentifier, src.TimeColumn)
	}
	for _, c := range src.SelectColumns {
		if !identRe.MatchString(c.Key) {
			return fmt.Errorf("%w: column %q", ErrUnsafeIdentifier, c.Key)
		}
	}
	return nil
}

// Filter 查询筛选条件（已由服务层按 catalog 的 FilterColumns 白名单过滤过）。
type Filter struct {
	// From/To 时间区间（作用于 src.TimeColumn）；零值表示不限。
	From time.Time
	To   time.Time
	// Keyword 关键词，作用于 src.SearchColumns（ILIKE OR）。
	Keyword string
	// Equals 等值筛选：列名 → 值（列名必须是该源 SelectColumns 中的列）。
	Equals map[string]string
}

// where 生成 WHERE 片段与参数。
func (f Filter) where(src *catalog.Source) (string, []any) {
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 4)
	if !f.From.IsZero() {
		clauses = append(clauses, src.TimeColumn+" >= ?")
		args = append(args, f.From)
	}
	if !f.To.IsZero() {
		clauses = append(clauses, src.TimeColumn+" <= ?")
		args = append(args, f.To)
	}
	if kw := strings.TrimSpace(f.Keyword); kw != "" && len(src.SearchColumns) > 0 {
		ors := make([]string, 0, len(src.SearchColumns))
		like := "%" + kw + "%"
		for _, c := range src.SearchColumns {
			ors = append(ors, c+" ILIKE ?")
			args = append(args, like)
		}
		clauses = append(clauses, "("+strings.Join(ors, " OR ")+")")
	}
	for _, key := range sortedKeys(f.Equals) {
		val := f.Equals[key]
		if val == "" {
			continue
		}
		clauses = append(clauses, key+" = ?")
		args = append(args, val)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

// selectList 生成投影列（id + TimeColumn + 白名单列，去重）。
func selectList(src *catalog.Source) string {
	seen := map[string]bool{}
	cols := make([]string, 0, len(src.SelectColumns)+2)
	add := func(c string) {
		if !seen[c] {
			seen[c] = true
			cols = append(cols, c)
		}
	}
	add("id")
	add(src.TimeColumn)
	for _, c := range src.SelectColumns {
		add(c.Key)
	}
	return strings.Join(cols, ", ")
}

// LogQueryRepository 日志源的通用查询/统计/删除能力。
type LogQueryRepository interface {
	// Query 分页查询（按时间列倒序，同值按 id 倒序保证稳定）。
	Query(ctx context.Context, src *catalog.Source, f Filter, page, pageSize int) ([]map[string]any, error)
	// Count 统计满足条件的总行数。
	Count(ctx context.Context, src *catalog.Source, f Filter) (int64, error)
	// FindByID 取单行详情；不存在返回 nil, nil。
	FindByID(ctx context.Context, src *catalog.Source, id uint64) (map[string]any, error)
	// CountRows 统计全表行数（源列表徽标）。
	CountRows(ctx context.Context, src *catalog.Source) (int64, error)
	// CountBefore 统计水位线之前的行数；withGuard=true 时叠加删除守卫。
	CountBefore(ctx context.Context, src *catalog.Source, before time.Time, withGuard bool) (int64, error)
	// RangeBefore 取候选区间的最早/最晚时间（预演展示用）。
	RangeBefore(ctx context.Context, src *catalog.Source, before time.Time) (from, to *time.Time, err error)
	// DeleteBatch 分批删除：ctid 子查询避免长事务锁全表，总是从最老的开始删。
	DeleteBatch(ctx context.Context, src *catalog.Source, before time.Time, limit int) (int64, error)
	// ExportCursor 游标分页导出（id > lastID，不用 OFFSET）。
	ExportCursor(ctx context.Context, src *catalog.Source, f Filter, lastID uint64, limit int) ([]map[string]any, error)
	// UsesIndex 用 EXPLAIN 判断清理 SQL 是否走索引（验收场景 29）。
	UsesIndex(ctx context.Context, src *catalog.Source, before time.Time) (bool, error)
}

type logQueryRepository struct {
	db *gorm.DB
}

// NewLogQueryRepository 创建通用日志查询仓储。
func NewLogQueryRepository(db *gorm.DB) LogQueryRepository {
	return &logQueryRepository{db: db}
}

func (r *logQueryRepository) Query(ctx context.Context, src *catalog.Source, f Filter, page, pageSize int) ([]map[string]any, error) {
	if err := validate(src); err != nil {
		return nil, err
	}
	where, args := f.where(src)
	sql := fmt.Sprintf("SELECT %s FROM %s%s ORDER BY %s DESC, id DESC LIMIT ? OFFSET ?",
		selectList(src), src.Table, where, src.TimeColumn)
	args = append(args, pageSize, (page-1)*pageSize)
	return r.scanRows(ctx, sql, args...)
}

func (r *logQueryRepository) Count(ctx context.Context, src *catalog.Source, f Filter) (int64, error) {
	if err := validate(src); err != nil {
		return 0, err
	}
	where, args := f.where(src)
	var total int64
	err := r.db.WithContext(ctx).
		Raw(fmt.Sprintf("SELECT count(*) FROM %s%s", src.Table, where), args...).
		Scan(&total).Error
	return total, err
}

func (r *logQueryRepository) FindByID(ctx context.Context, src *catalog.Source, id uint64) (map[string]any, error) {
	if err := validate(src); err != nil {
		return nil, err
	}
	sql := fmt.Sprintf("SELECT %s FROM %s WHERE id = ?", selectList(src), src.Table)
	rows, err := r.scanRows(ctx, sql, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func (r *logQueryRepository) CountRows(ctx context.Context, src *catalog.Source) (int64, error) {
	if err := validate(src); err != nil {
		return 0, err
	}
	var total int64
	err := r.db.WithContext(ctx).Table(src.Table).Count(&total).Error
	return total, err
}

func (r *logQueryRepository) CountBefore(ctx context.Context, src *catalog.Source, before time.Time, withGuard bool) (int64, error) {
	if err := validate(src); err != nil {
		return 0, err
	}
	sql := fmt.Sprintf("SELECT count(*) FROM %s WHERE %s < ? AND (%s)",
		src.Table, src.TimeColumn, guardedOrTrue(src.DeleteGuard, withGuard))
	var total int64
	err := r.db.WithContext(ctx).Raw(sql, before).Scan(&total).Error
	return total, err
}

func (r *logQueryRepository) RangeBefore(ctx context.Context, src *catalog.Source, before time.Time) (from, to *time.Time, err error) {
	if err := validate(src); err != nil {
		return nil, nil, err
	}
	sql := fmt.Sprintf("SELECT min(%s) AS min_t, max(%s) AS max_t FROM %s WHERE %s < ? AND (%s)",
		src.TimeColumn, src.TimeColumn, src.Table, src.TimeColumn, guardedOrTrue(src.DeleteGuard, true))
	var row struct {
		MinT *time.Time
		MaxT *time.Time
	}
	if err := r.db.WithContext(ctx).Raw(sql, before).Scan(&row).Error; err != nil {
		return nil, nil, err
	}
	return row.MinT, row.MaxT, nil
}

// DeleteBatch 分批删除水位线之前的行。
//
// 为什么用 ctid 而不是 id IN：部分表主键虽为 id，但统一用 ctid 通用（未来接入
// 复合主键表也不用改）；ORDER BY 时间列保证「总是从最老的开始删」，
// 不会出现「删了一批老的、下一批还是老的那批」的死循环。
func (r *logQueryRepository) DeleteBatch(ctx context.Context, src *catalog.Source, before time.Time, limit int) (int64, error) {
	if err := validate(src); err != nil {
		return 0, err
	}
	if limit <= 0 {
		limit = 5000
	}
	// 表名/列名/守卫都是代码常量；唯一的外部参数 before 用占位符绑定。
	sql := fmt.Sprintf(`
		DELETE FROM %s
		WHERE ctid IN (
			SELECT ctid FROM %s
			WHERE %s < ?
			  AND (%s)
			ORDER BY %s
			LIMIT %d
		)`, src.Table, src.Table, src.TimeColumn, guardedOrTrue(src.DeleteGuard, true), src.TimeColumn, limit)
	res := r.db.WithContext(ctx).Exec(sql, before)
	return res.RowsAffected, res.Error
}

func (r *logQueryRepository) ExportCursor(ctx context.Context, src *catalog.Source, f Filter, lastID uint64, limit int) ([]map[string]any, error) {
	if err := validate(src); err != nil {
		return nil, err
	}
	where, args := f.where(src)
	cursor := ""
	if lastID > 0 {
		if where == "" {
			cursor = " WHERE id > ?"
		} else {
			cursor = where + " AND id > ?"
		}
		args = append(args, lastID)
	} else if where != "" {
		cursor = where
	}
	sql := fmt.Sprintf("SELECT %s FROM %s%s ORDER BY id ASC LIMIT ?", selectList(src), src.Table, cursor)
	args = append(args, limit)
	return r.scanRows(ctx, sql, args...)
}

// UsesIndex 用 EXPLAIN 判断删除候选扫描是否命中索引。
//
// 只看是否出现 Seq Scan：数据量小的时候规划器可能仍选顺序扫描（那是合理的），
// 所以本方法仅用于诊断展示，不参与任何门禁判断。
func (r *logQueryRepository) UsesIndex(ctx context.Context, src *catalog.Source, before time.Time) (bool, error) {
	if err := validate(src); err != nil {
		return false, err
	}
	sql := fmt.Sprintf("EXPLAIN SELECT ctid FROM %s WHERE %s < ? LIMIT 1", src.Table, src.TimeColumn)
	var lines []string
	if err := r.db.WithContext(ctx).Raw(sql, before).Scan(&lines).Error; err != nil {
		return false, err
	}
	joined := strings.ToLower(strings.Join(lines, "\n"))
	return !strings.Contains(joined, "seq scan"), nil
}

// scanRows 执行原始 SQL 并把结果映射为 map（JSON 列自动解码）。
func (r *logQueryRepository) scanRows(ctx context.Context, sql string, args ...any) ([]map[string]any, error) {
	rows, err := r.db.WithContext(ctx).Raw(sql, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, 32)
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		item := make(map[string]any, len(cols))
		for i, name := range cols {
			item[name] = normalizeValue(vals[i])
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// normalizeValue 把驱动返回值转为 JSON 友好的形式。
//
// 时间统一成 "2006-01-02 15:04:05"（doc92 §9.3：前端不做时区换算，后端给本地时间串）；
// []byte 先尝试 JSON 解码（jsonb 列），失败则当字符串。
func normalizeValue(v any) any {
	switch val := v.(type) {
	case nil:
		return nil
	case time.Time:
		return val.Format("2006-01-02 15:04:05")
	case []byte:
		s := string(val)
		trimmed := strings.TrimSpace(s)
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			var decoded any
			if err := json.Unmarshal(val, &decoded); err == nil {
				return decoded
			}
		}
		return s
	case int32:
		return int64(val)
	case float32:
		return float64(val)
	default:
		return val
	}
}

// guardedOrTrue 返回删除守卫片段；withGuard=false 或守卫为空时返回 TRUE。
func guardedOrTrue(guard string, withGuard bool) string {
	if !withGuard || strings.TrimSpace(guard) == "" {
		return "TRUE"
	}
	return guard
}

// sortedKeys 对 map 的键排序，保证 SQL 片段稳定（便于缓存与测试断言）。
func sortedKeys(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// UpstreamLogRepository 上游接口日志落库。
type UpstreamLogRepository interface {
	CreateBatch(ctx context.Context, rows []logmodel.UpstreamAPILog) error
	// CountSince 统计时间点以来的行数（观测采集是否在工作）。
	CountSince(ctx context.Context, since time.Time) (int64, error)
}

type upstreamLogRepository struct {
	db *gorm.DB
}

// NewUpstreamLogRepository 创建上游日志仓储。
func NewUpstreamLogRepository(db *gorm.DB) UpstreamLogRepository {
	return &upstreamLogRepository{db: db}
}

func (r *upstreamLogRepository) CreateBatch(ctx context.Context, rows []logmodel.UpstreamAPILog) error {
	if len(rows) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(rows, 200).Error
}

func (r *upstreamLogRepository) CountSince(ctx context.Context, since time.Time) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&logmodel.UpstreamAPILog{}).
		Where("created_at >= ?", since).Count(&total).Error
	return total, err
}

// JobRunRepository 定时任务日志落库。
type JobRunRepository interface {
	// Start 写入 running 行；命中 uk_job_run_logs_running 时返回 ErrJobRunning。
	Start(ctx context.Context, row *logmodel.JobRunLog) error
	// Finish 收尾（写终态、耗时、摘要）。
	Finish(ctx context.Context, row *logmodel.JobRunLog) error
	// CountToday 统计某 job 当日已写入行数（软上限用）。
	CountToday(ctx context.Context, jobName string) (int64, error)
}

// ErrJobRunning 同一任务上一轮尚未结束。
var ErrJobRunning = errors.New("jobcenter: previous run still active")

type jobRunRepository struct {
	db *gorm.DB
}

// NewJobRunRepository 创建任务日志仓储。
func NewJobRunRepository(db *gorm.DB) JobRunRepository {
	return &jobRunRepository{db: db}
}

func (r *jobRunRepository) Start(ctx context.Context, row *logmodel.JobRunLog) error {
	err := r.db.WithContext(ctx).Create(row).Error
	if err == nil {
		return nil
	}
	if isUniqueViolation(err) {
		return ErrJobRunning
	}
	return err
}

func (r *jobRunRepository) Finish(ctx context.Context, row *logmodel.JobRunLog) error {
	return r.db.WithContext(ctx).Model(&logmodel.JobRunLog{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{
			"status":         row.Status,
			"finished_at":    row.FinishedAt,
			"duration_ms":    row.DurationMS,
			"items_scanned":  row.ItemsScanned,
			"items_affected": row.ItemsAffected,
			"summary":        nullIfEmpty(row.Summary),
			"error_message":  row.ErrorMessage,
		}).Error
}

func (r *jobRunRepository) CountToday(ctx context.Context, jobName string) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&logmodel.JobRunLog{}).
		Where("job_name = ? AND created_at::date = current_date", jobName).
		Count(&total).Error
	return total, err
}

// isUniqueViolation 判定 Postgres 唯一约束冲突（错误码 23505）。
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key value")
}

// nullIfEmpty 空串写 NULL（jsonb 列写空串会报错）。
func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// PolicyRepository 保留策略数据访问。
type PolicyRepository interface {
	List(ctx context.Context) ([]logmodel.LogRetentionPolicy, error)
	FindBySource(ctx context.Context, sourceKey string) (*logmodel.LogRetentionPolicy, error)
	// UpsertSeed 幂等插入 seed 行（已存在则不动，运营改过的保留期不被重启重置）。
	UpsertSeed(ctx context.Context, rows []logmodel.LogRetentionPolicy) error
	Update(ctx context.Context, row *logmodel.LogRetentionPolicy) error
	// MarkRun 回填上次执行时间与删除行数。
	MarkRun(ctx context.Context, sourceKey string, at time.Time, deleted int64) error
}

type policyRepository struct {
	db *gorm.DB
}

// NewPolicyRepository 创建保留策略仓储。
func NewPolicyRepository(db *gorm.DB) PolicyRepository {
	return &policyRepository{db: db}
}

func (r *policyRepository) List(ctx context.Context) ([]logmodel.LogRetentionPolicy, error) {
	var rows []logmodel.LogRetentionPolicy
	err := r.db.WithContext(ctx).Order("id ASC").Find(&rows).Error
	return rows, err
}

func (r *policyRepository) FindBySource(ctx context.Context, sourceKey string) (*logmodel.LogRetentionPolicy, error) {
	var row logmodel.LogRetentionPolicy
	err := r.db.WithContext(ctx).Where("source_key = ?", sourceKey).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *policyRepository) UpsertSeed(ctx context.Context, rows []logmodel.LogRetentionPolicy) error {
	if len(rows) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "source_key"}}, DoNothing: true}).
		CreateInBatches(rows, 50).Error
}

func (r *policyRepository) Update(ctx context.Context, row *logmodel.LogRetentionPolicy) error {
	return r.db.WithContext(ctx).Model(&logmodel.LogRetentionPolicy{}).
		Where("source_key = ?", row.SourceKey).
		Updates(map[string]any{
			"action":         row.Action,
			"retention_days": row.RetentionDays,
			"batch_size":     row.BatchSize,
			"enabled":        row.Enabled,
			"updated_by":     row.UpdatedBy,
			"remark":         row.Remark,
			"display_name":   row.DisplayName,
			"archive_cron":   row.ArchiveCron,
		}).Error
}

func (r *policyRepository) MarkRun(ctx context.Context, sourceKey string, at time.Time, deleted int64) error {
	return r.db.WithContext(ctx).Model(&logmodel.LogRetentionPolicy{}).
		Where("source_key = ?", sourceKey).
		Updates(map[string]any{"last_run_at": at, "last_deleted": deleted}).Error
}

// ExportRepository 导出文件元数据访问。
type ExportRepository interface {
	Create(ctx context.Context, row *logmodel.LogExportFile) error
	Update(ctx context.Context, row *logmodel.LogExportFile) error
	FindByID(ctx context.Context, id uint64) (*logmodel.LogExportFile, error)
	List(ctx context.Context, q ExportListQuery) ([]logmodel.LogExportFile, int64, error)
	// ExpiredUnreferenced 取过期且未被清理任务引用的文件（可安全删文件）。
	ExpiredUnreferenced(ctx context.Context, now time.Time, limit int) ([]logmodel.LogExportFile, error)
	// MarkDeleted 只改状态，行永久保留。
	MarkDeleted(ctx context.Context, id uint64) error
}

// ExportListQuery 导出文件列表查询。
type ExportListQuery struct {
	SourceKey string
	JobID     uint64
	Status    string
	StartTime string
	EndTime   string
	Page      int
	PageSize  int
}

type exportRepository struct {
	db *gorm.DB
}

// NewExportRepository 创建导出文件仓储。
func NewExportRepository(db *gorm.DB) ExportRepository {
	return &exportRepository{db: db}
}

func (r *exportRepository) Create(ctx context.Context, row *logmodel.LogExportFile) error {
	return r.db.WithContext(ctx).Create(row).Error
}

func (r *exportRepository) Update(ctx context.Context, row *logmodel.LogExportFile) error {
	return r.db.WithContext(ctx).Model(&logmodel.LogExportFile{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{
			"file_size":   row.FileSize,
			"row_count":   row.RowCount,
			"sha256":      row.SHA256,
			"status":      row.Status,
			"fail_reason": row.FailReason,
			"expires_at":  row.ExpiresAt,
			"job_id":      row.JobID,
		}).Error
}

func (r *exportRepository) FindByID(ctx context.Context, id uint64) (*logmodel.LogExportFile, error) {
	var row logmodel.LogExportFile
	err := r.db.WithContext(ctx).First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *exportRepository) List(ctx context.Context, q ExportListQuery) ([]logmodel.LogExportFile, int64, error) {
	tx := r.db.WithContext(ctx).Model(&logmodel.LogExportFile{})
	if q.SourceKey != "" {
		tx = tx.Where("source_key = ?", q.SourceKey)
	}
	if q.JobID > 0 {
		tx = tx.Where("job_id = ?", q.JobID)
	}
	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
	}
	if q.StartTime != "" {
		if t, err := parseTime(q.StartTime); err == nil {
			tx = tx.Where("created_at >= ?", t)
		}
	}
	if q.EndTime != "" {
		if t, err := parseTime(q.EndTime); err == nil {
			tx = tx.Where("created_at <= ?", t)
		}
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []logmodel.LogExportFile
	err := tx.Order("id DESC").Limit(q.PageSize).Offset((q.Page - 1) * q.PageSize).Find(&rows).Error
	return rows, total, err
}

// ExpiredUnreferenced 取「已过期 + 未被任何清理任务引用」的人工导出文件。
//
// 清理任务产生的备份是「删了哪些数据」的唯一凭据，永远不在这里被选中
// （doc92 §5.4）——所以除了 job_id = 0，还要排除被 log_cleanup_jobs.export_file_ids
// 引用过的文件（防手工把 job_id 关联丢掉后误删凭据）。
func (r *exportRepository) ExpiredUnreferenced(ctx context.Context, now time.Time, limit int) ([]logmodel.LogExportFile, error) {
	var rows []logmodel.LogExportFile
	err := r.db.WithContext(ctx).
		Where("status = ?", logmodel.ExportStatusSuccess).
		Where("expires_at IS NOT NULL AND expires_at < ?", now).
		Where("job_id = 0").
		Where(`NOT EXISTS (
			SELECT 1 FROM log_cleanup_jobs j
			WHERE j.export_file_ids IS NOT NULL
			  AND j.export_file_ids @> to_jsonb(log_export_files.id)
		)`).
		Order("id ASC").Limit(limit).Find(&rows).Error
	return rows, err
}

func (r *exportRepository) MarkDeleted(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&logmodel.LogExportFile{}).
		Where("id = ?", id).Update("status", logmodel.ExportStatusDeleted).Error
}

// CleanupRepository 清理任务数据访问。
type CleanupRepository interface {
	Create(ctx context.Context, row *logmodel.LogCleanupJob) error
	Update(ctx context.Context, row *logmodel.LogCleanupJob) error
	FindByID(ctx context.Context, id uint64) (*logmodel.LogCleanupJob, error)
	List(ctx context.Context, q CleanupListQuery) ([]logmodel.LogCleanupJob, int64, error)
	// HasScheduledToday 判断当日是否已有调度触发的任务（每日语义）。
	HasScheduledToday(ctx context.Context) (bool, error)
	// HasActive 是否存在非终态任务（互斥索引的友好提示路径）。
	HasActive(ctx context.Context) (bool, error)
}

// CleanupListQuery 清理任务列表查询。
type CleanupListQuery struct {
	Status    string
	Trigger   string
	StartTime string
	EndTime   string
	Page      int
	PageSize  int
}

type cleanupRepository struct {
	db *gorm.DB
}

// NewCleanupRepository 创建清理任务仓储。
func NewCleanupRepository(db *gorm.DB) CleanupRepository {
	return &cleanupRepository{db: db}
}

func (r *cleanupRepository) Create(ctx context.Context, row *logmodel.LogCleanupJob) error {
	err := r.db.WithContext(ctx).Create(row).Error
	if err != nil && isUniqueViolation(err) {
		return ErrJobRunning
	}
	return err
}

func (r *cleanupRepository) Update(ctx context.Context, row *logmodel.LogCleanupJob) error {
	return r.db.WithContext(ctx).Model(&logmodel.LogCleanupJob{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{
			"status":          row.Status,
			"scanned_rows":    row.ScannedRows,
			"deleted_rows":    row.DeletedRows,
			"expired_rows":    row.ExpiredRows,
			"progress":        row.Progress,
			"current_source":  row.CurrentSource,
			"error_message":   row.ErrorMessage,
			"export_file_ids": nullIfEmpty(row.ExportFileIDs),
			"sources":         nullIfEmpty(row.Sources),
			"started_at":      row.StartedAt,
			"finished_at":     row.FinishedAt,
			"before_time":     row.BeforeTime,
			"dry_run":         row.DryRun,
		}).Error
}

func (r *cleanupRepository) FindByID(ctx context.Context, id uint64) (*logmodel.LogCleanupJob, error) {
	var row logmodel.LogCleanupJob
	err := r.db.WithContext(ctx).First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *cleanupRepository) List(ctx context.Context, q CleanupListQuery) ([]logmodel.LogCleanupJob, int64, error) {
	tx := r.db.WithContext(ctx).Model(&logmodel.LogCleanupJob{})
	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
	}
	if q.Trigger != "" {
		tx = tx.Where("trigger_type = ?", q.Trigger)
	}
	if q.StartTime != "" {
		if t, err := parseTime(q.StartTime); err == nil {
			tx = tx.Where("created_at >= ?", t)
		}
	}
	if q.EndTime != "" {
		if t, err := parseTime(q.EndTime); err == nil {
			tx = tx.Where("created_at <= ?", t)
		}
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []logmodel.LogCleanupJob
	err := tx.Order("id DESC").Limit(q.PageSize).Offset((q.Page - 1) * q.PageSize).Find(&rows).Error
	return rows, total, err
}

func (r *cleanupRepository) HasScheduledToday(ctx context.Context) (bool, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&logmodel.LogCleanupJob{}).
		Where("trigger_type = ? AND created_at::date = current_date", logmodel.TriggerScheduled).
		Count(&total).Error
	return total > 0, err
}

func (r *cleanupRepository) HasActive(ctx context.Context) (bool, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&logmodel.LogCleanupJob{}).
		Where("status IN ?", logmodel.CleanupActiveStatuses()).
		Count(&total).Error
	return total > 0, err
}

// parseTime 宽松解析时间参数（RFC3339 / 空格分隔 / 纯日期）。
func parseTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("logcenter: 无法解析时间 %q", raw)
}

// ParseTime 导出宽松解析供服务层复用。
func ParseTime(raw string) (time.Time, error) { return parseTime(raw) }
