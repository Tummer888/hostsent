package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/user/model"
)

type RoleRepository interface {
	List(ctx context.Context) ([]model.Role, error)
	Create(ctx context.Context, role *model.Role) error
	FindByID(ctx context.Context, id uint64) (*model.Role, error)
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

func (r *roleRepository) List(ctx context.Context) ([]model.Role, error) {
	var roles []model.Role
	if err := r.db.WithContext(ctx).Order("id desc").Find(&roles).Error; err != nil {
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

func (r *roleRepository) Update(ctx context.Context, role *model.Role) error {
	return r.db.WithContext(ctx).Save(role).Error
}

func (r *roleRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 检查是否仍有关联的管理员
		var count int64
		if err := tx.Model(&model.UserRole{}).Where("role_id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("该角色仍有关联的管理员，无法直接删除，请先解除绑定")
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
