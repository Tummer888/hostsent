package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	notifydto "hostsent/backend/internal/modules/admin/notification/dto"
	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
)

// ChannelRepository 通知渠道实例数据访问。
type ChannelRepository interface {
	Create(ctx context.Context, c *notifymodel.NotificationChannel) error
	Update(ctx context.Context, c *notifymodel.NotificationChannel) error
	FindByID(ctx context.Context, id uint64) (*notifymodel.NotificationChannel, error)
	FindByCode(ctx context.Context, code string) (*notifymodel.NotificationChannel, error)
	List(ctx context.Context, q notifydto.ChannelListQuery) ([]notifymodel.NotificationChannel, int64, error)
	// ListEnabled 返回启用渠道，按 priority desc, weight desc, id asc 排序（路由用）。
	ListEnabled(ctx context.Context, category string) ([]notifymodel.NotificationChannel, error)
	// PickByCategoryAndScene 按类别 + 场景挑选渠道：显式场景匹配优先，其次默认渠道。
	PickByCategoryAndScene(ctx context.Context, category, scene string) (*notifymodel.NotificationChannel, error)
	// UpdateHealth 回写健康状态与检查时间（test 与发送路径共用）。
	UpdateHealth(ctx context.Context, id uint64, health, lastErr string) error
	// ClearDefault 清除同 category 下其他渠道的默认标记。
	ClearDefault(ctx context.Context, category string, exceptID uint64) error
	// DailySentCount 统计某渠道当日已发送条数（daily_limit 用；0=不限）。
	DailySentCount(ctx context.Context, channelID uint64) (int64, error)
}

type channelRepository struct {
	db *gorm.DB
}

// NewChannelRepository 创建渠道仓储。
func NewChannelRepository(db *gorm.DB) ChannelRepository {
	return &channelRepository{db: db}
}

func (r *channelRepository) Create(ctx context.Context, c *notifymodel.NotificationChannel) error {
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *channelRepository) Update(ctx context.Context, c *notifymodel.NotificationChannel) error {
	return r.db.WithContext(ctx).Save(c).Error
}

func (r *channelRepository) FindByID(ctx context.Context, id uint64) (*notifymodel.NotificationChannel, error) {
	var c notifymodel.NotificationChannel
	if err := r.db.WithContext(ctx).First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *channelRepository) FindByCode(ctx context.Context, code string) (*notifymodel.NotificationChannel, error) {
	var c notifymodel.NotificationChannel
	if err := r.db.WithContext(ctx).Where("channel_code = ?", code).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *channelRepository) List(ctx context.Context, q notifydto.ChannelListQuery) ([]notifymodel.NotificationChannel, int64, error) {
	base := r.db.WithContext(ctx).Model(&notifymodel.NotificationChannel{})
	if q.Category != "" {
		base = base.Where("category = ?", q.Category)
	}
	if q.Type != "" {
		base = base.Where("type = ?", q.Type)
	}
	if q.Status != nil {
		base = base.Where("status = ?", *q.Status)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		like := "%" + kw + "%"
		base = base.Where("name LIKE ? OR channel_code LIKE ?", like, like)
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
		size = 10
	}
	if size > 100 {
		size = 100
	}
	var items []notifymodel.NotificationChannel
	err := base.Order("priority desc, id desc").Offset((page - 1) * size).Limit(size).Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *channelRepository) ListEnabled(ctx context.Context, category string) ([]notifymodel.NotificationChannel, error) {
	q := r.db.WithContext(ctx).Where("status = ?", notifymodel.ChannelStatusEnabled)
	if category != "" {
		q = q.Where("category = ?", category)
	}
	var items []notifymodel.NotificationChannel
	err := q.Order("priority desc, weight desc, id asc").Find(&items).Error
	return items, err
}

// PickByCategoryAndScene 场景匹配优先、默认渠道兜底，同优先级按 id 升序取第一条。
func (r *channelRepository) PickByCategoryAndScene(ctx context.Context, category, scene string) (*notifymodel.NotificationChannel, error) {
	items, err := r.ListEnabled(ctx, category)
	if err != nil {
		return nil, err
	}
	var fallback *notifymodel.NotificationChannel
	for i := range items {
		c := items[i]
		if scene != "" && channelScenesMatch(c.Scenes, scene) {
			return &c, nil
		}
		if c.IsDefault && fallback == nil {
			fallback = &c
		}
	}
	if fallback != nil {
		return fallback, nil
	}
	// 无默认渠道时取排序第一条（ListEnabled 已按 priority/weight 排序）。
	if len(items) > 0 {
		return &items[0], nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *channelRepository) UpdateHealth(ctx context.Context, id uint64, health, lastErr string) error {
	now := timeNow()
	return r.db.WithContext(ctx).Model(&notifymodel.NotificationChannel{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"health_status": health,
			"last_error":    lastErr,
			"last_check_at": now,
		}).Error
}

func (r *channelRepository) ClearDefault(ctx context.Context, category string, exceptID uint64) error {
	q := r.db.WithContext(ctx).Model(&notifymodel.NotificationChannel{}).Where("id <> ?", exceptID)
	if category != "" {
		q = q.Where("category = ?", category)
	}
	return q.Update("is_default", false).Error
}

func (r *channelRepository) DailySentCount(ctx context.Context, channelID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&notifymodel.NotificationDelivery{}).
		Where("channel_id = ? AND send_status = ? AND created_at >= date_trunc('day', now())",
			channelID, notifymodel.DeliveryStatusSent).
		Count(&count).Error
	return count, err
}
