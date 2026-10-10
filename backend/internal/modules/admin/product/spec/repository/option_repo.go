package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hostsent/backend/internal/modules/admin/product/spec/dto"
	"hostsent/backend/internal/modules/admin/product/spec/model"
)

// SpecOptionRepository 平台配置项目录与取值库的数据访问。
type SpecOptionRepository interface {
	// ListSpecs 查某平台的配置项目录（含运营自定义项），按分组 + 排序。
	ListSpecs(ctx context.Context, providerType string, includeHidden bool) ([]model.ProviderOptionSpec, error)
	// FindSpec 按 (平台, 参数名) 取单条；不存在返回 (nil,nil)。
	FindSpec(ctx context.Context, providerType, optionKey string) (*model.ProviderOptionSpec, error)
	// FindSpecByID 按主键取配置项；不存在返回 (nil,nil)。
	FindSpecByID(ctx context.Context, id uint64) (*model.ProviderOptionSpec, error)
	// CreateSpec 新增配置项。
	CreateSpec(ctx context.Context, item *model.ProviderOptionSpec) error
	// UpdateSpec 更新配置项。
	UpdateSpec(ctx context.Context, item *model.ProviderOptionSpec) error
	// DeleteSpec 删除配置项（连同其取值）。
	DeleteSpec(ctx context.Context, providerType, optionKey string) error
	// UpsertSpecIfAbsent 幂等导入：已存在则跳过（不覆盖运营改过的行），返回是否新增。
	UpsertSpecIfAbsent(ctx context.Context, item *model.ProviderOptionSpec) (bool, error)

	// ListValues 查取值库（可按配置项过滤）。
	ListValues(ctx context.Context, q dto.OptionValueQuery) ([]model.PlatformOptionValue, error)
	// ListValuesByKeys 批量取若干配置项的取值（返回 option_key → values）。
	ListValuesByKeys(ctx context.Context, providerType string, optionKeys []string, includeOffline bool) (map[string][]model.PlatformOptionValue, error)
	// FindValue 按主键取一条取值。
	FindValue(ctx context.Context, id uint64) (*model.PlatformOptionValue, error)
	// UpsertValue 幂等写入取值（唯一键 provider_type+option_key+value+parent_value）。
	UpsertValue(ctx context.Context, item *model.PlatformOptionValue) error
	// OfflineValues 停用某配置项下的取值（可选只停 platform 来源的），返回影响行数。
	OfflineValues(ctx context.Context, providerType, optionKey string, onlyOrigin string) (int64, error)
	// DeleteValue 按主键删除取值。
	DeleteValue(ctx context.Context, id uint64) error
}

type specOptionRepository struct {
	db *gorm.DB
}

// NewSpecOptionRepository 创建平台配置项目录仓储。
func NewSpecOptionRepository(db *gorm.DB) SpecOptionRepository {
	return &specOptionRepository{db: db}
}

func (r *specOptionRepository) ListSpecs(ctx context.Context, providerType string, includeHidden bool) ([]model.ProviderOptionSpec, error) {
	var items []model.ProviderOptionSpec
	base := r.db.WithContext(ctx).Where("provider_type = ?", providerType)
	if !includeHidden {
		base = base.Where("hidden = ?", false)
	}
	if err := base.Order("group_name asc, sort_order asc, id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *specOptionRepository) FindSpec(ctx context.Context, providerType, optionKey string) (*model.ProviderOptionSpec, error) {
	var item model.ProviderOptionSpec
	err := r.db.WithContext(ctx).
		Where("provider_type = ? AND option_key = ?", providerType, optionKey).
		First(&item).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *specOptionRepository) FindSpecByID(ctx context.Context, id uint64) (*model.ProviderOptionSpec, error) {
	var item model.ProviderOptionSpec
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *specOptionRepository) CreateSpec(ctx context.Context, item *model.ProviderOptionSpec) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *specOptionRepository) UpdateSpec(ctx context.Context, item *model.ProviderOptionSpec) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *specOptionRepository) DeleteSpec(ctx context.Context, providerType, optionKey string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("provider_type = ? AND option_key = ?", providerType, optionKey).
			Delete(&model.PlatformOptionValue{}).Error; err != nil {
			return err
		}
		return tx.Where("provider_type = ? AND option_key = ?", providerType, optionKey).
			Delete(&model.ProviderOptionSpec{}).Error
	})
}

// UpsertSpecIfAbsent 幂等导入：靠唯一键判断，已存在直接跳过。
// 用 DO NOTHING 而不是 DO UPDATE——适配器声明是"最低保障"，运营在后台改过的
// 标签/默认值/必选标记不能被下次同步覆盖回原样。
func (r *specOptionRepository) UpsertSpecIfAbsent(ctx context.Context, item *model.ProviderOptionSpec) (bool, error) {
	res := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(item)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *specOptionRepository) ListValues(ctx context.Context, q dto.OptionValueQuery) ([]model.PlatformOptionValue, error) {
	var items []model.PlatformOptionValue
	base := r.db.WithContext(ctx).Where("provider_type = ?", strings.TrimSpace(q.ProviderType))
	if key := strings.TrimSpace(q.OptionKey); key != "" {
		base = base.Where("option_key = ?", key)
	}
	if !q.IncludeOffline {
		base = base.Where("status = ?", model.OptionValueActive)
	}
	if err := base.Order("option_key asc, sort_order asc, id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *specOptionRepository) ListValuesByKeys(ctx context.Context, providerType string, optionKeys []string, includeOffline bool) (map[string][]model.PlatformOptionValue, error) {
	out := map[string][]model.PlatformOptionValue{}
	if len(optionKeys) == 0 {
		return out, nil
	}
	var items []model.PlatformOptionValue
	base := r.db.WithContext(ctx).
		Where("provider_type = ?", providerType).
		Where("option_key IN ?", optionKeys)
	if !includeOffline {
		base = base.Where("status = ?", model.OptionValueActive)
	}
	if err := base.Order("option_key asc, sort_order asc, id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	for _, it := range items {
		out[it.OptionKey] = append(out[it.OptionKey], it)
	}
	return out, nil
}

func (r *specOptionRepository) FindValue(ctx context.Context, id uint64) (*model.PlatformOptionValue, error) {
	var item model.PlatformOptionValue
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

// UpsertValue 幂等写入：命中唯一键时更新标签/分组/状态（运营重复导入不该报错）。
func (r *specOptionRepository) UpsertValue(ctx context.Context, item *model.PlatformOptionValue) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "provider_type"}, {Name: "option_key"},
			{Name: "value"}, {Name: "parent_value"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"label", "group_label", "status", "origin", "sort_order"}),
	}).Create(item).Error
}

func (r *specOptionRepository) OfflineValues(ctx context.Context, providerType, optionKey string, onlyOrigin string) (int64, error) {
	base := r.db.WithContext(ctx).Model(&model.PlatformOptionValue{}).
		Where("provider_type = ? AND option_key = ?", providerType, optionKey)
	if onlyOrigin != "" {
		base = base.Where("origin = ?", onlyOrigin)
	}
	res := base.Update("status", model.OptionValueOffline)
	return res.RowsAffected, res.Error
}

func (r *specOptionRepository) DeleteValue(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.PlatformOptionValue{}, id).Error
}
