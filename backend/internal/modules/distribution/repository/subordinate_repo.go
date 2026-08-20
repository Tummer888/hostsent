package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/distribution/dto"
	"hostsent/backend/internal/modules/distribution/model"
)

type SubordinateRepository interface {
	List(ctx context.Context, query dto.SubordinateListQuery) ([]model.Subordinate, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.Subordinate, error)
	Create(ctx context.Context, item *model.Subordinate) error
	Update(ctx context.Context, item *model.Subordinate) error
	Delete(ctx context.Context, id uint64) error
}

type subordinateRepository struct {
	db *gorm.DB
}

func NewSubordinateRepository(db *gorm.DB) SubordinateRepository {
	return &subordinateRepository{db: db}
}

func (r *subordinateRepository) List(ctx context.Context, query dto.SubordinateListQuery) ([]model.Subordinate, int64, error) {
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

	base := r.db.WithContext(ctx).Model(&model.Subordinate{}).
		Joins("LEFT JOIN users ON users.id = distribution_subordinates.user_id").
		Joins("LEFT JOIN distribution_agents agent ON agent.id = distribution_subordinates.agent_id")
	if query.AgentID > 0 {
		base = base.Where("distribution_subordinates.agent_id = ?", query.AgentID)
	}
	if status := strings.TrimSpace(query.Status); status != "" {
		base = base.Where("distribution_subordinates.status = ?", status)
	}
	if query.LevelDepth > 0 {
		base = base.Where("distribution_subordinates.level_depth = ?", query.LevelDepth)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("users.username ILIKE ? OR users.real_name ILIKE ? OR users.phone ILIKE ? OR agent.invite_code ILIKE ?", like, like, like, like)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.Subordinate
	if err := base.Order("distribution_subordinates.id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *subordinateRepository) FindByID(ctx context.Context, id uint64) (*model.Subordinate, error) {
	var item model.Subordinate
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *subordinateRepository) Create(ctx context.Context, item *model.Subordinate) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *subordinateRepository) Update(ctx context.Context, item *model.Subordinate) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *subordinateRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.Subordinate{}, id).Error
}
