package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	providermodel "hostsent/backend/internal/modules/admin/resource/provider/model"
	"hostsent/backend/internal/modules/admin/resource/sync/dto"
	"hostsent/backend/internal/modules/admin/resource/sync/model"
)

// ErrScheduleNotFound 调度行不存在。
var ErrScheduleNotFound = errors.New("同步调度不存在")

// ProviderLister 渠道列表读取（供同步框架服务解析渠道名，避免依赖 provider 具体实现）。
type ProviderLister interface {
	ListAll(ctx context.Context) ([]providermodel.ResourceProvider, error)
}

// FrameworkRepository P3 同步框架数据访问：调度配置、增量游标、调价事件、差异记录，
// 以及全量对账所需的"本地快照"查询与软删除（迁移 031）。
type FrameworkRepository interface {
	// ---- sync_schedules ----
	ListDueSchedules(ctx context.Context, now time.Time) ([]model.SyncSchedule, error)
	ListSchedulesByProvider(ctx context.Context, providerID uint64) ([]model.SyncSchedule, error)
	ListAllSchedules(ctx context.Context) ([]model.SyncSchedule, error)
	CountSchedules(ctx context.Context) (int64, error)
	CreateSchedule(ctx context.Context, item *model.SyncSchedule) error
	UpdateScheduleConfig(ctx context.Context, item *model.SyncSchedule) error
	RecordScheduleRun(ctx context.Context, id uint64, status, errMsg string, lastRun, nextRun time.Time) error
	DeleteSchedulesByProvider(ctx context.Context, providerID uint64) error

	// ---- sync_cursors ----
	GetCursor(ctx context.Context, providerID uint64, scope string) (*model.SyncCursor, error)
	UpsertCursor(ctx context.Context, item *model.SyncCursor) error

	// ---- price_change_events ----
	CreatePriceChange(ctx context.Context, item *model.PriceChangeEvent) error
	FindPriceChange(ctx context.Context, id uint64) (*model.PriceChangeEvent, error)
	FindPendingPriceChange(ctx context.Context, providerID uint64, resourceProductID uint64, field string) (*model.PriceChangeEvent, error)
	UpdatePriceChange(ctx context.Context, item *model.PriceChangeEvent) error
	ListPriceChanges(ctx context.Context, query dto.PriceChangeListQuery) ([]model.PriceChangeEvent, int64, error)

	// ---- sync_diffs ----
	CreateDiffs(ctx context.Context, items []model.SyncDiff) error
	ListDiffs(ctx context.Context, query dto.SyncDiffListQuery) ([]model.SyncDiff, int64, error)
	CountDiffsByAction(ctx context.Context, providerID uint64, since time.Time) (map[string]int64, error)

	// ---- 全量对账 ----
	ListResourceProductsByProvider(ctx context.Context, providerID uint64) ([]ResourceProductSnapshot, error)
	ListPoolSnapshotsByProvider(ctx context.Context, providerID uint64) ([]PoolSnapshot, error)
	MarkResourceProductsOffline(ctx context.Context, providerID uint64, keepUpstreamIDs []string) (int64, error)
	MarkPoolsOffline(ctx context.Context, providerID uint64, keepUpstreamIDs []string) (int64, error)
	MarkInstancesOffline(ctx context.Context, providerID uint64, keepInstanceIDs []string) (int64, error)

	// PurgeFrameworkBefore 分批清理过保留期的差异/调价事件（任务与日志由既有 PurgeSyncDataBefore 负责）
	PurgeFrameworkBefore(ctx context.Context, before time.Time, batchSize int) (int64, error)
}

// ResourceProductSnapshot 全量对账用上游商品本地快照（不引入 product 仓储，避免耦合）。
type ResourceProductSnapshot struct {
	ID         uint64
	UpstreamID string
	Name       string
	CPU        int
	Memory     int
	Disk       int
	DiskType   string
	Bandwidth  int
	OS         string
	Region     string
	Zone       string
	CostPrice  float64
	SalePrice  float64
	Status     int
}

// PoolSnapshot 资源池本地快照。
type PoolSnapshot struct {
	ID         uint64
	UpstreamID string
	Name       string
	Status     int
}

type frameworkRepository struct {
	db *gorm.DB
}

// NewFrameworkRepository 创建 P3 同步框架仓储实现。
func NewFrameworkRepository(db *gorm.DB) FrameworkRepository {
	return &frameworkRepository{db: db}
}

// ---------------------------------------------------------------- schedules

func (r *frameworkRepository) ListDueSchedules(ctx context.Context, now time.Time) ([]model.SyncSchedule, error) {
	var items []model.SyncSchedule
	err := r.db.WithContext(ctx).
		Where("enabled = ?", true).
		Where("next_run_at IS NULL OR next_run_at <= ?", now).
		Order("priority desc, next_run_at asc nulls first, id asc").
		Find(&items).Error
	return items, err
}

func (r *frameworkRepository) ListSchedulesByProvider(ctx context.Context, providerID uint64) ([]model.SyncSchedule, error) {
	var items []model.SyncSchedule
	err := r.db.WithContext(ctx).
		Where("provider_id = ?", providerID).
		Order("priority desc, scope asc").Find(&items).Error
	return items, err
}

