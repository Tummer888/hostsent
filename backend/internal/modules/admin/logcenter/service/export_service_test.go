package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/modules/admin/logcenter/catalog"
	logmodel "hostsent/backend/internal/modules/admin/logcenter/model"
	logrepo "hostsent/backend/internal/modules/admin/logcenter/repository"
	"hostsent/backend/internal/pkg/storage"
)

// ---------------------------------------------------------------------------
// 测试替身：只为导出/校验路径提供最小实现。
// ---------------------------------------------------------------------------

type fakeQueryRepo struct {
	// count 返回 Count/CountBefore 的结果（校验时的「候选行数」）。
	count int64
	// countScopes 记录每次 Count 收到的筛选条件，用于断言备份范围 == 删除范围。
	countScopes []logrepo.Filter
	rows        []map[string]any
}

func (f *fakeQueryRepo) Query(context.Context, *catalog.Source, logrepo.Filter, int, int) ([]map[string]any, error) {
	return nil, nil
}
func (f *fakeQueryRepo) Count(_ context.Context, _ *catalog.Source, flt logrepo.Filter) (int64, error) {
	f.countScopes = append(f.countScopes, flt)
	return f.count, nil
}
func (f *fakeQueryRepo) FindByID(context.Context, *catalog.Source, uint64) (map[string]any, error) {
	return nil, nil
}
func (f *fakeQueryRepo) CountRows(context.Context, *catalog.Source) (int64, error) { return 0, nil }
func (f *fakeQueryRepo) CountBefore(context.Context, *catalog.Source, time.Time, bool) (int64, error) {
	return f.count, nil
}
func (f *fakeQueryRepo) RangeBefore(context.Context, *catalog.Source, time.Time) (*time.Time, *time.Time, error) {
	return nil, nil, nil
}
func (f *fakeQueryRepo) DeleteBatch(context.Context, *catalog.Source, time.Time, int) (int64, error) {
	return 0, nil
}
func (f *fakeQueryRepo) ExportCursor(_ context.Context, _ *catalog.Source, _ logrepo.Filter, lastID uint64, _ int) ([]map[string]any, error) {
	if len(f.rows) == 0 {
		return nil, nil
	}
	// 单批返回全部行，模拟「一次游标取完」。
	out := f.rows
	f.rows = nil
	return out, nil
}
func (f *fakeQueryRepo) UsesIndex(context.Context, *catalog.Source, time.Time) (bool, error) {
	return true, nil
}

type fakeExportRepo struct {
	row *logmodel.LogExportFile
	// findRow 是 FindByID 的返回值（与 Create 捕获的 row 分开，便于构造「已存在的行」）。
	findRow *logmodel.LogExportFile
	// expired 是 ExpiredUnreferenced 的返回值。
	expired []logmodel.LogExportFile
	// marked 记录被 MarkDeleted 的 ID。
	marked []uint64
}

func (f *fakeExportRepo) Create(_ context.Context, row *logmodel.LogExportFile) error {
	f.row = row
	return nil
}
func (f *fakeExportRepo) Update(context.Context, *logmodel.LogExportFile) error { return nil }
func (f *fakeExportRepo) FindByID(context.Context, uint64) (*logmodel.LogExportFile, error) {
	if f.findRow != nil {
		return f.findRow, nil
	}
	return f.row, nil
}
func (f *fakeExportRepo) List(context.Context, logrepo.ExportListQuery) ([]logmodel.LogExportFile, int64, error) {
	return nil, 0, nil
}
func (f *fakeExportRepo) ExpiredUnreferenced(context.Context, time.Time, int) ([]logmodel.LogExportFile, error) {
	return f.expired, nil
}
func (f *fakeExportRepo) MarkDeleted(_ context.Context, id uint64) error {
	f.marked = append(f.marked, id)
	return nil
}

// newTestExportService 建一个落临时目录的导出服务。
func newTestExportService(t *testing.T, q *fakeQueryRepo, e *fakeExportRepo) ExportService {
	t.Helper()
	store, err := storage.NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatalf("init store: %v", err)
	}
	return NewExportService(q, e, nil, store, ExportServiceOptions{RetentionDays: 30}, zap.NewNop())
}

// 每个源都要能从注册表按表名找到；用注册表里的真实源，避免手写假表。
func syncDiffSource(t *testing.T) *catalog.Source {
	t.Helper()
	src := catalog.Get("sync_diff")
	if src == nil {
		t.Fatal("catalog source sync_diff missing")
	}
	return src
}

// ---------------------------------------------------------------------------
// 场景 17：备份文件被篡改后，verifying 阶段必须失败（行数对不上 / sha 对不上）。
// ---------------------------------------------------------------------------

// 篡改文件内容 → sha256 不符 → Verify 失败（这是「备份不可信就不许删」的守门人）。
func TestVerifyRejectsTamperedFile(t *testing.T) {
	q := &fakeQueryRepo{count: 3, rows: []map[string]any{
		{"id": uint64(1), "created_at": time.Now()},
		{"id": uint64(2), "created_at": time.Now()},
		{"id": uint64(3), "created_at": time.Now()},
	}}
	e := &fakeExportRepo{}
	svc := newTestExportService(t, q, e)
	src := syncDiffSource(t)

	file, err := svc.CleanupExport(context.Background(), src, logrepo.Filter{To: time.Now()}, 1, 1, "admin")
	if err != nil {
		t.Fatalf("cleanup export: %v", err)
	}
	// 原始文件校验通过。
	if err := svc.Verify(context.Background(), file.ID, src, logrepo.Filter{To: time.Now()}); err != nil {
		t.Fatalf("untampered file should verify: %v", err)
	}

	// 追加一行 → sha256 与 file_size 都不符。
	f, err := os.OpenFile(filepath.Join(storeRootOf(t, svc), file.StoragePath), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open export file: %v", err)
	}
	if _, err := f.WriteString("TAMPERED\n"); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	_ = f.Close()

	err = svc.Verify(context.Background(), file.ID, src, logrepo.Filter{To: time.Now()})
	if err == nil {
		t.Fatal("tampered file must fail verification")
	}
	if !strings.Contains(err.Error(), "sha256") && !strings.Contains(err.Error(), "大小不符") {
		t.Errorf("unexpected verification error: %v", err)
	}
}

