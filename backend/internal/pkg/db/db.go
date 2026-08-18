package db

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/user/model"
	"hostsent/backend/internal/pkg/config"
)

type seedPermission struct {
	ParentCode string
	Name       string
	Code       string
	Type       string
	Path       string
	Component  string
	Icon       string
	SortOrder  int
	Status     string
}

func New(cfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode)
	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}

func AutoMigrate(database *gorm.DB) error {
	return database.AutoMigrate(
		&model.User{},
		&model.Role{},
		&model.Permission{},
		&model.RolePermission{},
		&model.UserRole{},
	)
}

func Seed(database *gorm.DB) error {
	return database.Transaction(func(tx *gorm.DB) error {
		permissions, err := seedPermissions(tx)
		if err != nil {
			return err
		}

		role, err := seedAdminRole(tx)
		if err != nil {
			return err
		}

		if err := seedRolePermissions(tx, role.ID, permissionIDs(permissions)); err != nil {
			return err
		}

		user, err := seedAdminUser(tx)
		if err != nil {
			return err
		}

		return seedUserRole(tx, user.ID, role.ID)
	})
}

func seedPermissions(tx *gorm.DB) ([]model.Permission, error) {
	defaults := []seedPermission{
		{Name: "系统管理", Code: "system", Type: "directory", Path: "/system", Icon: "settings", SortOrder: 1, Status: "active"},
		{ParentCode: "system", Name: "用户管理", Code: "system:user", Type: "menu", Path: "/system/users", Component: "system/users/index", Icon: "users", SortOrder: 1, Status: "active"},
		{ParentCode: "system", Name: "角色管理", Code: "system:role", Type: "menu", Path: "/system/roles", Component: "system/roles/index", Icon: "shield", SortOrder: 2, Status: "active"},
		{ParentCode: "system", Name: "权限管理", Code: "system:permission", Type: "menu", Path: "/system/permissions", Component: "system/permissions/index", Icon: "key", SortOrder: 3, Status: "active"},
		{ParentCode: "system:user", Name: "查看用户", Code: "user:list", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "system:user", Name: "创建用户", Code: "user:create", Type: "button", SortOrder: 2, Status: "active"},
		{ParentCode: "system:user", Name: "查看用户详情", Code: "user:detail", Type: "button", SortOrder: 3, Status: "active"},
		{ParentCode: "system:user", Name: "更新用户", Code: "user:update", Type: "button", SortOrder: 4, Status: "active"},
		{ParentCode: "system:user", Name: "更新用户状态", Code: "user:update-status", Type: "button", SortOrder: 5, Status: "active"},
		{ParentCode: "system:user", Name: "重置用户密码", Code: "user:reset-password", Type: "button", SortOrder: 6, Status: "active"},
		{ParentCode: "system:user", Name: "分配用户角色", Code: "user:assign-role", Type: "button", SortOrder: 7, Status: "active"},
		{ParentCode: "system:role", Name: "查看角色", Code: "role:list", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "system:role", Name: "创建角色", Code: "role:create", Type: "button", SortOrder: 2, Status: "active"},
		{ParentCode: "system:role", Name: "查看角色详情", Code: "role:detail", Type: "button", SortOrder: 3, Status: "active"},
		{ParentCode: "system:role", Name: "更新角色", Code: "role:update", Type: "button", SortOrder: 4, Status: "active"},
		{ParentCode: "system:role", Name: "删除角色", Code: "role:delete", Type: "button", SortOrder: 5, Status: "active"},
		{ParentCode: "system:role", Name: "查看角色权限", Code: "role:permission-list", Type: "button", SortOrder: 6, Status: "active"},
		{ParentCode: "system:role", Name: "分配角色权限", Code: "role:assign-permission", Type: "button", SortOrder: 7, Status: "active"},
		{ParentCode: "system:permission", Name: "查看权限", Code: "permission:list", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "system:permission", Name: "创建权限", Code: "permission:create", Type: "button", SortOrder: 2, Status: "active"},
		{ParentCode: "system:permission", Name: "更新权限", Code: "permission:update", Type: "button", SortOrder: 3, Status: "active"},
		{ParentCode: "system:permission", Name: "删除权限", Code: "permission:delete", Type: "button", SortOrder: 4, Status: "active"},
	}

	result := make([]model.Permission, 0, len(defaults))
	idByCode := make(map[string]uint64, len(defaults))
	for _, item := range defaults {
		permission := model.Permission{}
		if err := tx.Where("code = ?", item.Code).First(&permission).Error; err != nil {
			if err != gorm.ErrRecordNotFound {
				return nil, err
			}
			permission = model.Permission{Code: item.Code}
		}
		permission.ParentID = idByCode[item.ParentCode]
		permission.Name = item.Name
		permission.Type = item.Type
		permission.Path = item.Path
		permission.Component = item.Component
		permission.Icon = item.Icon
		permission.SortOrder = item.SortOrder
		permission.Status = item.Status
		if permission.ID == 0 {
			if err := tx.Create(&permission).Error; err != nil {
				return nil, err
			}
		} else {
			if err := tx.Save(&permission).Error; err != nil {
				return nil, err
			}
		}
		idByCode[item.Code] = permission.ID
		result = append(result, permission)
	}

	return result, nil
}

func seedAdminRole(tx *gorm.DB) (*model.Role, error) {
	role := model.Role{}
	if err := tx.Where("code = ?", "admin").First(&role).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
		role = model.Role{Name: "管理员", Code: "admin", Status: "active"}
		if err := tx.Create(&role).Error; err != nil {
			return nil, err
		}
		return &role, nil
	}
	role.Name = "管理员"
	role.Status = "active"
	if err := tx.Save(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func seedRolePermissions(tx *gorm.DB, roleID uint64, permissionIDs []uint64) error {
	if err := tx.Where("role_id = ?", roleID).Delete(&model.RolePermission{}).Error; err != nil {
		return err
	}
	rows := make([]model.RolePermission, 0, len(permissionIDs))
	for _, permissionID := range permissionIDs {
		rows = append(rows, model.RolePermission{RoleID: roleID, PermissionID: permissionID})
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

func seedAdminUser(tx *gorm.DB) (*model.User, error) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := model.User{}
	if err := tx.Where("username = ?", "admin").First(&user).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
		user = model.User{
			Username:     "admin",
			Email:        "admin@example.com",
			Phone:        "13800000000",
			PasswordHash: string(passwordHash),
			Status:       "active",
		}
		if err := tx.Create(&user).Error; err != nil {
			return nil, err
		}
		return &user, nil
	}
	user.Email = "admin@example.com"
	user.Phone = "13800000000"
	user.PasswordHash = string(passwordHash)
	user.Status = "active"
	if err := tx.Save(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func seedUserRole(tx *gorm.DB, userID, roleID uint64) error {
	if err := tx.Where("user_id = ?", userID).Delete(&model.UserRole{}).Error; err != nil {
		return err
	}
	return tx.Create(&model.UserRole{UserID: userID, RoleID: roleID}).Error
}

func permissionIDs(permissions []model.Permission) []uint64 {
	ids := make([]uint64, 0, len(permissions))
	for _, permission := range permissions {
		ids = append(ids, permission.ID)
	}
	return ids
}