func (r *frameworkRepository) ListAllSchedules(ctx context.Context) ([]model.SyncSchedule, error) {
	var items []model.SyncSchedule
	err := r.db.WithContext(ctx).
		Order("provider_id asc, priority desc, scope asc").Find(&items).Error
	return items, err
}

func (r *frameworkRepository) CountSchedules(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.SyncSchedule{}).Count(&n).Error
	return n, err
}

func (r *frameworkRepository) CreateSchedule(ctx context.Context, item *model.SyncSchedule) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *frameworkRepository) UpdateScheduleConfig(ctx context.Context, item *model.SyncSchedule) error {
	return r.db.WithContext(ctx).Model(&model.SyncSchedule{}).
		Where("id = ?", item.ID).
		Updates(map[string]any{
			"interval_seconds":           item.IntervalSeconds,
			"full_sync_interval_seconds": item.FullSyncIntervalSeconds,
			"enabled":                    item.Enabled,
			"priority":                   item.Priority,
			"window_start":               item.WindowStart,
			"window_end":                 item.WindowEnd,
			"next_run_at":                item.NextRunAt,
			"updated_at":                 time.Now(),
		}).Error
}

func (r *frameworkRepository) RecordScheduleRun(ctx context.Context, id uint64, status, errMsg string, lastRun, nextRun time.Time) error {
	return r.db.WithContext(ctx).Model(&model.SyncSchedule{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"last_status": status,
			"last_error":  errMsg,
			"last_run_at": lastRun,
			"next_run_at": nextRun,
			"updated_at":  time.Now(),
		}).Error
}

func (r *frameworkRepository) DeleteSchedulesByProvider(ctx context.Context, providerID uint64) error {
	return r.db.WithContext(ctx).Where("provider_id = ?", providerID).Delete(&model.SyncSchedule{}).Error
}

// ---------------------------------------------------------------- cursors

func (r *frameworkRepository) GetCursor(ctx context.Context, providerID uint64, scope string) (*model.SyncCursor, error) {
	var item model.SyncCursor
	if err := r.db.WithContext(ctx).
		Where("provider_id = ? AND scope = ?", providerID, scope).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *frameworkRepository) UpsertCursor(ctx context.Context, item *model.SyncCursor) error {
	return r.db.WithContext(ctx).Where("provider_id = ? AND scope = ?", item.ProviderID, item.Scope).
		Assign(map[string]any{
			"cursor":              item.Cursor,
			"last_incremental_at": item.LastIncrementalAt,
			"last_full_at":        item.LastFullAt,
			"last_seen_count":     item.LastSeenCount,
			"updated_at":          time.Now(),
		}).FirstOrCreate(item).Error
}

// ---------------------------------------------------------------- price changes

