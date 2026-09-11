package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	productmodel "hostsent/backend/internal/modules/admin/resource/product/model"
	"hostsent/backend/internal/modules/admin/resource/provider/dto"
	"hostsent/backend/internal/modules/admin/resource/provider/model"
)

// ProviderRepository 上游提供商仓储接口
type ProviderRepository interface {
	List(ctx context.Context, query dto.ProviderListQuery) ([]model.ResourceProvider, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.ResourceProvider, error)
	Create(ctx context.Context, item *model.ResourceProvider) error
	Update(ctx context.Context, item *model.ResourceProvider) error
	Delete(ctx context.Context, id uint64) error
	// CountAssociations 统计提供商关联的商品与资源池数量（删除前检查用）
	CountAssociations(ctx context.Context, id uint64) (products int64, pools int64, err error)
	// UpdateStats 回写提供商资源统计
	UpdateStats(ctx context.Context, providerID uint64, totalCPU, totalMemory, totalDisk, usedCPU, usedMemory, usedDisk int) error
	// MarkSynced 更新提供商最近同步时间
	MarkSynced(ctx context.Context, providerID uint64) error
	// RecordSyncSuccess 记录一次同步成功：刷新 last_sync_at/last_success_at，清零失败计数与错误
	RecordSyncSuccess(ctx context.Context, providerID uint64) error
	// RecordSyncFailure 记录一次同步失败：累加连续失败次数，达到 maxFailures 自动熔断暂停
	RecordSyncFailure(ctx context.Context, providerID uint64, errMsg string, maxFailures int) error
	// PauseSync 永久性错误（适配器未注册/凭证缺失等）立即熔断并写明原因
	PauseSync(ctx context.Context, providerID uint64, reason string) error
	// ResumeSync 手动恢复：解除熔断并清零失败计数
	ResumeSync(ctx context.Context, providerID uint64) error
	// ListAll 返回全部渠道（含禁用），供启动期凭证归一/注册表同步等批处理使用。
	ListAll(ctx context.Context) ([]model.ResourceProvider, error)
	// UpdateCredentials 仅更新 credentials 列，避免全量 Save 覆盖其他并发修改。
	UpdateCredentials(ctx context.Context, providerID uint64, credentialsJSON string) error
}

type providerRepository struct {
	db *gorm.DB
}

// NewProviderRepository 创建上游提供商仓储实现
func NewProviderRepository(db *gorm.DB) ProviderRepository {
	return &providerRepository{db: db}
}

func (r *providerRepository) List(ctx context.Context, query dto.ProviderListQuery) ([]model.ResourceProvider, int64, error) {
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

	base := r.db.WithContext(ctx).Model(&model.ResourceProvider{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("name ILIKE ? OR provider_type ILIKE ? OR api_endpoint ILIKE ?", like, like, like)
	}
	if pt := strings.TrimSpace(query.ProviderType); pt != "" {
		base = base.Where("provider_type = ?", pt)
	}
	if k := strings.TrimSpace(query.Kind); k != "" {
		base = base.Where("kind = ?", k)
	}
	if query.Status != 0 {
		base = base.Where("status = ?", query.Status)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.ResourceProvider
	if err := base.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *providerRepository) FindByID(ctx context.Context, id uint64) (*model.ResourceProvider, error) {
	var item model.ResourceProvider
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *providerRepository) Create(ctx context.Context, item *model.ResourceProvider) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *providerRepository) Update(ctx context.Context, item *model.ResourceProvider) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *providerRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.ResourceProvider{}, id).Error
}

func (r *providerRepository) CountAssociations(ctx context.Context, id uint64) (int64, int64, error) {
	var products, pools int64
	if err := r.db.WithContext(ctx).Model(&productmodel.ResourceProduct{}).
		Where("provider_id = ?", id).Count(&products).Error; err != nil {
		return 0, 0, err
	}
	if err := r.db.WithContext(ctx).Model(&model.ResourcePool{}).
		Where("provider_id = ?", id).Count(&pools).Error; err != nil {
		return 0, 0, err
	}
	return products, pools, nil
}

func (r *providerRepository) UpdateStats(ctx context.Context, providerID uint64, totalCPU, totalMemory, totalDisk, usedCPU, usedMemory, usedDisk int) error {
	return r.db.WithContext(ctx).Model(&model.ResourceProvider{}).
		Where("id = ?", providerID).
		Updates(map[string]interface{}{
			"total_cpu":    totalCPU,
			"total_memory": totalMemory,
			"total_disk":   totalDisk,
			"used_cpu":     usedCPU,
			"used_memory":  usedMemory,
			"used_disk":    usedDisk,
		}).Error
}

