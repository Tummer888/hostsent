// Package repository 提供生命周期模块的数据访问。
package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	lifecyclemodel "hostsent/backend/internal/modules/admin/lifecycle/model"
)

// PolicyRepository 生命周期策略仓库（单行表，ID=1）。
type PolicyRepository interface {
	Get(ctx context.Context) (*lifecyclemodel.LifecyclePolicy, error)
	Update(ctx context.Context, policy *lifecyclemodel.LifecyclePolicy) error
}

type policyRepository struct {
	db *gorm.DB
}

// NewPolicyRepository 创建策略仓库。
func NewPolicyRepository(db *gorm.DB) PolicyRepository {
	return &policyRepository{db: db}
}

func (r *policyRepository) Get(ctx context.Context) (*lifecyclemodel.LifecyclePolicy, error) {
	var policy lifecyclemodel.LifecyclePolicy
	if err := r.db.WithContext(ctx).First(&policy, 1).Error; err != nil {
		return nil, err
	}
	return &policy, nil
}

func (r *policyRepository) Update(ctx context.Context, policy *lifecyclemodel.LifecyclePolicy) error {
	return r.db.WithContext(ctx).Save(policy).Error
}

// ensureDefaultPolicy 读取默认策略；不存在时注入 seed 默认行（ID=1）。
func ensureDefaultPolicy(tx *gorm.DB) (*lifecyclemodel.LifecyclePolicy, error) {
	var policy lifecyclemodel.LifecyclePolicy
	err := tx.First(&policy, 1).Error
	if err == nil {
		return &policy, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	policy = lifecyclemodel.LifecyclePolicy{
		ID:              1,
		RemindDays:      "7,3,1",
		GraceDays:       7,
		DestroyKeepDays: 30,
		Status:          "active",
	}
	if err := tx.Create(&policy).Error; err != nil {
		return nil, err
	}
	return &policy, nil
}
