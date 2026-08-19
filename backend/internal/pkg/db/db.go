package db

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	menumodel "hostsent/backend/internal/modules/menu/model"
	usermodel "hostsent/backend/internal/modules/user/model"
	config "hostsent/backend/internal/pkg/config"
)

type seedPermission struct {
	ParentCode string
	Name       string
	Code       string
	Type       string
	SortOrder  int
	Status     string
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

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&usermodel.User{},
		&usermodel.UserGroup{},
		&usermodel.Role{},
		&usermodel.Permission{},
		&usermodel.UserInstance{},
		&usermodel.UserOrder{},
		&usermodel.UserBill{},
		&usermodel.UserTransaction{},
		&usermodel.UserTicket{},
		&menumodel.Menu{},
	); err != nil {
		return err
	}

	return nil
}

func SeedDefaults(db *gorm.DB, cfg config.Config) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := seedRoles(tx); err != nil {
			return err
		}
		if err := seedPermissions(tx); err != nil {
			return err
		}
		if err := seedRolePermissions(tx); err != nil {
			return err
		}
		if err := seedMenus(tx); err != nil {
			return err
		}
		if err := seedAdminUser(tx, cfg); err != nil {
			return err
		}
		if err := seedDemoUsers(tx); err != nil {
			return err
		}
		if err := seedDemoUserDetails(tx); err != nil {
			return err
		}
		return nil
	})
}

func seedRoles(tx *gorm.DB) error {
	defaults := []usermodel.Role{
		{Code: "super_admin", Name: "超级管理员", Status: "active"},
		{Code: "ops_admin", Name: "运维管理员", Status: "active"},
		{Code: "finance_admin", Name: "财务管理员", Status: "active"},
		{Code: "user", Name: "普通用户", Status: "active"},
	}

	for _, role := range defaults {
		var existing usermodel.Role
		if err := tx.Where("code = ?", role.Code).First(&existing).Error; err == nil {
			continue
		} else if err != gorm.ErrRecordNotFound {
			return err
		}
		if err := tx.Create(&role).Error; err != nil {
			return err
		}
	}
	return nil
}

func seedPermissions(tx *gorm.DB) error {
	defaults := []seedPermission{
		{Name: "系统管理", Code: "system", Type: "catalog", SortOrder: 1, Status: "active"},
		{Name: "菜单管理", Code: "system:menu", Type: "menu", SortOrder: 1, Status: "active"},
		{ParentCode: "system:menu", Name: "查看菜单", Code: "menu:view", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "system:menu", Name: "创建菜单", Code: "menu:create", Type: "button", SortOrder: 2, Status: "active"},
		{ParentCode: "system:menu", Name: "更新菜单", Code: "menu:update", Type: "button", SortOrder: 3, Status: "active"},
		{ParentCode: "system:menu", Name: "删除菜单", Code: "menu:delete", Type: "button", SortOrder: 4, Status: "active"},
		{Name: "用户管理", Code: "system:user", Type: "catalog", SortOrder: 2, Status: "active"},
		{Name: "用户列表", Code: "system:user:list", Type: "menu", SortOrder: 1, Status: "active"},
		{ParentCode: "system:user", Name: "查看用户详情", Code: "user:detail", Type: "button", SortOrder: 3, Status: "active"},
		{ParentCode: "system:user", Name: "重置用户密码", Code: "user:reset_password", Type: "button", SortOrder: 4, Status: "active"},
		{ParentCode: "system:user", Name: "修改用户状态", Code: "user:update_status", Type: "button", SortOrder: 5, Status: "active"},
		{Name: "角色管理", Code: "system:role", Type: "catalog", SortOrder: 3, Status: "active"},
		{Name: "角色列表", Code: "system:role:list", Type: "menu", SortOrder: 1, Status: "active"},
		{ParentCode: "system:role", Name: "创建角色", Code: "role:create", Type: "button", SortOrder: 2, Status: "active"},
		{ParentCode: "system:role", Name: "更新角色", Code: "role:update", Type: "button", SortOrder: 3, Status: "active"},
		{ParentCode: "system:role", Name: "删除角色", Code: "role:delete", Type: "button", SortOrder: 4, Status: "active"},
		{ParentCode: "system:role", Name: "分配权限", Code: "role:assign_permissions", Type: "button", SortOrder: 5, Status: "active"},
	}

	permissionMap := make(map[string]uint64)
	for _, item := range defaults {
		var parentID uint64
		if item.ParentCode != "" {
			pid, ok := permissionMap[item.ParentCode]
			if !ok {
				var parent usermodel.Permission
				if err := tx.Where("code = ?", item.ParentCode).First(&parent).Error; err != nil {
					return err
				}
				pid = parent.ID
				permissionMap[item.ParentCode] = pid
			}
			parentID = pid
		}

		var existing usermodel.Permission
		if err := tx.Where("code = ?", item.Code).First(&existing).Error; err == nil {
			permissionMap[item.Code] = existing.ID
			continue
		} else if err != gorm.ErrRecordNotFound {
			return err
		}

		record := usermodel.Permission{ParentID: parentID, Name: item.Name, Code: item.Code, Type: item.Type, SortOrder: item.SortOrder, Status: item.Status}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		permissionMap[item.Code] = record.ID
	}

	return nil
}

