package repository

import (
	"context"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/payment/dto"
	"hostsent/backend/internal/modules/admin/payment/model"
)

// PayoutRepository 打款单数据访问。
type PayoutRepository interface {
	Create(ctx context.Context, p *model.PaymentPayout) error
	Update(ctx context.Context, p *model.PaymentPayout) error
	FindByID(ctx context.Context, id uint64) (*model.PaymentPayout, error)
	FindByNo(ctx context.Context, payoutNo string) (*model.PaymentPayout, error)
	// FindByWithdrawID 按业务域 + 提现单 ID 查打款单（幂等键，doc86 §2.5）。
	FindByWithdrawID(ctx context.Context, bizType string, withdrawID uint64) (*model.PaymentPayout, error)
	List(ctx context.Context, q dto.PayoutListQuery) ([]model.PaymentPayout, int64, error)
}

type payoutRepository struct {
	db *gorm.DB
}

// NewPayoutRepository 创建打款单仓储。
func NewPayoutRepository(db *gorm.DB) PayoutRepository {
	return &payoutRepository{db: db}
}

func (r *payoutRepository) Create(ctx context.Context, p *model.PaymentPayout) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *payoutRepository) Update(ctx context.Context, p *model.PaymentPayout) error {
	return r.db.WithContext(ctx).Save(p).Error
}

func (r *payoutRepository) FindByID(ctx context.Context, id uint64) (*model.PaymentPayout, error) {
	var p model.PaymentPayout
	if err := r.db.WithContext(ctx).First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *payoutRepository) FindByNo(ctx context.Context, payoutNo string) (*model.PaymentPayout, error) {
	var p model.PaymentPayout
	if err := r.db.WithContext(ctx).Where("payout_no = ?", payoutNo).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// FindByWithdrawID 按业务域 + 提现单 ID 查最新打款单；bizType 为空时按财务提现域兜底（存量语义）。
func (r *payoutRepository) FindByWithdrawID(ctx context.Context, bizType string, withdrawID uint64) (*model.PaymentPayout, error) {
	if bizType == "" {
		bizType = model.PayoutBizWithdraw
	}
	var p model.PaymentPayout
	if err := r.db.WithContext(ctx).
		Where("biz_type = ? AND withdraw_id = ?", bizType, withdrawID).
		Order("id desc").First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *payoutRepository) List(ctx context.Context, q dto.PayoutListQuery) ([]model.PaymentPayout, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.PaymentPayout{})
	if q.BizType != "" {
		base = base.Where("biz_type = ?", q.BizType)
	}
	if q.UserID > 0 {
		base = base.Where("user_id = ?", q.UserID)
	}
	if q.Status != "" {
		base = base.Where("status = ?", q.Status)
	}
	if q.Mode != "" {
		base = base.Where("mode = ?", q.Mode)
	}
	if start := normalizeTime(q.StartTime); start != nil {
		base = base.Where("created_at >= ?", *start)
	}
	if end := normalizeTime(q.EndTime); end != nil {
		base = base.Where("created_at <= ?", *end)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.PaymentPayout
	err := base.Order("id desc").
		Offset((normalizePage(q.Page) - 1) * normalizePageSize(q.PageSize)).
		Limit(normalizePageSize(q.PageSize)).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
