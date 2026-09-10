// Package repository 提供分销模块的佣金数据访问实现。
package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hostsent/backend/internal/modules/admin/user/distribution/model"
)

// AccrualRepository 提供佣金自动计提所需的只读查询与幂等写入（P6-02）。
type AccrualRepository interface {
	// AgentByUserID 按用户查代理；非代理返回 gorm.ErrRecordNotFound。
	AgentByUserID(ctx context.Context, userID uint64) (*model.Agent, error)
	// AgentByID 按代理 ID 查代理。
	AgentByID(ctx context.Context, id uint64) (*model.Agent, error)
	// SubordinateByUserID 查该用户作为下级的分销关系；无关系返回 gorm.ErrRecordNotFound。
	SubordinateByUserID(ctx context.Context, userID uint64) (*model.Subordinate, error)
	// AgentLevelByID 查代理等级。
	AgentLevelByID(ctx context.Context, id uint64) (*model.AgentLevel, error)
	// CreateCommissionIfAbsent 幂等插入佣金：已存在 (order_id, agent_id) 时返回 false 且不报错。
	CreateCommissionIfAbsent(ctx context.Context, item *model.Commission) (bool, error)
}

type accrualRepository struct {
	db *gorm.DB
}

// NewAccrualRepository 创建佣金计提仓储。
func NewAccrualRepository(db *gorm.DB) AccrualRepository {
	return &accrualRepository{db: db}
}

func (r *accrualRepository) AgentByUserID(ctx context.Context, userID uint64) (*model.Agent, error) {
	var agent model.Agent
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&agent).Error; err != nil {
		return nil, err
	}
	return &agent, nil
}

func (r *accrualRepository) AgentByID(ctx context.Context, id uint64) (*model.Agent, error) {
	var agent model.Agent
	if err := r.db.WithContext(ctx).First(&agent, id).Error; err != nil {
		return nil, err
	}
	return &agent, nil
}

func (r *accrualRepository) SubordinateByUserID(ctx context.Context, userID uint64) (*model.Subordinate, error) {
	var sub model.Subordinate
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("id asc").
		First(&sub).Error; err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *accrualRepository) AgentLevelByID(ctx context.Context, id uint64) (*model.AgentLevel, error) {
	var level model.AgentLevel
	if err := r.db.WithContext(ctx).First(&level, id).Error; err != nil {
		return nil, err
	}
	return &level, nil
}

func (r *accrualRepository) CreateCommissionIfAbsent(ctx context.Context, item *model.Commission) (bool, error) {
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(item)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}
