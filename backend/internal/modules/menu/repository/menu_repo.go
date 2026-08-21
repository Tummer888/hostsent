// Package repository 提供菜单模块的数据访问实现。
package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/menu/model"
)

// MenuRepository 定义菜单数据访问所需的仓储能力。
type MenuRepository interface {
	List(ctx context.Context, platform string) ([]model.Menu, error)
	Create(ctx context.Context, menu *model.Menu) error
	FindByID(ctx context.Context, id uint64) (*model.Menu, error)
	Update(ctx context.Context, menu *model.Menu) error
	// Delete 递归删除节点及其全部子孙节点。
	Delete(ctx context.Context, id uint64) error
}

type menuRepository struct {
	db *gorm.DB
}

// NewMenuRepository 创建菜单仓储实现。
func NewMenuRepository(db *gorm.DB) MenuRepository {
	return &menuRepository{db: db}
}

func (r *menuRepository) List(ctx context.Context, platform string) ([]model.Menu, error) {
	var menus []model.Menu
	query := r.db.WithContext(ctx).Order("platform asc, parent_id asc, sort_order asc, id asc")
	if platform != "" {
		query = query.Where("platform = ?", platform)
	}
	if err := query.Find(&menus).Error; err != nil {
		return nil, err
	}
	return menus, nil
}

func (r *menuRepository) Create(ctx context.Context, menu *model.Menu) error {
	return r.db.WithContext(ctx).Create(menu).Error
}

func (r *menuRepository) FindByID(ctx context.Context, id uint64) (*model.Menu, error) {
	var menu model.Menu
	if err := r.db.WithContext(ctx).First(&menu, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &menu, nil
}

func (r *menuRepository) Update(ctx context.Context, menu *model.Menu) error {
	return r.db.WithContext(ctx).Save(menu).Error
}

func (r *menuRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 自底向上收集待删除 ID，避免外键残留与孤儿节点。
		ids := []uint64{id}
		if err := collectChildIDs(tx, id, &ids); err != nil {
			return err
		}
		return tx.Where("id IN ?", ids).Delete(&model.Menu{}).Error
	})
}

func collectChildIDs(tx *gorm.DB, parentID uint64, ids *[]uint64) error {
	var children []uint64
	if err := tx.Model(&model.Menu{}).Where("parent_id = ?", parentID).Pluck("id", &children).Error; err != nil {
		return err
	}
	for _, childID := range children {
		*ids = append(*ids, childID)
		if err := collectChildIDs(tx, childID, ids); err != nil {
			return err
		}
	}
	return nil
}
