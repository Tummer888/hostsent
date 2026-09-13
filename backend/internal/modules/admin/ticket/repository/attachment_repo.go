package repository

import (
	"context"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/ticket/model"
)

// AttachmentRepository 工单附件数据访问（S2）。
type AttachmentRepository interface {
	Create(ctx context.Context, item *model.TicketAttachment) error
	FindByID(ctx context.Context, id uint64) (*model.TicketAttachment, error)
	// ListByTicket 工单下全部附件（含内部附件，调用方按视角过滤）。
	ListByTicket(ctx context.Context, ticketID uint64) ([]model.TicketAttachment, error)
	// ListByTicketVisible 用户端可见附件：排除内部附件，且排除待复核/已驳回回复的附件。
	ListByTicketVisible(ctx context.Context, ticketID uint64) ([]model.TicketAttachment, error)
	// ListByIDs 批量取附件（回复挂载时校验归属）。
	ListByIDs(ctx context.Context, ids []uint64) ([]model.TicketAttachment, error)
	// BindTicket 把未挂载（ticket_id=0）的附件关联到工单。
	BindTicket(ctx context.Context, ticketID uint64, ids []uint64) error
	// DeleteByIDs 删除附件记录（撤回上传时用）。
	DeleteByIDs(ctx context.Context, ids []uint64) error
}

type attachmentRepository struct {
	db *gorm.DB
}

// NewAttachmentRepository 创建附件仓储实现。
func NewAttachmentRepository(db *gorm.DB) AttachmentRepository {
	return &attachmentRepository{db: db}
}

func (r *attachmentRepository) Create(ctx context.Context, item *model.TicketAttachment) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *attachmentRepository) FindByID(ctx context.Context, id uint64) (*model.TicketAttachment, error) {
	var item model.TicketAttachment
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *attachmentRepository) ListByTicket(ctx context.Context, ticketID uint64) ([]model.TicketAttachment, error) {
	var items []model.TicketAttachment
	if err := r.db.WithContext(ctx).
		Where("ticket_id = ?", ticketID).
		Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListByTicketVisible 用户端可见附件：内部附件不可见；
// 挂在待复核/已驳回回复上的附件同样不可见（否则等于绕过回复级复核泄露内容）。
func (r *attachmentRepository) ListByTicketVisible(ctx context.Context, ticketID uint64) ([]model.TicketAttachment, error) {
	var items []model.TicketAttachment
	if err := r.db.WithContext(ctx).
		Where("ticket_id = ? AND is_internal = ?", ticketID, false).
		Where("reply_id = 0 OR reply_id NOT IN (?)",
			r.db.Model(&model.TicketReply{}).
				Select("id").
				Where("ticket_id = ? AND review_status IN ?", ticketID, []string{model.ReviewStatusPending, model.ReviewStatusRejected}),
		).
		Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *attachmentRepository) ListByIDs(ctx context.Context, ids []uint64) ([]model.TicketAttachment, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var items []model.TicketAttachment
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *attachmentRepository) DeleteByIDs(ctx context.Context, ids []uint64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("id IN ?", ids).Delete(&model.TicketAttachment{}).Error
}

// BindTicket 把未挂载附件关联到工单：只动 ticket_id=0 的行，
// 避免把已属其它工单的附件搬走。
func (r *attachmentRepository) BindTicket(ctx context.Context, ticketID uint64, ids []uint64) error {
	if len(ids) == 0 || ticketID == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&model.TicketAttachment{}).
		Where("id IN ? AND ticket_id = 0", ids).
		Update("ticket_id", ticketID).Error
}