func seedRolePermissions(tx *gorm.DB) error {
	rolePermissionCodes := map[string][]string{
		"super_admin": {
			"system",
			"system:menu",
			"menu:view",
			"menu:create",
			"menu:update",
			"menu:delete",
			"system:user",
			"system:user:list",
			"user:detail",
			"user:reset_password",
			"user:update_status",
			"system:role",
			"system:role:list",
			"role:create",
			"role:update",
			"role:delete",
			"role:assign_permissions",
		},
		"ops_admin": {
			"system:user",
			"system:user:list",
			"user:detail",
			"user:update_status",
			"system:role",
			"system:role:list",
		},
		"finance_admin": {
			"system:user",
			"system:user:list",
			"user:detail",
		},
		"user": {
			"system:user",
			"system:user:list",
			"user:detail",
		},
	}

	roleIDs := make(map[string]uint64, len(rolePermissionCodes))
	for roleCode := range rolePermissionCodes {
		var role usermodel.Role
		if err := tx.Where("code = ?", roleCode).First(&role).Error; err != nil {
			return err
		}
		roleIDs[roleCode] = role.ID
	}

	permissionIDs := make(map[string]uint64)
	for _, codes := range rolePermissionCodes {
		for _, code := range codes {
			if _, ok := permissionIDs[code]; ok {
				continue
			}
			var permission usermodel.Permission
			if err := tx.Where("code = ?", code).First(&permission).Error; err != nil {
				return err
			}
			permissionIDs[code] = permission.ID
		}
	}

	for roleCode, codes := range rolePermissionCodes {
		roleID := roleIDs[roleCode]
		for _, code := range codes {
			var count int64
			if err := tx.Table("role_permissions").Where("role_id = ? AND permission_id = ?", roleID, permissionIDs[code]).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				continue
			}
			if err := tx.Table("role_permissions").Create(map[string]any{"role_id": roleID, "permission_id": permissionIDs[code]}).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

func seedMenus(tx *gorm.DB) error {
	defaults := []seedMenu{
		{Platform: menumodel.PlatformAdmin, Name: "仪表盘", Type: menumodel.TypeDirectory, Path: "/dashboard", Icon: "dashboard", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/dashboard", Platform: menumodel.PlatformAdmin, Name: "概览", Type: menumodel.TypeMenu, Path: "/dashboard/base", Component: "dashboard/base/index", Icon: "dashboard", SortOrder: 1, Status: menumodel.StatusActive},
		{Platform: menumodel.PlatformAdmin, Name: "用户管理", Type: menumodel.TypeDirectory, Path: "/users", Icon: "user", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/users", Platform: menumodel.PlatformAdmin, Name: "用户总览", Type: menumodel.TypeMenu, Path: "/users/overview", Component: "users/overview/index", Icon: "dashboard", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/users", Platform: menumodel.PlatformAdmin, Name: "账户管理", Type: menumodel.TypeDirectory, Path: "/users/accounts", Icon: "usergroup", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/users/accounts", Platform: menumodel.PlatformAdmin, Name: "用户列表", Type: menumodel.TypeMenu, Path: "/users/accounts/list", Component: "users/accounts/list/index", Icon: "user-list", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/users/accounts", Platform: menumodel.PlatformAdmin, Name: "用户组/组织管理", Type: menumodel.TypeMenu, Path: "/users/accounts/groups", Component: "users/accounts/groups/index", Icon: "control-platform", SortOrder: 2, Status: menumodel.StatusActive},
	}

	menuMap := make(map[string]uint64)
	for _, item := range defaults {
		var parentID uint64
		if item.ParentKey != "" {
			pid, ok := menuMap[item.ParentKey]
			if !ok {
				var parent menumodel.Menu
				parts := strings.SplitN(item.ParentKey, ":", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid parent key: %s", item.ParentKey)
				}
				if err := tx.Where("platform = ? AND path = ?", parts[0], parts[1]).First(&parent).Error; err != nil {
					return err
				}
				pid = parent.ID
				menuMap[item.ParentKey] = pid
			}
			parentID = pid
		}

		var existing menumodel.Menu
		if err := tx.Where("platform = ? AND path = ?", item.Platform, item.Path).First(&existing).Error; err == nil {
			menuMap[item.Platform+":"+item.Path] = existing.ID
			continue
		} else if err != gorm.ErrRecordNotFound {
			return err
		}

		record := menumodel.Menu{ParentID: parentID, Platform: item.Platform, Name: item.Name, Type: item.Type, Path: item.Path, Component: item.Component, Icon: item.Icon, SortOrder: item.SortOrder, Status: item.Status}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		menuMap[item.Platform+":"+item.Path] = record.ID
	}

	return nil
}

func seedAdminUser(tx *gorm.DB, _ config.Config) error {
	const adminUsername = "admin"
	const adminEmail = "admin@hostsent.local"
	const adminPassword = "Admin@123456"

	var existing usermodel.User
	if err := tx.Where("username = ?", adminUsername).First(&existing).Error; err == nil {
		return nil
	} else if err != gorm.ErrRecordNotFound {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	admin := usermodel.User{Username: adminUsername, Email: adminEmail, PasswordHash: string(hash), Status: "active"}
	if err := tx.Create(&admin).Error; err != nil {
		return err
	}

	var superAdmin usermodel.Role
	if err := tx.Where("code = ?", "super_admin").First(&superAdmin).Error; err != nil {
		return err
	}

	return ensureUserRole(tx, admin.ID, superAdmin.ID)
}

func seedDemoUsers(tx *gorm.DB) error {
	var count int64
	if err := tx.Model(&usermodel.User{}).Where("username LIKE ?", "user_%").Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte("User@123456"), bcrypt.DefaultCost)
		if err != nil {
			return err
		}

		now := time.Now()
		newUserTime := now.Add(-2 * time.Hour)
		loginA := now.Add(-35 * time.Minute)
		loginB := now.Add(-5 * time.Hour)
		loginC := now.Add(-28 * time.Hour)
		loginD := now.Add(-72 * time.Hour)

		defaults := []usermodel.User{
			{Username: "user_east_01", Email: "east01@hostsent.local", Phone: "13900000001", PasswordHash: string(hash), Status: "active", RealName: "李东", Region: "华东", Balance: 1280.50, LastLoginAt: &loginA},
			{Username: "user_north_01", Email: "north01@hostsent.local", Phone: "13900000002", PasswordHash: string(hash), Status: "active", RealName: "王北", Region: "华北", Balance: 860.00, LastLoginAt: &loginB},
			{Username: "user_south_01", Email: "south01@hostsent.local", Phone: "13900000003", PasswordHash: string(hash), Status: "active", RealName: "陈南", Region: "华南", Balance: 420.35, LastLoginAt: &loginC},
			{Username: "user_west_01", Email: "west01@hostsent.local", Phone: "13900000004", PasswordHash: string(hash), Status: "disabled", RealName: "赵西", Region: "西南", Balance: 0, LastLoginAt: &loginD},
			{Username: "user_central_01", Email: "central01@hostsent.local", Phone: "13900000005", PasswordHash: string(hash), Status: "pending", RealName: "", Region: "华中", Balance: 66.60},
			{Username: "user_east_02", Email: "east02@hostsent.local", Phone: "13900000006", PasswordHash: string(hash), Status: "cancelled", RealName: "孙城", Region: "华东", Balance: 0},
			{Username: "user_new_01", Email: "new01@hostsent.local", Phone: "13900000007", PasswordHash: string(hash), Status: "active", RealName: "周新", Region: "华北", Balance: 218.88, CreatedAt: newUserTime, UpdatedAt: newUserTime, LastLoginAt: &loginA},
			{Username: "user_nw_01", Email: "nw01@hostsent.local", Phone: "13900000008", PasswordHash: string(hash), Status: "active", RealName: "", Region: "西北", Balance: 0},
			{Username: "user_ne_01", Email: "ne01@hostsent.local", Phone: "13900000009", PasswordHash: string(hash), Status: "disabled", RealName: "刘北", Region: "东北", Balance: 52.10, LastLoginAt: &loginD},
			{Username: "user_oversea_01", Email: "os01@hostsent.local", Phone: "13900000010", PasswordHash: string(hash), Status: "pending", RealName: "吴洋", Region: "海外", Balance: 999.99},
		}

		for i := range defaults {
			if defaults[i].CreatedAt.IsZero() {
				defaults[i].CreatedAt = now.Add(-time.Duration((i+1)*24) * time.Hour)
			}
			if defaults[i].UpdatedAt.IsZero() {
				defaults[i].UpdatedAt = defaults[i].CreatedAt
			}
		}

		if err := tx.Create(&defaults).Error; err != nil {
			return err
		}
	}

	var userRole usermodel.Role
	if err := tx.Where("code = ?", "user").First(&userRole).Error; err != nil {
		return err
	}

	var opsRole usermodel.Role
	if err := tx.Where("code = ?", "ops_admin").First(&opsRole).Error; err != nil {
		return err
	}

	var users []usermodel.User
	if err := tx.Where("username LIKE ?", "user_%").Find(&users).Error; err != nil {
		return err
	}
	for _, user := range users {
		if err := ensureUserRole(tx, user.ID, userRole.ID); err != nil {
			return err
		}
		if user.Username == "user_nw_01" {
			if err := ensureUserRole(tx, user.ID, opsRole.ID); err != nil {
				return err
			}
		}
	}

	return nil
}

func ensureUserRole(tx *gorm.DB, userID, roleID uint64) error {
	var count int64
	if err := tx.Table("user_roles").Where("user_id = ? AND role_id = ?", userID, roleID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return tx.Table("user_roles").Create(map[string]any{"user_id": userID, "role_id": roleID}).Error
}

func seedDemoUserDetails(tx *gorm.DB) error {
	var target usermodel.User
	if err := tx.Where("username = ?", "user_nw_01").First(&target).Error; err != nil {
		return nil
	}

	var instanceCount int64
	if err := tx.Model(&usermodel.UserInstance{}).Where("user_id = ?", target.ID).Count(&instanceCount).Error; err != nil {
		return err
	}
	if instanceCount == 0 {
		instances := []usermodel.UserInstance{
			{UserID: target.ID, Name: "web-prod-01", Region: "上海一区", Specs: "4C8G / 80G SSD / Ubuntu 22.04", Status: "active", ExpireAt: time.Now().AddDate(0, 1, 0)},
			{UserID: target.ID, Name: "db-standby-01", Region: "上海一区", Specs: "8C16G / 200G SSD / Debian 12", Status: "pending", ExpireAt: time.Now().AddDate(0, 0, 9)},
			{UserID: target.ID, Name: "ci-agent-01", Region: "杭州一区", Specs: "2C4G / 50G SSD / CentOS Stream", Status: "disabled", ExpireAt: time.Now().AddDate(0, 1, 20)},
		}
		if err := tx.Create(&instances).Error; err != nil {
			return err
		}
	}

	var orderCount int64
	if err := tx.Model(&usermodel.UserOrder{}).Where("user_id = ?", target.ID).Count(&orderCount).Error; err != nil {
		return err
	}
	if orderCount == 0 {
		orders := []usermodel.UserOrder{
			{UserID: target.ID, OrderNo: "OD202608180031", Product: "高主频云主机 4C8G", Amount: 688, Status: "paid"},
			{UserID: target.ID, OrderNo: "OD202607260014", Product: "对象存储流量包", Amount: 199, Status: "completed"},
			{UserID: target.ID, OrderNo: "OD202607120003", Product: "云主机续费 2C4G", Amount: 366, Status: "pending"},
		}
		if err := tx.Create(&orders).Error; err != nil {
			return err
		}
	}

	var billCount int64
	if err := tx.Model(&usermodel.UserBill{}).Where("user_id = ?", target.ID).Count(&billCount).Error; err != nil {
		return err
	}
	if billCount == 0 {
			bills := []usermodel.UserBill{
			{UserID: target.ID, BillingMonth: "2026-08", Amount: 1253, Status: "pending"},
			{UserID: target.ID, BillingMonth: "2026-07", Amount: 866, Status: "paid"},
		}
		if err := tx.Create(&bills).Error; err != nil {
			return err
		}
	}

	var txnCount int64
	if err := tx.Model(&usermodel.UserTransaction{}).Where("user_id = ?", target.ID).Count(&txnCount).Error; err != nil {
		return err
	}
	if txnCount == 0 {
		transactions := []usermodel.UserTransaction{
			{UserID: target.ID, TxnNo: "TX202608190002", Type: "recharge", Amount: 2000},
			{UserID: target.ID, TxnNo: "TX202608180021", Type: "consume", Amount: -688},
		}
		if err := tx.Create(&transactions).Error; err != nil {
			return err
		}
	}

	var ticketCount int64
	if err := tx.Model(&usermodel.UserTicket{}).Where("user_id = ?", target.ID).Count(&ticketCount).Error; err != nil {
		return err
	}
	if ticketCount == 0 {
		tickets := []usermodel.UserTicket{
			{UserID: target.ID, TicketNo: "TK20260819005", Title: "实例公网带宽波动", Category: "网络问题", Priority: "high", Status: "processing"},
			{UserID: target.ID, TicketNo: "TK20260811001", Title: "发票抬头更新申请", Category: "财务支持", Priority: "medium", Status: "waiting"},
			{UserID: target.ID, TicketNo: "TK20260730008", Title: "续费后实例未自动开机", Category: "产品使用", Priority: "medium", Status: "resolved"},
		}
		if err := tx.Create(&tickets).Error; err != nil {
			return err
		}
	}

	return nil
}
