// Package repository 提供订单域的数据访问实现。
package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/order/dto"
	"hostsent/backend/internal/modules/admin/order/model"
)

// OrderRepository 定义订单数据访问能力。
type OrderRepository interface {
	List(ctx context.Context, query dto.OrderListQuery) ([]model.Order, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.Order, error)
	FindByIDs(ctx context.Context, ids []uint64) ([]model.Order, error)
	Create(ctx context.Context, item *model.Order) error
	Update(ctx context.Context, item *model.Order) error
	// StatsCountToday 今日订单量
	StatsCountToday(ctx context.Context) (int64, error)
	// StatsSalesToday 今日销售额（已支付口径）
	StatsSalesToday(ctx context.Context) (float64, error)
	// StatsTotal 累计（订单总量 + 销售总额）
	StatsTotal(ctx context.Context) (total int64, sales float64, err error)
	// CountByStatus 订单状态分布
	CountByStatus(ctx context.Context) ([]model.OrderStatusCount, error)
	// StatsTrend 近 N 日订单趋势（销售额 + 订单量）
	StatsTrend(ctx context.Context, days int) ([]model.OrderTrendPoint, error)
}

type orderRepository struct {
	db *gorm.DB
}

// NewOrderRepository 创建订单仓储实现。
func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &orderRepository{db: db}
}

func (r *orderRepository) List(ctx context.Context, query dto.OrderListQuery) ([]model.Order, int64, error) {
	page := normalizePage(query.Page)
	pageSize := normalizePageSize(query.PageSize)

	base := r.db.WithContext(ctx).Model(&model.Order{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("order_no ILIKE ? OR product_name ILIKE ?", like, like)
	}
	if userKeyword := strings.TrimSpace(query.UserKeyword); userKeyword != "" {
		like := "%" + userKeyword + "%"
		base = base.Joins("JOIN users ON users.id = orders.user_id").
			Where("users.username ILIKE ? OR users.email ILIKE ? OR users.phone ILIKE ?", like, like, like)
	}
	if query.UserID > 0 {
		base = base.Where("user_id = ?", query.UserID)
	}
	if query.ProductID > 0 {
		base = base.Where("product_id = ?", query.ProductID)
	}
	if query.Status != "" {
		base = base.Where("status = ?", query.Status)
	}
	if query.PayMethod != "" {
		base = base.Where("pay_method = ?", query.PayMethod)
	}
	if start := normalizeTime(query.StartTime); start != nil {
		base = base.Where("created_at >= ?", *start)
	}
	if end := normalizeTime(query.EndTime); end != nil {
		base = base.Where("created_at <= ?", *end)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.Order
	if err := base.Order("created_at desc, id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *orderRepository) FindByID(ctx context.Context, id uint64) (*model.Order, error) {
	var item model.Order
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *orderRepository) FindByIDs(ctx context.Context, ids []uint64) ([]model.Order, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var items []model.Order
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *orderRepository) Create(ctx context.Context, item *model.Order) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *orderRepository) Update(ctx context.Context, item *model.Order) error {
	return r.db.WithContext(ctx).Save(item).Error
}

// paidStatuses 统计销售额时视为已支付的订单状态集合。
var paidStatuses = []string{
	model.OrderStatusPaid,
	model.OrderStatusProvisioning,
	model.OrderStatusActive,
	model.OrderStatusRefunding,
	model.OrderStatusRefunded,
	model.OrderStatusCompleted,
}

func (r *orderRepository) StatsCountToday(ctx context.Context) (int64, error) {
	var count int64
	start := dayStart(time.Now())
	if err := r.db.WithContext(ctx).Model(&model.Order{}).Where("created_at >= ?", start).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *orderRepository) StatsSalesToday(ctx context.Context) (float64, error) {
	var sum float64
	start := dayStart(time.Now())
	if err := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("created_at >= ? AND status IN ?", start, paidStatuses).
		Select("COALESCE(SUM(paid_amount), 0)").Scan(&sum).Error; err != nil {
		return 0, err
	}
	return sum, nil
}

func (r *orderRepository) StatsTotal(ctx context.Context) (int64, float64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.Order{}).Count(&total).Error; err != nil {
		return 0, 0, err
	}
	var sales float64
	if err := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("status IN ?", paidStatuses).
		Select("COALESCE(SUM(paid_amount), 0)").Scan(&sales).Error; err != nil {
		return 0, 0, err
	}
	return total, sales, nil
}

func (r *orderRepository) CountByStatus(ctx context.Context) ([]model.OrderStatusCount, error) {
	var counts []model.OrderStatusCount
	if err := r.db.WithContext(ctx).Model(&model.Order{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&counts).Error; err != nil {
		return nil, err
	}
	return counts, nil
}

func (r *orderRepository) StatsTrend(ctx context.Context, days int) ([]model.OrderTrendPoint, error) {
	if days <= 0 {
		days = 7
	}
	start := dayStart(time.Now()).AddDate(0, 0, -(days - 1))
	salesIn := strings.Join(paidStatuses, "','")
	var points []model.OrderTrendPoint
	if err := r.db.WithContext(ctx).Model(&model.Order{}).
		Select("to_char(created_at, 'YYYY-MM-DD') as date, SUM(CASE WHEN status IN ('"+salesIn+"') THEN paid_amount ELSE 0 END) as sales, COUNT(*) as orders").
		Where("created_at >= ?", start).
		Group("date").
		Order("date asc").
		Scan(&points).Error; err != nil {
		return nil, err
	}
	return points, nil
}

func dayStart(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
