package repository

import (
	"context"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/payment/dto"
	"hostsent/backend/internal/modules/admin/payment/model"
)

// CallbackRepository 回调日志数据访问。
type CallbackRepository interface {
	Create(ctx context.Context, l *model.PaymentCallbackLog) error
	List(ctx context.Context, q dto.CallbackListQuery) ([]model.PaymentCallbackLog, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.PaymentCallbackLog, error)
	// ExistsNotify 判断同渠道同 notify_id 是否已处理（幂等）。
	ExistsNotify(ctx context.Context, channelID uint64, notifyID string) (bool, error)
}

type callbackRepository struct {
	db *gorm.DB
}

// NewCallbackRepository 创建回调日志仓储。
func NewCallbackRepository(db *gorm.DB) CallbackRepository {
	return &callbackRepository{db: db}
}

func (r *callbackRepository) Create(ctx context.Context, l *model.PaymentCallbackLog) error {
	return r.db.WithContext(ctx).Create(l).Error
}

func (r *callbackRepository) FindByID(ctx context.Context, id uint64) (*model.PaymentCallbackLog, error) {
	var l model.PaymentCallbackLog
	if err := r.db.WithContext(ctx).First(&l, id).Error; err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *callbackRepository) ExistsNotify(ctx context.Context, channelID uint64, notifyID string) (bool, error) {
	if notifyID == "" {
		return false, nil
	}
	var count int64
	err := r.db.WithContext(ctx).Model(&model.PaymentCallbackLog{}).
		Where("channel_id = ? AND notify_id = ? AND handle_status IN ?", channelID, notifyID,
			[]string{"ok", "duplicate"}).
		Count(&count).Error
	return count > 0, err
}

func (r *callbackRepository) List(ctx context.Context, q dto.CallbackListQuery) ([]model.PaymentCallbackLog, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.PaymentCallbackLog{})
	if q.ChannelCode != "" {
		base = base.Where("channel_code = ?", q.ChannelCode)
	}
	if q.PaymentNo != "" {
		base = base.Where("payment_no = ?", q.PaymentNo)
	}
	if q.VerifyOK != nil {
		base = base.Where("verify_ok = ?", *q.VerifyOK)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.PaymentCallbackLog
	err := base.Order("id desc").
		Offset((normalizePage(q.Page) - 1) * normalizePageSize(q.PageSize)).
		Limit(normalizePageSize(q.PageSize)).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
