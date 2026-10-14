package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/order/model"
)

// RefundPolicyRepository 退款策略数据访问。
type RefundPolicyRepository interface {
	List(ctx context.Context) ([]model.RefundPolicy, error)
	FindByID(ctx context.Context, id uint64) (*model.RefundPolicy, error)
	Create(ctx context.Context, item *model.RefundPolicy) error
	Update(ctx context.Context, item *model.RefundPolicy) error
	Delete(ctx context.Context, id uint64) error
}

type refundPolicyRepository struct {
	db *gorm.DB
}

// NewRefundPolicyRepository 创建退款策略仓储。
func NewRefundPolicyRepository(db *gorm.DB) RefundPolicyRepository {
	return &refundPolicyRepository{db: db}
}

func (r *refundPolicyRepository) List(ctx context.Context) ([]model.RefundPolicy, error) {
	var items []model.RefundPolicy
	if err := r.db.WithContext(ctx).
		Order("sort_order asc, id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *refundPolicyRepository) FindByID(ctx context.Context, id uint64) (*model.RefundPolicy, error) {
	var item model.RefundPolicy
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *refundPolicyRepository) Create(ctx context.Context, item *model.RefundPolicy) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *refundPolicyRepository) Update(ctx context.Context, item *model.RefundPolicy) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *refundPolicyRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.RefundPolicy{}, id).Error
}

// RefundAttachmentRepository 退款凭据附件数据访问。
type RefundAttachmentRepository interface {
	ListByRefund(ctx context.Context, refundID uint64) ([]model.RefundAttachment, error)
	FindByID(ctx context.Context, id uint64) (*model.RefundAttachment, error)
	Create(ctx context.Context, item *model.RefundAttachment) error
}

type refundAttachmentRepository struct {
	db *gorm.DB
}

// NewRefundAttachmentRepository 创建退款凭据附件仓储。
func NewRefundAttachmentRepository(db *gorm.DB) RefundAttachmentRepository {
	return &refundAttachmentRepository{db: db}
}

func (r *refundAttachmentRepository) ListByRefund(ctx context.Context, refundID uint64) ([]model.RefundAttachment, error) {
	var items []model.RefundAttachment
	if err := r.db.WithContext(ctx).
		Where("refund_id = ?", refundID).
		Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *refundAttachmentRepository) FindByID(ctx context.Context, id uint64) (*model.RefundAttachment, error) {
	var item model.RefundAttachment
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *refundAttachmentRepository) Create(ctx context.Context, item *model.RefundAttachment) error {
	return r.db.WithContext(ctx).Create(item).Error
}

// SetRefundActivatedAtIfEmpty 补写订单开通成功时间（仅当为空时）。
//
// activated_at 的写入散布在三条链路（上游适配器置服务中 / 上游推送推进 / 人工重开），
// 用条件更新收口，避免竞态下互相覆盖。
func (r *orderRepository) SetActivatedAtIfEmpty(ctx context.Context, id uint64, at time.Time) error {
	return r.db.WithContext(ctx).Model(&model.Order{}).
		Where("id = ? AND activated_at IS NULL", id).
		Update("activated_at", at).Error
}
