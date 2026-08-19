package db

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	menumodel "hostsent/backend/internal/modules/menu/model"
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
		&menumodel.Menu{},
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

		if err := seedUserRole(tx, user.ID, role.ID); err != nil {
			return err
		}

		return seedMenus(tx)
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

type seedMenu struct {
	ParentKey string
	Platform  string
	Name      string
	Type      string
	Path      string
	Component string
	Icon      string
	SortOrder int
	Status    string
}

func seedMenus(tx *gorm.DB) error {
	defaults := []seedMenu{
		// ================= 管理员后台菜单 =================
		// 1. 仪表盘
		{Platform: menumodel.PlatformAdmin, Name: "仪表盘", Type: menumodel.TypeDirectory, Path: "/dashboard", Icon: "dashboard", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/dashboard", Platform: menumodel.PlatformAdmin, Name: "概览", Type: menumodel.TypeMenu, Path: "/dashboard/base", Component: "dashboard/base/index", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/dashboard", Platform: menumodel.PlatformAdmin, Name: "数据分析", Type: menumodel.TypeMenu, Path: "/dashboard/analysis", Component: "dashboard/analysis/index", SortOrder: 2, Status: menumodel.StatusActive},

		// 2. 用户管理
		{Platform: menumodel.PlatformAdmin, Name: "用户管理", Type: menumodel.TypeDirectory, Path: "/users", Icon: "user", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/users", Platform: menumodel.PlatformAdmin, Name: "用户列表", Type: menumodel.TypeMenu, Path: "/users/list", Component: "users/list/index", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/users", Platform: menumodel.PlatformAdmin, Name: "角色权限", Type: menumodel.TypeMenu, Path: "/users/roles", Component: "users/roles/index", SortOrder: 2, Status: menumodel.StatusActive},

		// 3. 资源管理
		{Platform: menumodel.PlatformAdmin, Name: "资源管理", Type: menumodel.TypeDirectory, Path: "/resources", Icon: "layers", SortOrder: 3, Status: menumodel.StatusActive},
		{ParentKey: "admin:/resources", Platform: menumodel.PlatformAdmin, Name: "云主机", Type: menumodel.TypeDirectory, Path: "/resources/instances", Component: "resources/instances/index", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/resources/instances)", Platform: menumodel.PlatformAdmin, Name: "实例列表", Type: menumodel.TypeMenu, Path: "/resources/instances/list", Component: "resources/instances/index", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/resources/instances)", Platform: menumodel.PlatformAdmin, Name: "快照管理", Type: menumodel.TypeMenu, Path: "/resources/instances/snapshots", Component: "resources/instances/snapshots", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/resources", Platform: menumodel.PlatformAdmin, Name: "镜像管理", Type: menumodel.TypeMenu, Path: "/resources/images", Component: "resources/images/index", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/resources", Platform: menumodel.PlatformAdmin, Name: "网络管理", Type: menumodel.TypeMenu, Path: "/resources/networks", Component: "resources/networks/index", SortOrder: 3, Status: menumodel.StatusActive},

		// 4. 产品管理
		{Platform: menumodel.PlatformAdmin, Name: "产品管理", Type: menumodel.TypeDirectory, Path: "/products", Icon: "apps", SortOrder: 4, Status: menumodel.StatusActive},
		{ParentKey: "admin:/products", Platform: menumodel.PlatformAdmin, Name: "产品列表", Type: menumodel.TypeMenu, Path: "/products/list", Component: "products/list/index", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/products", Platform: menumodel.PlatformAdmin, Name: "产品分类", Type: menumodel.TypeMenu, Path: "/products/categories", Component: "products/categories/index", SortOrder: 2, Status: menumodel.StatusActive},

		// 5. 订单管理
		{Platform: menumodel.PlatformAdmin, Name: "订单管理", Type: menumodel.TypeDirectory, Path: "/orders", Icon: "order", SortOrder: 5, Status: menumodel.StatusActive},
		{ParentKey: "admin:/orders", Platform: menumodel.PlatformAdmin, Name: "订单列表", Type: menumodel.TypeMenu, Path: "/orders/list", Component: "orders/list/index", SortOrder: 1, Status: menumodel.StatusActive},

		// 6. 财务管理
		{Platform: menumodel.PlatformAdmin, Name: "财务管理", Type: menumodel.TypeDirectory, Path: "/billing", Icon: "bill", SortOrder: 6, Status: menumodel.StatusActive},
		{ParentKey: "admin:/billing", Platform: menumodel.PlatformAdmin, Name: "账单查询", Type: menumodel.TypeMenu, Path: "/billing/bills", Component: "billing/bills/index", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/billing", Platform: menumodel.PlatformAdmin, Name: "交易流水", Type: menumodel.TypeMenu, Path: "/billing/transactions", Component: "billing/transactions/index", SortOrder: 2, Status: menumodel.StatusActive},

		// 7. 系统管理
		{Platform: menumodel.PlatformAdmin, Name: "系统管理", Type: menumodel.TypeDirectory, Path: "/system", Icon: "settings", SortOrder: 7, Status: menumodel.StatusActive},
		{ParentKey: "admin:/system", Platform: menumodel.PlatformAdmin, Name: "菜单管理", Type: menumodel.TypeMenu, Path: "/system/menus", Component: "system/menus/index", Icon: "menu", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/system", Platform: menumodel.PlatformAdmin, Name: "审计日志", Type: menumodel.TypeMenu, Path: "/system/audit", Component: "system/audit/index", SortOrder: 2, Status: menumodel.StatusActive},

		// 8. 工单支持
		{Platform: menumodel.PlatformAdmin, Name: "工单支持", Type: menumodel.TypeDirectory, Path: "/support", Icon: "service", SortOrder: 8, Status: menumodel.StatusActive},
		{ParentKey: "admin:/support", Platform: menumodel.PlatformAdmin, Name: "工单列表", Type: menumodel.TypeMenu, Path: "/support/tickets", Component: "support/tickets/index", SortOrder: 1, Status: menumodel.StatusActive},

		// ================= 用户中心菜单 =================
		{Platform: menumodel.PlatformUser, Name: "控制台", Type: menumodel.TypeMenu, Path: "/user/dashboard", Component: "user/dashboard/index", Icon: "dashboard", SortOrder: 1, Status: menumodel.StatusActive},
		{Platform: menumodel.PlatformUser, Name: "云主机", Type: menumodel.TypeDirectory, Path: "/user/instances", Icon: "cloud", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "user:/user/instances", Platform: menumodel.PlatformUser, Name: "我的主机", Type: menumodel.TypeMenu, Path: "/user/instances/list", Component: "user/instances/list/index", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "user:/user/instances", Platform: menumodel.PlatformUser, Name: "快照管理", Type: menumodel.TypeMenu, Path: "/user/instances/snapshots", Component: "user/instances/snapshots", SortOrder: 2, Status: menumodel.StatusActive},
		{Platform: menumodel.PlatformUser, Name: "订单中心", Type: menumodel.TypeDirectory, Path: "/user/orders", Icon: "order", SortOrder: 3, Status: menumodel.StatusActive},
		{ParentKey: "user:/user/orders", Platform: menumodel.PlatformUser, Name: "我的订单", Type: menumodel.TypeMenu, Path: "/user/orders/list", Component: "user/orders/list/index", SortOrder: 1, Status: menumodel.StatusActive},
		{Platform: menumodel.PlatformUser, Name: "账户管理", Type: menumodel.TypeDirectory, Path: "/user/account", Icon: "user", SortOrder: 4, Status: menumodel.StatusActive},
		{ParentKey: "user:/user/account", Platform: menumodel.PlatformUser, Name: "账户概览", Type: menumodel.TypeMenu, Path: "/user/account/overview", Component: "user/account/overview/index", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "user:/user/account", Platform: menumodel.PlatformUser, Name: "账单查询", Type: menumodel.TypeMenu, Path: "/user/account/bills", Component: "user/account/bills/index", SortOrder: 2, Status: menumodel.StatusActive},
		{Platform: menumodel.PlatformUser, Name: "工单支持", Type: menumodel.TypeMenu, Path: "/user/tickets", Component: "user/tickets/index", Icon: "service", SortOrder: 5, Status: menumodel.StatusActive},
	}

	idByKey := make(map[string]uint64, len(defaults))
	for _, item := range defaults {
		menu := menumodel.Menu{}
		if err := tx.Where("platform = ? AND name = ?", item.Platform, item.Name).First(&menu).Error; err != nil {
			if err != gorm.ErrRecordNotFound {
				return err
			}
			menu = menumodel.Menu{Platform: item.Platform, Name: item.Name}
		}
		menu.ParentID = idByKey[item.ParentKey]
		menu.Platform = item.Platform
		menu.Name = item.Name
		menu.Type = item.Type
		menu.Path = item.Path
		menu.Component = item.Component
		menu.Icon = item.Icon
		menu.SortOrder = item.SortOrder
		menu.Status = item.Status
		if menu.ID == 0 {
			if err := tx.Create(&menu).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Save(&menu).Error; err != nil {
				return err
			}
		}
		idByKey[item.Platform+":"+item.Path] = menu.ID
	}
	return nil
}
