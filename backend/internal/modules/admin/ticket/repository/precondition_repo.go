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
	// IsRealnameVerified 用户是否已完成实名认证（doc104 §5.3 的唯一口径）。
	// 判定口径：users.real_name_verified_at 非空，或存在 approved 的实名申请。
	//
	// 不得把「users.real_name 非空」算作已实名：那是展示名，用户在资料页
	// 改一次昵称就能把自己变成「已实名」（F13 后门）。
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

// IsRealnameVerified 工单侧的实名判定（doc104 §5.3）。
//
// 与 verification 模块的 IsUserVerified 保持同一口径，原因不只是「一致」：
// 工单分类按实名与否决定客户能否提交，若这里认 real_name 而核验模块不认，
// 同一用户在两个页面会得到相反的实名状态，用户会认为系统在骗他。
// 两条线取或中的第二条（approved 申请）是为历史数据兜底 —— 迁移 051 之前的
// 审核通过记录没有写 real_name_verified_at，只查信号列会把老用户判成未实名。
//
// 两条判据都用 COUNT 而不是把列扫进 *time.Time：SQL NULL 扫进指针的行为依赖
// 驱动（database/sql 会直接报 unsupported Scan），而「未实名」恰恰是最常见的
// 那条路径——用查询失败来表达「没有」是最不该有的失败模式。
func (r *preconditionRepository) IsRealnameVerified(ctx context.Context, userID uint64) (bool, error) {
	if userID == 0 {
		return false, nil
	}
	var verified int64
	if err := r.db.WithContext(ctx).Table("users").
		Where("id = ? AND real_name_verified_at IS NOT NULL", userID).
		Count(&verified).Error; err != nil {
		return false, err
	}
	if verified > 0 {
		return true, nil
	}
	var approved int64
	if err := r.db.WithContext(ctx).Table("verification_applications").
		Where("user_id = ? AND status = ?", userID, "approved").
		Count(&approved).Error; err != nil {
		return false, err
	}
	return approved > 0, nil
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
