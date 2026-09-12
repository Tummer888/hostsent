package repository

import (
	"context"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/payment/dto"
	"hostsent/backend/internal/modules/admin/payment/model"
)

// ChannelRepository 支付渠道实例数据访问。
type ChannelRepository interface {
	Create(ctx context.Context, c *model.PaymentChannel) error
	Update(ctx context.Context, c *model.PaymentChannel) error
	FindByID(ctx context.Context, id uint64) (*model.PaymentChannel, error)
	FindByCode(ctx context.Context, code string) (*model.PaymentChannel, error)
	List(ctx context.Context, q dto.ChannelListQuery) ([]model.PaymentChannel, int64, error)
	// ListEnabled 返回启用渠道，按 priority desc, weight desc, id asc 排序（路由用）。
	ListEnabled(ctx context.Context) ([]model.PaymentChannel, error)
	// ClearDefault 清除同场景下其他渠道的默认标记（保证单场景单默认）。
	ClearDefault(ctx context.Context, exceptID uint64) error
}

type channelRepository struct {
	db *gorm.DB
}

// NewChannelRepository 创建渠道仓储。
func NewChannelRepository(db *gorm.DB) ChannelRepository {
	return &channelRepository{db: db}
}

func (r *channelRepository) Create(ctx context.Context, c *model.PaymentChannel) error {
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *channelRepository) Update(ctx context.Context, c *model.PaymentChannel) error {
	return r.db.WithContext(ctx).Save(c).Error
}

func (r *channelRepository) FindByID(ctx context.Context, id uint64) (*model.PaymentChannel, error) {
	var c model.PaymentChannel
	if err := r.db.WithContext(ctx).First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *channelRepository) FindByCode(ctx context.Context, code string) (*model.PaymentChannel, error) {
	var c model.PaymentChannel
	if err := r.db.WithContext(ctx).Where("channel_code = ?", code).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *channelRepository) List(ctx context.Context, q dto.ChannelListQuery) ([]model.PaymentChannel, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.PaymentChannel{})
	if q.Type != "" {
		base = base.Where("type = ?", q.Type)
	}
	if q.Status != nil {
		base = base.Where("status = ?", *q.Status)
	}
	if kw := q.Keyword; kw != "" {
		like := "%" + kw + "%"
		base = base.Where("name LIKE ? OR channel_code LIKE ?", like, like)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.PaymentChannel
	err := base.Order("priority desc, id desc").
		Offset((normalizePage(q.Page) - 1) * normalizePageSize(q.PageSize)).
		Limit(normalizePageSize(q.PageSize)).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *channelRepository) ListEnabled(ctx context.Context) ([]model.PaymentChannel, error) {
	var items []model.PaymentChannel
	err := r.db.WithContext(ctx).
		Where("status = ?", model.StatusEnabled).
		Order("priority desc, weight desc, id asc").
		Find(&items).Error
	return items, err
}

func (r *channelRepository) ClearDefault(ctx context.Context, exceptID uint64) error {
	return r.db.WithContext(ctx).Model(&model.PaymentChannel{}).
		Where("id <> ?", exceptID).
		Update("is_default", false).Error
}
