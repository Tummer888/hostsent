package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	notifydto "hostsent/backend/internal/modules/admin/notification/dto"
	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
)

// DeliveryRepository 投递队列数据访问。
type DeliveryRepository interface {
	// Enqueue 单条入队；幂等唯一索引冲突时静默忽略（重复发布视为成功）。
	Enqueue(ctx context.Context, d *notifymodel.NotificationDelivery) error
	// BulkInsert 批量入队（群发），幂等冲突忽略。
	BulkInsert(ctx context.Context, rows []notifymodel.NotificationDelivery) error
	// ClaimDue 领取到期任务并置 sending（FOR UPDATE SKIP LOCKED，并发安全）。
	ClaimDue(ctx context.Context, limit int, lockTTL time.Duration) ([]notifymodel.NotificationDelivery, error)
	FindByID(ctx context.Context, id uint64) (*notifymodel.NotificationDelivery, error)
	List(ctx context.Context, q notifydto.DeliveryListQuery) ([]notifymodel.NotificationDelivery, int64, error)
	// FindBySource 按来源定位投递记录（通知记录页「重发」转发用）。
	FindBySource(ctx context.Context, sourceModule, sourceID, channel string, targetID uint64) (*notifymodel.NotificationDelivery, error)
	MarkSent(ctx context.Context, id uint64, msgID, code string, costFen int) error
	// MarkSentWithChannel 发送成功时一并记录实际使用的渠道实例（邮件/短信）。
	MarkSentWithChannel(ctx context.Context, id, channelID uint64, msgID, code string, costFen int) error
	MarkSkipped(ctx context.Context, id uint64, reason string) error
	// MarkFailure 记录一次失败；达到上游 max_attempts 时由调用方转 MarkDead。
	MarkFailure(ctx context.Context, id uint64, attempts int, nextRetry *time.Time, reason string) error
	MarkDead(ctx context.Context, id uint64, reason string) error
	// ResetForRetry 手动重投：attempts 归零、next_retry_at 置空、状态回 pending。
	ResetForRetry(ctx context.Context, id uint64) error
	// CountByBatch 统计批次内各状态条数（群发结果页）。
	CountByBatch(ctx context.Context, batchID string) (map[string]int64, error)
}

type deliveryRepository struct {
	db *gorm.DB
}

// NewDeliveryRepository 创建投递队列仓储。
func NewDeliveryRepository(db *gorm.DB) DeliveryRepository {
	return &deliveryRepository{db: db}
}

