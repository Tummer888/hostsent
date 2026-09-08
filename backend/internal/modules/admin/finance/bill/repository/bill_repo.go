package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hostsent/backend/internal/modules/admin/finance/bill/dto"
	"hostsent/backend/internal/modules/admin/finance/bill/model"
)

// BillRepository 账单数据访问。
type BillRepository interface {
	Upsert(ctx context.Context, b *model.Bill) error
	FindByID(ctx context.Context, id uint64) (*model.Bill, error)
	FindByUserPeriod(ctx context.Context, userID uint64, period string) (*model.Bill, error)
	List(ctx context.Context, q dto.BillListQuery) ([]model.Bill, int64, error)
	Close(ctx context.Context, id uint64) error
}

type billRepository struct {
	db *gorm.DB
}

// NewBillRepository 创建账单仓储。
func NewBillRepository(db *gorm.DB) BillRepository {
	return &billRepository{db: db}
}

// Upsert 依据 (user_id, period) 唯一约束幂等写入账单。
func (r *billRepository) Upsert(ctx context.Context, b *model.Bill) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "period"}},
		DoUpdates: clause.AssignmentColumns([]string{"total_amount", "refund_amount", "status", "detail", "updated_at"}),
	}).Create(b).Error
}

func (r *billRepository) FindByID(ctx context.Context, id uint64) (*model.Bill, error) {
	var b model.Bill
	if err := r.db.WithContext(ctx).First(&b, id).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *billRepository) FindByUserPeriod(ctx context.Context, userID uint64, period string) (*model.Bill, error) {
	var b model.Bill
	if err := r.db.WithContext(ctx).Where("user_id = ? AND period = ?", userID, period).First(&b).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *billRepository) List(ctx context.Context, q dto.BillListQuery) ([]model.Bill, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.Bill{})
	if q.UserID > 0 {
		base = base.Where("user_id = ?", q.UserID)
	}
	if q.Period != "" {
		base = base.Where("period = ?", q.Period)
	}
	if q.Status != "" {
		base = base.Where("status = ?", q.Status)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.Bill
	err := base.Order("period desc, id desc").
		Offset((normalizePage(q.Page) - 1) * normalizePageSize(q.PageSize)).
		Limit(normalizePageSize(q.PageSize)).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *billRepository) Close(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&model.Bill{}).Where("id = ?", id).Update("status", model.BillStatusClosed).Error
}
