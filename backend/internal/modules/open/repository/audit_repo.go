package repository

import (
	"context"

	"gorm.io/gorm"

	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	openmodel "hostsent/backend/internal/modules/open/model"
)

// AuditRepository 对账数据源（T6.6）：本应用的幂等请求与订单。
type AuditRepository interface {
	// ListRequests 本应用幂等请求分页。
	ListRequests(ctx context.Context, appID uint64, page, pageSize int) ([]openmodel.OpenRequest, int64, error)
	// ListOrders 本应用代客下单订单分页（orders.open_app_id）。
	ListOrders(ctx context.Context, appID uint64, page, pageSize int) ([]ordermodel.Order, int64, error)
}

type auditRepository struct {
	db *gorm.DB
}

// NewAuditRepository 构造对账仓储。
func NewAuditRepository(db *gorm.DB) AuditRepository {
	return &auditRepository{db: db}
}

func (r *auditRepository) ListRequests(ctx context.Context, appID uint64, page, pageSize int) ([]openmodel.OpenRequest, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	q := r.db.WithContext(ctx).Model(&openmodel.OpenRequest{}).Where("app_id = ?", appID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []openmodel.OpenRequest
	err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error
	return rows, total, err
}

func (r *auditRepository) ListOrders(ctx context.Context, appID uint64, page, pageSize int) ([]ordermodel.Order, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	q := r.db.WithContext(ctx).Model(&ordermodel.Order{}).Where("open_app_id = ? AND channel = 'open'", appID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []ordermodel.Order
	err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error
	return rows, total, err
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
