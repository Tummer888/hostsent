// Package repository 提供产品管理模块的数据访问实现。
package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/product/dto"
	"hostsent/backend/internal/modules/admin/product/model"
)

// ProductRepository 定义产品数据访问能力。
type ProductRepository interface {
	List(ctx context.Context, query dto.ProductListQuery) ([]model.Product, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.Product, error)
	Create(ctx context.Context, item *model.Product) error
	Update(ctx context.Context, item *model.Product) error
	Delete(ctx context.Context, id uint64) error
	ListSpecs(ctx context.Context, productID uint64) ([]model.ProductSpec, error)
	AddHistory(ctx context.Context, history *model.ProductHistory) error
	ListHistory(ctx context.Context, productID uint64) ([]model.ProductHistory, error)
}

type productRepository struct {
	db *gorm.DB
}

// NewProductRepository 创建产品仓储实现。
func NewProductRepository(db *gorm.DB) ProductRepository {
	return &productRepository{db: db}
}

func (r *productRepository) List(ctx context.Context, query dto.ProductListQuery) ([]model.Product, int64, error) {
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

	base := r.db.WithContext(ctx).Model(&model.Product{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("name ILIKE ? OR code ILIKE ?", like, like)
	}
	if query.CategoryID > 0 {
		base = base.Where("category_id = ?", query.CategoryID)
	}
	if query.Status != 0 {
		base = base.Where("status = ?", query.Status)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.Product
	if err := base.Order("sort_order asc, id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *productRepository) FindByID(ctx context.Context, id uint64) (*model.Product, error) {
	var item model.Product
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *productRepository) Create(ctx context.Context, item *model.Product) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *productRepository) Update(ctx context.Context, item *model.Product) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *productRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.Product{}, id).Error
}

func (r *productRepository) ListSpecs(ctx context.Context, productID uint64) ([]model.ProductSpec, error) {
	var items []model.ProductSpec
	if err := r.db.WithContext(ctx).Where("product_id = ?", productID).Order("sort_order asc, id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *productRepository) AddHistory(ctx context.Context, history *model.ProductHistory) error {
	return r.db.WithContext(ctx).Create(history).Error
}

func (r *productRepository) ListHistory(ctx context.Context, productID uint64) ([]model.ProductHistory, error) {
	var items []model.ProductHistory
	if err := r.db.WithContext(ctx).Where("product_id = ?", productID).Order("created_at desc, id desc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
