package repository

import (
	"context"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/finance/dto"
	"hostsent/backend/internal/modules/admin/finance/model"
)

// RechargeRepository 充值单数据访问。
type RechargeRepository interface {
	Create(ctx context.Context, rc *model.Recharge) error
	Update(ctx context.Context, rc *model.Recharge) error
	FindByID(ctx context.Context, id uint64) (*model.Recharge, error)
	FindByNo(ctx context.Context, rechargeNo string) (*model.Recharge, error)
	List(ctx context.Context, q dto.RechargeListQuery) ([]model.Recharge, int64, error)
}

type rechargeRepository struct {
	db *gorm.DB
}

// NewRechargeRepository 创建充值单仓储。
func NewRechargeRepository(db *gorm.DB) RechargeRepository {
	return &rechargeRepository{db: db}
}

func (r *rechargeRepository) Create(ctx context.Context, rc *model.Recharge) error {
	return r.db.WithContext(ctx).Create(rc).Error
}

func (r *rechargeRepository) Update(ctx context.Context, rc *model.Recharge) error {
	return r.db.WithContext(ctx).Save(rc).Error
}

func (r *rechargeRepository) FindByID(ctx context.Context, id uint64) (*model.Recharge, error) {
	var rc model.Recharge
	if err := r.db.WithContext(ctx).First(&rc, id).Error; err != nil {
		return nil, err
	}
	return &rc, nil
}

func (r *rechargeRepository) FindByNo(ctx context.Context, rechargeNo string) (*model.Recharge, error) {
	var rc model.Recharge
	if err := r.db.WithContext(ctx).Where("recharge_no = ?", rechargeNo).First(&rc).Error; err != nil {
		return nil, err
	}
	return &rc, nil
}

func (r *rechargeRepository) List(ctx context.Context, q dto.RechargeListQuery) ([]model.Recharge, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.Recharge{})
	if q.UserID > 0 {
		base = base.Where("user_id = ?", q.UserID)
	}
	if q.Status != "" {
		base = base.Where("status = ?", q.Status)
	}
	if q.Method != "" {
		base = base.Where("method = ?", q.Method)
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
	var items []model.Recharge
	err := base.Order("id desc").
		Offset((normalizePage(q.Page) - 1) * normalizePageSize(q.PageSize)).
		Limit(normalizePageSize(q.PageSize)).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
