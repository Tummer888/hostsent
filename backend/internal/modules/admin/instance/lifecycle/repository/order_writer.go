package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	ordermodel "hostsent/backend/internal/modules/admin/order/model"
)

// OrderWriter 续费订单写入器（复用 orders 表，renewal_id 列标识续费订单）。
type OrderWriter interface {
	CreateOrder(ctx context.Context, order *ordermodel.Order) error
	MarkOrderPaid(ctx context.Context, orderID uint64, paidAmount float64, payMethod string, operatorID uint64) error
}

type orderWriter struct {
	db *gorm.DB
}

// NewOrderWriter 创建订单写入器。
func NewOrderWriter(db *gorm.DB) OrderWriter {
	return &orderWriter{db: db}
}

func (r *orderWriter) CreateOrder(ctx context.Context, order *ordermodel.Order) error {
	return r.db.WithContext(ctx).Create(order).Error
}

func (r *orderWriter) MarkOrderPaid(ctx context.Context, orderID uint64, paidAmount float64, payMethod string, operatorID uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&ordermodel.Order{}).
		Where("id = ? AND status = ?", orderID, ordermodel.OrderStatusPending).
		Updates(map[string]interface{}{
			"status":      ordermodel.OrderStatusPaid,
			"paid_amount": paidAmount,
			"pay_method":  payMethod,
			"pay_time":    &now,
			"operator_id": operatorID,
		}).Error
}

// genOrderNo 生成续费订单号：RO + yyyymmddHHMMSS + 6 位随机。
func genOrderNo() string {
	return fmt.Sprintf("RO%s%06d", time.Now().Format("20060102150405"), time.Now().UnixNano()%1000000)
}
