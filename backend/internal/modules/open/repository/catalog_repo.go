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

// ListEnabledPools 启用中的资源池（标准区域来源）。
//
// 容量型池（node 宿主机 / storage 存储）不算区域：魔方云同步会把每台宿主机注册成一条
// node 池，若不过滤，开放接口 /regions 会把这些宿主机当成区域返回。区域维度只保留
// region/zone 与非容量型池，与历史行为一致（兼容既有演示渠道的 compute/memory 池）。
func (r *catalogRepository) ListEnabledPools(ctx context.Context) ([]providermodel.ResourcePool, error) {
	var pools []providermodel.ResourcePool
	err := r.db.WithContext(ctx).
		Where("status = 1").
		Where("pool_type NOT IN ?", []string{"node", "storage"}).
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
