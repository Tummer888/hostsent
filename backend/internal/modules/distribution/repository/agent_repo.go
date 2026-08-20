package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/distribution/dto"
	"hostsent/backend/internal/modules/distribution/model"
)

type AgentRepository interface {
	List(ctx context.Context, query dto.AgentListQuery) ([]model.Agent, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.Agent, error)
	Create(ctx context.Context, item *model.Agent) error
	Update(ctx context.Context, item *model.Agent) error
	Delete(ctx context.Context, id uint64) error
}

type agentRepository struct {
	db *gorm.DB
}

// NewAgentRepository 创建代理仓储实现。
func NewAgentRepository(db *gorm.DB) AgentRepository {
	return &agentRepository{db: db}
}

func (r *agentRepository) List(ctx context.Context, query dto.AgentListQuery) ([]model.Agent, int64, error) {
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

	base := r.db.WithContext(ctx).Model(&model.Agent{}).
		Joins("LEFT JOIN users ON users.id = distribution_agents.user_id").
		Joins("LEFT JOIN agent_levels ON agent_levels.id = distribution_agents.agent_level_id")
	if status := strings.TrimSpace(query.Status); status != "" {
		base = base.Where("distribution_agents.status = ?", status)
	}
	if query.AgentLevelID > 0 {
		base = base.Where("distribution_agents.agent_level_id = ?", query.AgentLevelID)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("users.username ILIKE ? OR users.real_name ILIKE ? OR users.email ILIKE ? OR distribution_agents.invite_code ILIKE ? OR agent_levels.name ILIKE ?", like, like, like, like, like)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.Agent
	if err := base.Order("distribution_agents.id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *agentRepository) FindByID(ctx context.Context, id uint64) (*model.Agent, error) {
	var item model.Agent
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *agentRepository) Create(ctx context.Context, item *model.Agent) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *agentRepository) Update(ctx context.Context, item *model.Agent) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *agentRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.Agent{}, id).Error
}
