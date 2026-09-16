// Package repository 提供菜单模块的数据访问实现。
package repository

import (
	"context"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/menu/model"
)

// MenuRepository 菜单数据访问接口。
//
// 只读：menus 表的唯一写入点是 seed（internal/pkg/db 的 SeedMenus，doc102 M0）。
type MenuRepository interface {
	List(ctx context.Context, platform string) ([]model.Menu, error)
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
