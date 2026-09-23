package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/account/model"
)

type RoleRepository interface {
	// List 按作用域列出角色；scope 为空时只返回后台角色（scope=admin），
	// 客户角色不进后台权限树（doc81 §4.1）。传 RoleScopeUser 可单独取客户角色。
	List(ctx context.Context, scope string) ([]model.Role, error)
	Create(ctx context.Context, role *model.Role) error
	FindByID(ctx context.Context, id uint64) (*model.Role, error)
	// FindByIDs 按 ID 批量取角色（含 scope），供跨域校验用。
	FindByIDs(ctx context.Context, ids []uint64) ([]model.Role, error)
	Update(ctx context.Context, role *model.Role) error
	Delete(ctx context.Context, id uint64) error
	GetPermissionIDs(ctx context.Context, roleID uint64) ([]uint64, error)
	UpsertPermissions(ctx context.Context, roleID uint64, permissionIDs []uint64) error
}

type roleRepository struct {
	db *gorm.DB
}

func NewRoleRepository(db *gorm.DB) RoleRepository {
	return &roleRepository{db: db}
}

func (r *roleRepository) List(ctx context.Context, scope string) ([]model.Role, error) {
	var roles []model.Role
	q := r.db.WithContext(ctx).Order("id desc")
	// 空 scope 与 admin 等价：老前端不带参数，行为必须与改造前一致（只出后台角色）。
	if scope == "" || scope == model.RoleScopeAdmin {
		q = q.Where("scope <> ?", model.RoleScopeUser)
	} else {
		q = q.Where("scope = ?", scope)
	}
	if err := q.Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *roleRepository) Create(ctx context.Context, role *model.Role) error {
	return r.db.WithContext(ctx).Create(role).Error
}

func (r *roleRepository) FindByID(ctx context.Context, id uint64) (*model.Role, error) {
	var role model.Role
	if err := r.db.WithContext(ctx).First(&role, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &role, nil
}

func (r *roleRepository) FindByIDs(ctx context.Context, ids []uint64) ([]model.Role, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var roles []model.Role
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Order("id ASC").Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *roleRepository) Update(ctx context.Context, role *model.Role) error {
	return r.db.WithContext(ctx).Save(role).Error
}

func (r *roleRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 检查是否仍有关联的员工。角色是**后台**权限树的一环，绑定关系落在
		// admin_roles；旧实现查的是 user_roles（客户侧角色绑定），既拦不住
		// 「删掉还有员工在用的角色」，又会被历史跨域脏数据（客户挂后台角色）
		// 挡住合法删除。
		var count int64
		if err := tx.Table("admin_roles").Where("role_id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("该角色仍有关联的员工，无法直接删除，请先解除绑定")
		}

		// 客户侧的历史绑定一并清理：角色删掉后 user_roles 里的行会变成悬空引用。
		if err := tx.Table("user_roles").Where("role_id = ?", id).Delete(nil).Error; err != nil {
			return err
		}

		// 删除角色权限关联
		if err := tx.Where("role_id = ?", id).Delete(&model.RolePermission{}).Error; err != nil {
			return err
		}

		// 删除角色本身
		return tx.Delete(&model.Role{}, id).Error
	})
}

func (r *roleRepository) GetPermissionIDs(ctx context.Context, roleID uint64) ([]uint64, error) {
	var rows []model.RolePermission
	if err := r.db.WithContext(ctx).Where("role_id = ?", roleID).Order("permission_id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.PermissionID)
	}
	return ids, nil
}

func (r *roleRepository) UpsertPermissions(ctx context.Context, roleID uint64, permissionIDs []uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&model.RolePermission{}).Error; err != nil {
			return err
		}
		if len(permissionIDs) == 0 {
			return nil
		}
		rows := make([]model.RolePermission, 0, len(permissionIDs))
		for _, permissionID := range permissionIDs {
			rows = append(rows, model.RolePermission{RoleID: roleID, PermissionID: permissionID})
		}
		return tx.Create(&rows).Error
	})
}
