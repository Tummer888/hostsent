// Package repository 提供折扣策略（price policy）子域的数据访问实现。
package repository

import (
	"context"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/product/discount/dto"
	"hostsent/backend/internal/modules/admin/product/discount/model"
)

// PricePolicyRepository 定义折扣策略数据访问能力。
type PricePolicyRepository interface {
	List(ctx context.Context, query dto.PolicyQuery) ([]model.PricePolicy, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.PricePolicy, error)
	Create(ctx context.Context, policy *model.PricePolicy) error
	Update(ctx context.Context, policy *model.PricePolicy) error
	Delete(ctx context.Context, id uint64) error
	Items(ctx context.Context, policyID uint64) ([]model.PricePolicyItem, error)
	ReplaceItems(ctx context.Context, policyID uint64, items []model.PricePolicyItem) error
	// GroupPolicyIDForUser 返回用户所属用户组绑定的折扣策略 ID（0 表示未绑定）。
	GroupPolicyIDForUser(ctx context.Context, userID uint64) (uint64, error)
	// AgentPolicyIDForUser 返回用户所属代理等级绑定的折扣策略 ID（0 表示非代理/未绑定，P6-01）。
	AgentPolicyIDForUser(ctx context.Context, userID uint64) (uint64, error)
}

type pricePolicyRepository struct {
	db *gorm.DB
}

// NewPricePolicyRepository 创建折扣策略仓储。
func NewPricePolicyRepository(db *gorm.DB) PricePolicyRepository {
	return &pricePolicyRepository{db: db}
}

func (r *pricePolicyRepository) List(ctx context.Context, query dto.PolicyQuery) ([]model.PricePolicy, int64, error) {
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
	base := r.db.WithContext(ctx).Model(&model.PricePolicy{})
	if query.Status != "" {
		base = base.Where("status = ?", query.Status)
	}
	if query.Scope != "" {
		base = base.Where("scope = ?", query.Scope)
	}
	if query.Keyword != "" {
		like := "%" + query.Keyword + "%"
		base = base.Where("name ILIKE ? OR code ILIKE ?", like, like)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.PricePolicy
	if err := base.Order("priority desc, id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *pricePolicyRepository) FindByID(ctx context.Context, id uint64) (*model.PricePolicy, error) {
	var item model.PricePolicy
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *pricePolicyRepository) Create(ctx context.Context, policy *model.PricePolicy) error {
	return r.db.WithContext(ctx).Create(policy).Error
}

func (r *pricePolicyRepository) Update(ctx context.Context, policy *model.PricePolicy) error {
	return r.db.WithContext(ctx).Save(policy).Error
}

func (r *pricePolicyRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("policy_id = ?", id).Delete(&model.PricePolicyItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.PricePolicy{}, id).Error
	})
}

func (r *pricePolicyRepository) Items(ctx context.Context, policyID uint64) ([]model.PricePolicyItem, error) {
	var items []model.PricePolicyItem
	if err := r.db.WithContext(ctx).Where("policy_id = ?", policyID).Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ReplaceItems 覆盖式写入策略条目（先删后插，事务内保证一致）。
func (r *pricePolicyRepository) ReplaceItems(ctx context.Context, policyID uint64, items []model.PricePolicyItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("policy_id = ?", policyID).Delete(&model.PricePolicyItem{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		return tx.Create(&items).Error
	})
}

// GroupPolicyIDForUser 通过 users.user_group_id 反查用户组绑定的折扣策略。
func (r *pricePolicyRepository) GroupPolicyIDForUser(ctx context.Context, userID uint64) (uint64, error) {
	var policyID *uint64
	err := r.db.WithContext(ctx).
		Table("users").
		Select("user_groups.price_policy_id").
		Joins("JOIN user_groups ON user_groups.id = users.user_group_id").
		Where("users.id = ?", userID).
		Limit(1).
		Scan(&policyID).Error
	if err != nil {
		return 0, err
	}
	if policyID == nil {
		return 0, nil
	}
	return *policyID, nil
}

// AgentPolicyIDForUser 通过 distribution_agents + agent_levels 反查代理价格策略（P6-01）。
// 仅启用状态的代理与等级参与算价；非代理用户返回 0。
func (r *pricePolicyRepository) AgentPolicyIDForUser(ctx context.Context, userID uint64) (uint64, error) {
	var policyID *uint64
	err := r.db.WithContext(ctx).
		Table("distribution_agents").
		Select("agent_levels.price_policy_id").
		Joins("JOIN agent_levels ON agent_levels.id = distribution_agents.agent_level_id").
		Where("distribution_agents.user_id = ?", userID).
		Where("distribution_agents.status = ?", "active").
		Where("agent_levels.status = ?", "active").
		Limit(1).
		Scan(&policyID).Error
	if err != nil {
		return 0, err
	}
	if policyID == nil {
		return 0, nil
	}
	return *policyID, nil
}
