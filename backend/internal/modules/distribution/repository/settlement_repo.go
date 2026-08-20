package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/distribution/dto"
	"hostsent/backend/internal/modules/distribution/model"
)

type SettlementRepository interface {
	List(ctx context.Context, query dto.SettlementListQuery) ([]model.Settlement, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.Settlement, error)
	FindBySettlementNo(ctx context.Context, settlementNo string) (*model.Settlement, error)
	Create(ctx context.Context, item *model.Settlement) error
	Update(ctx context.Context, item *model.Settlement) error
	Delete(ctx context.Context, id uint64) error
}

type settlementRepository struct {
	db *gorm.DB
}

// NewSettlementRepository 创建结算单仓储实现。
func NewSettlementRepository(db *gorm.DB) SettlementRepository {
	return &settlementRepository{db: db}
}

func (r *settlementRepository) List(ctx context.Context, query dto.SettlementListQuery) ([]model.Settlement, int64, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	base := r.db.WithContext(ctx).Model(&model.Settlement{})
	if query.AgentID > 0 {
		base = base.Where("agent_id = ?", query.AgentID)
	}
	if status := strings.TrimSpace(query.Status); status != "" {
		base = base.Where("status = ?", status)
	}
	if !query.StartDate.IsZero() {
		base = base.Where("period_start >= ?", beginningOfDay(query.StartDate))
	}
	if !query.EndDate.IsZero() {
		base = base.Where("period_end <= ?", endOfDay(query.EndDate))
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("settlement_no ILIKE ? OR remark ILIKE ?", like, like)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.Settlement
	if err := base.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *settlementRepository) FindByID(ctx context.Context, id uint64) (*model.Settlement, error) {
	var item model.Settlement
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *settlementRepository) FindBySettlementNo(ctx context.Context, settlementNo string) (*model.Settlement, error) {
	var item model.Settlement
	if err := r.db.WithContext(ctx).Where("settlement_no = ?", settlementNo).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *settlementRepository) Create(ctx context.Context, item *model.Settlement) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *settlementRepository) Update(ctx context.Context, item *model.Settlement) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *settlementRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.Settlement{}, id).Error
}

func beginningOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func endOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, t.Location())
}
