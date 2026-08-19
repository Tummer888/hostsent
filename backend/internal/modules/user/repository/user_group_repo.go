package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/user/dto"
	"hostsent/backend/internal/modules/user/model"
)

type UserGroupRepository interface {
	List(ctx context.Context, query dto.UserGroupListQuery) ([]model.UserGroup, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.UserGroup, error)
	Create(ctx context.Context, group *model.UserGroup) error
	Update(ctx context.Context, group *model.UserGroup) error
	Delete(ctx context.Context, id uint64) error
}

type userGroupRepository struct {
	db *gorm.DB
}

func NewUserGroupRepository(db *gorm.DB) UserGroupRepository {
	return &userGroupRepository{db: db}
}

func (r *userGroupRepository) List(ctx context.Context, query dto.UserGroupListQuery) ([]model.UserGroup, int64, error) {
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

	base := r.db.WithContext(ctx).Model(&model.UserGroup{})
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

	var items []model.UserGroup
	if err := base.Order("sort_order asc, id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *userGroupRepository) FindByID(ctx context.Context, id uint64) (*model.UserGroup, error) {
	var group model.UserGroup
	if err := r.db.WithContext(ctx).First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *userGroupRepository) Create(ctx context.Context, group *model.UserGroup) error {
	return r.db.WithContext(ctx).Create(group).Error
}

func (r *userGroupRepository) Update(ctx context.Context, group *model.UserGroup) error {
	return r.db.WithContext(ctx).Save(group).Error
}

func (r *userGroupRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.UserGroup{}, id).Error
}
