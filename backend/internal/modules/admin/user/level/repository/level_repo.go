// Package repository 提供用户等级模块的数据访问实现。
package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/level/dto"
	"hostsent/backend/internal/modules/admin/user/level/model"
)

// UserLevelRepository 定义用户等级的持久化操作。
type UserLevelRepository interface {
	List(ctx context.Context, query dto.ListQuery) ([]model.UserLevel, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.UserLevel, error)
	FindByCode(ctx context.Context, code string) (*model.UserLevel, error)
	Create(ctx context.Context, item *model.UserLevel) error
	Update(ctx context.Context, item *model.UserLevel) error
	Delete(ctx context.Context, id uint64) error
}

type userLevelRepository struct {
	db *gorm.DB
}

// NewUserLevelRepository 创建用户等级仓储实现。
func NewUserLevelRepository(db *gorm.DB) UserLevelRepository {
	return &userLevelRepository{db: db}
}

func (r *userLevelRepository) List(ctx context.Context, query dto.ListQuery) ([]model.UserLevel, int64, error) {
	page, pageSize := normalizePage(query.Page, query.PageSize)
	base := r.db.WithContext(ctx).Model(&model.UserLevel{})
	if status := strings.TrimSpace(query.Status); status != "" {
		base = base.Where("status = ?", status)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("name ILIKE ? OR code ILIKE ? OR description ILIKE ?", like, like, like)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.UserLevel
	if err := base.Order("weight desc, id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *userLevelRepository) FindByID(ctx context.Context, id uint64) (*model.UserLevel, error) {
	var item model.UserLevel
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *userLevelRepository) FindByCode(ctx context.Context, code string) (*model.UserLevel, error) {
	var item model.UserLevel
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *userLevelRepository) Create(ctx context.Context, item *model.UserLevel) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *userLevelRepository) Update(ctx context.Context, item *model.UserLevel) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *userLevelRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.UserLevel{}, id).Error
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