func (r *frameworkRepository) CreatePriceChange(ctx context.Context, item *model.PriceChangeEvent) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *frameworkRepository) FindPriceChange(ctx context.Context, id uint64) (*model.PriceChangeEvent, error) {
	var item model.PriceChangeEvent
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *frameworkRepository) FindPendingPriceChange(ctx context.Context, providerID uint64, resourceProductID uint64, field string) (*model.PriceChangeEvent, error) {
	var item model.PriceChangeEvent
	if err := r.db.WithContext(ctx).
		Where("provider_id = ? AND resource_product_id = ? AND field = ? AND status = ?",
			providerID, resourceProductID, field, model.PriceChangePending).
		Order("id desc").First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *frameworkRepository) UpdatePriceChange(ctx context.Context, item *model.PriceChangeEvent) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *frameworkRepository) ListPriceChanges(ctx context.Context, query dto.PriceChangeListQuery) ([]model.PriceChangeEvent, int64, error) {
	page, pageSize := normalizePage(query.Page, query.PageSize)
	base := r.db.WithContext(ctx).Model(&model.PriceChangeEvent{})
	if query.ProviderID > 0 {
		base = base.Where("provider_id = ?", query.ProviderID)
	}
	if st := strings.TrimSpace(query.Status); st != "" {
		base = base.Where("status = ?", st)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.PriceChangeEvent
	if err := base.Order("status = 'pending' desc, id desc").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ---------------------------------------------------------------- diffs

func (r *frameworkRepository) CreateDiffs(ctx context.Context, items []model.SyncDiff) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(items, 200).Error
}

func (r *frameworkRepository) ListDiffs(ctx context.Context, query dto.SyncDiffListQuery) ([]model.SyncDiff, int64, error) {
	page, pageSize := normalizePage(query.Page, query.PageSize)
	base := r.db.WithContext(ctx).Model(&model.SyncDiff{})
	if query.ProviderID > 0 {
		base = base.Where("provider_id = ?", query.ProviderID)
	}
	if query.TaskID > 0 {
		base = base.Where("task_id = ?", query.TaskID)
	}
	if s := strings.TrimSpace(query.Scope); s != "" {
		base = base.Where("scope = ?", model.NormalizeScope(s))
	}
	if a := strings.TrimSpace(query.Action); a != "" {
		base = base.Where("action = ?", a)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.SyncDiff
	if err := base.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *frameworkRepository) CountDiffsByAction(ctx context.Context, providerID uint64, since time.Time) (map[string]int64, error) {
	type row struct {
		Action string
		Total  int64
	}
	var rows []row
	base := r.db.WithContext(ctx).Model(&model.SyncDiff{}).Select("action, count(*) as total")
	if providerID > 0 {
		base = base.Where("provider_id = ?", providerID)
	}
	if !since.IsZero() {
		base = base.Where("created_at >= ?", since)
	}
	if err := base.Group("action").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, item := range rows {
		out[item.Action] = item.Total
	}
	return out, nil
}

// ---------------------------------------------------------------- 全量对账

func (r *frameworkRepository) ListResourceProductsByProvider(ctx context.Context, providerID uint64) ([]ResourceProductSnapshot, error) {
	var items []ResourceProductSnapshot
	err := r.db.WithContext(ctx).Table("resource_products").
		Select("id, upstream_id, name, cpu, memory, disk, disk_type, bandwidth, os, region, zone, cost_price, sale_price, status").
		Where("provider_id = ?", providerID).Scan(&items).Error
	return items, err
}

func (r *frameworkRepository) ListPoolSnapshotsByProvider(ctx context.Context, providerID uint64) ([]PoolSnapshot, error) {
	var items []PoolSnapshot
	err := r.db.WithContext(ctx).Table("resource_pools").
		Select("id, upstream_id, name, status").
		Where("provider_id = ?", providerID).Scan(&items).Error
	return items, err
}

// MarkResourceProductsOffline 把本地存在、本轮上游快照未出现的商品置为下线（软删除）。
// 仅改 status=0，不物理删除：历史订单/规格绑定仍可追溯；上游重新出现时由同步写回 status=1。
func (r *frameworkRepository) MarkResourceProductsOffline(ctx context.Context, providerID uint64, keepUpstreamIDs []string) (int64, error) {
	base := r.db.WithContext(ctx).Model(&resourceProductTable{}).
		Where("provider_id = ? AND status <> 0", providerID)
	if len(keepUpstreamIDs) > 0 {
		base = base.Where("upstream_id NOT IN ?", keepUpstreamIDs)
	}
	res := base.Updates(map[string]any{"status": 0, "updated_at": time.Now()})
	return res.RowsAffected, res.Error
}

func (r *frameworkRepository) MarkPoolsOffline(ctx context.Context, providerID uint64, keepUpstreamIDs []string) (int64, error) {
	base := r.db.WithContext(ctx).Table("resource_pools").
		Where("provider_id = ? AND status <> 0", providerID)
	if len(keepUpstreamIDs) > 0 {
		base = base.Where("upstream_id NOT IN ?", keepUpstreamIDs)
	}
	res := base.Updates(map[string]any{"status": 0, "updated_at": time.Now()})
	return res.RowsAffected, res.Error
}

// MarkInstancesOffline 把上游链路实例中本轮未出现的置 offline。
// 自营链路（source_mode='self'）实例不受上游同步影响，一律不标记。
func (r *frameworkRepository) MarkInstancesOffline(ctx context.Context, providerID uint64, keepInstanceIDs []string) (int64, error) {
	base := r.db.WithContext(ctx).Table("instances").
		Where("provider_id = ? AND source_mode = ?", providerID, "upstream").
		Where("status <> ?", "offline")
	if len(keepInstanceIDs) > 0 {
		base = base.Where("instance_id NOT IN ?", keepInstanceIDs)
	}
	res := base.Updates(map[string]any{"status": "offline", "updated_at": time.Now()})
	return res.RowsAffected, res.Error
}

// PurgeFrameworkBefore 分批清理差异与调价事件。
func (r *frameworkRepository) PurgeFrameworkBefore(ctx context.Context, before time.Time, batchSize int) (int64, error) {
	if batchSize <= 0 {
		batchSize = 5000
	}
	var total int64
	for _, table := range []string{"sync_diffs"} {
		for {
			res := r.db.WithContext(ctx).Exec(
				"DELETE FROM "+table+" WHERE id IN (SELECT id FROM "+table+" WHERE created_at < ? ORDER BY id LIMIT ?)",
				before, batchSize)
			if res.Error != nil {
				return total, res.Error
			}
			total += res.RowsAffected
			if res.RowsAffected < int64(batchSize) {
				break
			}
		}
	}
	// 调价事件保留更久（财务审计），仅清理已处置且超过保留期的记录。
	res := r.db.WithContext(ctx).Exec(
		`DELETE FROM price_change_events
		  WHERE id IN (SELECT id FROM price_change_events
		                WHERE created_at < ? AND status <> ? ORDER BY id LIMIT ?)`,
		before, model.PriceChangePending, batchSize)
	if res.Error != nil {
		return total, res.Error
	}
	total += res.RowsAffected
	return total, nil
}

// resourceProductTable 仅用于构造表名，避免 sync 模块依赖 resource/product/model。
type resourceProductTable struct{}

// TableName 指定表名
func (resourceProductTable) TableName() string { return "resource_products" }

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}
