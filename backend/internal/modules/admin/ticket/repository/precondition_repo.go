package repository

import (
	"context"

	"gorm.io/gorm"
)

// PreconditionChecker 提供工单提交前置条件的判定能力（S2）。
//
// 实名认证、订单/实例归属、客户角色分别属于别的模块，这里只做最小必要的
// 只读查询，避免 ticket → verification/order/instance 的模块级依赖。
type PreconditionChecker interface {
	// IsRealnameVerified 用户是否已完成实名认证。
	// 判定口径：存在 approved 的实名申请，或 users.real_name 已填写（两条线取或）。
	IsRealnameVerified(ctx context.Context, userID uint64) (bool, error)
	// OwnsOrderOrInstance 判断订单/实例是否属于该账号家族（主账号 + 子账号）。
	// 两个 ID 都为 0 时返回 false；只填其一则只校验其一。
	OwnsOrderOrInstance(ctx context.Context, userIDs []uint64, orderID, instanceID uint64) (bool, error)
	// UserRoleCodes 返回客户的用户侧角色码；无绑定角色时回落隐式默认角色 "user"。
	UserRoleCodes(ctx context.Context, userID uint64) ([]string, error)
}

type preconditionRepository struct {
	db *gorm.DB
}

// NewPreconditionRepository 创建工单前置条件查询实现。
func NewPreconditionRepository(db *gorm.DB) PreconditionChecker {
	return &preconditionRepository{db: db}
}

func (r *preconditionRepository) IsRealnameVerified(ctx context.Context, userID uint64) (bool, error) {
	if userID == 0 {
		return false, nil
	}
	var realName string
	if err := r.db.WithContext(ctx).Table("users").
		Select("COALESCE(real_name, '')").Where("id = ?", userID).
		Scan(&realName).Error; err != nil {
		return false, err
	}
	if realName != "" {
		return true, nil
	}
	var count int64
	if err := r.db.WithContext(ctx).Table("verification_applications").
		Where("user_id = ? AND status = ?", userID, "approved").
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *preconditionRepository) OwnsOrderOrInstance(ctx context.Context, userIDs []uint64, orderID, instanceID uint64) (bool, error) {
	if len(userIDs) == 0 {
		return false, nil
	}
	if orderID > 0 {
		var count int64
		if err := r.db.WithContext(ctx).Table("orders").
			Where("id = ? AND user_id IN ?", orderID, userIDs).
			Count(&count).Error; err != nil {
			return false, err
		}
		if count == 0 {
			return false, nil
		}
	}
	if instanceID > 0 {
		var count int64
		if err := r.db.WithContext(ctx).Table("instances").
			Where("id = ? AND user_id IN ?", instanceID, userIDs).
			Count(&count).Error; err != nil {
			return false, err
		}
		if count == 0 {
			return false, nil
		}
	}
	return true, nil
}

// UserRoleCodes 取客户侧角色码。role scope 为 user 的角色才计入，
// 避免把后台员工角色当成客户角色；未绑定任何角色的存量用户按隐式默认角色 "user" 处理，
// 否则默认分类配置里写 "user" 会把全部普通客户挡在门外。
func (r *preconditionRepository) UserRoleCodes(ctx context.Context, userID uint64) ([]string, error) {
	if userID == 0 {
		return nil, nil
	}
	var codes []string
	if err := r.db.WithContext(ctx).Table("user_roles").
		Joins("JOIN roles ON roles.id = user_roles.role_id AND roles.scope = ?", "user").
		Where("user_roles.user_id = ?", userID).
		Order("roles.id ASC").
		Pluck("roles.code", &codes).Error; err != nil {
		return nil, err
	}
	if len(codes) == 0 {
		return []string{"user"}, nil
	}
	return codes, nil
}
