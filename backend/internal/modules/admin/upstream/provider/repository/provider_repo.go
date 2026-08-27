package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hostsent/backend/internal/modules/admin/upstream/provider/dto"
	"hostsent/backend/internal/modules/admin/upstream/provider/model"
	productmodel "hostsent/backend/internal/modules/admin/upstream/product/model"
)

// ProviderRepository 上游提供商仓储接口
type ProviderRepository interface {
	List(ctx context.Context, query dto.ProviderListQuery) ([]model.UpstreamProvider, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.UpstreamProvider, error)
	Create(ctx context.Context, item *model.UpstreamProvider) error
	Update(ctx context.Context, item *model.UpstreamProvider) error
	Delete(ctx context.Context, id uint64) error
	// CountAssociations 统计提供商关联的商品与资源池数量（删除前检查用）
	CountAssociations(ctx context.Context, id uint64) (products int64, pools int64, err error)
	// UpdateStats 回写提供商资源统计
	UpdateStats(ctx context.Context, providerID uint64, totalCPU, totalMemory, totalDisk, usedCPU, usedMemory, usedDisk int) error
	// MarkSynced 更新提供商最近同步时间
	MarkSynced(ctx context.Context, providerID uint64) error
}

type providerRepository struct {
	db *gorm.DB
}

// NewProviderRepository 创建上游提供商仓储实现
func NewProviderRepository(db *gorm.DB) ProviderRepository {
	return &providerRepository{db: db}
}

func (r *providerRepository) List(ctx context.Context, query dto.ProviderListQuery) ([]model.UpstreamProvider, int64, error) {
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

	base := r.db.WithContext(ctx).Model(&model.UpstreamProvider{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("name ILIKE ? OR provider_type ILIKE ? OR api_endpoint ILIKE ?", like, like, like)
	}
	if pt := strings.TrimSpace(query.ProviderType); pt != "" {
		base = base.Where("provider_type = ?", pt)
	}
	if query.Status != 0 {
		base = base.Where("status = ?", query.Status)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.UpstreamProvider
	if err := base.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *providerRepository) FindByID(ctx context.Context, id uint64) (*model.UpstreamProvider, error) {
	var item model.UpstreamProvider
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *providerRepository) Create(ctx context.Context, item *model.UpstreamProvider) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *providerRepository) Update(ctx context.Context, item *model.UpstreamProvider) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *providerRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.UpstreamProvider{}, id).Error
}

func (r *providerRepository) CountAssociations(ctx context.Context, id uint64) (int64, int64, error) {
	var products, pools int64
	if err := r.db.WithContext(ctx).Model(&productmodel.UpstreamProduct{}).
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
	return r.db.WithContext(ctx).Model(&model.UpstreamProvider{}).
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
	return r.db.WithContext(ctx).Model(&model.UpstreamProvider{}).
		Where("id = ?", providerID).
		Update("last_sync_at", &now).Error
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