package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/order/dto"
	"hostsent/backend/internal/modules/admin/order/model"
)

// RefundRepository 定义退款单数据访问能力。
type RefundRepository interface {
	List(ctx context.Context, query dto.RefundListQuery) ([]model.OrderRefund, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.OrderRefund, error)
	FindByOrder(ctx context.Context, orderID uint64) ([]model.OrderRefund, error)
	Create(ctx context.Context, item *model.OrderRefund) error
	Update(ctx context.Context, item *model.OrderRefund) error
	// SumEffective 订单已生效（已通过/已退款）退款金额合计。
	SumEffective(ctx context.Context, orderID uint64) (float64, error)
	// SumRefundedAmount 全量已通过/已退款金额合计（用于退款率）。
	SumRefundedAmount(ctx context.Context) (float64, error)
}

type refundRepository struct {
	db *gorm.DB
}

// NewRefundRepository 创建退款单仓储实现。
func NewRefundRepository(db *gorm.DB) RefundRepository {
	return &refundRepository{db: db}
}

func (r *refundRepository) List(ctx context.Context, query dto.RefundListQuery) ([]model.OrderRefund, int64, error) {
	page := normalizePage(query.Page)
	pageSize := normalizePageSize(query.PageSize)

	base := r.db.WithContext(ctx).Model(&model.OrderRefund{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("refund_no ILIKE ? OR reason ILIKE ? OR audit_by_name ILIKE ?", like, like, like)
	}
	if query.OrderID > 0 {
		base = base.Where("order_id = ?", query.OrderID)
	}
	if query.Status != "" {
		base = base.Where("status = ?", query.Status)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.OrderRefund
	if err := base.Order("created_at desc, id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *refundRepository) FindByID(ctx context.Context, id uint64) (*model.OrderRefund, error) {
	var item model.OrderRefund
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *refundRepository) FindByOrder(ctx context.Context, orderID uint64) ([]model.OrderRefund, error) {
	var items []model.OrderRefund
	if err := r.db.WithContext(ctx).Where("order_id = ?", orderID).Order("created_at desc, id desc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *refundRepository) Create(ctx context.Context, item *model.OrderRefund) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *refundRepository) Update(ctx context.Context, item *model.OrderRefund) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *refundRepository) SumEffective(ctx context.Context, orderID uint64) (float64, error) {
	var sum float64
	if err := r.db.WithContext(ctx).Model(&model.OrderRefund{}).
		Where("order_id = ? AND status IN ?", orderID, []string{model.RefundStatusApproved, model.RefundStatusDone}).
		Select("COALESCE(SUM(amount), 0)").Scan(&sum).Error; err != nil {
		return 0, err
	}
	return sum, nil
}

func (r *refundRepository) SumRefundedAmount(ctx context.Context) (float64, error) {
	var sum float64
	if err := r.db.WithContext(ctx).Model(&model.OrderRefund{}).
		Where("status IN ?", []string{model.RefundStatusApproved, model.RefundStatusDone}).
		Select("COALESCE(SUM(amount), 0)").Scan(&sum).Error; err != nil {
		return 0, err
	}
	return sum, nil
}
