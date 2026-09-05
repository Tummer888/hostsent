// Package repository 提供系统配置模块的数据访问实现。
package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/system/model"
)

// ConfigRepository 定义系统配置数据访问所需的仓储能力。
type ConfigRepository interface {
	// List 分页查询配置列表：group 精确过滤，keyword 对配置键/描述模糊匹配。
	List(ctx context.Context, group, keyword string, offset, limit int) ([]model.SystemConfig, int64, error)
	// FindByKey 按配置键查询配置项，未找到时返回 gorm.ErrRecordNotFound。
	FindByKey(ctx context.Context, key string) (*model.SystemConfig, error)
	// FindByID 按主键查询配置项，未找到时返回 gorm.ErrRecordNotFound。
	FindByID(ctx context.Context, id uint64) (*model.SystemConfig, error)
	// Create 新增配置项。
	Create(ctx context.Context, config *model.SystemConfig) error
	// Update 保存配置项的全部字段。
	Update(ctx context.Context, config *model.SystemConfig) error
	// Delete 按主键删除配置项。
	Delete(ctx context.Context, id uint64) error
}

type configRepository struct {
	db *gorm.DB
}

// NewConfigRepository 创建系统配置仓储实现。
func NewConfigRepository(db *gorm.DB) ConfigRepository {
	return &configRepository{db: db}
}

func (r *configRepository) List(ctx context.Context, group, keyword string, offset, limit int) ([]model.SystemConfig, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.SystemConfig{})
	if group != "" {
		query = query.Where("config_group = ?", group)
	}
	if keyword != "" {
		// 模糊匹配配置键或描述
		like := "%" + keyword + "%"
		query = query.Where("config_key ILIKE ? OR description ILIKE ?", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var configs []model.SystemConfig
	if err := query.Order("config_group asc, sort_order asc, id asc").
		Offset(offset).Limit(limit).
		Find(&configs).Error; err != nil {
		return nil, 0, err
	}
	return configs, total, nil
}

func (r *configRepository) FindByKey(ctx context.Context, key string) (*model.SystemConfig, error) {
	var config model.SystemConfig
	if err := r.db.WithContext(ctx).Where("config_key = ?", key).First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &config, nil
}

func (r *configRepository) FindByID(ctx context.Context, id uint64) (*model.SystemConfig, error) {
	var config model.SystemConfig
	if err := r.db.WithContext(ctx).First(&config, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &config, nil
}

func (r *configRepository) Create(ctx context.Context, config *model.SystemConfig) error {
	return r.db.WithContext(ctx).Create(config).Error
}

func (r *configRepository) Update(ctx context.Context, config *model.SystemConfig) error {
	return r.db.WithContext(ctx).Save(config).Error
}

func (r *configRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.SystemConfig{}, id).Error
}
