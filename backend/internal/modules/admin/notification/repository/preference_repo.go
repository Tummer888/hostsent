package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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
	// 不能走 Save：inbox_on 带 `default:true` 标签，GORM 对「零值 + 有 default」
	// 的字段在 INSERT 时不会写入该列，DB 默认值 true 会把用户显式关掉的 false 覆盖掉。
	return r.upsert(ctx, []notifymodel.NotificationPreference{*p})
}

func (r *preferenceRepository) BatchUpsert(ctx context.Context, items []notifymodel.NotificationPreference) error {
	if len(items) == 0 {
		return nil
	}
	return r.upsert(ctx, items)
}

// upsert 用 ON CONFLICT DO UPDATE 一次写全三列。
//
// 不用 Save/Create 的原因：inbox_on 带 `default:true`，GORM 对「零值 + 有 default」
// 的字段会从 INSERT 列里剔除，DB 默认值把用户显式关掉的 false 覆盖成 true ——
// 于是「关掉某事件站内信」永远存不下去（doc90 §11 场景 16 直击此点）。
// 显式列出三列并由 DB 做冲突更新，布尔零值才能真正落库。
func (r *preferenceRepository) upsert(ctx context.Context, items []notifymodel.NotificationPreference) error {
	rows := make([]map[string]any, 0, len(items))
	for i := range items {
		rows = append(rows, map[string]any{
			"user_id":    items[i].UserID,
			"event":      items[i].Event,
			"inbox_on":   items[i].InboxOn,
			"mail_on":    items[i].MailOn,
			"sms_on":     items[i].SmsOn,
			"updated_at": time.Now(),
		})
	}
	return r.db.WithContext(ctx).Table("notification_preferences").
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "event"}},
			DoUpdates: clause.AssignmentColumns([]string{"inbox_on", "mail_on", "sms_on", "updated_at"}),
		}).
		Create(rows).Error
}
