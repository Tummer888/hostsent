package server

// 财务模块装配（本轮整理优化）：
//
//   - 财务统计：/api/v1/admin/finance/stats 全量聚合，供财务总览与财务报表共用；
//   - 财务参数：/api/v1/admin/finance/settings 白名单读写 system_configs 的 finance 分组；
//   - 配置读取接线：调账开关 / 对账容差 / 余额预警阈值分别注入消费方（账务核心、对账、统计）。
//
// 依赖方向：finance 子域不 import 系统配置模块，读写 system_configs 统一经装配层实现；
// 白名单在 finance/settings/service.specs 中声明，本文件只提供存取原语。

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	settingsservice "hostsent/backend/internal/modules/admin/finance/settings/service"
	systemmodel "hostsent/backend/internal/modules/admin/system/config/model"
)

// financeConfigStore 财务参数存取：只读写 system_configs，不校验键（白名单在 service 层）。
type financeConfigStore struct {
	db *gorm.DB
}

// newFinanceConfigStore 创建财务参数存取实现。
func newFinanceConfigStore(db *gorm.DB) settingsservice.ConfigStore {
	return &financeConfigStore{db: db}
}

// Get 按键读取原文；键不存在返回 found=false 而非错误。
func (s *financeConfigStore) Get(ctx context.Context, key string) (string, bool, error) {
	var item systemmodel.SystemConfig
	if err := s.db.WithContext(ctx).Where("config_key = ?", key).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return item.ConfigValue, true, nil
}

// Save 按键 upsert：存在则更新取值并保持分组/排序，不存在则按财务分组创建。
func (s *financeConfigStore) Save(ctx context.Context, key, value, valueType, description, group string) error {
	var item systemmodel.SystemConfig
	err := s.db.WithContext(ctx).Where("config_key = ?", key).First(&item).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		create := systemmodel.SystemConfig{
			ConfigKey:   key,
			ConfigValue: value,
			ValueType:   valueType,
			Group:       group,
			Description: description,
			Status:      systemmodel.StatusActive,
		}
		return s.db.WithContext(ctx).Create(&create).Error
	}
	item.ConfigValue = value
	item.ValueType = valueType
	item.Group = group
	if strings.TrimSpace(item.Description) == "" {
		item.Description = description
	}
	return s.db.WithContext(ctx).Save(&item).Error
}
