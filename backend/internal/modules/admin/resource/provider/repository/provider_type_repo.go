package repository

import (
	"context"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/resource/provider/model"
)

// ProviderTypeRepository 渠道类型注册表（provider_types）数据访问。
type ProviderTypeRepository interface {
	List(ctx context.Context) ([]model.ProviderType, error)
	FindByType(ctx context.Context, providerType string) (*model.ProviderType, error)
	// Upsert 按 type 幂等写入（不存在则插入，存在则更新描述符等展示字段）。
	Upsert(ctx context.Context, item *model.ProviderType) error
}

type providerTypeRepository struct {
	db *gorm.DB
}

// NewProviderTypeRepository 创建渠道类型注册表仓储。
func NewProviderTypeRepository(db *gorm.DB) ProviderTypeRepository {
	return &providerTypeRepository{db: db}
}

func (r *providerTypeRepository) List(ctx context.Context) ([]model.ProviderType, error) {
	var items []model.ProviderType
	if err := r.db.WithContext(ctx).
		Order("sort_order asc, id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *providerTypeRepository) FindByType(ctx context.Context, providerType string) (*model.ProviderType, error) {
	var item model.ProviderType
	if err := r.db.WithContext(ctx).Where("type = ?", providerType).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *providerTypeRepository) Upsert(ctx context.Context, item *model.ProviderType) error {
	return r.db.WithContext(ctx).Where("type = ?", item.Type).
		Assign(map[string]any{
			"name":            item.Name,
			"kind":            item.Kind,
			"descriptor":      item.DescriptorJSON,
			"adapter_version": item.AdapterVersion,
		}).
		FirstOrCreate(item).Error
}