func (r *providerRepository) MarkSynced(ctx context.Context, providerID uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.ResourceProvider{}).
		Where("id = ?", providerID).
		Update("last_sync_at", &now).Error
}

// RecordSyncSuccess 成功用例：同步间隔以 last_sync_at 为准，失败也要 touch（见 RecordSyncFailure）。
func (r *providerRepository) RecordSyncSuccess(ctx context.Context, providerID uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.ResourceProvider{}).
		Where("id = ?", providerID).
		Updates(map[string]interface{}{
			"last_sync_at":         &now,
			"last_success_at":      &now,
			"consecutive_failures": 0,
			"last_sync_error":      "",
		}).Error
}

// RecordSyncFailure 失败用例：失败同样刷新 last_sync_at，使 sync_interval 对失败渠道也生效，
// 避免坏渠道每 60 秒重试；连续失败达到 maxFailures 时置 sync_paused（由调度器跳过）。
func (r *providerRepository) RecordSyncFailure(ctx context.Context, providerID uint64, errMsg string, maxFailures int) error {
	now := time.Now()
	if maxFailures <= 0 {
		maxFailures = 5
	}
	// 单条 UPDATE 内自增并用旧值判定，避免读改写竞态。
	return r.db.WithContext(ctx).Model(&model.ResourceProvider{}).
		Where("id = ?", providerID).
		Updates(map[string]interface{}{
			"last_sync_at":         &now,
			"last_sync_error":      errMsg,
			"consecutive_failures": gorm.Expr("consecutive_failures + 1"),
			"sync_paused":          gorm.Expr("(consecutive_failures + 1) >= ?", maxFailures),
		}).Error
}

func (r *providerRepository) PauseSync(ctx context.Context, providerID uint64, reason string) error {
	return r.db.WithContext(ctx).Model(&model.ResourceProvider{}).
		Where("id = ?", providerID).
		Updates(map[string]interface{}{
			"sync_paused":     true,
			"last_sync_error": reason,
		}).Error
}

func (r *providerRepository) ResumeSync(ctx context.Context, providerID uint64) error {
	return r.db.WithContext(ctx).Model(&model.ResourceProvider{}).
		Where("id = ?", providerID).
		Updates(map[string]interface{}{
			"sync_paused":          false,
			"consecutive_failures": 0,
			"last_sync_error":      "",
		}).Error
}

func (r *providerRepository) ListAll(ctx context.Context) ([]model.ResourceProvider, error) {
	var items []model.ResourceProvider
	if err := r.db.WithContext(ctx).Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *providerRepository) UpdateCredentials(ctx context.Context, providerID uint64, credentialsJSON string) error {
	return r.db.WithContext(ctx).Model(&model.ResourceProvider{}).
		Where("id = ?", providerID).
		Update("credentials", credentialsJSON).Error
}

// PoolRepository 资源池仓储接口
type PoolRepository interface {
	List(ctx context.Context, query dto.PoolListQuery) ([]model.ResourcePool, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.ResourcePool, error)
	UpsertPools(ctx context.Context, providerID uint64, items []model.ResourcePool) error
}

// NewPoolRepository 创建资源池仓储实现
func NewPoolRepository(db *gorm.DB) PoolRepository {
	return &poolRepository{db: db}
}

type poolRepository struct {
	db *gorm.DB
}

func (r *poolRepository) List(ctx context.Context, query dto.PoolListQuery) ([]model.ResourcePool, int64, error) {
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

	base := r.db.WithContext(ctx).Model(&model.ResourcePool{})
	if query.ProviderID > 0 {
		base = base.Where("provider_id = ?", query.ProviderID)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.ResourcePool
	if err := base.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *poolRepository) FindByID(ctx context.Context, id uint64) (*model.ResourcePool, error) {
	var item model.ResourcePool
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *poolRepository) UpsertPools(ctx context.Context, providerID uint64, items []model.ResourcePool) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "provider_id"}, {Name: "upstream_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "pool_type", "total_cpu", "total_memory", "total_disk", "used_cpu", "used_memory", "used_disk", "status", "last_sync_at", "updated_at"}),
	}).Create(&items).Error
}
