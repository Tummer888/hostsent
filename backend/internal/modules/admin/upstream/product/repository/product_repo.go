package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hostsent/backend/internal/modules/admin/upstream/product/dto"
	"hostsent/backend/internal/modules/admin/upstream/product/model"
)

// ProductRepository 上游商品仓储接口
type ProductRepository interface {
	List(ctx context.Context, query dto.ProductListQuery) ([]model.UpstreamProduct, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.UpstreamProduct, error)
	Update(ctx context.Context, item *model.UpstreamProduct) error
	UpsertMany(ctx context.Context, providerID uint64, items []model.UpstreamProduct) error
}

type productRepository struct {
	db *gorm.DB
}

// NewProductRepository 创建上游商品仓储实现
func NewProductRepository(db *gorm.DB) ProductRepository {
	return &productRepository{db: db}
}

func (r *productRepository) List(ctx context.Context, query dto.ProductListQuery) ([]model.UpstreamProduct, int64, error) {
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

	base := r.db.WithContext(ctx).Model(&model.UpstreamProduct{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("name ILIKE ? OR upstream_id ILIKE ?", like, like)
	}
	if query.ProviderID > 0 {
		base = base.Where("provider_id = ?", query.ProviderID)
	}
	if query.Status != 0 {
		base = base.Where("status = ?", query.Status)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.UpstreamProduct
	if err := base.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *productRepository) FindByID(ctx context.Context, id uint64) (*model.UpstreamProduct, error) {
	var item model.UpstreamProduct
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *productRepository) Update(ctx context.Context, item *model.UpstreamProduct) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *productRepository) UpsertMany(ctx context.Context, providerID uint64, items []model.UpstreamProduct) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "provider_id"}, {Name: "upstream_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "cpu", "memory", "disk", "disk_type", "bandwidth", "os", "region", "zone", "specs", "raw_specs", "cost_price", "sale_price", "status", "updated_at"}),
	}).Create(&items).Error
}