package repository

import (
	"context"

	"gorm.io/gorm"

	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
)

// ChannelTypeRepository 渠道类型注册表数据访问。
type ChannelTypeRepository interface {
	List(ctx context.Context) ([]notifymodel.NotificationChannelType, error)
	FindByType(ctx context.Context, providerType string) (*notifymodel.NotificationChannelType, error)
	// Upsert 幂等写入（type 唯一），用于启动时把注册表描述符回写落库。
	Upsert(ctx context.Context, t *notifymodel.NotificationChannelType) error
}

type channelTypeRepository struct {
	db *gorm.DB
}

// NewChannelTypeRepository 创建渠道类型仓储。
func NewChannelTypeRepository(db *gorm.DB) ChannelTypeRepository {
	return &channelTypeRepository{db: db}
}

func (r *channelTypeRepository) List(ctx context.Context) ([]notifymodel.NotificationChannelType, error) {
	var items []notifymodel.NotificationChannelType
	if err := r.db.WithContext(ctx).Order("sort_order asc, id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *channelTypeRepository) FindByType(ctx context.Context, providerType string) (*notifymodel.NotificationChannelType, error) {
	var t notifymodel.NotificationChannelType
	if err := r.db.WithContext(ctx).Where("type = ?", providerType).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *channelTypeRepository) Upsert(ctx context.Context, t *notifymodel.NotificationChannelType) error {
	var existing notifymodel.NotificationChannelType
	err := r.db.WithContext(ctx).Where("type = ?", t.Type).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.WithContext(ctx).Create(t).Error
	}
	if err != nil {
		return err
	}
	// 只回写注册表负责的字段，保留运营在后台改过的名称/图标/文档。
	updates := map[string]any{
		"category":        t.Category,
		"mode":            t.Mode,
		"descriptor":      t.DescriptorJSON,
		"adapter_version": t.AdapterVersion,
	}
	if t.Name != "" && existing.Name == "" {
		updates["name"] = t.Name
	}
	return r.db.WithContext(ctx).Model(&notifymodel.NotificationChannelType{}).
		Where("id = ?", existing.ID).Updates(updates).Error
}
