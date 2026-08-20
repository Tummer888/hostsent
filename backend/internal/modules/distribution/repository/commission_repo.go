// Package repository 提供分销模块的佣金数据访问实现。
package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/distribution/dto"
	"hostsent/backend/internal/modules/distribution/model"
)

type CommissionRepository interface {
	List(ctx context.Context, query dto.CommissionListQuery) ([]model.Commission, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.Commission, error)
	Create(ctx context.Context, item *model.Commission) error
	Update(ctx context.Context, item *model.Commission) error
	Delete(ctx context.Context, id uint64) error
}

type commissionRepository struct {
	db *gorm.DB
}

// NewCommissionRepository 创建佣金仓储实现。
func NewCommissionRepository(db *gorm.DB) CommissionRepository {
	return &commissionRepository{db: db}
}

func (r *commissionRepository) List(ctx context.Context, query dto.CommissionListQuery) ([]model.Commission, int64, error) {
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

	base := r.db.WithContext(ctx).Model(&model.Commission{})
	if query.AgentID > 0 {
		base = base.Where("agent_id = ?", query.AgentID)
	}
	if status := strings.TrimSpace(query.Status); status != "" {
		base = base.Where("status = ?", status)
	}
	if commissionType := strings.TrimSpace(query.CommissionType); commissionType != "" {
		base = base.Where("commission_type = ?", commissionType)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("order_no ILIKE ? OR remark ILIKE ?", like, like)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.Commission
	if err := base.Order("id desc").Offset((page-1)*pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *commissionRepository) FindByID(ctx context.Context, id uint64) (*model.Commission, error) {
	var item model.Commission
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *commissionRepository) Create(ctx context.Context, item *model.Commission) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *commissionRepository) Update(ctx context.Context, item *model.Commission) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *commissionRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.Commission{}, id).Error
}
