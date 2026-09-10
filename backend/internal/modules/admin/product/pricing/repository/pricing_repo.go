// Package repository 提供定价与计费（pricing）子域的数据访问实现。
package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/product/pricing/dto"
	"hostsent/backend/internal/modules/admin/product/pricing/model"
)

// PricingRepository 定义价格策略数据访问能力。
type PricingRepository interface {
	List(ctx context.Context, query dto.PricingQuery) ([]model.ProductPricing, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.ProductPricing, error)
	// FindByProductID 取商品当前启用的计费模板（基础价来源，P5-02）；无则返回 nil。
	FindByProductID(ctx context.Context, productID uint64) (*model.ProductPricing, error)
	Create(ctx context.Context, item *model.ProductPricing) error
	Update(ctx context.Context, item *model.ProductPricing) error
	Delete(ctx context.Context, id uint64) error
}

type pricingRepository struct {
	db *gorm.DB
}

func NewPricingRepository(db *gorm.DB) PricingRepository {
	return &pricingRepository{db: db}
}

func (r *pricingRepository) List(ctx context.Context, query dto.PricingQuery) ([]model.ProductPricing, int64, error) {
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
	base := r.db.WithContext(ctx).Model(&model.ProductPricing{})
	if query.ProductID > 0 {
		base = base.Where("product_id = ?", query.ProductID)
	}
	if query.BillingMode != "" {
		base = base.Where("billing_mode = ?", query.BillingMode)
	}
	if query.Status != 0 {
		base = base.Where("status = ?", query.Status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.ProductPricing
	if err := base.Order("product_id asc, id asc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *pricingRepository) FindByID(ctx context.Context, id uint64) (*model.ProductPricing, error) {
	var item model.ProductPricing
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// FindByProductID 取商品启用中的计费模板；商品未配置时返回 (nil, nil)。
func (r *pricingRepository) FindByProductID(ctx context.Context, productID uint64) (*model.ProductPricing, error) {
	var item model.ProductPricing
	err := r.db.WithContext(ctx).
		Where("product_id = ? AND status = ?", productID, model.PricingEnabled).
		Order("id asc").
		First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *pricingRepository) Create(ctx context.Context, item *model.ProductPricing) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *pricingRepository) Update(ctx context.Context, item *model.ProductPricing) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *pricingRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.ProductPricing{}, id).Error
}
