package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	openmodel "hostsent/backend/internal/modules/open/model"
)

// NotifyRepository 事件回调投递仓储（T6.5）。
type NotifyRepository interface {
	// Create 落一条待投递事件。
	Create(ctx context.Context, delivery *openmodel.OpenNotifyDelivery) error
	// ListDue 领取到期待投递记录（pending/failed 且 next_retry_at 到期）。
	ListDue(ctx context.Context, limit int) ([]openmodel.OpenNotifyDelivery, error)
	// MarkSuccess 投递成功。
	MarkSuccess(ctx context.Context, id uint64) error
	// MarkFailed 投递失败：累加 attempts 并按退避排定下次重试。
	MarkFailed(ctx context.Context, id uint64, attempts int, nextRetryAt time.Time, lastError string) error
	// MarkDead 次数耗尽转死信。
	MarkDead(ctx context.Context, id uint64, attempts int, lastError string) error
	// GetByID 读取单条。
	GetByID(ctx context.Context, id uint64) (*openmodel.OpenNotifyDelivery, error)
	// Redeliver 人工重投死信：状态复位 pending、次数清零。
	Redeliver(ctx context.Context, id uint64) error
	// ListDeliveries 投递记录列表（appID=0 时返回全部，运维排查用）。
	ListDeliveries(ctx context.Context, appID uint64, limit int) ([]openmodel.OpenNotifyDelivery, error)
	// AppIDLookup 按字符串 app_id 解析数字主键（CLI 用）。
	AppIDLookup(ctx context.Context, appID string) (uint64, error)
}

type notifyRepository struct {
	db *gorm.DB
}

// NewNotifyRepository 构造回调投递仓储。
func NewNotifyRepository(db *gorm.DB) NotifyRepository {
	return &notifyRepository{db: db}
}

func (r *notifyRepository) Create(ctx context.Context, delivery *openmodel.OpenNotifyDelivery) error {
	return r.db.WithContext(ctx).Create(delivery).Error
}

func (r *notifyRepository) ListDue(ctx context.Context, limit int) ([]openmodel.OpenNotifyDelivery, error) {
	var rows []openmodel.OpenNotifyDelivery
	err := r.db.WithContext(ctx).
		Where("status IN ? AND (next_retry_at IS NULL OR next_retry_at <= ?)",
			[]string{openmodel.OpenNotifyPending, openmodel.OpenNotifyFailed}, time.Now()).
		Order("id").Limit(limit).Find(&rows).Error
	return rows, err
}

func (r *notifyRepository) MarkSuccess(ctx context.Context, id uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&openmodel.OpenNotifyDelivery{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":       openmodel.OpenNotifySuccess,
			"delivered_at": now,
			"last_error":   "",
			"updated_at":   now,
		}).Error
}

func (r *notifyRepository) MarkFailed(ctx context.Context, id uint64, attempts int, nextRetryAt time.Time, lastError string) error {
	return r.db.WithContext(ctx).Model(&openmodel.OpenNotifyDelivery{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":        openmodel.OpenNotifyFailed,
			"attempts":      attempts,
			"next_retry_at": nextRetryAt,
			"last_error":    truncate(lastError, 500),
			"updated_at":    time.Now(),
		}).Error
}

func (r *notifyRepository) MarkDead(ctx context.Context, id uint64, attempts int, lastError string) error {
	return r.db.WithContext(ctx).Model(&openmodel.OpenNotifyDelivery{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":     openmodel.OpenNotifyDead,
			"attempts":   attempts,
			"last_error": truncate(lastError, 500),
			"updated_at": time.Now(),
		}).Error
}

func (r *notifyRepository) GetByID(ctx context.Context, id uint64) (*openmodel.OpenNotifyDelivery, error) {
	var row openmodel.OpenNotifyDelivery
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAppNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (r *notifyRepository) Redeliver(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&openmodel.OpenNotifyDelivery{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":        openmodel.OpenNotifyPending,
			"attempts":      0,
			"next_retry_at": time.Now(),
			"updated_at":    time.Now(),
		}).Error
}

func (r *notifyRepository) ListDeliveries(ctx context.Context, appID uint64, limit int) ([]openmodel.OpenNotifyDelivery, error) {
	if limit <= 0 {
		limit = 100
	}
	q := r.db.WithContext(ctx).Model(&openmodel.OpenNotifyDelivery{})
	if appID != 0 {
		q = q.Where("app_id = ?", appID)
	}
	var rows []openmodel.OpenNotifyDelivery
	err := q.Order("id DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

func (r *notifyRepository) AppIDLookup(ctx context.Context, appID string) (uint64, error) {
	var id uint64
	err := r.db.WithContext(ctx).Model(&openmodel.OpenApp{}).
		Select("id").Where("app_id = ?", appID).Scan(&id).Error
	if err != nil {
		return 0, err
	}
	if id == 0 {
		return 0, ErrAppNotFound
	}
	return id, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