// 备份范围必须与删除范围一致：Verify 传给 Count 的筛选条件应当是
// 「水位线开区间 + 删除守卫」，否则会出现「备份 12050 行、候选 12000 行」的假告警。
func TestVerifyUsesDeleteScope(t *testing.T) {
	q := &fakeQueryRepo{count: 2, rows: []map[string]any{
		{"id": uint64(1), "created_at": time.Now()},
		{"id": uint64(2), "created_at": time.Now()},
	}}
	svc := newTestExportService(t, q, &fakeExportRepo{})
	src := syncDiffSource(t)
	watermark := time.Now().Add(-24 * time.Hour)

	file, err := svc.CleanupExport(context.Background(), src, logrepo.Filter{To: watermark}, 1, 1, "admin")
	if err != nil {
		t.Fatalf("cleanup export: %v", err)
	}
	q.countScopes = nil
	if err := svc.Verify(context.Background(), file.ID, src, logrepo.Filter{To: watermark}); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(q.countScopes) != 1 {
		t.Fatalf("expected exactly 1 Count call, got %d", len(q.countScopes))
	}
	got := q.countScopes[0]
	if !got.ToExclusive {
		t.Error("verify must count with an exclusive watermark (same as DeleteBatch)")
	}
	if !got.WithGuard {
		t.Error("verify must apply the source delete guard (same as DeleteBatch)")
	}
	if !got.From.IsZero() {
		t.Error("verify must not bound the lower end: the backup contains everything before the watermark")
	}
}

// 行数对不上（文件行数 > 候选行数）→ 校验失败，避免「备份没覆盖全部待删行」被放行。
func TestVerifyRejectsRowCountMismatch(t *testing.T) {
	q := &fakeQueryRepo{count: 5, rows: []map[string]any{
		{"id": uint64(1), "created_at": time.Now()},
		{"id": uint64(2), "created_at": time.Now()},
	}}
	svc := newTestExportService(t, q, &fakeExportRepo{})
	src := syncDiffSource(t)

	file, err := svc.CleanupExport(context.Background(), src, logrepo.Filter{To: time.Now()}, 1, 1, "admin")
	if err != nil {
		t.Fatalf("cleanup export: %v", err)
	}
	err = svc.Verify(context.Background(), file.ID, src, logrepo.Filter{To: time.Now()})
	if err == nil || !strings.Contains(err.Error(), "候选行数不符") {
		t.Fatalf("expected row-count mismatch error, got %v", err)
	}
}

// storeRootOf 取服务内部 LocalStore 的根目录（测试需要直接改文件）。
func storeRootOf(t *testing.T, svc ExportService) string {
	t.Helper()
	impl, ok := svc.(*exportService)
	if !ok {
		t.Fatalf("unexpected export service type %T", svc)
	}
	return impl.store.Root()
}

// ---------------------------------------------------------------------------
// 场景 27/28：备份凭据不可删；过期的人工导出只删文件、行永久保留。
// ---------------------------------------------------------------------------

// 清理任务的备份（job_id != 0）必须拒绝删除 —— 它是「删了哪些行」的唯一凭据。
func TestDeleteRejectsCleanupBackup(t *testing.T) {
	repo := &fakeExportRepo{findRow: &logmodel.LogExportFile{
		ID: 9, SourceKey: "sync_diff", JobID: 3, Status: logmodel.ExportStatusSuccess,
	}}
	svc := newTestExportService(t, &fakeQueryRepo{}, repo)

	err := svc.Delete(context.Background(), 9)
	if err == nil {
		t.Fatal("cleanup backup must not be deletable")
	}
	if !strings.Contains(err.Error(), "备份凭据") {
		t.Errorf("unexpected error: %v", err)
	}
	if len(repo.marked) != 0 {
		t.Errorf("rejected delete must not touch the row, marked=%v", repo.marked)
	}
}

// 过期的人工导出：删掉磁盘文件并把状态改成 deleted，但行本身永久保留。
func TestPurgeExpiredKeepsRow(t *testing.T) {
	repo := &fakeExportRepo{}
	svc := newTestExportService(t, &fakeQueryRepo{}, repo)
	root := storeRootOf(t, svc)

	// 落一个真实文件，确保 PurgeExpired 真去删了磁盘。
	expired := &logmodel.LogExportFile{
		ID: 11, SourceKey: "sync_log", JobID: 0,
		StoragePath: "log-exports/sync_log/gone.jsonl", Status: logmodel.ExportStatusSuccess,
	}
	if err := os.MkdirAll(filepath.Join(root, "log-exports/sync_log"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, expired.StoragePath), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	repo.expired = []logmodel.LogExportFile{*expired}

	n, err := svc.PurgeExpired(context.Background(), 10)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 purged file, got %d", n)
	}
	if _, err := os.Stat(filepath.Join(root, expired.StoragePath)); !os.IsNotExist(err) {
		t.Error("expired export file should be removed from disk")
	}
	if len(repo.marked) != 1 || repo.marked[0] != 11 {
		t.Errorf("row must be marked deleted (not dropped), marked=%v", repo.marked)
	}
}
