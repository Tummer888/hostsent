// Package repository 提供工单域的数据访问实现。
package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/ticket/dto"
	"hostsent/backend/internal/modules/admin/ticket/model"
)

// TicketRepository 定义工单数据访问能力。
type TicketRepository interface {
	List(ctx context.Context, query dto.TicketListQuery) ([]model.Ticket, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.Ticket, error)
	Create(ctx context.Context, item *model.Ticket) error
	Update(ctx context.Context, item *model.Ticket) error
	// CountByTicketIDs 批量统计工单回复数（列表展示）
	CountByTicketIDs(ctx context.Context, ids []uint64) (map[uint64]int64, error)
	// UserNamesByIDs 批量查询用户账号（工单列表展示提交人）
	UserNamesByIDs(ctx context.Context, ids []uint64) (map[uint64]string, error)
	// AdminNameByID 查询管理员名称（分配时快照处理人）
	AdminNameByID(ctx context.Context, id uint64) (string, error)
	// —— 统计 ——
	// StatsTotal 工单总量
	StatsTotal(ctx context.Context) (int64, error)
	// CountByStatus 工单状态分布
	CountByStatus(ctx context.Context) ([]model.TicketStatusCount, error)
	// AvgFirstReplySeconds 平均首次响应时长（秒）
	AvgFirstReplySeconds(ctx context.Context) (int64, error)
	// StatsTrend 近 N 日工单量趋势
	StatsTrend(ctx context.Context, days int) ([]model.TicketTrendPoint, error)
	// CountByCategory 分类工单分布
	CountByCategory(ctx context.Context) ([]model.TicketCategoryStat, error)
}

type ticketRepository struct {
	db *gorm.DB
}

// NewTicketRepository 创建工单仓储实现。
func NewTicketRepository(db *gorm.DB) TicketRepository {
	return &ticketRepository{db: db}
}

func (r *ticketRepository) List(ctx context.Context, query dto.TicketListQuery) ([]model.Ticket, int64, error) {
	page := normalizePage(query.Page)
	pageSize := normalizePageSize(query.PageSize)

	base := r.db.WithContext(ctx).Model(&model.Ticket{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("ticket_no ILIKE ? OR title ILIKE ?", like, like)
	}
	if userKeyword := strings.TrimSpace(query.UserKeyword); userKeyword != "" {
		like := "%" + userKeyword + "%"
		base = base.Joins("JOIN users ON users.id = tickets.user_id").
			Where("users.username ILIKE ? OR users.email ILIKE ? OR users.phone ILIKE ?", like, like, like)
	}
	if query.UserID > 0 {
		base = base.Where("tickets.user_id = ?", query.UserID)
	}
	if category := strings.TrimSpace(query.Category); category != "" {
		base = base.Where("tickets.category = ?", category)
	}
	if query.Priority != "" {
		base = base.Where("tickets.priority = ?", query.Priority)
	}
	if query.Status != "" {
		base = base.Where("tickets.status = ?", query.Status)
	}
	if query.AssignedTo > 0 {
		base = base.Where("tickets.assigned_to = ?", query.AssignedTo)
	}
	if start := normalizeTime(query.StartTime); start != nil {
		base = base.Where("tickets.created_at >= ?", *start)
	}
	if end := normalizeTime(query.EndTime); end != nil {
		base = base.Where("tickets.created_at <= ?", *end)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.Ticket
	if err := base.Order("tickets.created_at desc, tickets.id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *ticketRepository) FindByID(ctx context.Context, id uint64) (*model.Ticket, error) {
	var item model.Ticket
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *ticketRepository) Create(ctx context.Context, item *model.Ticket) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *ticketRepository) Update(ctx context.Context, item *model.Ticket) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *ticketRepository) CountByTicketIDs(ctx context.Context, ids []uint64) (map[uint64]int64, error) {
	result := make(map[uint64]int64, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []struct {
		TicketID uint64 `gorm:"column:ticket_id"`
		Count    int64  `gorm:"column:count"`
	}
	if err := r.db.WithContext(ctx).Model(&model.TicketReply{}).
		Select("ticket_id, count(*) as count").
		Where("ticket_id IN ?", ids).
		Group("ticket_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.TicketID] = row.Count
	}
	return result, nil
}

func (r *ticketRepository) UserNamesByIDs(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	result := make(map[uint64]string, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []struct {
		ID       uint64 `gorm:"column:id"`
		Username string `gorm:"column:username"`
	}
	if err := r.db.WithContext(ctx).Table("users").
		Select("id, username").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID] = row.Username
	}
	return result, nil
}

func (r *ticketRepository) AdminNameByID(ctx context.Context, id uint64) (string, error) {
	var row struct {
		Username string `gorm:"column:username"`
	}
	if err := r.db.WithContext(ctx).Table("admins").
		Select("username").
		Where("id = ?", id).
		Scan(&row).Error; err != nil {
		return "", err
	}
	return row.Username, nil
}

func (r *ticketRepository) StatsTotal(ctx context.Context) (int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.Ticket{}).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *ticketRepository) CountByStatus(ctx context.Context) ([]model.TicketStatusCount, error) {
	var counts []model.TicketStatusCount
	if err := r.db.WithContext(ctx).Model(&model.Ticket{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&counts).Error; err != nil {
		return nil, err
	}
	return counts, nil
}

func (r *ticketRepository) AvgFirstReplySeconds(ctx context.Context) (int64, error) {
	var avg float64
	err := r.db.WithContext(ctx).Model(&model.Ticket{}).
		Select("COALESCE(AVG(EXTRACT(EPOCH FROM (first_reply_at - created_at))), 0)").
		Where("first_reply_at IS NOT NULL").
		Scan(&avg).Error
	if err != nil {
		return 0, err
	}
	return int64(avg + 0.5), nil
}

func (r *ticketRepository) StatsTrend(ctx context.Context, days int) ([]model.TicketTrendPoint, error) {
	if days <= 0 {
		days = 7
	}
	start := dayStart(time.Now()).AddDate(0, 0, -(days - 1))
	var points []model.TicketTrendPoint
	if err := r.db.WithContext(ctx).Model(&model.Ticket{}).
		Select("to_char(created_at, 'YYYY-MM-DD') as date, COUNT(*) as count").
		Where("created_at >= ?", start).
		Group("date").
		Order("date asc").
		Scan(&points).Error; err != nil {
		return nil, err
	}
	return points, nil
}

func (r *ticketRepository) CountByCategory(ctx context.Context) ([]model.TicketCategoryStat, error) {
	var stats []model.TicketCategoryStat
	if err := r.db.WithContext(ctx).Model(&model.Ticket{}).
		Select("category, count(*) as count").
		Group("category").
		Order("count desc").
		Scan(&stats).Error; err != nil {
		return nil, err
	}
	return stats, nil
}
