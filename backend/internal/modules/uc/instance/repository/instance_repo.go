// Package repository 提供用户中心主机模块的数据访问实现。
package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/uc/instance/model"
)

// ErrNotFound 主机不存在或不属于当前用户。
var ErrNotFound = errors.New("主机不存在或无权访问")

// InstanceRepository 用户中心主机仓储接口。
type InstanceRepository interface {
	ListByUser(ctx context.Context, userID uint64) ([]model.Instance, error)
	FindByUser(ctx context.Context, userID, id uint64) (*model.Instance, error)
}

type instanceRepository struct {
	db *gorm.DB
}

// NewInstanceRepository 创建用户中心主机仓储。
func NewInstanceRepository(db *gorm.DB) InstanceRepository {
	return &instanceRepository{db: db}
}

// ListByUser 查询某用户的全部主机（附带操作人用户名，用于「操作人」列 P4-09）。
func (r *instanceRepository) ListByUser(ctx context.Context, userID uint64) ([]model.Instance, error) {
	var items []model.Instance
	if err := r.db.WithContext(ctx).
		Select("instances.*, actor.username AS actor_name").
		Joins("LEFT JOIN users AS actor ON actor.id = instances.actor_user_id").
		Where("instances.user_id = ?", userID).
		Order("instances.id desc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// FindByUser 按 id + userID 查询主机（校验归属）。
func (r *instanceRepository) FindByUser(ctx context.Context, userID, id uint64) (*model.Instance, error) {
	var item model.Instance
	err := r.db.WithContext(ctx).
		Select("instances.*, actor.username AS actor_name").
		Joins("LEFT JOIN users AS actor ON actor.id = instances.actor_user_id").
		Where("instances.user_id = ? AND instances.id = ?", userID, id).
		First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}
