package repository

import (
	"context"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/payment/dto"
	"hostsent/backend/internal/modules/admin/payment/model"
)

// OrderRepository 支付单数据访问。
type OrderRepository interface {
	Create(ctx context.Context, o *model.PaymentOrder) error
	Update(ctx context.Context, o *model.PaymentOrder) error
	FindByID(ctx context.Context, id uint64) (*model.PaymentOrder, error)
	FindByNo(ctx context.Context, paymentNo string) (*model.PaymentOrder, error)
	FindByChannelTx(ctx context.Context, channelID uint64, channelTx string) (*model.PaymentOrder, error)
	// FindPaidByBiz 查同业务已支付单（退款原路退回时定位渠道与渠道单号）。
	FindPaidByBiz(ctx context.Context, bizType, bizNo string) (*model.PaymentOrder, error)
	// FindPendingByBiz 查同业务未支付单（避免重复下单）。
	FindPendingByBiz(ctx context.Context, bizType string, bizID uint64) (*model.PaymentOrder, error)
	List(ctx context.Context, q dto.OrderListQuery) ([]model.PaymentOrder, int64, error)
	// ListPaidForRecon 按账期汇总已支付单（渠道对账用），period 为空取全量。
	SumPaid(ctx context.Context, channelCode, period string) (amountFen int64, count int64, err error)
}

type orderRepository struct {
	db *gorm.DB
}

// NewOrderRepository 创建支付单仓储。
func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &orderRepository{db: db}
}

func (r *orderRepository) Create(ctx context.Context, o *model.PaymentOrder) error {
	return r.db.WithContext(ctx).Create(o).Error
}

func (r *orderRepository) Update(ctx context.Context, o *model.PaymentOrder) error {
	return r.db.WithContext(ctx).Save(o).Error
}

func (r *orderRepository) FindByID(ctx context.Context, id uint64) (*model.PaymentOrder, error) {
	var o model.PaymentOrder
	if err := r.db.WithContext(ctx).First(&o, id).Error; err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *orderRepository) FindByNo(ctx context.Context, paymentNo string) (*model.PaymentOrder, error) {
	var o model.PaymentOrder
	if err := r.db.WithContext(ctx).Where("payment_no = ?", paymentNo).First(&o).Error; err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *orderRepository) FindByChannelTx(ctx context.Context, channelID uint64, channelTx string) (*model.PaymentOrder, error) {
	var o model.PaymentOrder
	if err := r.db.WithContext(ctx).
		Where("channel_id = ? AND channel_tx = ?", channelID, channelTx).
		First(&o).Error; err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *orderRepository) FindPendingByBiz(ctx context.Context, bizType string, bizID uint64) (*model.PaymentOrder, error) {
	var o model.PaymentOrder
	if err := r.db.WithContext(ctx).
		Where("biz_type = ? AND biz_id = ? AND status IN ?", bizType, bizID,
			[]string{model.OrderStatusPending, model.OrderStatusPaying}).
		Order("id desc").First(&o).Error; err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *orderRepository) FindPaidByBiz(ctx context.Context, bizType, bizNo string) (*model.PaymentOrder, error) {
	var o model.PaymentOrder
	if err := r.db.WithContext(ctx).
		Where("biz_type = ? AND biz_no = ? AND status = ?", bizType, bizNo, model.OrderStatusPaid).
		Order("id desc").First(&o).Error; err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *orderRepository) List(ctx context.Context, q dto.OrderListQuery) ([]model.PaymentOrder, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.PaymentOrder{})
	if q.UserID > 0 {
		base = base.Where("user_id = ?", q.UserID)
	}
	if q.PaymentNo != "" {
		base = base.Where("payment_no LIKE ?", "%"+q.PaymentNo+"%")
	}
	if q.BizType != "" {
		base = base.Where("biz_type = ?", q.BizType)
	}
	if q.ChannelCode != "" {
		base = base.Where("channel_code = ?", q.ChannelCode)
	}
	if q.Status != "" {
		base = base.Where("status = ?", q.Status)
	}
	if start := normalizeTime(q.StartTime); start != nil {
		base = base.Where("created_at >= ?", *start)
	}
	if end := normalizeTime(q.EndTime); end != nil {
		base = base.Where("created_at <= ?", *end)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.PaymentOrder
	err := base.Order("id desc").
		Offset((normalizePage(q.Page) - 1) * normalizePageSize(q.PageSize)).
		Limit(normalizePageSize(q.PageSize)).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *orderRepository) SumPaid(ctx context.Context, channelCode, period string) (int64, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.PaymentOrder{}).Where("status = ?", model.OrderStatusPaid)
	if channelCode != "" {
		base = base.Where("channel_code = ?", channelCode)
	}
	if start, end, ok := normalizePeriodRange(period); ok {
		base = base.Where("paid_at >= ? AND paid_at < ?", start, end)
	}
	var row struct {
		Amount int64
		Cnt    int64
	}
	err := base.Select("COALESCE(SUM(amount_fen),0) AS amount, COUNT(*) AS cnt").Scan(&row).Error
	if err != nil {
		return 0, 0, err
	}
	return row.Amount, row.Cnt, nil
}
