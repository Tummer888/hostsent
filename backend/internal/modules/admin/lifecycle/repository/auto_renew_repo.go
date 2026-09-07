package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	lifecyclemodel "hostsent/backend/internal/modules/admin/lifecycle/model"
)

// AutoRenewRepository 实例自动续费开关仓库。
type AutoRenewRepository interface {
	FindByInstance(ctx context.Context, instanceID uint64) (*lifecyclemodel.InstanceAutoRenewal, error)
	Upsert(ctx context.Context, record *lifecyclemodel.InstanceAutoRenewal) error
	ListEnabledByUser(ctx context.Context, userID uint64) ([]lifecyclemodel.InstanceAutoRenewal, error)
	ListEnabled(ctx context.Context) ([]lifecyclemodel.InstanceAutoRenewal, error)
	DeleteByInstance(ctx context.Context, instanceID uint64) error
}

type autoRenewRepository struct {
	db *gorm.DB
}

// NewAutoRenewRepository 创建自动续费开关仓库。
func NewAutoRenewRepository(db *gorm.DB) AutoRenewRepository {
	return &autoRenewRepository{db: db}
}

func (r *autoRenewRepository) FindByInstance(ctx context.Context, instanceID uint64) (*lifecyclemodel.InstanceAutoRenewal, error) {
	var record lifecyclemodel.InstanceAutoRenewal
	if err := r.db.WithContext(ctx).Where("instance_id = ?", instanceID).First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *autoRenewRepository) Upsert(ctx context.Context, record *lifecyclemodel.InstanceAutoRenewal) error {
	return r.db.WithContext(ctx).
		Where("instance_id = ?", record.InstanceID).
		Assign(map[string]interface{}{
			"user_id":      record.UserID,
			"enabled":      record.Enabled,
			"period_count": record.PeriodCount,
		}).
		FirstOrCreate(record).Error
}

func (r *autoRenewRepository) ListEnabledByUser(ctx context.Context, userID uint64) ([]lifecyclemodel.InstanceAutoRenewal, error) {
	var items []lifecyclemodel.InstanceAutoRenewal
	err := r.db.WithContext(ctx).Where("user_id = ? AND enabled = ?", userID, true).Find(&items).Error
	return items, err
}

func (r *autoRenewRepository) ListEnabled(ctx context.Context) ([]lifecyclemodel.InstanceAutoRenewal, error) {
	var items []lifecyclemodel.InstanceAutoRenewal
	err := r.db.WithContext(ctx).Where("enabled = ?", true).Find(&items).Error
	return items, err
}

func (r *autoRenewRepository) DeleteByInstance(ctx context.Context, instanceID uint64) error {
	return r.db.WithContext(ctx).Where("instance_id = ?", instanceID).Delete(&lifecyclemodel.InstanceAutoRenewal{}).Error
}

var _ = errors.Is // 保留 errors 引用（未来扩展）