// Enqueue 入队；命中 uk_notify_deliveries_idem 时忽略（重复发布视为成功）。
func (r *deliveryRepository) Enqueue(ctx context.Context, d *notifymodel.NotificationDelivery) error {
	if d.MaxAttempts <= 0 {
		d.MaxAttempts = notifymodel.DefaultMaxAttempts
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(d).Error
}

func (r *deliveryRepository) BulkInsert(ctx context.Context, rows []notifymodel.NotificationDelivery) error {
	if len(rows) == 0 {
		return nil
	}
	for i := range rows {
		if rows[i].MaxAttempts <= 0 {
			rows[i].MaxAttempts = notifymodel.DefaultMaxAttempts
		}
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		CreateInBatches(rows, 500).Error
}

// ClaimDue 领取到期任务：SELECT ... FOR UPDATE SKIP LOCKED，随后置 sending 并加锁。
func (r *deliveryRepository) ClaimDue(ctx context.Context, limit int, lockTTL time.Duration) ([]notifymodel.NotificationDelivery, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []notifymodel.NotificationDelivery
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("send_status IN ?", []string{notifymodel.DeliveryStatusPending, notifymodel.DeliveryStatusFailed}).
			Where("(next_retry_at IS NULL OR next_retry_at <= now())").
			Where("(locked_until IS NULL OR locked_until < now())").
			Order("created_at ASC").
			Limit(limit).
			Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		ids := make([]uint64, 0, len(rows))
		for i := range rows {
			ids = append(ids, rows[i].ID)
		}
		until := time.Now().Add(lockTTL)
		if err := tx.Model(&notifymodel.NotificationDelivery{}).
			Where("id IN ?", ids).
			Updates(map[string]any{
				"send_status":  notifymodel.DeliveryStatusSending,
				"locked_until": until,
			}).Error; err != nil {
			return err
		}
		for i := range rows {
			rows[i].SendStatus = notifymodel.DeliveryStatusSending
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *deliveryRepository) FindByID(ctx context.Context, id uint64) (*notifymodel.NotificationDelivery, error) {
	var d notifymodel.NotificationDelivery
	if err := r.db.WithContext(ctx).First(&d, id).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *deliveryRepository) FindBySource(ctx context.Context, sourceModule, sourceID, channel string, targetID uint64) (*notifymodel.NotificationDelivery, error) {
	var d notifymodel.NotificationDelivery
	q := r.db.WithContext(ctx).Where("source_module = ? AND source_id = ?", sourceModule, sourceID)
	if channel != "" {
		q = q.Where("channel = ?", channel)
	}
	if targetID > 0 {
		q = q.Where("target_id = ?", targetID)
	}
	if err := q.Order("id DESC").First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *deliveryRepository) List(ctx context.Context, q notifydto.DeliveryListQuery) ([]notifymodel.NotificationDelivery, int64, error) {
	base := r.db.WithContext(ctx).Model(&notifymodel.NotificationDelivery{})
	if q.Event != "" {
		base = base.Where("event = ?", q.Event)
	}
	if q.Channel != "" {
		base = base.Where("channel = ?", q.Channel)
	}
	if q.SendStatus != "" {
		base = base.Where("send_status = ?", q.SendStatus)
	}
	if q.TargetType != "" {
		base = base.Where("target_type = ?", q.TargetType)
	}
	if q.BatchID != "" {
		base = base.Where("batch_id = ?", q.BatchID)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		like := "%" + kw + "%"
		base = base.Where("recipient LIKE ? OR title LIKE ? OR target_name LIKE ?", like, like, like)
	}
	if t, ok := parseTime(q.StartTime); ok {
		base = base.Where("created_at >= ?", t)
	}
	if t, ok := parseTime(q.EndTime); ok {
		base = base.Where("created_at <= ?", t)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	size := q.PageSize
	if size <= 0 {
		size = 20
	}
	if size > 200 {
		size = 200
	}
	var items []notifymodel.NotificationDelivery
	if err := base.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *deliveryRepository) MarkSent(ctx context.Context, id uint64, msgID, code string, costFen int) error {
	return r.MarkSentWithChannel(ctx, id, 0, msgID, code, costFen)
}

// MarkSentWithChannel channelID=0 时不覆盖已有值（站内信无渠道）。
func (r *deliveryRepository) MarkSentWithChannel(ctx context.Context, id, channelID uint64, msgID, code string, costFen int) error {
	now := time.Now()
	updates := map[string]any{
		"send_status":     notifymodel.DeliveryStatusSent,
		"provider_msg_id": msgID,
		"provider_code":   code,
		"cost_fen":        costFen,
		"fail_reason":     "",
		"next_retry_at":   nil,
		"locked_until":    nil,
		"sent_at":         now,
	}
	if channelID > 0 {
		updates["channel_id"] = channelID
	}
	return r.db.WithContext(ctx).Model(&notifymodel.NotificationDelivery{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *deliveryRepository) MarkSkipped(ctx context.Context, id uint64, reason string) error {
	return r.db.WithContext(ctx).Model(&notifymodel.NotificationDelivery{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"send_status":   notifymodel.DeliveryStatusSkipped,
			"fail_reason":   truncate(reason, 500),
			"next_retry_at": nil,
			"locked_until":  nil,
		}).Error
}

func (r *deliveryRepository) MarkFailure(ctx context.Context, id uint64, attempts int, nextRetry *time.Time, reason string) error {
	return r.db.WithContext(ctx).Model(&notifymodel.NotificationDelivery{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"send_status":   notifymodel.DeliveryStatusFailed,
			"attempts":      attempts,
			"next_retry_at": nextRetry,
			"locked_until":  nil,
			"fail_reason":   truncate(reason, 500),
		}).Error
}

func (r *deliveryRepository) MarkDead(ctx context.Context, id uint64, reason string) error {
	return r.db.WithContext(ctx).Model(&notifymodel.NotificationDelivery{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"send_status":   notifymodel.DeliveryStatusDead,
			"fail_reason":   truncate(reason, 500),
			"next_retry_at": nil,
			"locked_until":  nil,
		}).Error
}

func (r *deliveryRepository) ResetForRetry(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&notifymodel.NotificationDelivery{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"send_status":   notifymodel.DeliveryStatusPending,
			"attempts":      0,
			"next_retry_at": nil,
			"locked_until":  nil,
			"fail_reason":   "",
		}).Error
}

func (r *deliveryRepository) CountByBatch(ctx context.Context, batchID string) (map[string]int64, error) {
	type row struct {
		SendStatus string
		Count      int64
	}
	var rows []row
	if err := r.db.WithContext(ctx).Model(&notifymodel.NotificationDelivery{}).
		Select("send_status, count(*) AS count").
		Where("batch_id = ?", batchID).
		Group("send_status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.SendStatus] = r.Count
	}
	return out, nil
}

// parseTime 解析 ISO8601 / "2006-01-02 15:04:05" / 日期三种格式。
func parseTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	// 按 rune 截断，避免把 UTF-8 字符切坏。
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
