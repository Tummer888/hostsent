package repository

import (
	"context"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/order/model"
)

// OrderItemRepository 定义订单明细数据访问能力。
type OrderItemRepository interface {
	ListByOrder(ctx context.Context, orderID uint64) ([]model.OrderItem, error)
	ListByOrders(ctx context.Context, orderIDs []uint64) ([]model.OrderItem, error)
	Create(ctx context.Context, item *model.OrderItem) error
}

type orderItemRepository struct {
	db *gorm.DB
}

// NewOrderItemRepository 创建订单明细仓储实现。
func NewOrderItemRepository(db *gorm.DB) OrderItemRepository {
	return &orderItemRepository{db: db}
}

func (r *orderItemRepository) ListByOrder(ctx context.Context, orderID uint64) ([]model.OrderItem, error) {
	var items []model.OrderItem
	if err := r.db.WithContext(ctx).Where("order_id = ?", orderID).Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *orderItemRepository) ListByOrders(ctx context.Context, orderIDs []uint64) ([]model.OrderItem, error) {
	if len(orderIDs) == 0 {
		return nil, nil
	}
	var items []model.OrderItem
	if err := r.db.WithContext(ctx).Where("order_id IN ?", orderIDs).Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *orderItemRepository) Create(ctx context.Context, item *model.OrderItem) error {
	return r.db.WithContext(ctx).Create(item).Error
}
