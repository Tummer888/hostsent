package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hostsent/backend/internal/modules/admin/resource/sync/dto"
	"hostsent/backend/internal/modules/admin/resource/sync/model"
)

// SyncRepository 同步任务/日志/实例仓储接口
type SyncRepository interface {
	ListTasks(ctx context.Context, query dto.SyncTaskListQuery) ([]model.SyncTask, int64, error)
	FindTaskByID(ctx context.Context, id uint64) (*model.SyncTask, error)
	CreateTask(ctx context.Context, item *model.SyncTask) error
	UpdateTask(ctx context.Context, item *model.SyncTask) error
	CreateLog(ctx context.Context, item *model.SyncLog) error
	CountRunning(ctx context.Context, providerID uint64, taskType string) (int64, error)
	FindDueProviders(ctx context.Context, now time.Time) ([]uint64, error)
	ListLogs(ctx context.Context, query dto.SyncLogListQuery) ([]model.SyncLog, int64, error)
	ListInstances(ctx context.Context, query dto.InstanceListQuery) ([]model.Instance, int64, error)
	FindInstanceByID(ctx context.Context, id uint64) (*model.Instance, error)
	UpsertInstances(ctx context.Context, items []model.Instance) error
	CountInstancesByProductUser(ctx context.Context, userID, productID uint64) (int64, error)
}

type syncRepository struct {
	db *gorm.DB
}

// NewSyncRepository 创建同步仓储实现
func NewSyncRepository(db *gorm.DB) SyncRepository {
	return &syncRepository{db: db}
}

func (r *syncRepository) ListTasks(ctx context.Context, query dto.SyncTaskListQuery) ([]model.SyncTask, int64, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	base := r.db.WithContext(ctx).Model(&model.SyncTask{})
	if query.ProviderID > 0 {
		base = base.Where("provider_id = ?", query.ProviderID)
	}
	if tt := strings.TrimSpace(query.TaskType); tt != "" {
		base = base.Where("task_type = ?", tt)
	}
	if st := strings.TrimSpace(query.Status); st != "" {
		base = base.Where("status = ?", st)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.SyncTask
	if err := base.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *syncRepository) FindTaskByID(ctx context.Context, id uint64) (*model.SyncTask, error) {
	var item model.SyncTask
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *syncRepository) CreateTask(ctx context.Context, item *model.SyncTask) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *syncRepository) UpdateTask(ctx context.Context, item *model.SyncTask) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *syncRepository) CreateLog(ctx context.Context, item *model.SyncLog) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *syncRepository) CountRunning(ctx context.Context, providerID uint64, taskType string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.SyncTask{}).
		Where("provider_id = ? AND task_type = ? AND status IN ?", providerID, taskType, []string{"pending", "running"}).
		Count(&n).Error
	return n, err
}

func (r *syncRepository) FindDueProviders(ctx context.Context, now time.Time) ([]uint64, error) {
	var ids []uint64
	err := r.db.WithContext(ctx).
		Table("resource_providers AS p").
		Where("p.sync_enabled = ?", true).
		Where("p.status = ?", 1).
		Where("p.last_sync_at IS NULL OR p.last_sync_at + make_interval(secs => p.sync_interval) <= ?", now).
		Where("NOT EXISTS (SELECT 1 FROM sync_tasks t WHERE t.provider_id = p.id AND t.status IN ('pending','running'))").
		Pluck("p.id", &ids).Error
	return ids, err
}

func (r *syncRepository) ListLogs(ctx context.Context, query dto.SyncLogListQuery) ([]model.SyncLog, int64, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	base := r.db.WithContext(ctx).Model(&model.SyncLog{})
	if query.ProviderID > 0 {
		base = base.Where("provider_id = ?", query.ProviderID)
	}
	if query.TaskID > 0 {
		base = base.Where("task_id = ?", query.TaskID)
	}
	if st := strings.TrimSpace(query.Status); st != "" {
		base = base.Where("status = ?", st)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.SyncLog
	if err := base.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *syncRepository) ListInstances(ctx context.Context, query dto.InstanceListQuery) ([]model.Instance, int64, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	base := r.db.WithContext(ctx).Model(&model.Instance{})
	if query.ProviderID > 0 {
		base = base.Where("provider_id = ?", query.ProviderID)
	}
	if query.UserID > 0 {
		base = base.Where("user_id = ?", query.UserID)
	}
	if st := strings.TrimSpace(query.Status); st != "" {
		base = base.Where("status = ?", st)
	}
	if kw := strings.TrimSpace(query.Keyword); kw != "" {
		like := "%" + kw + "%"
		base = base.Where("name ILIKE ? OR instance_id ILIKE ?", like, like)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.Instance
	if err := base.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *syncRepository) FindInstanceByID(ctx context.Context, id uint64) (*model.Instance, error) {
	var item model.Instance
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *syncRepository) UpsertInstances(ctx context.Context, items []model.Instance) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "instance_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"status", "name", "public_ip", "private_ip", "cpu", "memory", "disk", "raw_data", "expire_at", "updated_at"}),
	}).Create(&items).Error
}

// CountInstancesByProductUser 统计某用户已开通某商品（instances）的实例数，用于开通幂等判断。
func (r *syncRepository) CountInstancesByProductUser(ctx context.Context, userID, productID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Instance{}).
		Where("user_id = ? AND product_id = ?", userID, productID).
		Count(&count).Error
	return count, err
}
