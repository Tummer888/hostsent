package repository

import (
	"context"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/payment/dto"
	"hostsent/backend/internal/modules/admin/payment/model"
)

// RefundRepository 渠道退款单数据访问。
type RefundRepository interface {
	Create(ctx context.Context, r *model.PaymentRefund) error
	Update(ctx context.Context, r *model.PaymentRefund) error
	FindByNo(ctx context.Context, refundNo string) (*model.PaymentRefund, error)
	SumSuccessByPayment(ctx context.Context, paymentNo string) (int64, error)
	List(ctx context.Context, q dto.RefundListQuery) ([]model.PaymentRefund, int64, error)
}

type refundRepository struct {
	db *gorm.DB
}

// NewRefundRepository 创建退款单仓储。
func NewRefundRepository(db *gorm.DB) RefundRepository {
	return &refundRepository{db: db}
}

func (r *refundRepository) Create(ctx context.Context, rf *model.PaymentRefund) error {
	return r.db.WithContext(ctx).Create(rf).Error
}

func (r *refundRepository) Update(ctx context.Context, rf *model.PaymentRefund) error {
	return r.db.WithContext(ctx).Save(rf).Error
}

func (r *refundRepository) FindByNo(ctx context.Context, refundNo string) (*model.PaymentRefund, error) {
	var rf model.PaymentRefund
	if err := r.db.WithContext(ctx).Where("refund_no = ?", refundNo).First(&rf).Error; err != nil {
		return nil, err
	}
	return &rf, nil
}

func (r *refundRepository) SumSuccessByPayment(ctx context.Context, paymentNo string) (int64, error) {
	var sum int64
	err := r.db.WithContext(ctx).Model(&model.PaymentRefund{}).
		Where("payment_no = ? AND status = ?", paymentNo, model.RefundStatusSuccess).
		Select("COALESCE(SUM(amount_fen),0)").Scan(&sum).Error
	return sum, err
}

func (r *refundRepository) List(ctx context.Context, q dto.RefundListQuery) ([]model.PaymentRefund, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.PaymentRefund{})
	if q.PaymentNo != "" {
		base = base.Where("payment_no = ?", q.PaymentNo)
	}
	if q.UserID > 0 {
		base = base.Where("user_id = ?", q.UserID)
	}
	if q.Status != "" {
		base = base.Where("status = ?", q.Status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.PaymentRefund
	err := base.Order("id desc").
		Offset((normalizePage(q.Page) - 1) * normalizePageSize(q.PageSize)).
		Limit(normalizePageSize(q.PageSize)).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
