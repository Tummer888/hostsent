package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
)

// NotificationRepository 通知记录仓库。
type NotificationRepository interface {
	CreateInbox(ctx context.Context, n *notifymodel.Notification) error
	FindByID(ctx context.Context, id uint64) (*notifymodel.Notification, error)
	List(ctx context.Context, q NotificationListParams) ([]notifymodel.Notification, int64, error)
	ListByUser(ctx context.Context, userID uint64, page, pageSize int) ([]notifymodel.Notification, int64, error)
	ListBroadcast(ctx context.Context, targetType string, page, pageSize int) ([]notifymodel.Notification, int64, error)
	CountUnread(ctx context.Context, userID uint64, targetType string) (int64, error)
	MarkRead(ctx context.Context, id, userID uint64) error
	MarkAllRead(ctx context.Context, userID uint64) error
	UpdateSendStatus(ctx context.Context, id uint64, status, failReason string) error
	CreateReadRecord(ctx context.Context, notificationID, userID uint64) error
	HasReadRecord(ctx context.Context, notificationID, userID uint64) (bool, error)
}

// NotificationListParams 通知列表查询参数。
type NotificationListParams struct {
	Event      string
	Channel    string
	SendStatus string
	TargetType string
	Keyword    string
	Offset     int
	Limit      int
}

type notificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) NotificationRepository {
	return &notificationRepository{db: db}
}

func (r *notificationRepository) CreateInbox(ctx context.Context, n *notifymodel.Notification) error {
	return r.db.WithContext(ctx).Create(n).Error
}

func (r *notificationRepository) FindByID(ctx context.Context, id uint64) (*notifymodel.Notification, error) {
	var n notifymodel.Notification
	if err := r.db.WithContext(ctx).First(&n, id).Error; err != nil {
		return nil, err
	}
	return &n, nil
}

func (r *notificationRepository) List(ctx context.Context, q NotificationListParams) ([]notifymodel.Notification, int64, error) {
	query := r.db.WithContext(ctx).Model(&notifymodel.Notification{})
	if q.Event != "" {
		query = query.Where("event = ?", q.Event)
	}
	if q.Channel != "" {
		query = query.Where("channel = ?", q.Channel)
	}
	if q.SendStatus != "" {
		query = query.Where("send_status = ?", q.SendStatus)
	}
	if q.TargetType != "" {
		query = query.Where("target_type = ?", q.TargetType)
	}
	if q.Keyword != "" {
		query = query.Where("title LIKE ?", "%"+q.Keyword+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []notifymodel.Notification
	if err := query.Order("created_at DESC").Offset(q.Offset).Limit(q.Limit).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *notificationRepository) ListByUser(ctx context.Context, userID uint64, page, pageSize int) ([]notifymodel.Notification, int64, error) {
	query := r.db.WithContext(ctx).Model(&notifymodel.Notification{}).
		Where("user_id = ? AND channel = ?", userID, notifymodel.ChannelInbox)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	var items []notifymodel.Notification
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *notificationRepository) ListBroadcast(ctx context.Context, targetType string, page, pageSize int) ([]notifymodel.Notification, int64, error) {
	query := r.db.WithContext(ctx).Model(&notifymodel.Notification{}).
		Where("user_id = 0 AND channel = ? AND target_type = ?", notifymodel.ChannelInbox, targetType)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	var items []notifymodel.Notification
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// CountUnread 未读数 = 定向通知未读 + 广播未读（排除已读表记录）。
func (r *notificationRepository) CountUnread(ctx context.Context, userID uint64, targetType string) (int64, error) {
	var directCount int64
	if err := r.db.WithContext(ctx).Model(&notifymodel.Notification{}).
		Where("user_id = ? AND target_type = ? AND channel = ? AND read_at IS NULL", userID, targetType, notifymodel.ChannelInbox).
		Count(&directCount).Error; err != nil {
		return 0, err
	}

	var broadcastCount int64
	subQuery := r.db.WithContext(ctx).Model(&notifymodel.NotificationRead{}).
		Select("notification_id").
		Where("user_id = ?", userID)
	if err := r.db.WithContext(ctx).Model(&notifymodel.Notification{}).
		Where("user_id = 0 AND target_type = ? AND channel = ?", targetType, notifymodel.ChannelInbox).
		Where("id NOT IN (?)", subQuery).
		Count(&broadcastCount).Error; err != nil {
		return 0, err
	}
	return directCount + broadcastCount, nil
}

func (r *notificationRepository) MarkRead(ctx context.Context, id, userID uint64) error {
	now := time.Now()
	// 定向通知：直接更新 read_at
	result := r.db.WithContext(ctx).Model(&notifymodel.Notification{}).
		Where("id = ? AND user_id = ? AND read_at IS NULL", id, userID).
		Update("read_at", &now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}
	// 广播通知：插入已读记录（幂等）
	return r.CreateReadRecord(ctx, id, userID)
}

func (r *notificationRepository) MarkAllRead(ctx context.Context, userID uint64) error {
	now := time.Now
	_ = now
	// 定向通知全部已读
	if err := r.db.WithContext(ctx).Model(&notifymodel.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).
		Update("read_at", time.Now()).Error; err != nil {
		return err
	}
	// 广播通知：批量插入已读记录
	var broadcastIDs []uint64
	if err := r.db.WithContext(ctx).Model(&notifymodel.Notification{}).
		Where("user_id = 0 AND channel = ?", notifymodel.ChannelInbox).
		Pluck("id", &broadcastIDs).Error; err != nil {
		return err
	}
	for _, nid := range broadcastIDs {
		_ = r.CreateReadRecord(ctx, nid, userID)
	}
	return nil
}

func (r *notificationRepository) UpdateSendStatus(ctx context.Context, id uint64, status, failReason string) error {
	return r.db.WithContext(ctx).Model(&notifymodel.Notification{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"send_status": status,
			"fail_reason": failReason,
		}).Error
}

func (r *notificationRepository) CreateReadRecord(ctx context.Context, notificationID, userID uint64) error {
	read := &notifymodel.NotificationRead{
		NotificationID: notificationID,
		UserID:         userID,
	}
	return r.db.WithContext(ctx).Create(read).Error
}

func (r *notificationRepository) HasReadRecord(ctx context.Context, notificationID, userID uint64) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&notifymodel.NotificationRead{}).
		Where("notification_id = ? AND user_id = ?", notificationID, userID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
