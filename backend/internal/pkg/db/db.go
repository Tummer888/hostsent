package db

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	distributionmodel "hostsent/backend/internal/modules/distribution/model"
	menumodel "hostsent/backend/internal/modules/menu/model"
	quotamodel "hostsent/backend/internal/modules/quota/model"
	securitymodel "hostsent/backend/internal/modules/security/model"
	usermodel "hostsent/backend/internal/modules/user/model"
	verificationmodel "hostsent/backend/internal/modules/verification/model"
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
		&distributionmodel.AgentLevel{},
		&distributionmodel.Agent{},
		&distributionmodel.Subordinate{},
		&distributionmodel.Commission{},
		&distributionmodel.Settlement{},
		&securitymodel.LoginLog{},
		&securitymodel.AuditLog{},
		&securitymodel.RiskEvent{},
		&securitymodel.Blacklist{},
		&securitymodel.Session{},
		&usermodel.Role{},
		&usermodel.Permission{},
		&usermodel.UserInstance{},
		&usermodel.UserOrder{},
		&usermodel.UserBill{},
		&usermodel.UserTransaction{},
		&usermodel.UserTicket{},
		&quotamodel.QuotaTemplate{},
		&quotamodel.QuotaTemplateItem{},
		&quotamodel.UserLevel{},
		&quotamodel.ResourceQuota{},
		&quotamodel.QuotaAdjustmentLog{},
		&verificationmodel.VerificationApplication{},
		&verificationmodel.VerificationEnterprise{},
		&verificationmodel.VerificationDocument{},
		&verificationmodel.VerificationReviewLog{},
		&verificationmodel.VerificationConfig{},
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
		if err := seedDemoSecurity(tx); err != nil {
			return err
		}
		if err := seedDemoQuota(tx); err != nil {
			return err
		}
		if err := seedDemoVerification(tx); err != nil {
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
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}

	now := time.Now()

	var instanceCount int64
	if err := tx.Model(&usermodel.UserInstance{}).Where("user_id = ?", target.ID).Count(&instanceCount).Error; err != nil {
		return err
	}
	if instanceCount == 0 {
		instances := []usermodel.UserInstance{
			{UserID: target.ID, Name: "web-prod-01", Region: "上海一区", Specs: "4C8G / 80G SSD / Ubuntu 22.04", Status: "active", ExpireAt: now.AddDate(0, 1, 0)},
			{UserID: target.ID, Name: "db-standby-01", Region: "上海一区", Specs: "8C16G / 200G SSD / Debian 12", Status: "pending", ExpireAt: now.AddDate(0, 0, 9)},
			{UserID: target.ID, Name: "ci-agent-01", Region: "杭州一区", Specs: "2C4G / 50G SSD / CentOS Stream", Status: "disabled", ExpireAt: now.AddDate(0, 1, 20)},
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

func seedDemoSecurity(tx *gorm.DB) error {
	users, err := loadSecuritySeedUsers(tx)
	if err != nil {
		return err
	}
	if len(users) == 0 {
		return nil
	}
	if err := seedDemoLoginLogs(tx, users); err != nil {
		return err
	}
	if err := seedDemoAuditLogs(tx, users); err != nil {
		return err
	}
	if err := seedDemoRiskEvents(tx, users); err != nil {
		return err
	}
	if err := seedDemoBlacklists(tx, users); err != nil {
		return err
	}
	if err := seedDemoSessions(tx, users); err != nil {
		return err
	}
	return nil
}

func loadSecuritySeedUsers(tx *gorm.DB) (map[string]usermodel.User, error) {
	names := []string{"admin", "user_east_01", "user_north_01", "user_south_01", "user_west_01", "user_nw_01"}
	var users []usermodel.User
	if err := tx.Where("username IN ?", names).Find(&users).Error; err != nil {
		return nil, err
	}
	result := make(map[string]usermodel.User, len(users))
	for _, user := range users {
		result[user.Username] = user
	}
	return result, nil
}

func seedDemoLoginLogs(tx *gorm.DB, users map[string]usermodel.User) error {
	var count int64
	if err := tx.Model(&securitymodel.LoginLog{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	now := time.Now()
	logs := []securitymodel.LoginLog{
		{UserID: users["user_east_01"].ID, Username: "user_east_01", LoginType: "password", Result: "success", IP: "101.32.10.12", IPRegion: "上海", UserAgent: "Chrome 139 / macOS", DeviceFingerprint: "fp-east-01", Platform: "web", RiskFlag: "normal", CreatedAt: now.Add(-25 * time.Minute)},
		{UserID: users["user_north_01"].ID, Username: "user_north_01", LoginType: "password", Result: "failed", FailureReason: "密码错误", IP: "43.132.88.9", IPRegion: "北京", UserAgent: "Chrome 139 / Windows", DeviceFingerprint: "fp-north-02", Platform: "web", RiskFlag: "brute_force", CreatedAt: now.Add(-2 * time.Hour)},
		{UserID: users["user_north_01"].ID, Username: "user_north_01", LoginType: "password", Result: "failed", FailureReason: "密码错误", IP: "43.132.88.9", IPRegion: "北京", UserAgent: "Chrome 139 / Windows", DeviceFingerprint: "fp-north-02", Platform: "web", RiskFlag: "brute_force", CreatedAt: now.Add(-110 * time.Minute)},
		{UserID: users["user_south_01"].ID, Username: "user_south_01", LoginType: "sms", Result: "success", IP: "119.29.22.7", IPRegion: "广州", UserAgent: "Mobile Safari / iOS", DeviceFingerprint: "fp-south-01", Platform: "mobile", RiskFlag: "normal", CreatedAt: now.Add(-6 * time.Hour)},
		{UserID: users["user_west_01"].ID, Username: "user_west_01", LoginType: "password", Result: "success", IP: "154.83.14.33", IPRegion: "海外", UserAgent: "Firefox / Linux", DeviceFingerprint: "fp-west-01", Platform: "web", RiskFlag: "suspicious_ip", CreatedAt: now.Add(-11 * time.Hour)},
		{UserID: users["user_nw_01"].ID, Username: "user_nw_01", LoginType: "password", Result: "success", IP: "10.10.2.16", IPRegion: "西安", UserAgent: "Edge / Windows", DeviceFingerprint: "fp-nw-01", Platform: "desktop", RiskFlag: "normal", CreatedAt: now.Add(-27 * time.Hour)},
	}
	return tx.Create(&logs).Error
}

func seedDemoAuditLogs(tx *gorm.DB, users map[string]usermodel.User) error {
	var count int64
	if err := tx.Model(&securitymodel.AuditLog{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	now := time.Now()
	logs := []securitymodel.AuditLog{
		{OperatorID: users["admin"].ID, OperatorName: "admin", Module: "security", ResourceType: "blacklist", ResourceID: "1", Action: "create", RequestMethod: "POST", RequestPath: "/api/v1/security/blacklists", RequestPayload: `{"type":"ip","target_value":"154.83.14.33"}`, ResponseCode: 200, ResponseMessage: "ok", IP: "127.0.0.1", UserAgent: "Chrome 139 / macOS", TraceID: "trace-sec-001", CreatedAt: now.Add(-90 * time.Minute)},
		{OperatorID: users["admin"].ID, OperatorName: "admin", Module: "security", ResourceType: "risk_event", ResourceID: "2", Action: "handle", RequestMethod: "POST", RequestPath: "/api/v1/security/risk-events/2/handle", RequestPayload: `{"note":"人工复核后转已处理"}`, ResponseCode: 200, ResponseMessage: "ok", IP: "127.0.0.1", UserAgent: "Chrome 139 / macOS", TraceID: "trace-sec-002", CreatedAt: now.Add(-70 * time.Minute)},
		{OperatorID: users["admin"].ID, OperatorName: "admin", Module: "security", ResourceType: "session", ResourceID: "3", Action: "revoke", RequestMethod: "POST", RequestPath: "/api/v1/security/sessions/3/revoke", RequestPayload: `{"reason":"异地风险登录"}`, ResponseCode: 200, ResponseMessage: "ok", IP: "127.0.0.1", UserAgent: "Chrome 139 / macOS", TraceID: "trace-sec-003", CreatedAt: now.Add(-45 * time.Minute)},
		{OperatorID: users["admin"].ID, OperatorName: "admin", Module: "menu", ResourceType: "menu", ResourceID: "8", Action: "update", RequestMethod: "PUT", RequestPath: "/api/v1/menus/8", RequestPayload: `{"name":"安全与风控"}`, ResponseCode: 200, ResponseMessage: "ok", IP: "127.0.0.1", UserAgent: "Chrome 139 / macOS", TraceID: "trace-sec-004", CreatedAt: now.Add(-20 * time.Minute)},
		{OperatorID: users["user_nw_01"].ID, OperatorName: "user_nw_01", Module: "auth", ResourceType: "login", ResourceID: "user_nw_01", Action: "login", RequestMethod: "POST", RequestPath: "/api/v1/auth/login", RequestPayload: `{"username":"user_nw_01"}`, ResponseCode: 200, ResponseMessage: "ok", IP: "10.10.2.16", UserAgent: "Edge / Windows", TraceID: "trace-sec-005", CreatedAt: now.Add(-15 * time.Minute)},
	}
	return tx.Create(&logs).Error
}

func seedDemoRiskEvents(tx *gorm.DB, users map[string]usermodel.User) error {
	var count int64
	if err := tx.Model(&securitymodel.RiskEvent{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	now := time.Now()
	handledBy := users["admin"].ID
	handledAt := now.Add(-50 * time.Minute)
	events := []securitymodel.RiskEvent{
		{RiskType: "brute_force", RiskLevel: "high", UserID: users["user_north_01"].ID, Username: "user_north_01", IP: "43.132.88.9", DeviceFingerprint: "fp-north-02", RuleCode: "LOGIN_FAIL_THRESHOLD", Summary: "短时间内连续登录失败", DetailPayload: `{"fail_count":5,"window_minutes":10}`, OccurCount: 5, FirstOccurredAt: now.Add(-130 * time.Minute), LastOccurredAt: now.Add(-105 * time.Minute), Status: "pending", CreatedAt: now.Add(-105 * time.Minute), UpdatedAt: now.Add(-105 * time.Minute)},
		{RiskType: "suspicious_ip", RiskLevel: "medium", UserID: users["user_west_01"].ID, Username: "user_west_01", IP: "154.83.14.33", DeviceFingerprint: "fp-west-01", RuleCode: "GEO_ABNORMAL_LOGIN", Summary: "非常用地区登录", DetailPayload: `{"usual_region":"西南","current_region":"海外"}`, OccurCount: 2, FirstOccurredAt: now.Add(-12 * time.Hour), LastOccurredAt: now.Add(-11 * time.Hour), Status: "handled", HandledBy: &handledBy, HandledAt: &handledAt, HandleNote: "已核验为代理节点登录，保留观察", CreatedAt: now.Add(-11 * time.Hour), UpdatedAt: handledAt},
		{RiskType: "device_change", RiskLevel: "low", UserID: users["user_south_01"].ID, Username: "user_south_01", IP: "119.29.22.7", DeviceFingerprint: "fp-south-new", RuleCode: "DEVICE_FINGERPRINT_CHANGED", Summary: "设备指纹发生变化", DetailPayload: `{"old":"fp-south-01","new":"fp-south-new"}`, OccurCount: 1, FirstOccurredAt: now.Add(-7 * time.Hour), LastOccurredAt: now.Add(-7 * time.Hour), Status: "ignored", HandledBy: &handledBy, HandledAt: &handledAt, HandleNote: "用户自助换机", CreatedAt: now.Add(-7 * time.Hour), UpdatedAt: handledAt},
	}
	return tx.Create(&events).Error
}

func seedDemoBlacklists(tx *gorm.DB, users map[string]usermodel.User) error {
	var count int64
	if err := tx.Model(&securitymodel.Blacklist{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	now := time.Now()
	expiredAt := now.AddDate(0, 0, 14)
	items := []securitymodel.Blacklist{
		{Type: "ip", TargetValue: "154.83.14.33", Status: "active", Source: "system", Reason: "命中异地高风险登录", EffectiveAt: now.Add(-10 * time.Hour), ExpiredAt: &expiredAt, HitCount: 3, CreatedBy: users["admin"].ID, UpdatedBy: users["admin"].ID, CreatedAt: now.Add(-10 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour)},
		{Type: "device", TargetValue: "fp-north-02", Status: "inactive", Source: "manual", Reason: "暴力破解后临时封禁，已解除", EffectiveAt: now.Add(-3 * 24 * time.Hour), HitCount: 5, CreatedBy: users["admin"].ID, UpdatedBy: users["admin"].ID, CreatedAt: now.Add(-3 * 24 * time.Hour), UpdatedAt: now.Add(-24 * time.Hour)},
		{Type: "user", TargetValue: "user_west_01", Status: "active", Source: "risk_event", Reason: "多次高风险地区尝试登录", EffectiveAt: now.Add(-9 * time.Hour), HitCount: 2, CreatedBy: users["admin"].ID, UpdatedBy: users["admin"].ID, CreatedAt: now.Add(-9 * time.Hour), UpdatedAt: now.Add(-9 * time.Hour)},
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&items).Error
}

func seedDemoSessions(tx *gorm.DB, users map[string]usermodel.User) error {
	var count int64
	if err := tx.Model(&securitymodel.Session{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	now := time.Now()
	expiresA := now.Add(7 * 24 * time.Hour)
	expiresB := now.Add(5 * 24 * time.Hour)
	expiresC := now.Add(3 * 24 * time.Hour)
	expiresD := now.Add(24 * time.Hour)
	revokedAt := now.Add(-40 * time.Minute)
	revokedBy := users["admin"].ID
	sessions := []securitymodel.Session{
		{SessionID: "sess_admin_001", UserID: users["admin"].ID, Username: "admin", Platform: "web", IP: "127.0.0.1", IPRegion: "本地", UserAgent: "Chrome 139 / macOS", DeviceFingerprint: "fp-admin-01", LoginAt: now.Add(-8 * time.Hour), LastActiveAt: now.Add(-5 * time.Minute), ExpiredAt: &expiresA, Status: "active", RiskFlag: "normal", CreatedAt: now.Add(-8 * time.Hour), UpdatedAt: now.Add(-5 * time.Minute)},
		{SessionID: "sess_east_001", UserID: users["user_east_01"].ID, Username: "user_east_01", Platform: "web", IP: "101.32.10.12", IPRegion: "上海", UserAgent: "Chrome 139 / macOS", DeviceFingerprint: "fp-east-01", LoginAt: now.Add(-6 * time.Hour), LastActiveAt: now.Add(-25 * time.Minute), ExpiredAt: &expiresB, Status: "active", RiskFlag: "normal", CreatedAt: now.Add(-6 * time.Hour), UpdatedAt: now.Add(-25 * time.Minute)},
		{SessionID: "sess_north_001", UserID: users["user_north_01"].ID, Username: "user_north_01", Platform: "web", IP: "43.132.88.9", IPRegion: "北京", UserAgent: "Chrome 139 / Windows", DeviceFingerprint: "fp-north-02", LoginAt: now.Add(-3 * time.Hour), LastActiveAt: now.Add(-2 * time.Hour), ExpiredAt: &expiresC, Status: "revoked", RiskFlag: "brute_force", RevokedReason: "异地风险登录", RevokedBy: &revokedBy, RevokedAt: &revokedAt, CreatedAt: now.Add(-3 * time.Hour), UpdatedAt: revokedAt},
		{SessionID: "sess_south_001", UserID: users["user_south_01"].ID, Username: "user_south_01", Platform: "mobile", IP: "119.29.22.7", IPRegion: "广州", UserAgent: "Mobile Safari / iOS", DeviceFingerprint: "fp-south-new", LoginAt: now.Add(-9 * time.Hour), LastActiveAt: now.Add(-7 * time.Hour), ExpiredAt: &expiresD, Status: "expired", RiskFlag: "device_change", CreatedAt: now.Add(-9 * time.Hour), UpdatedAt: now.Add(-7 * time.Hour)},
		{SessionID: "sess_nw_001", UserID: users["user_nw_01"].ID, Username: "user_nw_01", Platform: "desktop", IP: "10.10.2.16", IPRegion: "西安", UserAgent: "Edge / Windows", DeviceFingerprint: "fp-nw-01", LoginAt: now.Add(-26 * time.Hour), LastActiveAt: now.Add(-90 * time.Minute), ExpiredAt: &expiresA, Status: "active", RiskFlag: "normal", CreatedAt: now.Add(-26 * time.Hour), UpdatedAt: now.Add(-90 * time.Minute)},
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&sessions).Error
}

func seedDemoQuota(tx *gorm.DB) error {
	users, err := loadQuotaSeedUsers(tx)
	if err != nil {
		return err
	}
	if len(users) == 0 {
		return nil
	}

	adminID := users["admin"].ID
	templateIDs, err := seedDemoQuotaTemplates(tx, adminID)
	if err != nil {
		return err
	}
	levelIDs, err := seedDemoUserLevels(tx, adminID, templateIDs)
	if err != nil {
		return err
	}
	if err := seedDemoResourceQuotas(tx, users, adminID, templateIDs, levelIDs); err != nil {
		return err
	}
	if err := seedDemoQuotaAdjustmentLogs(tx, users, adminID, templateIDs, levelIDs); err != nil {
		return err
	}
	return nil
}

func loadQuotaSeedUsers(tx *gorm.DB) (map[string]usermodel.User, error) {
	names := []string{"admin", "user_east_01", "user_north_01", "user_south_01", "user_nw_01"}
	var users []usermodel.User
	if err := tx.Where("username IN ?", names).Find(&users).Error; err != nil {
		return nil, err
	}
	result := make(map[string]usermodel.User, len(users))
	for _, user := range users {
		result[user.Username] = user
	}
	return result, nil
}

func seedDemoQuotaTemplates(tx *gorm.DB, adminID uint64) (map[string]uint64, error) {
	templates := []struct {
		Template quotamodel.QuotaTemplate
		Items    []quotamodel.QuotaTemplateItem
	}{
		{
			Template: quotamodel.QuotaTemplate{
				Name:        "标准版模板",
				Code:        "starter_default",
				Scope:       "default",
				Status:      "active",
				Description: "适用于普通新注册用户的默认配额模板",
				Version:     1,
				CreatedBy:   adminID,
				UpdatedBy:   adminID,
			},
			Items: []quotamodel.QuotaTemplateItem{
				{QuotaCode: "instance_count", QuotaName: "云主机数量", QuotaType: "compute", LimitValue: 2, Unit: "count", Sort: 1},
				{QuotaCode: "cpu_cores", QuotaName: "CPU 核数", QuotaType: "compute", LimitValue: 4, Unit: "core", Sort: 2},
				{QuotaCode: "memory_gb", QuotaName: "内存", QuotaType: "compute", LimitValue: 8, Unit: "GB", Sort: 3},
				{QuotaCode: "disk_gb", QuotaName: "系统盘", QuotaType: "storage", LimitValue: 120, Unit: "GB", Sort: 4},
			},
		},
		{
			Template: quotamodel.QuotaTemplate{
				Name:        "企业版模板",
				Code:        "business_plus",
				Scope:       "level",
				Status:      "active",
				Description: "适用于进阶企业用户的配额模板",
				Version:     1,
				CreatedBy:   adminID,
				UpdatedBy:   adminID,
			},
			Items: []quotamodel.QuotaTemplateItem{
				{QuotaCode: "instance_count", QuotaName: "云主机数量", QuotaType: "compute", LimitValue: 10, Unit: "count", Sort: 1},
				{QuotaCode: "cpu_cores", QuotaName: "CPU 核数", QuotaType: "compute", LimitValue: 32, Unit: "core", Sort: 2},
				{QuotaCode: "memory_gb", QuotaName: "内存", QuotaType: "compute", LimitValue: 64, Unit: "GB", Sort: 3},
				{QuotaCode: "disk_gb", QuotaName: "系统盘", QuotaType: "storage", LimitValue: 1000, Unit: "GB", Sort: 4},
			},
		},
	}

	result := make(map[string]uint64, len(templates))
	for _, item := range templates {
		var existing quotamodel.QuotaTemplate
		if err := tx.Where("code = ?", item.Template.Code).First(&existing).Error; err == nil {
			result[item.Template.Code] = existing.ID
			continue
		} else if err != gorm.ErrRecordNotFound {
			return nil, err
		}
		if err := tx.Create(&item.Template).Error; err != nil {
			return nil, err
		}
		for i := range item.Items {
			item.Items[i].TemplateID = item.Template.ID
		}
		if err := tx.Create(&item.Items).Error; err != nil {
			return nil, err
		}
		result[item.Template.Code] = item.Template.ID
	}
	return result, nil
}

func seedDemoUserLevels(tx *gorm.DB, adminID uint64, templateIDs map[string]uint64) (map[string]uint64, error) {
	starterTemplateID := templateIDs["starter_default"]
	businessTemplateID := templateIDs["business_plus"]
	levels := []quotamodel.UserLevel{
		{
			Name:              "标准用户",
			Code:              "standard",
			Weight:            10,
			Status:            "active",
			DefaultTemplateID: &starterTemplateID,
			MaxInstanceCount:  2,
			MaxCPUCores:       4,
			MaxMemoryGB:       8,
			MaxDiskGB:         120,
			FeatureFlags:      "snapshot,backup",
			UpgradeCondition:  "累计消费满 1000 元",
			Description:       "默认用户等级",
			CreatedBy:         adminID,
			UpdatedBy:         adminID,
		},
		{
			Name:              "企业用户",
			Code:              "business",
			Weight:            20,
			Status:            "active",
			DefaultTemplateID: &businessTemplateID,
			MaxInstanceCount:  10,
			MaxCPUCores:       32,
			MaxMemoryGB:       64,
			MaxDiskGB:         1000,
			FeatureFlags:      "snapshot,backup,ha,custom-image",
			UpgradeCondition:  "通过企业实名认证并完成销售审核",
			Description:       "企业大客户等级",
			CreatedBy:         adminID,
			UpdatedBy:         adminID,
		},
	}

	result := make(map[string]uint64, len(levels))
	for _, level := range levels {
		var existing quotamodel.UserLevel
		if err := tx.Where("code = ?", level.Code).First(&existing).Error; err == nil {
			result[level.Code] = existing.ID
			continue
		} else if err != gorm.ErrRecordNotFound {
			return nil, err
		}
		if err := tx.Create(&level).Error; err != nil {
			return nil, err
		}
		result[level.Code] = level.ID
	}
	return result, nil
}

func seedDemoResourceQuotas(tx *gorm.DB, users map[string]usermodel.User, adminID uint64, templateIDs map[string]uint64, levelIDs map[string]uint64) error {
	type quotaSeed struct {
		Username string
		Code     string
		Name     string
		Type     string
		Limit    float64
		Used     float64
		Unit     string
		Source   string
		Template string
		Level    string
	}

	seeds := []quotaSeed{
		{Username: "user_east_01", Code: "instance_count", Name: "云主机数量", Type: "compute", Limit: 2, Used: 1, Unit: "count", Source: "template", Template: "starter_default", Level: "standard"},
		{Username: "user_east_01", Code: "cpu_cores", Name: "CPU 核数", Type: "compute", Limit: 4, Used: 2, Unit: "core", Source: "template", Template: "starter_default", Level: "standard"},
		{Username: "user_east_01", Code: "memory_gb", Name: "内存", Type: "compute", Limit: 8, Used: 4, Unit: "GB", Source: "template", Template: "starter_default", Level: "standard"},
		{Username: "user_north_01", Code: "instance_count", Name: "云主机数量", Type: "compute", Limit: 10, Used: 6, Unit: "count", Source: "level", Template: "business_plus", Level: "business"},
		{Username: "user_north_01", Code: "cpu_cores", Name: "CPU 核数", Type: "compute", Limit: 32, Used: 20, Unit: "core", Source: "level", Template: "business_plus", Level: "business"},
		{Username: "user_north_01", Code: "disk_gb", Name: "系统盘", Type: "storage", Limit: 1000, Used: 760, Unit: "GB", Source: "level", Template: "business_plus", Level: "business"},
		{Username: "user_south_01", Code: "memory_gb", Name: "内存", Type: "compute", Limit: 8, Used: 10, Unit: "GB", Source: "manual", Template: "starter_default", Level: "standard"},
		{Username: "user_nw_01", Code: "instance_count", Name: "云主机数量", Type: "compute", Limit: 12, Used: 9, Unit: "count", Source: "manual", Template: "business_plus", Level: "business"},
	}

	now := time.Now()
	for _, seed := range seeds {
		user, ok := users[seed.Username]
		if !ok {
			continue
		}
		var count int64
		if err := tx.Model(&quotamodel.ResourceQuota{}).Where("user_id = ? AND quota_code = ?", user.ID, seed.Code).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		available := seed.Limit - seed.Used
		templateID := templateIDs[seed.Template]
		levelID := levelIDs[seed.Level]
		record := quotamodel.ResourceQuota{
			UserID:          user.ID,
			QuotaCode:       seed.Code,
			QuotaName:       seed.Name,
			QuotaType:       seed.Type,
			LimitValue:      seed.Limit,
			UsedValue:       seed.Used,
			AvailableValue:  available,
			Unit:            seed.Unit,
			Status:          "active",
			Source:          seed.Source,
			TemplateID:      &templateID,
			LevelID:         &levelID,
			IsOverallocated: available < 0,
			UpdatedBy:       adminID,
			LastAdjustedAt:  now,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
	}
	return nil
}

func seedDemoQuotaAdjustmentLogs(tx *gorm.DB, users map[string]usermodel.User, adminID uint64, templateIDs map[string]uint64, levelIDs map[string]uint64) error {
	var count int64
	if err := tx.Model(&quotamodel.QuotaAdjustmentLog{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	templateStarter := templateIDs["starter_default"]
	templateBusiness := templateIDs["business_plus"]
	levelStandard := levelIDs["standard"]
	levelBusiness := levelIDs["business"]
	now := time.Now()
	logs := []quotamodel.QuotaAdjustmentLog{
		{UserID: users["user_south_01"].ID, Username: "user_south_01", QuotaCode: "memory_gb", QuotaName: "内存", BeforeValue: 6, AfterValue: 8, DeltaValue: 2, AdjustmentType: "manual", Source: "manual", TemplateID: &templateStarter, LevelID: &levelStandard, OperatorID: adminID, OperatorName: "admin", Reason: "活动期补充资源", TicketNo: "TK-QUOTA-20260801", BatchNo: "quota-batch-001", CreatedAt: now.Add(-48 * time.Hour)},
		{UserID: users["user_nw_01"].ID, Username: "user_nw_01", QuotaCode: "instance_count", QuotaName: "云主机数量", BeforeValue: 10, AfterValue: 12, DeltaValue: 2, AdjustmentType: "manual", Source: "manual", TemplateID: &templateBusiness, LevelID: &levelBusiness, OperatorID: adminID, OperatorName: "admin", Reason: "大客户扩容审批通过", TicketNo: "TK-QUOTA-20260802", BatchNo: "quota-batch-002", CreatedAt: now.Add(-24 * time.Hour)},
		{UserID: users["user_north_01"].ID, Username: "user_north_01", QuotaCode: "cpu_cores", QuotaName: "CPU 核数", BeforeValue: 24, AfterValue: 32, DeltaValue: 8, AdjustmentType: "upgrade", Source: "level", TemplateID: &templateBusiness, LevelID: &levelBusiness, OperatorID: adminID, OperatorName: "admin", Reason: "升级企业等级自动提升", TicketNo: "TK-QUOTA-20260803", BatchNo: "quota-batch-003", CreatedAt: now.Add(-12 * time.Hour)},
	}
	return tx.Create(&logs).Error
}
