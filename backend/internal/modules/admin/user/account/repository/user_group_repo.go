package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/account/dto"
	"hostsent/backend/internal/modules/admin/user/account/model"
)

type UserGroupRepository interface {
	List(ctx context.Context, query dto.UserGroupListQuery) ([]model.UserGroup, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.UserGroup, error)
	Create(ctx context.Context, group *model.UserGroup) error
	Update(ctx context.Context, group *model.UserGroup) error
	Delete(ctx context.Context, id uint64) error
	// DefaultGroupID 返回唯一的默认用户组 ID（仅 active）；未配置时返回 0，不报错。
	DefaultGroupID(ctx context.Context) (uint64, error)
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
	switch strings.TrimSpace(query.IsAgentGroup) {
	case "true":
		base = base.Where("is_agent_group = ?", true)
	case "false":
		base = base.Where("is_agent_group = ?", false)
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

// Create 写入用户组，并在同一事务内维护「有且仅有一个默认组」的不变量。
//
// 必须「先清后写」：先插入 is_default=true 再清理其他组会撞上部分唯一索引
// uk_user_groups_single_default（23505），导致切换默认组直接失败。
func (r *userGroupRepository) Create(ctx context.Context, group *model.UserGroup) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if group.IsDefault {
			if err := clearDefaultExcept(tx, 0); err != nil {
				return err
			}
		}
		return tx.Create(group).Error
	})
}

// Update 全量保存用户组，默认组唯一性的维护方式同 Create（先清后写、同事务）。
func (r *userGroupRepository) Update(ctx context.Context, group *model.UserGroup) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if group.IsDefault {
			if err := clearDefaultExcept(tx, group.ID); err != nil {
				return err
			}
		}
		return tx.Save(group).Error
	})
}

func (r *userGroupRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.UserGroup{}, id).Error
}

// DefaultGroupID 取被显式标记为默认的 active 用户组（is_default 全库唯一，由 create/update 维护）。
// 没有组被标记为默认时返回 0 —— 不做「取第一个组」的隐式兜底，避免把新用户静默塞进任意分组。
func (r *userGroupRepository) DefaultGroupID(ctx context.Context) (uint64, error) {
	var group model.UserGroup
	err := r.db.WithContext(ctx).
		Where("status = ? AND is_default = ?", "active", true).
		Order("id asc").
		First(&group).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return group.ID, nil
}

// clearDefaultExcept 把除 exceptID 之外所有组的 is_default 置 false（单条 UPDATE，天然幂等）。
// exceptID 传 0 表示清空全部默认标记（创建默认组前调用）。
func clearDefaultExcept(tx *gorm.DB, exceptID uint64) error {
	return tx.Model(&model.UserGroup{}).
		Where("is_default = ? AND id <> ?", true, exceptID).
		Update("is_default", false).Error
}
