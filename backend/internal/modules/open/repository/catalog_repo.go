package repository

import (
	"context"

	"gorm.io/gorm"

	providermodel "hostsent/backend/internal/modules/admin/resource/provider/model"
)

// CatalogRepository 开放目录只读数据源（区域/镜像聚合）。
type CatalogRepository interface {
	// ListEnabledPools 启用中的资源池（标准区域来源）。
	ListEnabledPools(ctx context.Context) ([]providermodel.ResourcePool, error)
	// ListOnSaleSkuSpecs 在售商品 SKU 的原子取值 JSON 列表（标准镜像聚合来源）。
	ListOnSaleSkuSpecs(ctx context.Context) ([]string, error)
}

type catalogRepository struct {
	db *gorm.DB
}

// NewCatalogRepository 构造开放目录仓储。
func NewCatalogRepository(db *gorm.DB) CatalogRepository {
	return &catalogRepository{db: db}
}

func (r *catalogRepository) ListEnabledPools(ctx context.Context) ([]providermodel.ResourcePool, error) {
	var pools []providermodel.ResourcePool
	err := r.db.WithContext(ctx).
		Where("status = 1").
		Order("id").Find(&pools).Error
	return pools, err
}

func (r *catalogRepository) ListOnSaleSkuSpecs(ctx context.Context) ([]string, error) {
	var specs []string
	err := r.db.WithContext(ctx).Table("product_specs AS ps").
		Joins("JOIN products p ON p.id = ps.product_id").
		Where("p.status = 1 AND ps.status = 1 AND ps.specs <> ''").
		Pluck("ps.specs", &specs).Error
	return specs, err
}
