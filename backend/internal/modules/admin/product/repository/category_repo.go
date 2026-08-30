package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/product/model"
)

// CategoryRepository 定义产品分类数据访问能力。
type CategoryRepository interface {
	List(ctx context.Context, status int) ([]model.ProductCategory, error)
	FindByID(ctx context.Context, id uint64) (*model.ProductCategory, error)
	Create(ctx context.Context, category *model.ProductCategory) error
	Update(ctx context.Context, category *model.ProductCategory) error
	// Delete 递归删除节点及其全部子孙分类。
	Delete(ctx context.Context, id uint64) error
	// ExistsChildren 判断指定分类下是否仍有子分类。
	ExistsChildren(ctx context.Context, id uint64) (bool, error)
}

type categoryRepository struct {
	db *gorm.DB
}

// NewCategoryRepository 创建分类仓储实现。
func NewCategoryRepository(db *gorm.DB) CategoryRepository {
	return &categoryRepository{db: db}
}

func (r *categoryRepository) List(ctx context.Context, status int) ([]model.ProductCategory, error) {
	var items []model.ProductCategory
	base := r.db.WithContext(ctx).Order("parent_id asc, sort_order asc, id asc")
	if status != 0 {
		base = base.Where("status = ?", status)
	}
	if err := base.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *categoryRepository) FindByID(ctx context.Context, id uint64) (*model.ProductCategory, error) {
	var category model.ProductCategory
	if err := r.db.WithContext(ctx).First(&category, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &category, nil
}

func (r *categoryRepository) Create(ctx context.Context, category *model.ProductCategory) error {
	return r.db.WithContext(ctx).Create(category).Error
}

func (r *categoryRepository) Update(ctx context.Context, category *model.ProductCategory) error {
	return r.db.WithContext(ctx).Save(category).Error
}

func (r *categoryRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 自底向上收集待删除 ID，避免外键残留与孤儿节点。
		ids := []uint64{id}
		if err := collectChildCategoryIDs(tx, id, &ids); err != nil {
			return err
		}
		return tx.Where("id IN ?", ids).Delete(&model.ProductCategory{}).Error
	})
}

func (r *categoryRepository) ExistsChildren(ctx context.Context, id uint64) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.ProductCategory{}).Where("parent_id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func collectChildCategoryIDs(tx *gorm.DB, parentID uint64, ids *[]uint64) error {
	var children []uint64
	if err := tx.Model(&model.ProductCategory{}).Where("parent_id = ?", parentID).Pluck("id", &children).Error; err != nil {
		return err
	}
	for _, childID := range children {
		*ids = append(*ids, childID)
		if err := collectChildCategoryIDs(tx, childID, ids); err != nil {
			return err
		}
	}
	return nil
}
