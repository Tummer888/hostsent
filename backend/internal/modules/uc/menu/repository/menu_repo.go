// Package repository 提供用户中心菜单模块的数据访问实现。
package repository

import (
	"context"

	"gorm.io/gorm"

	menumodel "hostsent/backend/internal/modules/admin/menu/model"
)

// MenuRepository 用户中心菜单数据访问接口。
// 数据实体复用 admin/menu/model.Menu（menus 表按 platform 区分管理端与用户端）。
type MenuRepository interface {
	// ListByPlatform 按平台查询启用状态的菜单记录，按父级、排序、ID 升序返回。
	ListByPlatform(ctx context.Context, platform string) ([]menumodel.Menu, error)
}

type menuRepository struct {
	db *gorm.DB
}

// NewMenuRepository 创建用户中心菜单仓储实现。
func NewMenuRepository(db *gorm.DB) MenuRepository {
	return &menuRepository{db: db}
}

func (r *menuRepository) ListByPlatform(ctx context.Context, platform string) ([]menumodel.Menu, error) {
	var menus []menumodel.Menu
	if err := r.db.WithContext(ctx).
		Where("platform = ?", platform).
		Where("status = ?", menumodel.StatusActive).
		Order("parent_id asc, sort_order asc, id asc").
		Find(&menus).Error; err != nil {
		return nil, err
	}
	return menus, nil
}
