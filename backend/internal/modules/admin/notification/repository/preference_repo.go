package repository

import (
	"context"

	"gorm.io/gorm"

	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
)

// PreferenceRepository 用户通知偏好仓库。
type PreferenceRepository interface {
	FindByUserAndEvent(ctx context.Context, userID uint64, event string) (*notifymodel.NotificationPreference, error)
	ListByUser(ctx context.Context, userID uint64) ([]notifymodel.NotificationPreference, error)
	Upsert(ctx context.Context, p *notifymodel.NotificationPreference) error
	BatchUpsert(ctx context.Context, items []notifymodel.NotificationPreference) error
}

type preferenceRepository struct {
	db *gorm.DB
}

func NewPreferenceRepository(db *gorm.DB) PreferenceRepository {
	return &preferenceRepository{db: db}
}

func (r *preferenceRepository) FindByUserAndEvent(ctx context.Context, userID uint64, event string) (*notifymodel.NotificationPreference, error) {
	var p notifymodel.NotificationPreference
	if err := r.db.WithContext(ctx).Where("user_id = ? AND event = ?", userID, event).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *preferenceRepository) ListByUser(ctx context.Context, userID uint64) ([]notifymodel.NotificationPreference, error) {
	var items []notifymodel.NotificationPreference
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *preferenceRepository) Upsert(ctx context.Context, p *notifymodel.NotificationPreference) error {
	return r.db.WithContext(ctx).Save(p).Error
}

func (r *preferenceRepository) BatchUpsert(ctx context.Context, items []notifymodel.NotificationPreference) error {
	for i := range items {
		var existing notifymodel.NotificationPreference
		err := r.db.WithContext(ctx).Where("user_id = ? AND event = ?", items[i].UserID, items[i].Event).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			// 使用 Select 强制写入布尔零值，避免 default 覆盖
			if err := r.db.WithContext(ctx).Select("user_id", "event", "inbox_on", "mail_on").Create(&items[i]).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			// 使用 Updates + Select 强制更新布尔字段
			if err := r.db.WithContext(ctx).Model(&existing).
				Select("inbox_on", "mail_on").
				Updates(map[string]any{
					"inbox_on": items[i].InboxOn,
					"mail_on":  items[i].MailOn,
				}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
