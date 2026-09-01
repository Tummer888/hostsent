package repository

import (
	"context"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/finance/dto"
	"hostsent/backend/internal/modules/admin/finance/model"
)

// WithdrawRepository 提现单数据访问。
type WithdrawRepository interface {
	Create(ctx context.Context, w *model.Withdraw) error
	Update(ctx context.Context, w *model.Withdraw) error
	FindByID(ctx context.Context, id uint64) (*model.Withdraw, error)
	List(ctx context.Context, q dto.WithdrawListQuery) ([]model.Withdraw, int64, error)
}

type withdrawRepository struct {
	db *gorm.DB
}

// NewWithdrawRepository 创建提现单仓储。
func NewWithdrawRepository(db *gorm.DB) WithdrawRepository {
	return &withdrawRepository{db: db}
}

func (r *withdrawRepository) Create(ctx context.Context, w *model.Withdraw) error {
	return r.db.WithContext(ctx).Create(w).Error
}

func (r *withdrawRepository) Update(ctx context.Context, w *model.Withdraw) error {
	return r.db.WithContext(ctx).Save(w).Error
}

func (r *withdrawRepository) FindByID(ctx context.Context, id uint64) (*model.Withdraw, error) {
	var w model.Withdraw
	if err := r.db.WithContext(ctx).First(&w, id).Error; err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *withdrawRepository) List(ctx context.Context, q dto.WithdrawListQuery) ([]model.Withdraw, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.Withdraw{})
	if q.UserID > 0 {
		base = base.Where("user_id = ?", q.UserID)
	}
	if q.Status != "" {
		base = base.Where("status = ?", q.Status)
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
	var items []model.Withdraw
	err := base.Order("id desc").
		Offset((normalizePage(q.Page) - 1) * normalizePageSize(q.PageSize)).
		Limit(normalizePageSize(q.PageSize)).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
