package repository

import (
	"context"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/ticket/model"
)

// ReplyRepository 定义工单回复数据访问能力。
type ReplyRepository interface {
	ListByTicket(ctx context.Context, ticketID uint64) ([]model.TicketReply, error)
	// ListByTicketVisible 用户端可见回复（S2/S3）：排除内部备注与待复核/已驳回回复。
	ListByTicketVisible(ctx context.Context, ticketID uint64) ([]model.TicketReply, error)
	Create(ctx context.Context, item *model.TicketReply) error
	FindByID(ctx context.Context, id uint64) (*model.TicketReply, error)
	// Update 全量保存（复核结果落库）。
	Update(ctx context.Context, item *model.TicketReply) error
	// CountByTicket 工单回复总数
	CountByTicket(ctx context.Context, ticketID uint64) (int64, error)
	// —— S3 复核 ——
	// ListPendingReviews 待复核回复队列（分页）。
	ListPendingReviews(ctx context.Context, departmentIDs []uint64, page, pageSize int) ([]model.TicketReply, int64, error)
	// BindAttachments 把回复与附件关联（附件先上传后回复的场景，S2）。
	BindAttachments(ctx context.Context, replyID uint64, attachmentIDs []uint64) error
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

// ListByTicketVisible 用户端可见回复：内部备注、待复核与已驳回的一律不返回。
// 用 SQL 过滤而非服务层裁剪，避免漏改导致内部信息外泄（doc86 §3.2）。
func (r *replyRepository) ListByTicketVisible(ctx context.Context, ticketID uint64) ([]model.TicketReply, error) {
	var items []model.TicketReply
	if err := r.db.WithContext(ctx).
		Where("ticket_id = ?", ticketID).
		Where("is_internal = ?", false).
		Where("review_status NOT IN ?", []string{model.ReviewStatusPending, model.ReviewStatusRejected}).
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

func (r *replyRepository) Update(ctx context.Context, item *model.TicketReply) error {
	return r.db.WithContext(ctx).Save(item).Error
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

// ListPendingReviews 待复核队列：按提交时间正序（先到先审）。
// departmentIDs 非空时仅返回这些部门的工单（主管数据范围），空表示全量（超管）。
func (r *replyRepository) ListPendingReviews(ctx context.Context, departmentIDs []uint64, page, pageSize int) ([]model.TicketReply, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	base := r.db.WithContext(ctx).Model(&model.TicketReply{}).
		Joins("JOIN tickets t ON t.id = ticket_replies.ticket_id").
		Where("ticket_replies.review_status = ?", model.ReviewStatusPending)
	if len(departmentIDs) > 0 {
		base = base.Where("t.department_id IN ?", departmentIDs)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.TicketReply
	if err := base.Select("ticket_replies.*").
		Order("ticket_replies.created_at asc, ticket_replies.id asc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// BindAttachments 把已上传的附件挂到回复上：仅绑定 reply_id=0 的附件，
// 避免把别的回复的附件抢过来。
func (r *replyRepository) BindAttachments(ctx context.Context, replyID uint64, attachmentIDs []uint64) error {
	if len(attachmentIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&model.TicketAttachment{}).
		Where("id IN ? AND reply_id = 0", attachmentIDs).
		Update("reply_id", replyID).Error
}
