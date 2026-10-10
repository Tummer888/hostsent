// Package repository 提供生命周期模块的数据访问。
package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	lifecyclemodel "hostsent/backend/internal/modules/admin/instance/lifecycle/model"
)

// PolicyRepository 生命周期策略仓库（单行表，ID=1）。
type PolicyRepository interface {
	Get(ctx context.Context) (*lifecyclemodel.LifecyclePolicy, error)
	Update(ctx context.Context, policy *lifecyclemodel.LifecyclePolicy) error
	// EnsureDefault 确保策略单行存在并补齐新增列的默认值（AutoEnforce=false / EnforceDryRun=true）。
	EnsureDefault(ctx context.Context) (*lifecyclemodel.LifecyclePolicy, error)
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

// EnsureDefault 确保策略单行存在；存在时补齐新增闸门列的默认值（AutoEnforce 保持现状不变，
// EnforceDryRun 缺失时置 true），保证存量行升级后语义为「不自动执行 + 预演」。
func (r *policyRepository) EnsureDefault(ctx context.Context) (*lifecyclemodel.LifecyclePolicy, error) {
	return ensureDefaultPolicy(r.db.WithContext(ctx))
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
		AutoEnforce:     false,
		EnforceDryRun:   true,
		RefundAction:    lifecyclemodel.RefundActionNone,
		Status:          "active",
	}
	if err := tx.Create(&policy).Error; err != nil {
		return nil, err
	}
	return &policy, nil
}
