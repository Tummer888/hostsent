// Package repository 提供后台 RBAC（员工-角色-权限）的数据访问。
package repository

import (
	"context"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/manager/model"
)

// RBACRepository 定义员工多角色与权限解析所需的仓储能力。
//
// 鉴权路径：admin_roles → roles → role_permissions → permissions。
// 只返回 status='active' 的角色与权限，避免被停用的配置继续放行。
type RBACRepository interface {
	// FindRoleIDsByAdminID 返回管理员绑定的角色 ID 集合。
	FindRoleIDsByAdminID(ctx context.Context, adminID uint64) ([]uint64, error)
	// FindRoleCodesByAdminID 返回管理员绑定的角色 code 集合。
	FindRoleCodesByAdminID(ctx context.Context, adminID uint64) ([]string, error)
	// FindPermissionCodesByAdminID 返回管理员经角色聚合后的权限 code 集合（DISTINCT）。
	FindPermissionCodesByAdminID(ctx context.Context, adminID uint64) ([]string, error)
	// FindAdminIDsByRoleID 返回某角色下的全部管理员 ID（用于角色变更后清缓存）。
	FindAdminIDsByRoleID(ctx context.Context, roleID uint64) ([]uint64, error)
	// ReplaceAdminRoles 覆盖式设置管理员角色（先删后插，事务内完成）。
	ReplaceAdminRoles(ctx context.Context, adminID uint64, roleIDs []uint64) error
	// HasAdminRoles 判断管理员是否已有任意角色绑定。
	HasAdminRoles(ctx context.Context, adminID uint64) (bool, error)
	// IsAdminActive 判断管理员是否处于启用状态（禁用后旧 token 立即失效）。
	IsAdminActive(ctx context.Context, adminID uint64) (bool, error)
	// FindDepartmentIDByAdminID 返回员工所属部门 ID（0 表示未归属），工单部门数据范围用。
	FindDepartmentIDByAdminID(ctx context.Context, adminID uint64) (uint64, error)
	// FindRoleCodesByIDs 按角色 ID 批量取 code。
	FindRoleCodesByIDs(ctx context.Context, roleIDs []uint64) ([]string, error)
	// FindRoleIDsByCodes 按角色 code 批量取 ID（用于兼容旧的单 role 字符串入参）。
	FindRoleIDsByCodes(ctx context.Context, codes []string) ([]uint64, error)
}

type rbacRepository struct {
	db *gorm.DB
}

// NewRBACRepository 创建 RBAC 仓储实现。
func NewRBACRepository(db *gorm.DB) RBACRepository {
	return &rbacRepository{db: db}
}

func (r *rbacRepository) FindRoleIDsByAdminID(ctx context.Context, adminID uint64) ([]uint64, error) {
	var ids []uint64
	err := r.db.WithContext(ctx).
		Table("admin_roles AS ar").
		Joins("JOIN roles r ON r.id = ar.role_id AND r.status = 'active'").
		Where("ar.admin_id = ?", adminID).
		Order("ar.role_id ASC").
		Pluck("r.id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *rbacRepository) FindRoleCodesByAdminID(ctx context.Context, adminID uint64) ([]string, error) {
	var codes []string
	err := r.db.WithContext(ctx).
		Table("admin_roles AS ar").
		Joins("JOIN roles r ON r.id = ar.role_id AND r.status = 'active'").
		Where("ar.admin_id = ?", adminID).
		Order("r.id ASC").
		Pluck("r.code", &codes).Error
	if err != nil {
		return nil, err
	}
	return codes, nil
}

func (r *rbacRepository) FindPermissionCodesByAdminID(ctx context.Context, adminID uint64) ([]string, error) {
	var codes []string
	err := r.db.WithContext(ctx).
		Table("admin_roles AS ar").
		Joins("JOIN role_permissions rp ON rp.role_id = ar.role_id").
		Joins("JOIN permissions p ON p.id = rp.permission_id AND p.status = 'active'").
		Joins("JOIN roles r ON r.id = ar.role_id AND r.status = 'active'").
		Where("ar.admin_id = ?", adminID).
		Distinct("p.code").
		Pluck("p.code", &codes).Error
	if err != nil {
		return nil, err
	}
	return codes, nil
}

func (r *rbacRepository) FindAdminIDsByRoleID(ctx context.Context, roleID uint64) ([]uint64, error) {
	var ids []uint64
	if err := r.db.WithContext(ctx).
		Model(&model.AdminRole{}).
		Where("role_id = ?", roleID).
		Pluck("admin_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *rbacRepository) ReplaceAdminRoles(ctx context.Context, adminID uint64, roleIDs []uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("admin_id = ?", adminID).Delete(&model.AdminRole{}).Error; err != nil {
			return err
		}
		if len(roleIDs) == 0 {
			return nil
		}
		rows := make([]model.AdminRole, 0, len(roleIDs))
		seen := make(map[uint64]struct{}, len(roleIDs))
		for _, roleID := range roleIDs {
			if roleID == 0 {
				continue
			}
			if _, ok := seen[roleID]; ok {
				continue
			}
			seen[roleID] = struct{}{}
			rows = append(rows, model.AdminRole{AdminID: adminID, RoleID: roleID})
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

func (r *rbacRepository) HasAdminRoles(ctx context.Context, adminID uint64) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.AdminRole{}).
		Where("admin_id = ?", adminID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *rbacRepository) IsAdminActive(ctx context.Context, adminID uint64) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.Admin{}).
		Where("id = ? AND status = ?", adminID, "active").
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// FindDepartmentIDByAdminID 返回员工所属部门 ID（未归属或员工不存在时为 0）。
// 工单部门数据范围据此判定，见 doc86 §2.4。
func (r *rbacRepository) FindDepartmentIDByAdminID(ctx context.Context, adminID uint64) (uint64, error) {
	if adminID == 0 {
		return 0, nil
	}
	var deptID uint64
	if err := r.db.WithContext(ctx).
		Model(&model.Admin{}).
		Select("COALESCE(department_id, 0)").
		Where("id = ?", adminID).
		Scan(&deptID).Error; err != nil {
		return 0, err
	}
	return deptID, nil
}

func (r *rbacRepository) FindRoleCodesByIDs(ctx context.Context, roleIDs []uint64) ([]string, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}
	var codes []string
	if err := r.db.WithContext(ctx).
		Model(&roleRow{}).
		Where("id IN ?", roleIDs).
		Order("id ASC").
		Pluck("code", &codes).Error; err != nil {
		return nil, err
	}
	return codes, nil
}

func (r *rbacRepository) FindRoleIDsByCodes(ctx context.Context, codes []string) ([]uint64, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	var ids []uint64
	if err := r.db.WithContext(ctx).
		Model(&roleRow{}).
		Where("code IN ?", codes).
		Order("id ASC").
		Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// roleRow 仅用于按列投影 roles 表，避免引入 account model 包。
type roleRow struct {
	ID   uint64 `gorm:"column:id;primaryKey"`
	Code string `gorm:"column:code"`
	// Scope 限制后台角色，避免把客户角色绑到员工身上。
	Scope string `gorm:"column:scope"`
}

func (roleRow) TableName() string { return "roles" }
