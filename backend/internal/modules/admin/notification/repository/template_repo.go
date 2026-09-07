package repository

import (
	"context"

	"gorm.io/gorm"

	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
)

// TemplateRepository 通知模板仓库。
type TemplateRepository interface {
	FindByEvent(ctx context.Context, event string) (*notifymodel.NotificationTemplate, error)
	List(ctx context.Context) ([]notifymodel.NotificationTemplate, error)
	Update(ctx context.Context, t *notifymodel.NotificationTemplate) error
	ListEvents(ctx context.Context) ([]string, error)
}

type templateRepository struct {
	db *gorm.DB
}

func NewTemplateRepository(db *gorm.DB) TemplateRepository {
	return &templateRepository{db: db}
}

func (r *templateRepository) FindByEvent(ctx context.Context, event string) (*notifymodel.NotificationTemplate, error) {
	var t notifymodel.NotificationTemplate
	if err := r.db.WithContext(ctx).Where("event = ?", event).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *templateRepository) List(ctx context.Context) ([]notifymodel.NotificationTemplate, error) {
	var items []notifymodel.NotificationTemplate
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *templateRepository) Update(ctx context.Context, t *notifymodel.NotificationTemplate) error {
	return r.db.WithContext(ctx).Save(t).Error
}

func (r *templateRepository) ListEvents(ctx context.Context) ([]string, error) {
	var events []string
	if err := r.db.WithContext(ctx).Model(&notifymodel.NotificationTemplate{}).
		Where("status = ?", notifymodel.TemplateStatusActive).
		Pluck("event", &events).Error; err != nil {
		return nil, err
	}
	return events, nil
}
