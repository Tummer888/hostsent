package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/instance/dto"
)

// relatedLimit 关联记录单类返回上限（详情页展示用，不分页）。
const relatedLimit = 20

// RelatedRepository 实例关联业务记录只读仓储接口。
//
// 本仓储只做跨表只读查询，与 lifecycle/repository/instance_reader.go 的既有做法一致：
// 实例运维台需要「一屏看全售后上下文」，为此跨模块读表比为每个关联域引入依赖更轻。
type RelatedRepository interface {
	Renewals(ctx context.Context, instanceID uint64) ([]dto.RelatedRenewalItem, error)
	Tickets(ctx context.Context, instanceID uint64) ([]dto.RelatedTicketItem, error)
	Orders(ctx context.Context, orderIDs []uint64) ([]dto.RelatedOrderItem, error)
	RenewalOrderIDs(ctx context.Context, instanceID uint64) ([]uint64, error)
}

type relatedRepository struct {
	db *gorm.DB
}

// NewRelatedRepository 创建关联记录仓储。
func NewRelatedRepository(db *gorm.DB) RelatedRepository {
	return &relatedRepository{db: db}
}

// renewalRow instance_renewals 读取行。
type renewalRow struct {
	ID           uint64
	RenewalNo    string
	Source       string
	Status       string
	PeriodCount  int
	Amount       float64
	OrderID      uint64
	ExpireBefore *time.Time
	ExpireAfter  *time.Time
	CreatedAt    time.Time
}

// Renewals 查询实例的续费记录。
func (r *relatedRepository) Renewals(ctx context.Context, instanceID uint64) ([]dto.RelatedRenewalItem, error) {
	var rows []renewalRow
	err := r.db.WithContext(ctx).Table("instance_renewals").
		Select("id, renewal_no, source, status, period_count, amount, order_id, expire_before, expire_after, created_at").
		Where("instance_id = ?", instanceID).
		Order("id DESC").
		Limit(relatedLimit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]dto.RelatedRenewalItem, 0, len(rows))
	for _, it := range rows {
		out = append(out, dto.RelatedRenewalItem{
			ID:           it.ID,
			RenewalNo:    it.RenewalNo,
			Source:       it.Source,
			Status:       it.Status,
			PeriodCount:  it.PeriodCount,
			Amount:       it.Amount,
			ExpireBefore: fmtTimePtr(it.ExpireBefore),
			ExpireAfter:  fmtTimePtr(it.ExpireAfter),
			CreatedAt:    it.CreatedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}

// ticketRow tickets 读取行。
type ticketRow struct {
	ID           uint64
	TicketNo     string
	Title        string
	Priority     string
	Status       string
	AssignedName string
	CreatedAt    time.Time
}

// Tickets 查询关联到该实例的工单（tickets.instance_id 存 instances.id）。
func (r *relatedRepository) Tickets(ctx context.Context, instanceID uint64) ([]dto.RelatedTicketItem, error) {
	var rows []ticketRow
	err := r.db.WithContext(ctx).Table("tickets").
		Select("id, ticket_no, title, priority, status, assigned_name, created_at").
		Where("instance_id = ?", instanceID).
		Order("id DESC").
		Limit(relatedLimit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]dto.RelatedTicketItem, 0, len(rows))
	for _, it := range rows {
		out = append(out, dto.RelatedTicketItem{
			ID:           it.ID,
			TicketNo:     it.TicketNo,
			Title:        it.Title,
			Priority:     it.Priority,
			Status:       it.Status,
			AssignedName: it.AssignedName,
			CreatedAt:    it.CreatedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}

// orderRow orders 读取行。
type orderRow struct {
	ID          uint64
	OrderNo     string
	Status      string
	ProductName string
	TotalAmount float64
	PaidAmount  float64
	PayMethod   string
	CreatedAt   time.Time
}

// Orders 按订单 ID 集合查询订单（来源订单 + 续费订单合并后的集合）。
func (r *relatedRepository) Orders(ctx context.Context, orderIDs []uint64) ([]dto.RelatedOrderItem, error) {
	if len(orderIDs) == 0 {
		return []dto.RelatedOrderItem{}, nil
	}
	var rows []orderRow
	err := r.db.WithContext(ctx).Table("orders").
		Select("id, order_no, status, product_name, total_amount, paid_amount, pay_method, created_at").
		Where("id IN ?", orderIDs).
		Order("id DESC").
		Limit(relatedLimit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]dto.RelatedOrderItem, 0, len(rows))
	for _, it := range rows {
		out = append(out, dto.RelatedOrderItem{
			ID:          it.ID,
			OrderNo:     it.OrderNo,
			Status:      it.Status,
			ProductName: it.ProductName,
			TotalAmount: it.TotalAmount,
			PaidAmount:  it.PaidAmount,
			PayMethod:   it.PayMethod,
			CreatedAt:   it.CreatedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}

// RenewalOrderIDs 返回该实例续费记录关联的订单 ID（供 Orders 合并查询）。
func (r *relatedRepository) RenewalOrderIDs(ctx context.Context, instanceID uint64) ([]uint64, error) {
	var ids []uint64
	err := r.db.WithContext(ctx).Table("instance_renewals").
		Where("instance_id = ? AND order_id > 0", instanceID).
		Pluck("order_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// fmtTimePtr 时间指针转 RFC3339，空值返回空串。
func fmtTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
