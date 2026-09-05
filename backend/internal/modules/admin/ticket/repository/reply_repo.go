package repository

import (
	"context"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/ticket/model"
)

// ReplyRepository 定义工单回复数据访问能力。
type ReplyRepository interface {
	ListByTicket(ctx context.Context, ticketID uint64) ([]model.TicketReply, error)
	Create(ctx context.Context, item *model.TicketReply) error
	FindByID(ctx context.Context, id uint64) (*model.TicketReply, error)
	// CountByTicket 工单回复总数
	CountByTicket(ctx context.Context, ticketID uint64) (int64, error)
}

type replyRepository struct {
	db *gorm.DB
}

// NewReplyRepository 创建工单回复仓储实现。
func NewReplyRepository(db *gorm.DB) ReplyRepository {
	return &replyRepository{db: db}
}

func (r *replyRepository) ListByTicket(ctx context.Context, ticketID uint64) ([]model.TicketReply, error) {
	var items []model.TicketReply
	if err := r.db.WithContext(ctx).
		Where("ticket_id = ?", ticketID).
		Order("created_at asc, id asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *replyRepository) Create(ctx context.Context, item *model.TicketReply) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *replyRepository) FindByID(ctx context.Context, id uint64) (*model.TicketReply, error) {
	var item model.TicketReply
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *replyRepository) CountByTicket(ctx context.Context, ticketID uint64) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.TicketReply{}).
		Where("ticket_id = ?", ticketID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
