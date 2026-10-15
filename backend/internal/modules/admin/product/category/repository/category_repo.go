// Package repository 提供商品分类（category）子域的数据访问实现。
package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/product/category/model"
)

// CategoryRepository 定义商品分类数据访问能力。
type CategoryRepository interface {
	// List 取分类；status=nil 时返回全部（含停用），非 nil 时按状态精确筛选。
	List(ctx context.Context, status *int) ([]model.ProductCategory, error)
	FindByID(ctx context.Context, id uint64) (*model.ProductCategory, error)
	Create(ctx context.Context, category *model.ProductCategory) error
	Update(ctx context.Context, category *model.ProductCategory) error
	// Delete 递归删除节点及其全部子孙分类。
	Delete(ctx context.Context, id uint64) error
	// ExistsChildren 判断指定分类下是否仍有子分类。
	ExistsChildren(ctx context.Context, id uint64) (bool, error)
	// CountProductsByCategory 返回「分类 ID → 该分类自身直接挂的商品数」。
	// 只统计自身归属，不做子树累加：树节点上的数字要与"点它会看到什么"口径一致，
	// 而点父节点时列表是按子树展开的，子树的量由前端按需累加。
	CountProductsByCategory(ctx context.Context) (map[uint64]int64, error)
	// CountUncategorizedProducts 统计未分类商品数。
	// 口径包含两种：category_id 为空（0），以及指向**已删除分类**的悬空值 ——
	// 后者在界面上同样显示为"未分类"，不并进来会让这个数字对不上列表。
	CountUncategorizedProducts(ctx context.Context) (int64, error)
	// CountProductsInSubtree 统计挂在指定分类**及其整棵子树**上的商品数。
	// 删除前校验用：删父分类会连带删掉子孙分类，子孙下的商品同样会变成悬空值。
	CountProductsInSubtree(ctx context.Context, rootID uint64) (int64, error)
}

type categoryRepository struct {
	db *gorm.DB
}

// NewCategoryRepository 创建分类仓储实现。
func NewCategoryRepository(db *gorm.DB) CategoryRepository {
	return &categoryRepository{db: db}
}

func (r *categoryRepository) List(ctx context.Context, status *int) ([]model.ProductCategory, error) {
	var items []model.ProductCategory
	base := r.db.WithContext(ctx).Order("parent_id asc, sort_order asc, id asc")
	if status != nil {
		base = base.Where("status = ?", *status)
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

// CountProductsByCategory 统计每个分类自身直接挂的商品数。
//
// 用原生表名而不是 import 商品域模型：分类域反向依赖商品域会形成循环，
// 且这里只读计数值（与同文件 CountInstancesByProduct 同一处置）。
// 只按 category_id 分组、不做子树累加：节点上的数字要与"点进去看什么"一致。
func (r *categoryRepository) CountProductsByCategory(ctx context.Context) (map[uint64]int64, error) {
	var rows []struct {
		CategoryID uint64
		Total      int64
	}
	if err := r.db.WithContext(ctx).Table("products").
		Select("category_id, count(*) AS total").
		Group("category_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[uint64]int64, len(rows))
	for _, row := range rows {
		out[row.CategoryID] = row.Total
	}
	return out, nil
}

// CountUncategorizedProducts 统计未分类商品数。
//
// 口径必须与「未分类」筛选、与列表里那一列显示的文字三者一致，
// 否则运营会看到"导航说 3 个、点进去 2 个"这类对不上的数字。包含两种：
//   - category_id = 0：从没选过分类（导入时留空、或建品未填）；
//   - category_id 指向**已删除的分类**：分类删掉后商品没跟着走，界面上同样只能显示"未分类"。
//
// 第二种是历史遗留（删分类时未校验占用），在界面上与第一种无从分辨，故合并计数；
// db.go 的 backfillOrphanProductCategories 会把它们归一到 0，两者最终收敛为一类。
func (r *categoryRepository) CountUncategorizedProducts(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("products").
		Where("category_id = 0 OR category_id NOT IN (SELECT id FROM product_categories)").
		Count(&n).Error
	return n, err
}

// CountProductsInSubtree 统计挂在该分类及其整棵子树上的商品数（删除前校验用）。
//
// 必须含子树：删除父分类会递归删掉子孙分类（见 Delete），子孙下的商品同样会
// 变成指向不存在分类的悬空值。只查自身会让"父分类没有商品、子分类有"的情况漏过去。
func (r *categoryRepository) CountProductsInSubtree(ctx context.Context, rootID uint64) (int64, error) {
	ids := []uint64{rootID}
	if err := collectChildCategoryIDs(r.db.WithContext(ctx), rootID, &ids); err != nil {
		return 0, err
	}
	var n int64
	err := r.db.WithContext(ctx).Table("products").Where("category_id IN ?", ids).Count(&n).Error
	return n, err
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
