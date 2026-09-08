// Package service 提供系统配置模块的增删改查业务。
package service

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/system/dto"
	"hostsent/backend/internal/modules/admin/system/model"
	"hostsent/backend/internal/modules/admin/system/repository"
)

// ErrConfigKeyDuplicate 配置键重复错误（由 Handler 映射为 400 业务错误码）。
var ErrConfigKeyDuplicate = errors.New("配置键已存在")

// ConfigService 定义系统配置增删改查所需的业务能力。
type ConfigService interface {
	// List 分页查询配置列表，group 精确过滤、keyword 模糊匹配配置键/描述。
	List(ctx context.Context, group, keyword string, page, pageSize int) ([]dto.ConfigInfo, int64, error)
	// GetByKey 按配置键查询配置项信息。
	GetByKey(ctx context.Context, key string) (*dto.ConfigInfo, error)
	// Create 创建配置项，配置键已存在时返回 ErrConfigKeyDuplicate。
	Create(ctx context.Context, req dto.ConfigCreateRequest) (*dto.ConfigInfo, error)
	// Update 更新指定配置项（配置键不可修改）。
	Update(ctx context.Context, id uint64, req dto.ConfigUpdateRequest) (*dto.ConfigInfo, error)
	// Delete 删除指定配置项。
	Delete(ctx context.Context, id uint64) error
	// ListByGroup 按分组查询全部配置项（按 sort_order 排序）。
	ListByGroup(ctx context.Context, group string) ([]dto.ConfigInfo, error)
	// BatchUpsert 按分组批量保存配置项（按 config_key 幂等 upsert），返回该分组最新配置。
	BatchUpsert(ctx context.Context, req dto.ConfigBatchUpsertRequest) ([]dto.ConfigInfo, error)
}

type configService struct {
	repo repository.ConfigRepository
}

// NewConfigService 创建系统配置服务实现。
func NewConfigService(repo repository.ConfigRepository) ConfigService {
	return &configService{repo: repo}
}

func (s *configService) List(ctx context.Context, group, keyword string, page, pageSize int) ([]dto.ConfigInfo, int64, error) {
	// 分页参数归一化：页码最小 1，每页条数默认 10、上限 100
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	configs, total, err := s.repo.List(ctx, group, keyword, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, err
	}

	items := make([]dto.ConfigInfo, 0, len(configs))
	for _, config := range configs {
		items = append(items, toConfigInfo(config))
	}
	return items, total, nil
}

func (s *configService) GetByKey(ctx context.Context, key string) (*dto.ConfigInfo, error) {
	config, err := s.repo.FindByKey(ctx, key)
	if err != nil {
		return nil, err
	}
	info := toConfigInfo(*config)
	return &info, nil
}

func (s *configService) Create(ctx context.Context, req dto.ConfigCreateRequest) (*dto.ConfigInfo, error) {
	// 配置键唯一性校验
	if _, err := s.repo.FindByKey(ctx, req.ConfigKey); err == nil {
		return nil, ErrConfigKeyDuplicate
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	config := &model.SystemConfig{
		ConfigKey:   req.ConfigKey,
		ConfigValue: req.ConfigValue,
		ValueType:   defaultValueType(req.ValueType),
		Group:       req.Group,
		Description: req.Description,
		SortOrder:   req.SortOrder,
		Status:      defaultStatus(req.Status),
	}
	if err := s.repo.Create(ctx, config); err != nil {
		return nil, err
	}
	info := toConfigInfo(*config)
	return &info, nil
}

func (s *configService) Update(ctx context.Context, id uint64, req dto.ConfigUpdateRequest) (*dto.ConfigInfo, error) {
	config, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	config.ConfigValue = req.ConfigValue
	config.ValueType = defaultValueType(req.ValueType)
	config.Group = req.Group
	config.Description = req.Description
	config.SortOrder = req.SortOrder
	config.Status = defaultStatus(req.Status)
	if err := s.repo.Update(ctx, config); err != nil {
		return nil, err
	}
	info := toConfigInfo(*config)
	return &info, nil
}

func (s *configService) Delete(ctx context.Context, id uint64) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// ListByGroup 按分组查询全部配置项。
func (s *configService) ListByGroup(ctx context.Context, group string) ([]dto.ConfigInfo, error) {
	configs, err := s.repo.ListByGroup(ctx, group)
	if err != nil {
		return nil, err
	}
	items := make([]dto.ConfigInfo, 0, len(configs))
	for _, config := range configs {
		items = append(items, toConfigInfo(config))
	}
	return items, nil
}

// BatchUpsert 按分组批量保存配置项：分组以请求体 config_group 为准，逐项按 config_key 幂等 upsert。
func (s *configService) BatchUpsert(ctx context.Context, req dto.ConfigBatchUpsertRequest) ([]dto.ConfigInfo, error) {
	configs := make([]*model.SystemConfig, 0, len(req.Items))
	for _, item := range req.Items {
		configs = append(configs, &model.SystemConfig{
			ConfigKey:   item.ConfigKey,
			ConfigValue: item.ConfigValue,
			ValueType:   defaultValueType(item.ValueType),
			Group:       req.Group,
			Description: item.Description,
			SortOrder:   item.SortOrder,
			Status:      defaultStatus(item.Status),
		})
	}
	if err := s.repo.BatchUpsert(ctx, configs); err != nil {
		return nil, err
	}
	// 返回该分组保存后的最新配置列表
	return s.ListByGroup(ctx, req.Group)
}

// toConfigInfo 将模型实体映射为响应 DTO。
func toConfigInfo(config model.SystemConfig) dto.ConfigInfo {
	return dto.ConfigInfo{
		ID:          config.ID,
		ConfigKey:   config.ConfigKey,
		ConfigValue: config.ConfigValue,
		ValueType:   config.ValueType,
		Group:       config.Group,
		Description: config.Description,
		SortOrder:   config.SortOrder,
		Status:      config.Status,
		CreatedAt:   config.CreatedAt,
		UpdatedAt:   config.UpdatedAt,
	}
}

// defaultValueType 值类型缺省为 string。
func defaultValueType(t string) string {
	if t == "" {
		return model.ValueTypeString
	}
	return t
}

// defaultStatus 状态缺省为 active。
func defaultStatus(status string) string {
	if status == "" {
		return model.StatusActive
	}
	return status
}
