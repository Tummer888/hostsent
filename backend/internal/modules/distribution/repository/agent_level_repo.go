package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/distribution/dto"
	"hostsent/backend/internal/modules/distribution/model"
)

type AgentLevelRepository interface {
	List(ctx context.Context, query dto.AgentLevelListQuery) ([]model.AgentLevel, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.AgentLevel, error)
	Create(ctx context.Context, item *model.AgentLevel) error
	Update(ctx context.Context, item *model.AgentLevel) error
	Delete(ctx context.Context, id uint64) error
}

type agentLevelRepository struct {
	db *gorm.DB
}

func NewAgentLevelRepository(db *gorm.DB) AgentLevelRepository {
	return &agentLevelRepository{db: db}
}

func (r *agentLevelRepository) List(ctx context.Context, query dto.AgentLevelListQuery) ([]model.AgentLevel, int64, error) {
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

	base := r.db.WithContext(ctx).Model(&model.AgentLevel{})
	if status := strings.TrimSpace(query.Status); status != "" {
		base = base.Where("status = ?", status)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("name ILIKE ? OR code ILIKE ? OR description ILIKE ?", like, like, like)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.AgentLevel
	if err := base.Order("weight desc, id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *agentLevelRepository) FindByID(ctx context.Context, id uint64) (*model.AgentLevel, error) {
	var item model.AgentLevel
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *agentLevelRepository) Create(ctx context.Context, item *model.AgentLevel) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *agentLevelRepository) Update(ctx context.Context, item *model.AgentLevel) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *agentLevelRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.AgentLevel{}, id).Error
}
