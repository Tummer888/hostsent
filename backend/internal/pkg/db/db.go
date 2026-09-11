package db

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	finaccountmodel "hostsent/backend/internal/modules/admin/finance/account/model"
	finbillmodel "hostsent/backend/internal/modules/admin/finance/bill/model"
	finrechmodel "hostsent/backend/internal/modules/admin/finance/recharge/model"
	fintransmodel "hostsent/backend/internal/modules/admin/finance/transaction/model"
	finwithdrawmodel "hostsent/backend/internal/modules/admin/finance/withdraw/model"
	instancemodel "hostsent/backend/internal/modules/admin/instance/model"
	lifecyclemodel "hostsent/backend/internal/modules/admin/lifecycle/model"
	adminmodel "hostsent/backend/internal/modules/admin/manager/model"
	menumodel "hostsent/backend/internal/modules/admin/menu/model"
	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	catalogmodel "hostsent/backend/internal/modules/admin/product/catalog/model"
	categorymodel "hostsent/backend/internal/modules/admin/product/category/model"
	discountmodel "hostsent/backend/internal/modules/admin/product/discount/model"
	pricingmodel "hostsent/backend/internal/modules/admin/product/pricing/model"
	promotionmodel "hostsent/backend/internal/modules/admin/product/promotion/model"
	specmodel "hostsent/backend/internal/modules/admin/product/spec/model"
	referralmodel "hostsent/backend/internal/modules/admin/referral/model"
	productmodel "hostsent/backend/internal/modules/admin/resource/product/model"
	providermodel "hostsent/backend/internal/modules/admin/resource/provider/model"
	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
	systemmodel "hostsent/backend/internal/modules/admin/system/model"
	ticketmodel "hostsent/backend/internal/modules/admin/ticket/model"
	usermodel "hostsent/backend/internal/modules/admin/user/account/model"
	levelmodel "hostsent/backend/internal/modules/admin/user/level/model"
	securitymodel "hostsent/backend/internal/modules/admin/user/security/model"
	verificationmodel "hostsent/backend/internal/modules/admin/user/verification/model"
	usercentermodel "hostsent/backend/internal/modules/uc/auth/model"
	membermodel "hostsent/backend/internal/modules/uc/member/model"
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
		&usercentermodel.User{}, // 用户中心模型，与 usermodel.User 共用 users 表，补齐 avatar/tier 列
		&usermodel.UserGroup{},
		&usermodel.SubAccountPermission{},
		&membermodel.OperationLog{},
		// 推广邀请返现（独立于现金钱包的三表）
		&referralmodel.ReferralAccount{},
		&referralmodel.ReferralTransaction{},
		&referralmodel.ReferralWithdrawal{},
		&securitymodel.LoginLog{},
		&securitymodel.AuditLog{},
		&securitymodel.RiskEvent{},
		&securitymodel.Blacklist{},
		&securitymodel.Session{},
		&usermodel.Role{},
		&usermodel.Permission{},
		// 冗余聚合表 user_instances/user_bills/user_transactions 已于 Phase 2
		// 将读路径收敛到实例/账务权威表并退役（见 migrations 013/014）：
		//   &usermodel.UserInstance{},
		//   &usermodel.UserBill{},
		//   &usermodel.UserTransaction{},
		&usermodel.UserOrder{},
		&usermodel.UserTicket{},
		&levelmodel.UserLevel{},
		&levelmodel.UserLevelChangeLog{},
		&verificationmodel.VerificationApplication{},
		&verificationmodel.VerificationEnterprise{},
		&verificationmodel.VerificationDocument{},
		&verificationmodel.VerificationReviewLog{},
		&verificationmodel.VerificationConfig{},
		&menumodel.Menu{},
		&adminmodel.Admin{},
		&adminmodel.AdminRole{},
		&adminmodel.AdminAuditLog{},
		// 资源管理模块核心表（第一阶段）
		&providermodel.ResourceProvider{},
		&providermodel.ResourcePool{},
		&providermodel.ProviderType{}, // 渠道类型注册表（P2/T2.2）
		&productmodel.ResourceProduct{},
		&syncmodel.SyncTask{},
		&syncmodel.SyncLog{},
		&syncmodel.Instance{},
		// P3 同步框架（迁移 031）：渠道×scope 调度 / 增量游标 / 调价事件 / 差异记录
		&syncmodel.SyncSchedule{},
		&syncmodel.SyncCursor{},
		&syncmodel.PriceChangeEvent{},
		&syncmodel.SyncDiff{},
		// 实例运维管理台：操作流水（见 docs/实施计划/61-实例运维管理台实施计划.md）
		&instancemodel.Operation{},
		// 产品管理（面向终端售卖）
		&categorymodel.ProductCategory{},
		&catalogmodel.Product{},
		&catalogmodel.ProductSpec{},
		&catalogmodel.ProductHistory{},
		&catalogmodel.ProductConfigOption{},
		&catalogmodel.ProductConfigOptionSub{},
		// 产品管理-规格管理（spec 子域）
		&specmodel.SpecTemplate{},
		&specmodel.SpecMapping{},
		// 规格契约（P2/T2.5）：原子字典 / 外部规格快照 / 双向绑定
		&specmodel.SpecAtom{},
		&specmodel.ExternalSpec{},
		&specmodel.SpecBinding{},
		// 产品管理-定价与计费（pricing 子域）
		&pricingmodel.ProductPricing{},
		// 产品管理-折扣策略（P5-01 统一算价管线）
		&discountmodel.PricePolicy{},
		&discountmodel.PricePolicyItem{},
		// 产品管理-促销管理（promotion 子域）
		&promotionmodel.Coupon{},
		&promotionmodel.CouponGrant{},
		&promotionmodel.Promotion{},
		// 订单管理
		&ordermodel.Order{},
		&ordermodel.OrderItem{},
		&ordermodel.OrderRefund{},
		// 工单支持（doc50）
		&ticketmodel.Ticket{},
		&ticketmodel.TicketReply{},
		&ticketmodel.TicketCategory{},
		&ticketmodel.TicketAttachment{},
		// 工单操作日志时间线（P2-02）
		&ticketmodel.TicketLog{},
		// 财务管理
		&finaccountmodel.WalletAccount{},
		&fintransmodel.WalletTransaction{},
		&finrechmodel.Recharge{},
		&finwithdrawmodel.Withdraw{},
		&finbillmodel.Bill{},
		// 系统管理（系统配置）
		&systemmodel.SystemConfig{},
		// 生命周期与续费（doc60）
		&lifecyclemodel.InstanceRenewal{},
		&lifecyclemodel.LifecyclePolicy{},
		&lifecyclemodel.InstanceAutoRenewal{},
		// 通知与消息中心（doc70）
		&notifymodel.Notification{},
		&notifymodel.Announcement{},
		&notifymodel.NotificationTemplate{},
		&notifymodel.NotificationRead{},
		&notifymodel.NotificationPreference{},
	); err != nil {
		return err
	}

	// 旧 user_tickets 数据一次性迁移至新 tickets 表（doc50 §6.6）
	if err := migrateLegacyTickets(db); err != nil {
		return err
	}

	// admins.role（单字符串）→ admin_roles 关联表回填（P1-01，等价 migrations/018）
	if err := backfillAdminRoles(db); err != nil {
		return err
	}

	// 商品供货模式回填：新增列后旧记录 provision_mode 可能为空，统一归一为 self（自营），
	// 避免空模式导致订单履约无法解析供货模式（GORM AutoMigrate 只加列不写默认值）。
	if err := backfillProductProvisionMode(db); err != nil {
		return err
	}

	return nil
}

// backfillAdminRoles 将存量 admins.role 按 roles.code 灌入 admin_roles。
// 幂等：仅对尚无任何角色关联的管理员补齐，不覆盖已有多角色绑定（R4）。
func backfillAdminRoles(db *gorm.DB) error {
	if !db.Migrator().HasTable("admin_roles") || !db.Migrator().HasTable("admins") || !db.Migrator().HasTable("roles") {
		return nil
	}
	return db.Exec(`INSERT INTO admin_roles (admin_id, role_id)
		SELECT a.id, r.id FROM admins a JOIN roles r ON r.code = a.role
		WHERE a.role <> ''
		  AND NOT EXISTS (SELECT 1 FROM admin_roles ar WHERE ar.admin_id = a.id)
		ON CONFLICT DO NOTHING`).Error
}

// backfillProductProvisionMode 回填存量商品的供货模式：空值/未知值一律归一。
// 关键修正（地雷 L5）：不能无脑归一为 self——已绑定上游渠道（source_provider_id != 0）
// 或已绑定上游商品（source_product_id != 0）的存量记录是上游克隆商品，应归为 clone，
// 否则会被误标为自营、进而在双链路判据 source_mode 上错分。
func backfillProductProvisionMode(db *gorm.DB) error {
	return db.Exec(
		`UPDATE products
		    SET provision_mode = CASE
		        WHEN COALESCE(source_provider_id, 0) <> 0 OR COALESCE(source_product_id, 0) <> 0 THEN 'clone'
		        ELSE 'self'
		    END
		  WHERE provision_mode IS NULL OR provision_mode = '' OR provision_mode NOT IN ('self', 'clone')`,
	).Error
}

// migrateLegacyTickets 将旧 user_tickets 表数据一次性迁移至新 tickets 表（doc50 §6.6）。
// 策略：新表为空且旧表存在数据时执行迁移，旧状态值映射为新状态机枚举
// （processing→in_progress、waiting→waiting_user），迁移完成后旧表重命名为 user_tickets_legacy 归档。
func migrateLegacyTickets(db *gorm.DB) error {
	if !db.Migrator().HasTable("user_tickets") {
		return nil
	}
	var ticketCount int64
	if err := db.Table("tickets").Count(&ticketCount).Error; err != nil {
		return err
	}
	if ticketCount > 0 {
		return nil // 新表已有数据，跳过迁移
	}
	var legacyCount int64
	if err := db.Table("user_tickets").Count(&legacyCount).Error; err != nil {
		return err
	}
	if legacyCount == 0 {
		return nil
	}

	var legacy []usermodel.UserTicket
	if err := db.Table("user_tickets").Find(&legacy).Error; err != nil {
		return err
	}
	for _, item := range legacy {
		status := item.Status
		switch status {
		case "processing":
			status = ticketmodel.TicketStatusInProgress
		case "waiting":
			status = ticketmodel.TicketStatusWaitingUser
		case "pending":
			status = ticketmodel.TicketStatusOpen
		case "done":
			status = ticketmodel.TicketStatusResolved
		}
		record := ticketmodel.Ticket{
			TicketNo:  item.TicketNo,
			UserID:    item.UserID,
			Title:     item.Title,
			Category:  item.Category,
			Priority:  item.Priority,
			Status:    status,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		}
		if err := db.Create(&record).Error; err != nil {
			return err
		}
	}
	// 旧表归档重命名
	return db.Migrator().RenameTable("user_tickets", "user_tickets_legacy")
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
		if err := seedSystemConfigs(tx); err != nil {
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
		if err := seedTicketCategories(tx); err != nil {
			return err
		}
		if err := seedDemoSecurity(tx); err != nil {
			return err
		}
		if err := seedUserLevels(tx); err != nil {
			return err
		}
		if err := seedDefaultUserGroup(tx); err != nil {
			return err
		}
		if err := backfillUserConsumeTotals(tx); err != nil {
			return err
		}
		if err := seedDemoVerification(tx); err != nil {
			return err
		}
		if err := seedUpstreamData(tx); err != nil {
			return err
		}
		if err := seedDemoOrders(tx); err != nil {
			return err
		}
		if err := seedDemoFinance(tx); err != nil {
			return err
		}
		if err := seedNotificationTemplates(tx); err != nil {
			return err
		}
		if err := seedSMTPConfigs(tx); err != nil {
			return err
		}
		return nil
	})
}

// seedUpstreamData 为资源管理模块写入演示业务数据（提供商/资源池/商品/实例/同步任务/日志）。
// 幂等：仅当尚无任何上游提供商时写入，避免重复与覆盖真实运营数据。
func seedUpstreamData(tx *gorm.DB) error {
	var providerCount int64
	if err := tx.Model(&providermodel.ResourceProvider{}).Count(&providerCount).Error; err != nil {
		return err
	}
	if providerCount > 0 {
		return nil
	}

	now := time.Now()
	timePtr := func(t time.Time) *time.Time { return &t }
	lastSync := now.Add(-30 * time.Minute)

	// —— 上游提供商 ——
	providers := []providermodel.ResourceProvider{
		{Name: "魔方云·华东旗舰", ProviderType: "mofangyun", APIEndpoint: "https://api.mofangyun.com/v3", APIKey: "seed_mfy_key_east", APISecret: "seed_mfy_secret_east", Region: "华东", Status: 1, SyncEnabled: true, SyncInterval: 1800, LastSyncAt: timePtr(lastSync), TotalCPU: 640, TotalMemory: 4096, TotalDisk: 200000, UsedCPU: 420, UsedMemory: 2710, UsedDisk: 132500},
		{Name: "华东 OpenStack 主池", ProviderType: "openstack", APIEndpoint: "https://ostack-east.hostsent.cn/v3", APIKey: "seed_os_key", APISecret: "seed_os_secret", Region: "华东", Status: 1, SyncEnabled: true, SyncInterval: 3600, LastSyncAt: timePtr(now.Add(-2 * time.Hour)), TotalCPU: 1280, TotalMemory: 8192, TotalDisk: 400000, UsedCPU: 990, UsedMemory: 6150, UsedDisk: 287000},
		{Name: "华南 Proxmox 集群", ProviderType: "proxmox", APIEndpoint: "https://pxm-south.hostsent.cn:8006", APIKey: "seed_pxm_key", APISecret: "seed_pxm_secret", Region: "华南", Status: 1, SyncEnabled: true, SyncInterval: 3600, LastSyncAt: timePtr(now.Add(-55 * time.Minute)), TotalCPU: 512, TotalMemory: 3072, TotalDisk: 150000, UsedCPU: 318, UsedMemory: 2020, UsedDisk: 96500},
		{Name: "AWS EC2 全球", ProviderType: "aws", APIEndpoint: "https://ec2.ap-southeast-1.amazonaws.com", APIKey: "seed_aws_key", APISecret: "seed_aws_secret", Region: "海外", Status: 1, SyncEnabled: false, SyncInterval: 86400, LastSyncAt: nil, TotalCPU: 256, TotalMemory: 1024, TotalDisk: 50000, UsedCPU: 0, UsedMemory: 0, UsedDisk: 0},
		{Name: "阿里云 ECS", ProviderType: "aliyun", APIEndpoint: "https://ecs.cn-shanghai.aliyuncs.com", APIKey: "seed_ali_key", APISecret: "seed_ali_secret", Region: "华东", Status: 0, SyncEnabled: true, SyncInterval: 3600, LastSyncAt: nil, TotalCPU: 96, TotalMemory: 384, TotalDisk: 20000, UsedCPU: 0, UsedMemory: 0, UsedDisk: 0},
	}

	var providerIDs []uint64
	for i := range providers {
		if err := tx.Create(&providers[i]).Error; err != nil {
			return err
		}
		providerIDs = append(providerIDs, providers[i].ID)
	}

	// —— 资源池 ——
	pools := []providermodel.ResourcePool{
		{ProviderID: providerIDs[0], UpstreamID: "pool_mfy_east_01", Name: "魔方云·通用计算池", PoolType: "compute", TotalCPU: 320, TotalMemory: 2048, TotalDisk: 100000, UsedCPU: 218, UsedMemory: 1420, UsedDisk: 66500, Status: 1, LastSyncAt: timePtr(lastSync)},
		{ProviderID: providerIDs[0], UpstreamID: "pool_mfy_east_02", Name: "魔方云·高内存池", PoolType: "memory", TotalCPU: 320, TotalMemory: 2048, TotalDisk: 100000, UsedCPU: 202, UsedMemory: 1290, UsedDisk: 66000, Status: 1, LastSyncAt: timePtr(lastSync)},
		{ProviderID: providerIDs[1], UpstreamID: "pool_os_east_01", Name: "华东 OpenStack 主池", PoolType: "compute", TotalCPU: 640, TotalMemory: 4096, TotalDisk: 200000, UsedCPU: 496, UsedMemory: 3080, UsedDisk: 143500, Status: 1, LastSyncAt: timePtr(now.Add(-2 * time.Hour))},
		{ProviderID: providerIDs[1], UpstreamID: "pool_os_east_02", Name: "华东 OpenStack 高密池", PoolType: "compute", TotalCPU: 640, TotalMemory: 4096, TotalDisk: 200000, UsedCPU: 494, UsedMemory: 3070, UsedDisk: 143500, Status: 0, LastSyncAt: timePtr(now.Add(-2 * time.Hour))},
		{ProviderID: providerIDs[2], UpstreamID: "pool_pxm_south_01", Name: "华南 Proxmox 集群", PoolType: "compute", TotalCPU: 512, TotalMemory: 3072, TotalDisk: 150000, UsedCPU: 318, UsedMemory: 2020, UsedDisk: 96500, Status: 1, LastSyncAt: timePtr(now.Add(-55 * time.Minute))},
		{ProviderID: providerIDs[3], UpstreamID: "pool_aws_sg_01", Name: "AWS 新加坡计算池", PoolType: "compute", TotalCPU: 256, TotalMemory: 1024, TotalDisk: 50000, UsedCPU: 0, UsedMemory: 0, UsedDisk: 0, Status: 1, LastSyncAt: nil},
	}
	for i := range pools {
		if err := tx.Create(&pools[i]).Error; err != nil {
			return err
		}
	}

	// —— 上游商品 ——
	products := []productmodel.ResourceProduct{
		{ProviderID: providerIDs[0], UpstreamID: "sku_mfy_c2m4d50", Name: "魔方云 标准型 c2-m4-d50", CPU: 2, Memory: 4, Disk: 50, DiskType: "ssd", Bandwidth: 5, OS: "CentOS 7.9", Region: "华东", Zone: "east-01", CostPrice: 42.50, SalePrice: 58.00, Status: 1},
		{ProviderID: providerIDs[0], UpstreamID: "sku_mfy_c4m8d100", Name: "魔方云 性能型 c4-m8-d100", CPU: 4, Memory: 8, Disk: 100, DiskType: "ssd", Bandwidth: 10, OS: "Ubuntu 22.04", Region: "华东", Zone: "east-01", CostPrice: 82.00, SalePrice: 118.00, Status: 1},
		{ProviderID: providerIDs[0], UpstreamID: "sku_mfy_c8m16d200", Name: "魔方云 内存型 c8-m16-d200", CPU: 8, Memory: 16, Disk: 200, DiskType: "ssd", Bandwidth: 20, OS: "Debian 12", Region: "华东", Zone: "east-02", CostPrice: 168.00, SalePrice: 236.00, Status: 1},
		{ProviderID: providerIDs[1], UpstreamID: "sku_os_c2m2d40", Name: "OpenStack 入门型 c2-m2-d40", CPU: 2, Memory: 2, Disk: 40, DiskType: "hdd", Bandwidth: 3, OS: "CentOS 7.9", Region: "华东", Zone: "east-01", CostPrice: 28.00, SalePrice: 39.90, Status: 1},
		{ProviderID: providerIDs[1], UpstreamID: "sku_os_c4m8d80", Name: "OpenStack 平衡型 c4-m8-d80", CPU: 4, Memory: 8, Disk: 80, DiskType: "ssd", Bandwidth: 5, OS: "Ubuntu 22.04", Region: "华东", Zone: "east-01", CostPrice: 66.00, SalePrice: 92.00, Status: 1},
		{ProviderID: providerIDs[2], UpstreamID: "sku_pxm_c4m8d120", Name: "Proxmox 高能型 c4-m8-d120", CPU: 4, Memory: 8, Disk: 120, DiskType: "ssd", Bandwidth: 10, OS: "Debian 12", Region: "华南", Zone: "south-01", CostPrice: 88.00, SalePrice: 129.00, Status: 1},
		{ProviderID: providerIDs[3], UpstreamID: "sku_aws_t3medium", Name: "AWS t3.medium (美国西部)", CPU: 2, Memory: 4, Disk: 80, DiskType: "ssd", Bandwidth: 0, OS: "Amazon Linux 2", Region: "海外", Zone: "us-west-2a", CostPrice: 68.00, SalePrice: 96.00, Status: 1},
	}
	for i := range products {
		specJSON := fmt.Sprintf(`{"cpu":%d,"memory":%d,"disk":%d,"disk_type":%q,"bandwidth":%d,"os":%q,"region":%q,"zone":%q}`, products[i].CPU, products[i].Memory, products[i].Disk, products[i].DiskType, products[i].Bandwidth, products[i].OS, products[i].Region, products[i].Zone)
		products[i].Specs = specJSON
		products[i].RawSpecs = specJSON
		if err := tx.Create(&products[i]).Error; err != nil {
			return err
		}
	}

	// —— 云主机实例 ——
	instances := []syncmodel.Instance{
		{InstanceID: "i-mfy-a1b2c3d4", ProviderID: providerIDs[0], UserID: 1, ProductID: 1, Name: "web-prod-01", CPU: 2, Memory: 4, Disk: 50, DiskType: "ssd", Bandwidth: 5, OS: "CentOS 7.9", Region: "华东", Zone: "east-01", Status: "running", PrivateIP: "10.0.1.11", PublicIP: "118.31.10.21", BillingMode: "monthly"},
		{InstanceID: "i-mfy-e5f6a7b8", ProviderID: providerIDs[0], UserID: 2, ProductID: 2, Name: "app-worker-02", CPU: 4, Memory: 8, Disk: 100, DiskType: "ssd", Bandwidth: 10, OS: "Ubuntu 22.04", Region: "华东", Zone: "east-01", Status: "running", PrivateIP: "10.0.1.12", PublicIP: "118.31.10.22", BillingMode: "monthly"},
		{InstanceID: "i-mfy-c9d0e1f2", ProviderID: providerIDs[0], UserID: 3, ProductID: 3, Name: "db-primary-03", CPU: 8, Memory: 16, Disk: 200, DiskType: "ssd", Bandwidth: 20, OS: "Debian 12", Region: "华东", Zone: "east-02", Status: "running", PrivateIP: "10.0.2.11", PublicIP: "118.31.10.23", BillingMode: "monthly"},
		{InstanceID: "i-os-0a1b2c3d", ProviderID: providerIDs[1], UserID: 4, ProductID: 4, Name: "test-node-04", CPU: 2, Memory: 2, Disk: 40, DiskType: "hdd", Bandwidth: 3, OS: "CentOS 7.9", Region: "华东", Zone: "east-01", Status: "stopped", PrivateIP: "10.0.3.11", PublicIP: "118.31.11.21", BillingMode: "hourly"},
		{InstanceID: "i-pxm-1a2b3c4d", ProviderID: providerIDs[2], UserID: 5, ProductID: 6, Name: "build-runner-05", CPU: 4, Memory: 8, Disk: 120, DiskType: "ssd", Bandwidth: 10, OS: "Debian 12", Region: "华南", Zone: "south-01", Status: "error", PrivateIP: "10.0.4.11", PublicIP: "120.24.12.21", BillingMode: "monthly"},
		{InstanceID: "i-aws-5a6b7c8d", ProviderID: providerIDs[3], UserID: 6, ProductID: 7, Name: "us-www-06", CPU: 2, Memory: 4, Disk: 80, DiskType: "ssd", Bandwidth: 0, OS: "Amazon Linux 2", Region: "海外", Zone: "us-west-2a", Status: "running", PrivateIP: "172.31.0.16", PublicIP: "54.215.10.20", BillingMode: "hourly"},
		{InstanceID: "i-ali-9e8f7a6b", ProviderID: providerIDs[4], UserID: 7, ProductID: 0, Name: "snapshot-legacy-07", CPU: 2, Memory: 4, Disk: 60, DiskType: "ssd", Bandwidth: 5, OS: "CentOS 7.9", Region: "华东", Zone: "cn-shanghai-b", Status: "deleted", PrivateIP: "", PublicIP: "", BillingMode: "hourly"},
	}
	for i := range instances {
		if err := tx.Create(&instances[i]).Error; err != nil {
			return err
		}
	}

	// —— 同步任务与日志 ——
	tasks := []syncmodel.SyncTask{
		{ProviderID: providerIDs[0], TaskType: "product", Status: "success", TotalCount: 3, SuccessCount: 3, StartedAt: timePtr(now.Add(-40 * time.Minute)), CompletedAt: timePtr(now.Add(-38 * time.Minute))},
		{ProviderID: providerIDs[0], TaskType: "instance", Status: "success", TotalCount: 3, SuccessCount: 3, StartedAt: timePtr(now.Add(-38 * time.Minute)), CompletedAt: timePtr(now.Add(-37 * time.Minute))},
		{ProviderID: providerIDs[1], TaskType: "product", Status: "success", TotalCount: 2, SuccessCount: 2, StartedAt: timePtr(now.Add(-2 * time.Hour)), CompletedAt: timePtr(now.Add(-115 * time.Minute))},
		{ProviderID: providerIDs[2], TaskType: "product", Status: "failed", TotalCount: 1, SuccessCount: 0, ErrorMessage: "上游 HTTP 500：服务端临时不可用", StartedAt: timePtr(now.Add(-30 * time.Minute)), CompletedAt: timePtr(now.Add(-29 * time.Minute))},
		{ProviderID: providerIDs[2], TaskType: "instance", Status: "failed", TotalCount: 1, SuccessCount: 0, ErrorMessage: "上游鉴权失败：无效的 API 密钥", StartedAt: timePtr(now.Add(-28 * time.Minute)), CompletedAt: timePtr(now.Add(-27 * time.Minute))},
		{ProviderID: providerIDs[3], TaskType: "pool", Status: "skipped", TotalCount: 0, SuccessCount: 0, ErrorMessage: ""},
	}
	for i := range tasks {
		if err := tx.Create(&tasks[i]).Error; err != nil {
			return err
		}
	}

	logs := []syncmodel.SyncLog{
		{TaskID: tasks[0].ID, ProviderID: providerIDs[0], SyncType: "product", Status: "success", TotalCount: 3, SuccessCount: 3, Details: `{"synced":3,"skipped":0,"n":["sku_mfy_c2m4d50","sku_mfy_c4m8d100","sku_mfy_c8m16d200"]}`},
		{TaskID: tasks[1].ID, ProviderID: providerIDs[0], SyncType: "instance", Status: "success", TotalCount: 3, SuccessCount: 3, Details: `{"synced":3,"skipped":0}`},
		{TaskID: tasks[2].ID, ProviderID: providerIDs[1], SyncType: "product", Status: "success", TotalCount: 2, SuccessCount: 2, Details: `{"synced":2,"skipped":0}`},
		{TaskID: tasks[3].ID, ProviderID: providerIDs[2], SyncType: "product", Status: "failed", TotalCount: 1, SuccessCount: 0, ErrorMessage: "上游 HTTP 500：服务端临时不可用", Details: `{"synced":0,"failed":1,"reason":"http 500"}`},
		{TaskID: tasks[4].ID, ProviderID: providerIDs[2], SyncType: "instance", Status: "failed", TotalCount: 1, SuccessCount: 0, ErrorMessage: "上游鉴权失败：无效的 API 密钥", Details: `{"synced":0,"failed":1,"reason":"auth"}`},
	}
	for i := range logs {
		if err := tx.Create(&logs[i]).Error; err != nil {
			return err
		}
	}

	return nil
}

// seedDemoOrders 为订单管理模块写入演示订单/明细/退款数据。
// 幂等：仅当 orders 表尚无数据时写入。
func seedDemoOrders(tx *gorm.DB) error {
	var count int64
	if err := tx.Model(&ordermodel.Order{}).Count(&count).Error; err != nil {
		return err
	}
	// 取真实用户 ID 用于演示归属，避免依赖固定用户名
	usernames := []string{"user_nw_01", "user_east_01", "user_north_01", "user_south_01"}
	userIDs := make(map[string]uint64, len(usernames))
	var realIDs []uint64
	if err := tx.Model(&usermodel.User{}).Order("id asc").Limit(4).Pluck("id", &realIDs).Error; err != nil {
		return err
	}
	if len(realIDs) == 0 {
		realIDs = []uint64{0}
	}
	for i, uname := range usernames {
		userIDs[uname] = realIDs[i%len(realIDs)]
	}

	// 矫正既有演示数据：补全归属为 0 的订单与退款
	if count > 0 {
		var orphanOrders []ordermodel.Order
		if err := tx.Where("user_id = 0").Order("id asc").Find(&orphanOrders).Error; err != nil {
			return err
		}
		for i := range orphanOrders {
			orphanOrders[i].UserID = realIDs[i%len(realIDs)]
			if err := tx.Save(&orphanOrders[i]).Error; err != nil {
				return err
			}
		}
		var orphanRefunds []ordermodel.OrderRefund
		if err := tx.Where("user_id = 0").Order("id asc").Find(&orphanRefunds).Error; err != nil {
			return err
		}
		for i := range orphanRefunds {
			orphanRefunds[i].UserID = realIDs[i%len(realIDs)]
			if err := tx.Save(&orphanRefunds[i]).Error; err != nil {
				return err
			}
		}
		return nil
	}

	now := time.Now()
	timePtr := func(t time.Time) *time.Time { return &t }
	daysAgo := func(d int) time.Time { return now.Add(-time.Duration(d) * 24 * time.Hour) }

	orders := []ordermodel.Order{
		{OrderNo: "OD202608180031", UserID: userIDs["user_nw_01"], ProductID: 1, ProductName: "高主频云主机 4C8G", Specs: `{"cpu":4,"memory":8,"disk":100}`, Quantity: 1, PriceModel: "monthly", TotalAmount: 688, PaidAmount: 688, Status: ordermodel.OrderStatusActive, PayMethod: ordermodel.PayMethodBalance, PayTime: timePtr(daysAgo(0)), ExpireTime: timePtr(now.AddDate(0, 1, 0)), CreatedAt: daysAgo(0), UpdatedAt: daysAgo(0)},
		{OrderNo: "OD202608120007", UserID: userIDs["user_east_01"], ProductID: 2, ProductName: "云主机续费 2C4G", Specs: `{"cpu":2,"memory":4,"disk":50}`, Quantity: 1, PriceModel: "monthly", TotalAmount: 366, PaidAmount: 366, Status: ordermodel.OrderStatusPaid, PayMethod: ordermodel.PayMethodAlipay, PayTime: timePtr(daysAgo(1)), CreatedAt: daysAgo(1), UpdatedAt: daysAgo(1)},
		{OrderNo: "OD202608100018", UserID: userIDs["user_east_01"], ProductID: 3, ProductName: "对象存储流量包", Specs: `{"type":"traffic","size":100}`, Quantity: 1, PriceModel: "fixed", TotalAmount: 199, PaidAmount: 0, Status: ordermodel.OrderStatusPending, PayMethod: "", CreatedAt: daysAgo(2), UpdatedAt: daysAgo(2)},
		{OrderNo: "OD202608050002", UserID: userIDs["user_north_01"], ProductID: 4, ProductName: "云主机 4C8G 华东一区", Specs: `{"cpu":4,"memory":8,"disk":200}`, Quantity: 1, PriceModel: "monthly", TotalAmount: 1280, PaidAmount: 1280, Status: ordermodel.OrderStatusPaid, PayMethod: ordermodel.PayMethodWeChat, PayTime: timePtr(daysAgo(5)), CreatedAt: daysAgo(5), UpdatedAt: daysAgo(5)},
		{OrderNo: "OD202608010003", UserID: userIDs["user_south_01"], ProductID: 5, ProductName: "云主机 2C2G 华南一区", Specs: `{"cpu":2,"memory":2,"disk":40}`, Quantity: 1, PriceModel: "monthly", TotalAmount: 39.90, PaidAmount: 39.90, Status: ordermodel.OrderStatusCancelled, PayMethod: ordermodel.PayMethodManual, PayTime: timePtr(daysAgo(8)), CreatedAt: daysAgo(8), UpdatedAt: daysAgo(8)},
		{OrderNo: "OD202607280009", UserID: userIDs["user_nw_01"], ProductID: 6, ProductName: "企业级云主机 8C16G", Specs: `{"cpu":8,"memory":16,"disk":400}`, Quantity: 1, PriceModel: "monthly", TotalAmount: 2360, PaidAmount: 2360, Status: ordermodel.OrderStatusRefunding, PayMethod: ordermodel.PayMethodAlipay, PayTime: timePtr(daysAgo(10)), CreatedAt: daysAgo(10), UpdatedAt: daysAgo(2)},
		{OrderNo: "OD202607200011", UserID: userIDs["user_north_01"], ProductID: 7, ProductName: "VPS 2C4G", Specs: `{"cpu":2,"memory":4,"disk":60}`, Quantity: 1, PriceModel: "monthly", TotalAmount: 96, PaidAmount: 96, Status: ordermodel.OrderStatusRefunded, PayMethod: ordermodel.PayMethodBalance, PayTime: timePtr(daysAgo(12)), CreatedAt: daysAgo(12), UpdatedAt: daysAgo(11)},
	}

	for i := range orders {
		if err := tx.Create(&orders[i]).Error; err != nil {
			return err
		}
		item := ordermodel.OrderItem{
			OrderID: orders[i].ID, ProductID: orders[i].ProductID, ProductName: orders[i].ProductName,
			SpecCode: fmt.Sprintf("SPEC-%d", orders[i].ProductID), Specs: orders[i].Specs,
			Price: orders[i].TotalAmount, Quantity: orders[i].Quantity, Amount: orders[i].TotalAmount,
		}
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
	}

	refunds := []ordermodel.OrderRefund{
		{RefundNo: "RF202607280001", OrderID: orders[5].ID, UserID: orders[5].UserID, Amount: 1180, Reason: "业务调整，申请部分退款", Status: ordermodel.RefundStatusPending},
		{RefundNo: "RF202607200001", OrderID: orders[6].ID, UserID: orders[6].UserID, Amount: 96, Reason: "申请退货退款", Status: ordermodel.RefundStatusApproved, AuditBy: 1, AuditByName: "admin", AuditedAt: timePtr(daysAgo(11))},
	}
	for i := range refunds {
		if err := tx.Create(&refunds[i]).Error; err != nil {
			return err
		}
	}

	return nil
}

// seedDemoFinance 为财务管理模块写入演示数据（钱包账户/流水/充值单/提现单/账单）。
// 幂等：仅当 wallet_accounts 尚无数据时写入；余额与流水保持一致（净变动 = 账户余额）。
func seedDemoFinance(tx *gorm.DB) error {
	var count int64
	if err := tx.Model(&finaccountmodel.WalletAccount{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	var userIDs []uint64
	if err := tx.Model(&usermodel.User{}).Order("id asc").Limit(4).Pluck("id", &userIDs).Error; err != nil {
		return err
	}
	if len(userIDs) == 0 {
		return nil
	}

	now := time.Now()
	currPeriod := now.Format("200601")

	for i, uid := range userIDs {
		balance, income, expense := 0.0, 0.0, 0.0
		var txs []fintransmodel.WalletTransaction

		appendTx := func(txType string, dir int, amount float64, refNo, biz string) {
			before := balance
			balance += float64(dir) * amount
			if dir > 0 {
				income += amount
			} else {
				expense += amount
			}
			txs = append(txs, fintransmodel.WalletTransaction{
				TxNo:          fmt.Sprintf("SEEDW%02d%02d", i, len(txs)),
				UserID:        uid,
				Type:          txType,
				Direction:     dir,
				Amount:        amount,
				BalanceBefore: before,
				BalanceAfter:  balance,
				RefNo:         refNo,
				BizType:       biz,
				Remark:        "演示数据",
				CreatedAt:     now.Add(-time.Duration(i) * time.Hour),
			})
		}

		appendTx(fintransmodel.TxTypeRecharge, 1, 50, fmt.Sprintf("RCDEMO%03d", i), "recharge")
		appendTx(fintransmodel.TxTypeConsume, -1, 30, fmt.Sprintf("ODDEMO%03d", i), "consume")
		appendTx(fintransmodel.TxTypeRefund, 1, 10, fmt.Sprintf("RFDEMO%03d", i), "refund")

		acc := &finaccountmodel.WalletAccount{UserID: uid, Balance: balance, Frozen: 0, TotalIncome: income, TotalExpense: expense, Version: 1}
		if err := tx.Create(acc).Error; err != nil {
			return err
		}
		for k := range txs {
			if err := tx.Create(&txs[k]).Error; err != nil {
				return err
			}
		}

		rc := &finrechmodel.Recharge{
			RechargeNo: fmt.Sprintf("RCDEMO%03d", i), UserID: uid, Amount: 50, Method: "manual",
			Status: finrechmodel.RechargeStatusSuccess, ChannelTx: fmt.Sprintf("channel-%d", i),
			PaidAt: &now, Remark: "演示充值",
		}
		if err := tx.Create(rc).Error; err != nil {
			return err
		}

		wd := &finwithdrawmodel.Withdraw{
			WithdrawNo: fmt.Sprintf("WDDEMO%03d", i), UserID: uid, Amount: 20, Channel: "bank",
			Account: "622202****0001", Status: finwithdrawmodel.WithdrawStatusPending, Remark: "演示提现",
		}
		if err := tx.Create(wd).Error; err != nil {
			return err
		}

		bill := &finbillmodel.Bill{
			BillNo: fmt.Sprintf("BILLDEMO%03d", i), UserID: uid, Period: currPeriod,
			TotalAmount: 20, RefundAmount: 10, Status: finbillmodel.BillStatusUnpaid,
			Detail: `{"consume":30,"refund":10}`,
		}
		if err := tx.Create(bill).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedSystemConfigs 为系统配置模块写入默认配置项。
// 幂等：按 config_key 查重，已存在则跳过，不覆盖运营期修改。
func seedSystemConfigs(tx *gorm.DB) error {
	defaults := []systemmodel.SystemConfig{
		{ConfigKey: "site_name", ConfigValue: "HostSent 云主机管理系统", ValueType: systemmodel.ValueTypeString, Group: systemmodel.ConfigGroupSite, Description: "站点名称", SortOrder: 1, Status: systemmodel.StatusActive},
		{ConfigKey: "default_billing_cycle", ConfigValue: "monthly", ValueType: systemmodel.ValueTypeString, Group: systemmodel.ConfigGroupBilling, Description: "默认计费周期", SortOrder: 1, Status: systemmodel.StatusActive},
		{ConfigKey: "enable_user_register", ConfigValue: "true", ValueType: systemmodel.ValueTypeBool, Group: systemmodel.ConfigGroupFeature, Description: "是否开放用户注册", SortOrder: 1, Status: systemmodel.StatusActive},
		{ConfigKey: "enable_mfa_required", ConfigValue: "false", ValueType: systemmodel.ValueTypeBool, Group: systemmodel.ConfigGroupSecurity, Description: "是否强制管理员开启MFA", SortOrder: 1, Status: systemmodel.StatusActive},
		{ConfigKey: "order_expire_minutes", ConfigValue: "30", ValueType: systemmodel.ValueTypeInt, Group: systemmodel.ConfigGroupOrder, Description: "待支付订单过期时间(分钟)", SortOrder: 1, Status: systemmodel.StatusActive},
		// 推广邀请返现：全局三档比率 + 最低提现金额 + 开关（返现模块运行时读取）
		{ConfigKey: referralmodel.ConfigKeyEnabled, ConfigValue: "true", ValueType: systemmodel.ValueTypeBool, Group: systemmodel.ConfigGroupReferral, Description: "启用推广邀请返现", SortOrder: 1, Status: systemmodel.StatusActive},
		{ConfigKey: referralmodel.ConfigKeyFirstOrder, ConfigValue: "0.10", ValueType: systemmodel.ValueTypeString, Group: systemmodel.ConfigGroupReferral, Description: "首单返现比率（0-1）", SortOrder: 2, Status: systemmodel.StatusActive},
		{ConfigKey: referralmodel.ConfigKeySubsequent, ConfigValue: "0.05", ValueType: systemmodel.ValueTypeString, Group: systemmodel.ConfigGroupReferral, Description: "后续订单返现比率（0-1）", SortOrder: 3, Status: systemmodel.StatusActive},
		{ConfigKey: referralmodel.ConfigKeyRenewal, ConfigValue: "0.03", ValueType: systemmodel.ValueTypeString, Group: systemmodel.ConfigGroupReferral, Description: "续费返现比率（0-1）", SortOrder: 4, Status: systemmodel.StatusActive},
		{ConfigKey: referralmodel.ConfigKeyMinWithdraw, ConfigValue: "50", ValueType: systemmodel.ValueTypeString, Group: systemmodel.ConfigGroupReferral, Description: "返现最低提现金额（元）", SortOrder: 5, Status: systemmodel.StatusActive},
	}
	for _, config := range defaults {
		var existing systemmodel.SystemConfig
		if err := tx.Where("config_key = ?", config.ConfigKey).First(&existing).Error; err == nil {
			continue
		} else if err != gorm.ErrRecordNotFound {
			return err
		}
		if err := tx.Create(&config).Error; err != nil {
			return err
		}
	}
	return nil
}

func seedRoles(tx *gorm.DB) error {
	defaults := []usermodel.Role{
		{Code: "super_admin", Name: "超级管理员", Scope: usermodel.RoleScopeAdmin, Status: "active"},
		{Code: "ops_admin", Name: "运维管理员", Scope: usermodel.RoleScopeAdmin, Status: "active"},
		{Code: "finance_admin", Name: "财务管理员", Scope: usermodel.RoleScopeAdmin, Status: "active"},
		// 客户角色：不进后台权限树（scope=user），本方案不使用，保留兼容历史 seed。
		{Code: "user", Name: "普通用户", Scope: usermodel.RoleScopeUser, Status: "active"},
	}

	for _, role := range defaults {
		var existing usermodel.Role
		if err := tx.Where("code = ?", role.Code).First(&existing).Error; err == nil {
			// 幂等：补齐缺失的 scope，避免老库升级后后台权限树混入客户角色。
			if existing.Scope == "" {
				if err := tx.Model(&existing).Update("scope", role.Scope).Error; err != nil {
					return err
				}
			}
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
		// —— 系统配置（系统管理模块）
		{Name: "系统配置", Code: "system:config", Type: "menu", SortOrder: 4, Status: "active"},
		{ParentCode: "system:config", Name: "查看配置", Code: "system:config:view", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "system:config", Name: "创建配置", Code: "system:config:create", Type: "button", SortOrder: 2, Status: "active"},
		{ParentCode: "system:config", Name: "更新配置", Code: "system:config:update", Type: "button", SortOrder: 3, Status: "active"},
		{ParentCode: "system:config", Name: "删除配置", Code: "system:config:delete", Type: "button", SortOrder: 4, Status: "active"},
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
		{Name: "上游对接", Code: "resource", Type: "catalog", SortOrder: 4, Status: "active"},
		{ParentCode: "resource", Name: "上游提供商", Code: "resource:provider", Type: "menu", SortOrder: 1, Status: "active"},
		{ParentCode: "resource:provider", Name: "创建提供商", Code: "provider:create", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "resource:provider", Name: "更新提供商", Code: "provider:update", Type: "button", SortOrder: 2, Status: "active"},
		{ParentCode: "resource:provider", Name: "删除提供商", Code: "provider:delete", Type: "button", SortOrder: 3, Status: "active"},
		{ParentCode: "resource:provider", Name: "测试连接", Code: "provider:test", Type: "button", SortOrder: 4, Status: "active"},
		{ParentCode: "resource", Name: "上游商品", Code: "resource:product", Type: "menu", SortOrder: 2, Status: "active"},
		{ParentCode: "resource:product", Name: "更新商品定价", Code: "product:update_price", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "resource:product", Name: "同步商品", Code: "product:sync", Type: "button", SortOrder: 2, Status: "active"},
		{ParentCode: "resource", Name: "同步任务", Code: "resource:sync", Type: "menu", SortOrder: 3, Status: "active"},
		{ParentCode: "resource:sync", Name: "创建同步任务", Code: "sync:create", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "resource:sync", Name: "查看同步日志", Code: "sync:log", Type: "button", SortOrder: 2, Status: "active"},
		// P3 同步框架：调度配置 / 调价待确认（L2：新权限码必须登记，否则菜单会被过滤）
		{ParentCode: "resource:sync", Name: "同步调度配置", Code: "sync:schedule", Type: "button", SortOrder: 3, Status: "active"},
		{ParentCode: "resource:sync", Name: "查看调价事件", Code: "sync:price", Type: "button", SortOrder: 4, Status: "active"},
		{ParentCode: "resource:sync", Name: "确认调价", Code: "sync:price:confirm", Type: "button", SortOrder: 5, Status: "active"},
		{ParentCode: "resource", Name: "云主机", Code: "resource:instance", Type: "menu", SortOrder: 4, Status: "active"},
		{ParentCode: "resource:instance", Name: "实例操作", Code: "instance:action", Type: "button", SortOrder: 1, Status: "active"},
		// 实例运维台敏感动作细分权限（见 docs/实施计划/61-实例运维管理台实施计划.md §6.1）
		{ParentCode: "resource:instance", Name: "远程控制台", Code: "instance:console", Type: "button", SortOrder: 2, Status: "active"},
		{ParentCode: "resource:instance", Name: "实例变配", Code: "instance:resize", Type: "button", SortOrder: 3, Status: "active"},
		{ParentCode: "resource:instance", Name: "销毁实例", Code: "instance:destroy", Type: "button", SortOrder: 4, Status: "active"},
		{Name: "商品销售", Code: "product", Type: "catalog", SortOrder: 5, Status: "active"},
		{ParentCode: "product", Name: "产品列表", Code: "product:list", Type: "menu", SortOrder: 1, Status: "active"},
		{ParentCode: "product:list", Name: "创建产品", Code: "product:create", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "product:list", Name: "编辑产品", Code: "product:update", Type: "button", SortOrder: 2, Status: "active"},
		{ParentCode: "product:list", Name: "删除产品", Code: "product:delete", Type: "button", SortOrder: 3, Status: "active"},
		{ParentCode: "product:list", Name: "上下架产品", Code: "product:publish", Type: "button", SortOrder: 4, Status: "active"},
		{ParentCode: "product", Name: "分类管理", Code: "product:category", Type: "menu", SortOrder: 2, Status: "active"},
		{ParentCode: "product:category", Name: "创建分类", Code: "product:category:create", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "product:category", Name: "编辑分类", Code: "product:category:update", Type: "button", SortOrder: 2, Status: "active"},
		{ParentCode: "product:category", Name: "删除分类", Code: "product:category:delete", Type: "button", SortOrder: 3, Status: "active"},
		{ParentCode: "product", Name: "定价管理", Code: "product:price", Type: "menu", SortOrder: 3, Status: "active"},
		{ParentCode: "product:price", Name: "修改价格", Code: "product:price:update", Type: "button", SortOrder: 1, Status: "active"},
		{Name: "订单管理", Code: "order", Type: "catalog", SortOrder: 6, Status: "active"},
		{ParentCode: "order", Name: "订单列表", Code: "order:list", Type: "menu", SortOrder: 1, Status: "active"},
		{ParentCode: "order:list", Name: "取消订单", Code: "order:cancel", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "order:list", Name: "订单备注", Code: "order:remark", Type: "button", SortOrder: 2, Status: "active"},
		{ParentCode: "order:list", Name: "发起退款", Code: "order:refund", Type: "button", SortOrder: 3, Status: "active"},
		{ParentCode: "order:list", Name: "重新开通", Code: "order:activate", Type: "button", SortOrder: 4, Status: "active"},
		{ParentCode: "order", Name: "退款管理", Code: "order:refunds", Type: "menu", SortOrder: 2, Status: "active"},
		{ParentCode: "order:refunds", Name: "审核退款", Code: "order:refund:audit", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "order", Name: "订单统计", Code: "order:stats", Type: "menu", SortOrder: 3, Status: "active"},
		{Name: "财务管理", Code: "finance", Type: "catalog", SortOrder: 7, Status: "active"},
		{ParentCode: "finance", Name: "钱包/流水", Code: "finance:wallet", Type: "menu", SortOrder: 1, Status: "active"},
		{ParentCode: "finance:wallet", Name: "人工调账", Code: "finance:adjust", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "finance", Name: "充值管理", Code: "finance:recharge", Type: "menu", SortOrder: 2, Status: "active"},
		{ParentCode: "finance:recharge", Name: "确认到账", Code: "finance:recharge:approve", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "finance", Name: "提现管理", Code: "finance:withdraw", Type: "menu", SortOrder: 3, Status: "active"},
		{ParentCode: "finance:withdraw", Name: "审核提现", Code: "finance:withdraw:audit", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "finance", Name: "账单管理", Code: "finance:bill", Type: "menu", SortOrder: 4, Status: "active"},
		{ParentCode: "finance:bill", Name: "关账", Code: "finance:bill:close", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "finance:bill", Name: "对账", Code: "finance:bill:recon", Type: "button", SortOrder: 2, Status: "active"},
		// —— 工单支持（doc50 §7.4）
		{Name: "工单支持", Code: "ticket", Type: "catalog", SortOrder: 8, Status: "active"},
		{ParentCode: "ticket", Name: "工单列表", Code: "ticket:list", Type: "menu", SortOrder: 1, Status: "active"},
		{ParentCode: "ticket:list", Name: "查看工单", Code: "ticket:view", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "ticket:list", Name: "回复工单", Code: "ticket:reply", Type: "button", SortOrder: 2, Status: "active"},
		{ParentCode: "ticket:list", Name: "分配工单", Code: "ticket:assign", Type: "button", SortOrder: 3, Status: "active"},
		{ParentCode: "ticket:list", Name: "更新状态", Code: "ticket:update", Type: "button", SortOrder: 4, Status: "active"},
		{ParentCode: "ticket:list", Name: "关闭工单", Code: "ticket:close", Type: "button", SortOrder: 5, Status: "active"},
		{ParentCode: "ticket", Name: "分类管理", Code: "ticket:category", Type: "menu", SortOrder: 2, Status: "active"},
		{ParentCode: "ticket:category", Name: "管理分类", Code: "ticket:manage", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "ticket", Name: "工单统计", Code: "ticket:stats", Type: "menu", SortOrder: 3, Status: "active"},
		// —— 生命周期管理（doc60）
		{Name: "生命周期管理", Code: "lifecycle", Type: "catalog", SortOrder: 9, Status: "active"},
		{ParentCode: "lifecycle", Name: "到期管理", Code: "lifecycle:expiring", Type: "menu", SortOrder: 1, Status: "active"},
		{ParentCode: "lifecycle:expiring", Name: "实例代续费", Code: "lifecycle:renew", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "lifecycle", Name: "续费记录", Code: "lifecycle:renewals", Type: "menu", SortOrder: 2, Status: "active"},
		{ParentCode: "lifecycle", Name: "生命周期策略", Code: "lifecycle:policy", Type: "menu", SortOrder: 3, Status: "active"},
		{ParentCode: "lifecycle:policy", Name: "更新策略", Code: "lifecycle:policy:update", Type: "button", SortOrder: 1, Status: "active"},
		// —— 消息中心（doc70）
		{Name: "消息中心", Code: "notification", Type: "catalog", SortOrder: 10, Status: "active"},
		{ParentCode: "notification", Name: "公告管理", Code: "notify:announcement", Type: "menu", SortOrder: 1, Status: "active"},
		{ParentCode: "notify:announcement", Name: "管理公告", Code: "notify:manage", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "notification", Name: "通知记录", Code: "notify:record", Type: "menu", SortOrder: 2, Status: "active"},
		{ParentCode: "notify:record", Name: "查看记录", Code: "notify:view", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "notification", Name: "通知模板", Code: "notify:template", Type: "menu", SortOrder: 3, Status: "active"},
		// —— 推广邀请返现（替代原代理/分销域）
		{Name: "推广返现", Code: "referral", Type: "catalog", SortOrder: 11, Status: "active"},
		{ParentCode: "referral", Name: "返现台账", Code: "referral:cashback:list", Type: "menu", SortOrder: 1, Status: "active"},
		{ParentCode: "referral", Name: "提现管理", Code: "referral:withdraw:list", Type: "menu", SortOrder: 2, Status: "active"},
		{ParentCode: "referral:withdraw:list", Name: "审核提现", Code: "referral:withdraw:audit", Type: "button", SortOrder: 1, Status: "active"},

		// —— 账号体系与权限分级重构（81/82）补充权限码 ——
		// 员工管理（超管独占）
		{ParentCode: "system", Name: "员工管理", Code: "staff:list", Type: "menu", SortOrder: 4, Status: "active"},
		{ParentCode: "staff:list", Name: "查看员工", Code: "staff:view", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "staff:list", Name: "新建员工", Code: "staff:create", Type: "button", SortOrder: 2, Status: "active"},
		{ParentCode: "staff:list", Name: "编辑员工", Code: "staff:update", Type: "button", SortOrder: 3, Status: "active"},
		{ParentCode: "staff:list", Name: "删除员工", Code: "staff:delete", Type: "button", SortOrder: 4, Status: "active"},
		{ParentCode: "staff:list", Name: "重置密码", Code: "staff:reset_password", Type: "button", SortOrder: 5, Status: "active"},
		{ParentCode: "staff:list", Name: "分配角色", Code: "staff:assign_role", Type: "button", SortOrder: 6, Status: "active"},
		// 权限管理
		{ParentCode: "system", Name: "权限管理", Code: "system:permission:view", Type: "menu", SortOrder: 5, Status: "active"},
		{ParentCode: "system:permission:view", Name: "创建权限", Code: "permission:create", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "system:permission:view", Name: "更新权限", Code: "permission:update", Type: "button", SortOrder: 2, Status: "active"},
		{ParentCode: "system:permission:view", Name: "删除权限", Code: "permission:delete", Type: "button", SortOrder: 3, Status: "active"},
		// 用户管理补充
		{ParentCode: "system:user", Name: "创建用户", Code: "user:create", Type: "button", SortOrder: 6, Status: "active"},
		{ParentCode: "system:user", Name: "编辑用户", Code: "user:update", Type: "button", SortOrder: 7, Status: "active"},
		{ParentCode: "system:user", Name: "分配用户角色", Code: "user:assign_role", Type: "button", SortOrder: 8, Status: "active"},
		{ParentCode: "system:user", Name: "代登录用户", Code: "user:impersonate", Type: "button", SortOrder: 9, Status: "active"},
		// 用户组（折扣来源绑定）
		{ParentCode: "system:user", Name: "用户组", Code: "user:group:list", Type: "menu", SortOrder: 2, Status: "active"},
		{ParentCode: "user:group:list", Name: "创建用户组", Code: "user:group:create", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "user:group:list", Name: "编辑用户组", Code: "user:group:update", Type: "button", SortOrder: 2, Status: "active"},
		{ParentCode: "user:group:list", Name: "删除用户组", Code: "user:group:delete", Type: "button", SortOrder: 3, Status: "active"},
		// 用户等级（消费升级）
		{ParentCode: "system:user", Name: "用户等级", Code: "level:list", Type: "menu", SortOrder: 3, Status: "active"},
		{ParentCode: "level:list", Name: "创建等级", Code: "level:create", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "level:list", Name: "编辑等级", Code: "level:update", Type: "button", SortOrder: 2, Status: "active"},
		{ParentCode: "level:list", Name: "删除等级", Code: "level:delete", Type: "button", SortOrder: 3, Status: "active"},
		// 实名认证
		{ParentCode: "system:user", Name: "实名认证", Code: "verification:list", Type: "menu", SortOrder: 10, Status: "active"},
		{ParentCode: "verification:list", Name: "审核实名", Code: "verification:review", Type: "button", SortOrder: 1, Status: "active"},
		// 产品：规格/定价/促销
		{ParentCode: "product", Name: "规格管理", Code: "product:spec", Type: "menu", SortOrder: 4, Status: "active"},
		{ParentCode: "product:spec", Name: "规格模板查看", Code: "spec:template:list", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "product:spec", Name: "规格模板维护", Code: "spec:template:update", Type: "button", SortOrder: 2, Status: "active"},
		{ParentCode: "product:spec", Name: "规格映射查看", Code: "spec:mapping:list", Type: "button", SortOrder: 3, Status: "active"},
		{ParentCode: "product:spec", Name: "规格映射维护", Code: "spec:mapping:update", Type: "button", SortOrder: 4, Status: "active"},
		{ParentCode: "product:spec", Name: "规格契约查看", Code: "spec:contract:list", Type: "button", SortOrder: 5, Status: "active"},
		{ParentCode: "product:spec", Name: "规格契约维护", Code: "spec:contract:update", Type: "button", SortOrder: 6, Status: "active"},
		{ParentCode: "product", Name: "定价管理", Code: "product:pricing", Type: "menu", SortOrder: 5, Status: "active"},
		{ParentCode: "product:pricing", Name: "定价查看", Code: "pricing:list", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "product:pricing", Name: "定价维护", Code: "pricing:update", Type: "button", SortOrder: 2, Status: "active"},
		{ParentCode: "product", Name: "促销管理", Code: "product:promotion", Type: "menu", SortOrder: 6, Status: "active"},
		{ParentCode: "product:promotion", Name: "优惠券查看", Code: "promotion:coupon:list", Type: "button", SortOrder: 1, Status: "active"},
		{ParentCode: "product:promotion", Name: "优惠券维护", Code: "promotion:coupon:update", Type: "button", SortOrder: 2, Status: "active"},
		{ParentCode: "product:promotion", Name: "活动查看", Code: "promotion:activity:list", Type: "button", SortOrder: 3, Status: "active"},
		{ParentCode: "product:promotion", Name: "活动维护", Code: "promotion:activity:update", Type: "button", SortOrder: 4, Status: "active"},
		// 安全审计（后台）
		{ParentCode: "system", Name: "登录日志", Code: "security:login-log:list", Type: "menu", SortOrder: 6, Status: "active"},
		{ParentCode: "system", Name: "用户审计日志", Code: "security:audit:list", Type: "menu", SortOrder: 7, Status: "active"},
		{ParentCode: "system", Name: "风控事件", Code: "security:risk:list", Type: "menu", SortOrder: 8, Status: "active"},
		{ParentCode: "system", Name: "黑名单", Code: "security:blacklist:manage", Type: "menu", SortOrder: 9, Status: "active"},
		{ParentCode: "system", Name: "会话管理", Code: "security:session:manage", Type: "menu", SortOrder: 10, Status: "active"},
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
			"system:config",
			"system:config:view",
			"system:config:create",
			"system:config:update",
			"system:config:delete",
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
			"resource",
			"resource:provider",
			"provider:create",
			"provider:update",
			"provider:delete",
			"provider:test",
			"resource:product",
			"product:update_price",
			"product:sync",
			"resource:sync",
			"sync:create",
			"sync:log",
			"sync:schedule",
			"sync:price",
			"sync:price:confirm",
			"resource:instance",
			"instance:action",
			"instance:console",
			"instance:resize",
			"instance:destroy",
			"product",
			"product:list",
			"product:create",
			"product:update",
			"product:delete",
			"product:publish",
			"product:category",
			"product:category:create",
			"product:category:update",
			"product:category:delete",
			"product:price",
			"product:price:update",
			"order",
			"order:list",
			"order:cancel",
			"order:remark",
			"order:refund",
			"order:activate",
			"order:refunds",
			"order:refund:audit",
			"order:stats",
			// 工单支持权限（doc50 §7.4）
			"ticket",
			"ticket:list",
			"ticket:view",
			"ticket:reply",
			"ticket:assign",
			"ticket:update",
			"ticket:close",
			"ticket:category",
			"ticket:manage",
			"ticket:stats",
			// 生命周期管理权限（doc60）
			"lifecycle",
			"lifecycle:expiring",
			"lifecycle:renew",
			"lifecycle:renewals",
			"lifecycle:policy",
			"lifecycle:policy:update",
			// 消息中心权限（doc70）
			"notification",
			"notify:announcement",
			"notify:manage",
			"notify:record",
			"notify:view",
			"notify:template",
			// 推广邀请返现权限
			"referral",
			"referral:cashback:list",
			"referral:withdraw:list",
			"referral:withdraw:audit",
		},
		"ops_admin": {
			"system:user",
			"system:user:list",
			"user:detail",
			"user:update_status",
			"user:group:list",
			"level:list",
			"verification:list",
			"system:role",
			"system:role:list",
			"resource",
			"resource:provider",
			"provider:test",
			"resource:product",
			"product:sync",
			"resource:sync",
			"sync:log",
			"sync:schedule",
			"sync:price",
			"resource:instance",
			"instance:action",
			// 运维需要控制台排障，但不授予变配/销毁（见 61 实施计划 §6.1）。
			"instance:console",
			"product",
			"product:list",
			"product:update",
			"product:publish",
			"product:category",
			"product:category:update",
			"product:price",
			"product:price:update",
			"product:spec",
			"spec:template:list",
			"spec:mapping:list",
			"spec:contract:list",
			"product:pricing",
			"pricing:list",
			"product:promotion",
			"promotion:coupon:list",
			"promotion:activity:list",
			"order",
			"order:list",
			"order:cancel",
			"order:remark",
			"order:activate",
			"order:stats",
			"security:login-log:list",
			"security:audit:list",
			"security:risk:list",
			"security:blacklist:manage",
			"security:session:manage",
		},
		"finance_admin": {
			"system:user",
			"system:user:list",
			"user:detail",
			"order",
			"order:list",
			"order:refund",
			"order:refunds",
			"order:refund:audit",
			"order:stats",
			"finance",
			"finance:wallet",
			"finance:adjust",
			"finance:recharge",
			"finance:recharge:approve",
			"finance:withdraw",
			"finance:withdraw:audit",
			"finance:bill",
			"finance:bill:close",
			"finance:bill:recon",
			// 推广返现（财务核对返现台账与提现）
			"referral",
			"referral:cashback:list",
			"referral:withdraw:list",
			"referral:withdraw:audit",
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

// seedMenus 为各平台写入默认菜单树。
// 幂等：按唯一键查重跳过已存在菜单，字段变化时同步更新，支持默认菜单结构平滑升级。
func seedMenus(tx *gorm.DB) error {
	defaults := []seedMenu{
		{Platform: menumodel.PlatformAdmin, Name: "仪表盘", Type: menumodel.TypeDirectory, Path: "/dashboard", Icon: "dashboard", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/dashboard", Platform: menumodel.PlatformAdmin, Name: "概览", Type: menumodel.TypeMenu, Path: "/dashboard/base", Component: "dashboard/base/index", Icon: "dashboard", SortOrder: 1, Status: menumodel.StatusActive},
		{Platform: menumodel.PlatformAdmin, Name: "用户管理", Type: menumodel.TypeDirectory, Path: "/users", Icon: "user", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/users", Platform: menumodel.PlatformAdmin, Name: "用户总览", Type: menumodel.TypeMenu, Path: "/users/overview", Component: "users/overview/index", Icon: "dashboard", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/users", Platform: menumodel.PlatformAdmin, Name: "账户管理", Type: menumodel.TypeDirectory, Path: "/users/accounts", Icon: "usergroup", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/users/accounts", Platform: menumodel.PlatformAdmin, Name: "用户列表", Type: menumodel.TypeMenu, Path: "/users/accounts/list", Component: "users/accounts/list/index", Icon: "user-list", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/users/accounts", Platform: menumodel.PlatformAdmin, Name: "用户组/组织管理", Type: menumodel.TypeMenu, Path: "/users/accounts/groups", Component: "users/accounts/groups/index", Icon: "control-platform", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/users", Platform: menumodel.PlatformAdmin, Name: "安全与风控", Type: menumodel.TypeDirectory, Path: "/users/security", Icon: "key", SortOrder: 5, Status: menumodel.StatusActive},
		{ParentKey: "admin:/users/security", Platform: menumodel.PlatformAdmin, Name: "登录日志", Type: menumodel.TypeMenu, Path: "/users/security/login-logs", Component: "users/security/login-logs/index", Icon: "history", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/users/security", Platform: menumodel.PlatformAdmin, Name: "操作审计日志", Type: menumodel.TypeMenu, Path: "/users/security/audit-logs", Component: "users/security/audit-logs/index", Icon: "file", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/users/security", Platform: menumodel.PlatformAdmin, Name: "异常行为监控", Type: menumodel.TypeMenu, Path: "/users/security/risk", Component: "users/security/risk/index", Icon: "chart-bar", SortOrder: 3, Status: menumodel.StatusActive},
		{ParentKey: "admin:/users/security", Platform: menumodel.PlatformAdmin, Name: "黑名单管理", Type: menumodel.TypeMenu, Path: "/users/security/blacklist", Component: "users/security/blacklist/index", Icon: "stop", SortOrder: 4, Status: menumodel.StatusActive},
		{ParentKey: "admin:/users/security", Platform: menumodel.PlatformAdmin, Name: "会话管理", Type: menumodel.TypeMenu, Path: "/users/security/sessions", Component: "users/security/sessions/index", Icon: "refresh", SortOrder: 5, Status: menumodel.StatusActive},
		{ParentKey: "admin:/users", Platform: menumodel.PlatformAdmin, Name: "用户等级", Type: menumodel.TypeMenu, Path: "/users/levels", Component: "users/levels/index", Icon: "tag", SortOrder: 6, Status: menumodel.StatusActive},
		{ParentKey: "admin:/users", Platform: menumodel.PlatformAdmin, Name: "实名认证", Type: menumodel.TypeDirectory, Path: "/users/verification", Icon: "verify", SortOrder: 7, Status: menumodel.StatusActive},
		{ParentKey: "admin:/users/verification", Platform: menumodel.PlatformAdmin, Name: "待审核列表", Type: menumodel.TypeMenu, Path: "/users/verification/pending", Component: "users/verification/pending/index", Icon: "history", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/users/verification", Platform: menumodel.PlatformAdmin, Name: "审核通过列表", Type: menumodel.TypeMenu, Path: "/users/verification/approved", Component: "users/verification/approved/index", Icon: "check-circle", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/users/verification", Platform: menumodel.PlatformAdmin, Name: "审核拒绝列表", Type: menumodel.TypeMenu, Path: "/users/verification/rejected", Component: "users/verification/rejected/index", Icon: "error-circle", SortOrder: 3, Status: menumodel.StatusActive},
		{ParentKey: "admin:/users/verification", Platform: menumodel.PlatformAdmin, Name: "认证配置", Type: menumodel.TypeMenu, Path: "/users/verification/config", Component: "users/verification/config/index", Icon: "setting", SortOrder: 4, Status: menumodel.StatusActive},
		{Platform: menumodel.PlatformAdmin, Name: "资源管理", Type: menumodel.TypeDirectory, Path: "/resource", Icon: "resource", SortOrder: 3, Status: menumodel.StatusActive},
		// —— 资源总览（doc10 §5.1）
		{ParentKey: "admin:/resource", Platform: menumodel.PlatformAdmin, Name: "资源总览", Type: menumodel.TypeDirectory, Path: "/resource/overview", Icon: "dashboard", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/resource/overview", Platform: menumodel.PlatformAdmin, Name: "资源总览", Type: menumodel.TypeMenu, Path: "/resource/dashboard", Component: "resource/dashboard/index", Icon: "dashboard", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/resource/overview", Platform: menumodel.PlatformAdmin, Name: "同步监控", Type: menumodel.TypeMenu, Path: "/resource/sync-monitor", Component: "resource/sync-monitor/index", Icon: "data-checked", SortOrder: 2, Status: menumodel.StatusActive},
		// —— 上游对接管理（doc10 §5.2）
		{ParentKey: "admin:/resource", Platform: menumodel.PlatformAdmin, Name: "上游对接管理", Type: menumodel.TypeDirectory, Path: "/resource/connection", Icon: "cloud", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/resource/connection", Platform: menumodel.PlatformAdmin, Name: "上游提供商", Type: menumodel.TypeMenu, Path: "/resource/providers", Component: "resource/providers/index", Icon: "cloud", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/resource/connection", Platform: menumodel.PlatformAdmin, Name: "资源池管理", Type: menumodel.TypeMenu, Path: "/resource/pools", Component: "resource/pools/index", Icon: "layers", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/resource/connection", Platform: menumodel.PlatformAdmin, Name: "连接测试", Type: menumodel.TypeMenu, Path: "/resource/connectivity", Component: "resource/connectivity/index", Icon: "link", SortOrder: 3, Status: menumodel.StatusActive},
		// —— 同步与调度（T3.6：合并原「同步任务/同步日志/对账报告」三处重复页面，
		//    并取代产品管理下的 /product/sync/*；旧路径保留 redirect，菜单置 disabled 隐藏）
		// 目录路径用 sync-group，叶子保持 /resource/sync-center 与前端路由一致（避免同路径冲突）。
		{ParentKey: "admin:/resource", Platform: menumodel.PlatformAdmin, Name: "同步与调度", Type: menumodel.TypeDirectory, Path: "/resource/sync-group", Icon: "refresh", SortOrder: 3, Status: menumodel.StatusActive},
		{ParentKey: "admin:/resource/sync-group", Platform: menumodel.PlatformAdmin, Name: "同步与调度", Type: menumodel.TypeMenu, Path: "/resource/sync-center", Component: "resource/sync-center/index", Icon: "refresh", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/resource/sync-group", Platform: menumodel.PlatformAdmin, Name: "同步任务", Type: menumodel.TypeMenu, Path: "/resource/sync", Component: "resource/sync/index", Icon: "refresh", SortOrder: 91, Status: menumodel.StatusDisabled},
		{ParentKey: "admin:/resource/sync-group", Platform: menumodel.PlatformAdmin, Name: "同步日志", Type: menumodel.TypeMenu, Path: "/resource/logs", Component: "resource/logs/index", Icon: "history", SortOrder: 92, Status: menumodel.StatusDisabled},
		{ParentKey: "admin:/resource/sync-group", Platform: menumodel.PlatformAdmin, Name: "对账报告", Type: menumodel.TypeMenu, Path: "/resource/reconciliation", Component: "resource/reconciliation/index", Icon: "verify", SortOrder: 93, Status: menumodel.StatusDisabled},
		// —— 资源商品管理（doc10 §5.4）
		{ParentKey: "admin:/resource", Platform: menumodel.PlatformAdmin, Name: "资源商品管理", Type: menumodel.TypeDirectory, Path: "/resource/products-center", Icon: "product", SortOrder: 4, Status: menumodel.StatusActive},
		{ParentKey: "admin:/resource/products-center", Platform: menumodel.PlatformAdmin, Name: "商品列表", Type: menumodel.TypeMenu, Path: "/resource/products", Component: "resource/products/index", Icon: "product", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/resource/products-center", Platform: menumodel.PlatformAdmin, Name: "商品同步", Type: menumodel.TypeMenu, Path: "/resource/product-sync", Component: "resource/product-sync/index", Icon: "cloud-download", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/resource/products-center", Platform: menumodel.PlatformAdmin, Name: "定价管理", Type: menumodel.TypeMenu, Path: "/resource/pricing", Component: "resource/pricing/index", Icon: "money", SortOrder: 3, Status: menumodel.StatusActive},
		// —— 实例资源
		{ParentKey: "admin:/resource", Platform: menumodel.PlatformAdmin, Name: "实例资源", Type: menumodel.TypeDirectory, Path: "/resource/instance", Icon: "server", SortOrder: 5, Status: menumodel.StatusActive},
		{ParentKey: "admin:/resource/instance", Platform: menumodel.PlatformAdmin, Name: "云主机实例", Type: menumodel.TypeMenu, Path: "/resource/instances", Component: "resource/instances/index", Icon: "server", SortOrder: 1, Status: menumodel.StatusActive},
		// —— 实例管理（一级菜单，跨用户操作、维护与售后）：见 docs/实施计划/61-实例运维管理台实施计划.md
		// 父级必须排在子项之前，否则 ParentKey 解析取不到父节点会导致 seed 失败。
		{Platform: menumodel.PlatformAdmin, Name: "实例管理", Type: menumodel.TypeDirectory, Path: "/instances", Icon: "server", SortOrder: 4, Status: menumodel.StatusActive},
		{ParentKey: "admin:/instances", Platform: menumodel.PlatformAdmin, Name: "实例运维台", Type: menumodel.TypeMenu, Path: "/instances/list", Component: "instances/index", Icon: "server", SortOrder: 1, Status: menumodel.StatusActive},
		// —— 运维工具（doc10 §5.5）
		{ParentKey: "admin:/resource", Platform: menumodel.PlatformAdmin, Name: "运维工具", Type: menumodel.TypeDirectory, Path: "/resource/ops", Icon: "setting", SortOrder: 6, Status: menumodel.StatusActive},
		{ParentKey: "admin:/resource/ops", Platform: menumodel.PlatformAdmin, Name: "API测试", Type: menumodel.TypeMenu, Path: "/resource/api-test", Component: "resource/api-test/index", Icon: "ai-tool", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/resource/ops", Platform: menumodel.PlatformAdmin, Name: "异常处理", Type: menumodel.TypeMenu, Path: "/resource/anomalies", Component: "resource/anomalies/index", Icon: "error-circle", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/resource/ops", Platform: menumodel.PlatformAdmin, Name: "系统配置", Type: menumodel.TypeMenu, Path: "/resource/settings", Component: "resource/settings/index", Icon: "setting", SortOrder: 3, Status: menumodel.StatusActive},

		// —— 产品管理（面向终端售卖，三层树）
		{Platform: menumodel.PlatformAdmin, Name: "产品管理", Type: menumodel.TypeDirectory, Path: "/product", Icon: "product", SortOrder: 5, Status: menumodel.StatusActive},
		// 1. 商品管理
		{ParentKey: "admin:/product", Platform: menumodel.PlatformAdmin, Name: "商品管理", Type: menumodel.TypeDirectory, Path: "/product/mgmt", Icon: "product", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/product/mgmt", Platform: menumodel.PlatformAdmin, Name: "商品列表", Type: menumodel.TypeMenu, Path: "/product/products", Component: "product/products/index", Icon: "product", SortOrder: 1, Status: menumodel.StatusActive},
		// 2. 规格管理
		{ParentKey: "admin:/product", Platform: menumodel.PlatformAdmin, Name: "规格管理", Type: menumodel.TypeDirectory, Path: "/product/spec", Icon: "layers", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/product/spec", Platform: menumodel.PlatformAdmin, Name: "规格模板", Type: menumodel.TypeMenu, Path: "/product/spec/templates", Component: "product/spec/templates/index", Icon: "catalog", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/product/spec", Platform: menumodel.PlatformAdmin, Name: "自定义规格", Type: menumodel.TypeMenu, Path: "/product/spec/custom", Component: "product/spec/custom/index", Icon: "add", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/product/spec", Platform: menumodel.PlatformAdmin, Name: "规格映射", Type: menumodel.TypeMenu, Path: "/product/spec/mappings", Component: "product/spec/mappings/index", Icon: "link", SortOrder: 3, Status: menumodel.StatusActive},
		// 3. 定价与计费
		{ParentKey: "admin:/product", Platform: menumodel.PlatformAdmin, Name: "定价与计费", Type: menumodel.TypeDirectory, Path: "/product/pricing-center", Icon: "money", SortOrder: 3, Status: menumodel.StatusActive},
		{ParentKey: "admin:/product/pricing-center", Platform: menumodel.PlatformAdmin, Name: "价格策略", Type: menumodel.TypeMenu, Path: "/product/pricing", Component: "product/pricing/index", Icon: "money", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/product/pricing-center", Platform: menumodel.PlatformAdmin, Name: "价格计算器", Type: menumodel.TypeMenu, Path: "/product/pricing/calculator", Component: "product/pricing/calculator/index", Icon: "chart-bar", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/product/pricing-center", Platform: menumodel.PlatformAdmin, Name: "价格历史", Type: menumodel.TypeMenu, Path: "/product/pricing/history", Component: "product/pricing/history/index", Icon: "history", SortOrder: 3, Status: menumodel.StatusActive},
		{ParentKey: "admin:/product/pricing-center", Platform: menumodel.PlatformAdmin, Name: "折扣策略", Type: menumodel.TypeMenu, Path: "/product/pricing/policies", Component: "product/pricing/policies/index", Icon: "discount", SortOrder: 4, Status: menumodel.StatusActive},
		// 4. 促销管理
		{ParentKey: "admin:/product", Platform: menumodel.PlatformAdmin, Name: "促销管理", Type: menumodel.TypeDirectory, Path: "/product/promotion", Icon: "tag", SortOrder: 4, Status: menumodel.StatusActive},
		{ParentKey: "admin:/product/promotion", Platform: menumodel.PlatformAdmin, Name: "优惠券管理", Type: menumodel.TypeMenu, Path: "/product/promotion/coupons", Component: "product/promotion/coupons/index", Icon: "ticket", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/product/promotion", Platform: menumodel.PlatformAdmin, Name: "折扣活动", Type: menumodel.TypeMenu, Path: "/product/promotion/activities", Component: "product/promotion/activities/index", Icon: "chart-bar", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/product/promotion", Platform: menumodel.PlatformAdmin, Name: "套餐组合", Type: menumodel.TypeMenu, Path: "/product/promotion/bundles", Component: "product/promotion/bundles/index", Icon: "app", SortOrder: 3, Status: menumodel.StatusActive},
		{ParentKey: "admin:/product/promotion", Platform: menumodel.PlatformAdmin, Name: "推荐位管理", Type: menumodel.TypeMenu, Path: "/product/promotion/recommends", Component: "product/promotion/recommends/index", Icon: "star", SortOrder: 4, Status: menumodel.StatusActive},
		// 5. 商品分类
		{ParentKey: "admin:/product", Platform: menumodel.PlatformAdmin, Name: "商品分类", Type: menumodel.TypeDirectory, Path: "/product/category", Icon: "folder", SortOrder: 5, Status: menumodel.StatusActive},
		{ParentKey: "admin:/product/category", Platform: menumodel.PlatformAdmin, Name: "分类管理", Type: menumodel.TypeMenu, Path: "/product/categories", Component: "product/categories/index", Icon: "tag", SortOrder: 1, Status: menumodel.StatusActive},
		// 6. 上游商品同步
		{ParentKey: "admin:/product", Platform: menumodel.PlatformAdmin, Name: "上游商品同步", Type: menumodel.TypeDirectory, Path: "/product/sync-center", Icon: "cloud-download", SortOrder: 6, Status: menumodel.StatusActive},
		{ParentKey: "admin:/product/sync-center", Platform: menumodel.PlatformAdmin, Name: "同步任务", Type: menumodel.TypeMenu, Path: "/product/sync/tasks", Component: "product/sync/tasks/index", Icon: "refresh", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/product/sync-center", Platform: menumodel.PlatformAdmin, Name: "同步日志", Type: menumodel.TypeMenu, Path: "/product/sync/logs", Component: "product/sync/logs/index", Icon: "history", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/product/sync-center", Platform: menumodel.PlatformAdmin, Name: "差异对比", Type: menumodel.TypeMenu, Path: "/product/sync/diff", Component: "product/sync/diff/index", Icon: "data-checked", SortOrder: 3, Status: menumodel.StatusActive},

		// —— 订单管理（doc16）
		{Platform: menumodel.PlatformAdmin, Name: "订单管理", Type: menumodel.TypeDirectory, Path: "/orders", Icon: "order", SortOrder: 6, Status: menumodel.StatusActive},
		{ParentKey: "admin:/orders", Platform: menumodel.PlatformAdmin, Name: "订单列表", Type: menumodel.TypeMenu, Path: "/orders/list", Component: "order/index", Icon: "order", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/orders", Platform: menumodel.PlatformAdmin, Name: "退款管理", Type: menumodel.TypeMenu, Path: "/orders/refunds", Component: "order/refunds/index", Icon: "money", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/orders", Platform: menumodel.PlatformAdmin, Name: "订单统计", Type: menumodel.TypeMenu, Path: "/orders/stats", Component: "order/stats/index", Icon: "chart-bar", SortOrder: 3, Status: menumodel.StatusActive},

		// —— 财务管理（doc32，分组树：叶子 + 二级目录）
		{Platform: menumodel.PlatformAdmin, Name: "财务管理", Type: menumodel.TypeDirectory, Path: "/finance", Icon: "wallet", SortOrder: 7, Status: menumodel.StatusActive},
		// 1. 财务总览
		{ParentKey: "admin:/finance", Platform: menumodel.PlatformAdmin, Name: "财务总览", Type: menumodel.TypeMenu, Path: "/finance/overview", Component: "finance/overview/index", Icon: "dashboard", SortOrder: 1, Status: menumodel.StatusActive},
		// 2. 账户管理
		{ParentKey: "admin:/finance", Platform: menumodel.PlatformAdmin, Name: "账户管理", Type: menumodel.TypeDirectory, Path: "/finance/accounts", Icon: "usergroup", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/finance/accounts", Platform: menumodel.PlatformAdmin, Name: "用户钱包", Type: menumodel.TypeMenu, Path: "/finance/accounts/wallets", Component: "finance/accounts/wallets/index", Icon: "wallet", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/finance/accounts", Platform: menumodel.PlatformAdmin, Name: "人工调账", Type: menumodel.TypeMenu, Path: "/finance/accounts/adjust", Component: "finance/accounts/adjust", Icon: "money", SortOrder: 2, Status: menumodel.StatusActive},
		// 3. 交易流水
		{ParentKey: "admin:/finance", Platform: menumodel.PlatformAdmin, Name: "交易流水", Type: menumodel.TypeDirectory, Path: "/finance/transactions-center", Icon: "money", SortOrder: 3, Status: menumodel.StatusActive},
		{ParentKey: "admin:/finance/transactions-center", Platform: menumodel.PlatformAdmin, Name: "资金流水", Type: menumodel.TypeMenu, Path: "/finance/transactions", Component: "finance/transactions/index", Icon: "money", SortOrder: 1, Status: menumodel.StatusActive},
		// 4. 充值提现
		{ParentKey: "admin:/finance", Platform: menumodel.PlatformAdmin, Name: "充值提现", Type: menumodel.TypeDirectory, Path: "/finance/recharge-center", Icon: "download", SortOrder: 4, Status: menumodel.StatusActive},
		{ParentKey: "admin:/finance/recharge-center", Platform: menumodel.PlatformAdmin, Name: "充值管理", Type: menumodel.TypeMenu, Path: "/finance/recharges", Component: "finance/recharge/index", Icon: "download", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/finance/recharge-center", Platform: menumodel.PlatformAdmin, Name: "提现管理", Type: menumodel.TypeMenu, Path: "/finance/withdrawals", Component: "finance/withdraw/index", Icon: "upload", SortOrder: 2, Status: menumodel.StatusActive},
		// 5. 账单管理
		{ParentKey: "admin:/finance", Platform: menumodel.PlatformAdmin, Name: "账单管理", Type: menumodel.TypeDirectory, Path: "/finance/bill-center", Icon: "file", SortOrder: 5, Status: menumodel.StatusActive},
		{ParentKey: "admin:/finance/bill-center", Platform: menumodel.PlatformAdmin, Name: "账单管理", Type: menumodel.TypeMenu, Path: "/finance/bills", Component: "finance/bills/index", Icon: "file", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/finance/bill-center", Platform: menumodel.PlatformAdmin, Name: "对账中心", Type: menumodel.TypeMenu, Path: "/finance/recon", Component: "finance/bills/recon", Icon: "verify", SortOrder: 2, Status: menumodel.StatusActive},
		// 6. 财务报表
		{ParentKey: "admin:/finance", Platform: menumodel.PlatformAdmin, Name: "财务报表", Type: menumodel.TypeMenu, Path: "/finance/report", Component: "finance/report/index", Icon: "chart-bar", SortOrder: 6, Status: menumodel.StatusActive},
		// 7. 财务配置
		{ParentKey: "admin:/finance", Platform: menumodel.PlatformAdmin, Name: "财务配置", Type: menumodel.TypeMenu, Path: "/finance/config", Component: "finance/config/index", Icon: "setting", SortOrder: 7, Status: menumodel.StatusActive},

		// —— 推广返现（替代原代理/分销域）
		{Platform: menumodel.PlatformAdmin, Name: "推广返现", Type: menumodel.TypeDirectory, Path: "/referral", Icon: "share", SortOrder: 12, Status: menumodel.StatusActive},
		{ParentKey: "admin:/referral", Platform: menumodel.PlatformAdmin, Name: "返现台账", Type: menumodel.TypeMenu, Path: "/referral/cashbacks", Component: "referral/cashbacks/index", Icon: "money", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/referral", Platform: menumodel.PlatformAdmin, Name: "提现审核", Type: menumodel.TypeMenu, Path: "/referral/withdrawals", Component: "referral/withdrawals/index", Icon: "upload", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/referral", Platform: menumodel.PlatformAdmin, Name: "邀请关系", Type: menumodel.TypeMenu, Path: "/referral/invitees", Component: "referral/invitees/index", Icon: "usergroup", SortOrder: 3, Status: menumodel.StatusActive},

		// —— 工单支持（doc50 §5.3，admin 平台 SortOrder=9）
		{Platform: menumodel.PlatformAdmin, Name: "工单支持", Type: menumodel.TypeDirectory, Path: "/tickets", Icon: "service", SortOrder: 9, Status: menumodel.StatusActive},
		{ParentKey: "admin:/tickets", Platform: menumodel.PlatformAdmin, Name: "工单列表", Type: menumodel.TypeMenu, Path: "/tickets/list", Component: "ticket/index", Icon: "ticket", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/tickets", Platform: menumodel.PlatformAdmin, Name: "工单分类管理", Type: menumodel.TypeMenu, Path: "/tickets/categories", Component: "ticket/categories/index", Icon: "folder", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/tickets", Platform: menumodel.PlatformAdmin, Name: "工单统计", Type: menumodel.TypeMenu, Path: "/tickets/stats", Component: "ticket/stats/index", Icon: "chart-bar", SortOrder: 3, Status: menumodel.StatusActive},

		// —— 系统管理（doc40 系统管理模块，二级目录 + 三级叶子）
		{Platform: menumodel.PlatformAdmin, Name: "系统管理", Type: menumodel.TypeDirectory, Path: "/system", Icon: "setting", SortOrder: 8, Status: menumodel.StatusActive},
		// 1. 权限管理
		{ParentKey: "admin:/system", Platform: menumodel.PlatformAdmin, Name: "权限管理", Type: menumodel.TypeDirectory, Path: "/system/permission-center", Icon: "lock-on", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/system/permission-center", Platform: menumodel.PlatformAdmin, Name: "菜单管理", Type: menumodel.TypeMenu, Path: "/system/menus", Component: "system/menus/index", Icon: "menu", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/system/permission-center", Platform: menumodel.PlatformAdmin, Name: "角色列表", Type: menumodel.TypeMenu, Path: "/system/roles", Component: "system/roles/index", Icon: "usergroup", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/system/permission-center", Platform: menumodel.PlatformAdmin, Name: "权限分配", Type: menumodel.TypeMenu, Path: "/system/permissions", Component: "system/permissions/index", Icon: "lock-on", SortOrder: 3, Status: menumodel.StatusActive},
		{ParentKey: "admin:/system/permission-center", Platform: menumodel.PlatformAdmin, Name: "管理员列表", Type: menumodel.TypeMenu, Path: "/system/admins", Component: "system/admins/index", Icon: "user-list", SortOrder: 4, Status: menumodel.StatusActive},
		// 2. 系统配置
		{ParentKey: "admin:/system", Platform: menumodel.PlatformAdmin, Name: "系统配置", Type: menumodel.TypeDirectory, Path: "/system/config-center", Icon: "setting", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/system/config-center", Platform: menumodel.PlatformAdmin, Name: "系统配置", Type: menumodel.TypeMenu, Path: "/system/config", Component: "system/config/index", Icon: "setting", SortOrder: 1, Status: menumodel.StatusActive},
		// 3. 安全审计
		{ParentKey: "admin:/system", Platform: menumodel.PlatformAdmin, Name: "安全审计", Type: menumodel.TypeDirectory, Path: "/system/audit-center", Icon: "history", SortOrder: 3, Status: menumodel.StatusActive},
		{ParentKey: "admin:/system/audit-center", Platform: menumodel.PlatformAdmin, Name: "操作审计", Type: menumodel.TypeMenu, Path: "/system/audit-logs", Component: "system/audit-logs/index", Icon: "history", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/system/audit-center", Platform: menumodel.PlatformAdmin, Name: "公告管理", Type: menumodel.TypeMenu, Path: "/system/announcements", Component: "notification/announcements/index", Icon: "sound", SortOrder: 2, Status: menumodel.StatusActive},

		// —— 生命周期管理（doc60，admin 平台 SortOrder=10）
		{Platform: menumodel.PlatformAdmin, Name: "生命周期管理", Type: menumodel.TypeDirectory, Path: "/lifecycle", Icon: "history", SortOrder: 10, Status: menumodel.StatusActive},
		{ParentKey: "admin:/lifecycle", Platform: menumodel.PlatformAdmin, Name: "到期管理", Type: menumodel.TypeMenu, Path: "/lifecycle/expiring", Component: "lifecycle/expiring/index", Icon: "history", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/lifecycle", Platform: menumodel.PlatformAdmin, Name: "续费记录", Type: menumodel.TypeMenu, Path: "/lifecycle/renewals", Component: "lifecycle/renewals/index", Icon: "order", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "admin:/lifecycle", Platform: menumodel.PlatformAdmin, Name: "生命周期策略", Type: menumodel.TypeMenu, Path: "/lifecycle/policy", Component: "lifecycle/policy/index", Icon: "setting", SortOrder: 3, Status: menumodel.StatusActive},

		// —— 管理员后台 - 消息中心（doc70，公告管理已归类到系统管理/安全审计）
		{Platform: menumodel.PlatformAdmin, Name: "消息中心", Type: menumodel.TypeDirectory, Path: "/notification", Icon: "mail", SortOrder: 11, Status: menumodel.StatusActive},
		{ParentKey: "admin:/notification", Platform: menumodel.PlatformAdmin, Name: "通知记录", Type: menumodel.TypeMenu, Path: "/notification/records", Component: "notification/records/index", Icon: "mail", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "admin:/notification", Platform: menumodel.PlatformAdmin, Name: "通知模板", Type: menumodel.TypeMenu, Path: "/notification/templates", Component: "notification/templates/index", Icon: "root-list", SortOrder: 2, Status: menumodel.StatusActive},

		// —— 用户中心菜单（platform=user）
		{Platform: menumodel.PlatformUser, Name: "控制台", Type: menumodel.TypeMenu, Path: "/dashboard", Icon: "dashboard", SortOrder: 1, Status: menumodel.StatusActive},
		{Platform: menumodel.PlatformUser, Name: "云产品", Type: menumodel.TypeDirectory, Path: "/cloud", Icon: "cloud", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "user:/cloud", Platform: menumodel.PlatformUser, Name: "我的云主机", Type: menumodel.TypeMenu, Path: "/cloud/instances", Icon: "server", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "user:/cloud", Platform: menumodel.PlatformUser, Name: "镜像管理", Type: menumodel.TypeMenu, Path: "/cloud/images", Icon: "layers", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "user:/cloud", Platform: menumodel.PlatformUser, Name: "续费管理", Type: menumodel.TypeMenu, Path: "/cloud/renewals", Icon: "refresh", SortOrder: 3, Status: menumodel.StatusActive},
		{Platform: menumodel.PlatformUser, Name: "我的订单", Type: menumodel.TypeMenu, Path: "/order", Icon: "order", SortOrder: 3, Status: menumodel.StatusActive},
		{Platform: menumodel.PlatformUser, Name: "费用中心", Type: menumodel.TypeMenu, Path: "/billing", Icon: "wallet", SortOrder: 4, Status: menumodel.StatusActive},
		{Platform: menumodel.PlatformUser, Name: "工单中心", Type: menumodel.TypeDirectory, Path: "/support", Icon: "service", SortOrder: 5, Status: menumodel.StatusActive},
		{ParentKey: "user:/support", Platform: menumodel.PlatformUser, Name: "我的工单", Type: menumodel.TypeMenu, Path: "/support/tickets", Icon: "ticket", SortOrder: 1, Status: menumodel.StatusActive},
		{Platform: menumodel.PlatformUser, Name: "个人中心", Type: menumodel.TypeMenu, Path: "/profile", Icon: "user", SortOrder: 6, Status: menumodel.StatusActive},
		{ParentKey: "user:/profile", Platform: menumodel.PlatformUser, Name: "我的消息", Type: menumodel.TypeMenu, Path: "/profile/messages", Icon: "mail", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "user:/profile", Platform: menumodel.PlatformUser, Name: "通知偏好", Type: menumodel.TypeMenu, Path: "/profile/preferences", Icon: "setting", SortOrder: 2, Status: menumodel.StatusActive},
		// 推广邀请返现（用户自助；子账号可看，提现与转出后端硬拒）
		{Platform: menumodel.PlatformUser, Name: "推广邀请", Type: menumodel.TypeDirectory, Path: "/referral", Icon: "share", SortOrder: 7, Status: menumodel.StatusActive},
		{ParentKey: "user:/referral", Platform: menumodel.PlatformUser, Name: "推广概览", Type: menumodel.TypeMenu, Path: "/referral/overview", Icon: "dashboard", SortOrder: 1, Status: menumodel.StatusActive},
		{ParentKey: "user:/referral", Platform: menumodel.PlatformUser, Name: "我的邀请", Type: menumodel.TypeMenu, Path: "/referral/invitees", Icon: "usergroup", SortOrder: 2, Status: menumodel.StatusActive},
		{ParentKey: "user:/referral", Platform: menumodel.PlatformUser, Name: "返现明细", Type: menumodel.TypeMenu, Path: "/referral/cashbacks", Icon: "money", SortOrder: 3, Status: menumodel.StatusActive},
		{ParentKey: "user:/referral", Platform: menumodel.PlatformUser, Name: "提现与转出", Type: menumodel.TypeMenu, Path: "/referral/withdrawals", Icon: "wallet", SortOrder: 4, Status: menumodel.StatusActive},
		{ParentKey: "user:/referral", Platform: menumodel.PlatformUser, Name: "推广素材", Type: menumodel.TypeMenu, Path: "/referral/materials", Icon: "share", SortOrder: 5, Status: menumodel.StatusActive},
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
			// 幂等迁移：菜单已存在，但父级或关键字段发生变化时同步更新，
			// 使既有「两级」结构可平滑升级为「二级目录+三级子菜单」结构（旧叶子被重挂到新目录下）。
			if existing.ParentID != parentID || existing.Name != item.Name || existing.Type != item.Type ||
				existing.Component != item.Component || existing.Icon != item.Icon ||
				existing.SortOrder != item.SortOrder || existing.Status != item.Status {
				existing.ParentID = parentID
				existing.Name = item.Name
				existing.Type = item.Type
				existing.Component = item.Component
				existing.Icon = item.Icon
				existing.SortOrder = item.SortOrder
				existing.Status = item.Status
				if err := tx.Save(&existing).Error; err != nil {
					return err
				}
			}
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

// seedAdminUser 仅在初始管理员不存在时创建；已存在则只补齐缺失的 role/status，
// 绝不覆盖 PasswordHash —— 否则运营改密后重启会被默认口令重置（见 P0-01）。
func seedAdminUser(tx *gorm.DB, _ config.Config) error {
	const adminUsername = "admin"
	const adminEmail = "admin@hostsent.local"
	const adminPassword = "123456"

	var existing adminmodel.Admin
	if err := tx.Where("username = ?", adminUsername).First(&existing).Error; err == nil {
		updates := map[string]any{}
		if existing.Role == "" {
			updates["role"] = "super_admin"
		}
		if existing.Status != "active" {
			updates["status"] = "active"
		}
		if len(updates) == 0 {
			return nil
		}
		return tx.Model(&existing).Updates(updates).Error
	} else if err != gorm.ErrRecordNotFound {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	admin := adminmodel.Admin{
		Username:           adminUsername,
		Email:              adminEmail,
		PasswordHash:       string(hash),
		Role:               "super_admin",
		Status:             "active",
		MustChangePassword: true, // 初始口令要求首次登录强制改密（P1-08）
	}
	return tx.Create(&admin).Error
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
			{Username: "user_east_01", Email: "east01@hostsent.local", Phone: "13900000001", PasswordHash: string(hash), Status: "active", RealName: "李东", Region: "华东", Balance: 1280.50, LastLoginAt: &loginA, OAuthProvider: "wechat", OAuthOpenID: "wx_o_01"},
			{Username: "user_north_01", Email: "north01@hostsent.local", Phone: "13900000002", PasswordHash: string(hash), Status: "active", RealName: "王北", Region: "华北", Balance: 860.00, LastLoginAt: &loginB, OAuthProvider: "github", OAuthOpenID: "gh_o_02"},
			{Username: "user_south_01", Email: "south01@hostsent.local", Phone: "13900000003", PasswordHash: string(hash), Status: "active", RealName: "陈南", Region: "华南", Balance: 420.35, LastLoginAt: &loginC, OAuthProvider: "qq", OAuthOpenID: "qq_o_03"},
			{Username: "user_west_01", Email: "west01@hostsent.local", Phone: "13900000004", PasswordHash: string(hash), Status: "disabled", RealName: "赵西", Region: "西南", Balance: 0, LastLoginAt: &loginD},
			{Username: "user_central_01", Email: "central01@hostsent.local", Phone: "13900000005", PasswordHash: string(hash), Status: "pending", RealName: "", Region: "华中", Balance: 66.60, OAuthProvider: "alipay", OAuthOpenID: "ali_o_05"},
			{Username: "user_east_02", Email: "east02@hostsent.local", Phone: "13900000006", PasswordHash: string(hash), Status: "cancelled", RealName: "孙城", Region: "华东", Balance: 0},
			{Username: "user_new_01", Email: "new01@hostsent.local", Phone: "13900000007", PasswordHash: string(hash), Status: "active", RealName: "周新", Region: "华北", Balance: 218.88, CreatedAt: newUserTime, UpdatedAt: newUserTime, LastLoginAt: &loginA, OAuthProvider: "wecom", OAuthOpenID: "ww_o_07"},
			{Username: "user_nw_01", Email: "nw01@hostsent.local", Phone: "13900000008", PasswordHash: string(hash), Status: "active", RealName: "", Region: "西北", Balance: 0},
			{Username: "user_ne_01", Email: "ne01@hostsent.local", Phone: "13900000009", PasswordHash: string(hash), Status: "disabled", RealName: "刘北", Region: "东北", Balance: 52.10, LastLoginAt: &loginD},
			{Username: "user_oversea_01", Email: "os01@hostsent.local", Phone: "13900000010", PasswordHash: string(hash), Status: "pending", RealName: "吴洋", Region: "海外", Balance: 999.99, OAuthProvider: "github", OAuthOpenID: "gh_o_10"},
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

	// user_instances 已退役（Phase 2 013）：其聚合读路径改走权威表 instances。

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

	// user_bills 已退役（Phase 2 014）：其聚合读路径改走权威表 bills。

	var ticketCount int64
	if err := tx.Model(&ticketmodel.Ticket{}).Where("user_id = ?", target.ID).Count(&ticketCount).Error; err != nil {
		return err
	}
	if ticketCount == 0 {
		// 演示工单：分类编码对齐 ticket_categories seed（doc50 §6.5）
		tickets := []ticketmodel.Ticket{
			{TicketNo: "TK20260819005", UserID: target.ID, Title: "实例公网带宽波动", Description: "晚间高峰期实例公网带宽持续波动，影响线上业务访问，请协助排查。", Category: "technical", Priority: "high", Status: "in_progress"},
			{TicketNo: "TK20260811001", UserID: target.ID, Title: "发票抬头更新申请", Description: "需要将发票抬头更新为公司全称，请协助处理。", Category: "billing", Priority: "medium", Status: "waiting_user"},
			{TicketNo: "TK20260730008", UserID: target.ID, Title: "续费后实例未自动开机", Description: "实例续费完成后未自动开机，已手动处理，请确认后续计费正常。", Category: "aftersales", Priority: "medium", Status: "resolved"},
		}
		if err := tx.Create(&tickets).Error; err != nil {
			return err
		}
		// 演示对话回复：与工单状态保持一致
		replies := []ticketmodel.TicketReply{
			{TicketID: tickets[0].ID, SenderType: "admin", SenderName: "admin", Content: "您好，已收到反馈，正在排查带宽波动问题，稍后同步进展。"},
			{TicketID: tickets[1].ID, SenderType: "admin", SenderName: "admin", Content: "已登记发票抬头变更申请，请提供新抬头全称与税号。"},
			{TicketID: tickets[2].ID, SenderType: "admin", SenderName: "admin", Content: "实例已恢复开机，计费已核实无误，感谢反馈。"},
			{TicketID: tickets[2].ID, SenderType: "user", SenderID: target.ID, SenderName: target.Username, Content: "收到，问题已解决，感谢支持。"},
		}
		if err := tx.Create(&replies).Error; err != nil {
			return err
		}
	}

	return nil
}

// seedTicketCategories 写入默认工单分类（doc50 §6.5）。幂等：按 code 查重跳过。
func seedTicketCategories(tx *gorm.DB) error {
	defaults := []ticketmodel.TicketCategory{
		{Name: "售前咨询", Code: "presales", Description: "产品价格、功能咨询", SortOrder: 1, Status: "active"},
		{Name: "售后问题", Code: "aftersales", Description: "使用问题、故障报修", SortOrder: 2, Status: "active"},
		{Name: "账单问题", Code: "billing", Description: "充值、扣费、退款", SortOrder: 3, Status: "active"},
		{Name: "技术支持", Code: "technical", Description: "配置、部署、API技术", SortOrder: 4, Status: "active"},
		{Name: "投诉建议", Code: "complaint", Description: "服务投诉、改进建议", SortOrder: 5, Status: "active"},
	}
	for _, item := range defaults {
		var existing ticketmodel.TicketCategory
		if err := tx.Where("code = ?", item.Code).First(&existing).Error; err == nil {
			continue
		} else if err != gorm.ErrRecordNotFound {
			return err
		}
		if err := tx.Create(&item).Error; err != nil {
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
	names := []string{"user_east_01", "user_north_01", "user_south_01", "user_west_01", "user_nw_01"}
	var users []usermodel.User
	if err := tx.Where("username IN ?", names).Find(&users).Error; err != nil {
		return nil, err
	}
	result := make(map[string]usermodel.User, len(users)+1)
	for _, user := range users {
		result[user.Username] = user
	}
	admin, err := loadAdminAsUser(tx)
	if err != nil {
		return nil, err
	}
	result["admin"] = admin
	return result, nil
}

// loadAdminAsUser 从 admins 表取 admin，映射为 usermodel.User 供各 seed 记录操作人 ID。
func loadAdminAsUser(tx *gorm.DB) (usermodel.User, error) {
	var admin adminmodel.Admin
	if err := tx.Where("username = ?", "admin").First(&admin).Error; err != nil {
		return usermodel.User{}, err
	}
	return usermodel.User{ID: admin.ID, Username: admin.Username}, nil
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
		{OperatorID: users["admin"].ID, OperatorName: "admin", Module: "security", ResourceType: "blacklist", ResourceID: "1", Action: "create", RequestMethod: "POST", RequestPath: "/api/v1/admin/security/blacklists", RequestPayload: `{"type":"ip","target_value":"154.83.14.33"}`, ResponseCode: 200, ResponseMessage: "ok", IP: "127.0.0.1", UserAgent: "Chrome 139 / macOS", TraceID: "trace-sec-001", CreatedAt: now.Add(-90 * time.Minute)},
		{OperatorID: users["admin"].ID, OperatorName: "admin", Module: "security", ResourceType: "risk_event", ResourceID: "2", Action: "handle", RequestMethod: "POST", RequestPath: "/api/v1/admin/security/risk-events/2/handle", RequestPayload: `{"note":"人工复核后转已处理"}`, ResponseCode: 200, ResponseMessage: "ok", IP: "127.0.0.1", UserAgent: "Chrome 139 / macOS", TraceID: "trace-sec-002", CreatedAt: now.Add(-70 * time.Minute)},
		{OperatorID: users["admin"].ID, OperatorName: "admin", Module: "security", ResourceType: "session", ResourceID: "3", Action: "revoke", RequestMethod: "POST", RequestPath: "/api/v1/admin/security/sessions/3/revoke", RequestPayload: `{"reason":"异地风险登录"}`, ResponseCode: 200, ResponseMessage: "ok", IP: "127.0.0.1", UserAgent: "Chrome 139 / macOS", TraceID: "trace-sec-003", CreatedAt: now.Add(-45 * time.Minute)},
		{OperatorID: users["admin"].ID, OperatorName: "admin", Module: "menu", ResourceType: "menu", ResourceID: "8", Action: "update", RequestMethod: "PUT", RequestPath: "/api/v1/admin/menus/8", RequestPayload: `{"name":"安全与风控"}`, ResponseCode: 200, ResponseMessage: "ok", IP: "127.0.0.1", UserAgent: "Chrome 139 / macOS", TraceID: "trace-sec-004", CreatedAt: now.Add(-20 * time.Minute)},
		{OperatorID: users["user_nw_01"].ID, OperatorName: "user_nw_01", Module: "auth", ResourceType: "login", ResourceID: "user_nw_01", Action: "login", RequestMethod: "POST", RequestPath: "/api/v1/admin/auth/login", RequestPayload: `{"username":"user_nw_01"}`, ResponseCode: 200, ResponseMessage: "ok", IP: "10.10.2.16", UserAgent: "Edge / Windows", TraceID: "trace-sec-005", CreatedAt: now.Add(-15 * time.Minute)},
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

// seedDefaultUserGroup 保证存在且仅存在一个默认用户组。
//
// is_default 是注册（uc/auth）与后台建号（admin user）的兜底分组：新用户未指定
// 分组时归入该组，因此必须有且只有一个默认组，否则兜底能力形同虚设。
//
// 处理顺序：已存在默认组 → 不动数据；已存在 code=default 的组 → 提升为默认；
// 其余情况 → 新建「默认用户组」。只新增/标记，绝不改动既有组的折扣策略，
// 因此对存量数据的算价零影响（新建的默认组不绑定 price_policy_id）。
func seedDefaultUserGroup(tx *gorm.DB) error {
	var defaultCount int64
	if err := tx.Model(&usermodel.UserGroup{}).Where("is_default = ?", true).Count(&defaultCount).Error; err != nil {
		return err
	}
	if defaultCount > 0 {
		return nil
	}

	// 已有 code=default 的组时只打默认标记，避免出现重复的「默认用户组」。
	// 用 Find 而非 First：未命中是正常分支，不需要 gorm 打印 record not found。
	var existing []usermodel.UserGroup
	if err := tx.Where("code = ?", "default").Limit(1).Find(&existing).Error; err != nil {
		return err
	}
	if len(existing) > 0 {
		return tx.Model(&usermodel.UserGroup{}).Where("id = ?", existing[0].ID).Update("is_default", true).Error
	}

	group := usermodel.UserGroup{
		Name:        "默认用户组",
		Code:        "default",
		Description: "新用户未指定分组时的兜底分组，可在用户组管理中调整默认组与折扣策略。",
		Status:      "active",
		SortOrder:   1,
		IsDefault:   true,
	}
	return tx.Create(&group).Error
}

// seedUserLevels 注入默认用户等级。
//
// 等级是「消费升级」的载体：按累计消费自动升级（只升不降），不参与折扣计算。
// 原先与等级同模块的资源配额（模板/上限/调整记录）已移除，见 migrations/017。
// 升级门槛升级为按累计消费（P3-02）：standard 0 / business 10000 / enterprise 50000，
// 子账号上限分别为 1 / 5 / 20；门槛为示例值，运营可在「用户等级」页调整。
func seedUserLevels(tx *gorm.DB) error {
	admin, err := loadAdminAsUser(tx)
	if err != nil {
		return err
	}
	levels := []levelmodel.UserLevel{
		{
			Name:             "标准用户",
			Code:             "standard",
			Weight:           10,
			Status:           "active",
			FeatureFlags:     "snapshot,backup",
			UpgradeCondition: "注册即获得",
			UpgradeThreshold: 0,
			MaxSubAccounts:   1,
			Benefits:         `{"benefits":["基础工单支持","每周自动备份"]}`,
			Description:      "默认用户等级",
			CreatedBy:        admin.ID,
			UpdatedBy:        admin.ID,
		},
		{
			Name:             "企业用户",
			Code:             "business",
			Weight:           20,
			Status:           "active",
			FeatureFlags:     "snapshot,backup,ha,custom-image",
			UpgradeCondition: "累计消费满 10000 元",
			UpgradeThreshold: 10000,
			MaxSubAccounts:   5,
			Benefits:         `{"benefits":["高优先级工单","每日自动备份","自定义镜像"]}`,
			Description:      "企业大客户等级",
			CreatedBy:        admin.ID,
			UpdatedBy:        admin.ID,
		},
		{
			Name:             "高级企业",
			Code:             "enterprise",
			Weight:           30,
			Status:           "active",
			FeatureFlags:     "snapshot,backup,ha,custom-image,dedicated-support",
			UpgradeCondition: "累计消费满 50000 元",
			UpgradeThreshold: 50000,
			MaxSubAccounts:   20,
			Benefits:         `{"benefits":["专属客户经理","SLA 保障","每日自动备份","自定义镜像"]}`,
			Description:      "高级企业等级",
			CreatedBy:        admin.ID,
			UpdatedBy:        admin.ID,
		},
	}

	for i := range levels {
		level := levels[i]
		var existing levelmodel.UserLevel
		if err := tx.Where("code = ?", level.Code).First(&existing).Error; err == nil {
			// 已存在的等级不覆盖运营改过的配置；仅在门槛/上限仍为 0（老数据）时补齐默认值。
			updates := map[string]any{}
			if existing.UpgradeThreshold == 0 && level.UpgradeThreshold > 0 {
				updates["upgrade_threshold"] = level.UpgradeThreshold
			}
			if existing.MaxSubAccounts == 0 && level.MaxSubAccounts > 0 {
				updates["max_sub_accounts"] = level.MaxSubAccounts
			}
			if existing.Benefits == "" && level.Benefits != "" {
				updates["benefits"] = level.Benefits
			}
			if len(updates) > 0 {
				if err := tx.Model(&levelmodel.UserLevel{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
					return err
				}
			}
			continue
		} else if err != gorm.ErrRecordNotFound {
			return err
		}
		if err := tx.Create(&level).Error; err != nil {
			return err
		}
	}
	return nil
}

// backfillUserConsumeTotals 将 users.total_consume_amount 落列的历史数据补齐（P3-01）。
// 口径与 P0-02 归一后的 wallet_transactions.type='consume' 一致；仅回填仍为 0 的用户，
// 幂等且不会覆盖消费升级服务已累加的值。
func backfillUserConsumeTotals(tx *gorm.DB) error {
	return tx.Exec(`
		UPDATE users u
		SET total_consume_amount = stats.total
		FROM (
			SELECT user_id, COALESCE(SUM(ABS(amount)), 0) AS total
			FROM wallet_transactions
			WHERE type = ?
			GROUP BY user_id
		) AS stats
		WHERE stats.user_id = u.id
		  AND u.total_consume_amount = 0
		  AND stats.total > 0`, "consume").Error
}

// seedNotificationTemplates 注入通知事件默认模板（doc70 §6.2）。
func seedNotificationTemplates(tx *gorm.DB) error {
	defaults := []notifymodel.NotificationTemplate{
		{Event: notifymodel.EventOrderPaid, TitleTpl: "订单 {order_no} 支付成功", ContentTpl: "您的订单 {order_no} 已支付成功，金额 ¥{amount}，感谢您的支持。", InboxOn: true, MailOn: false, Status: notifymodel.TemplateStatusActive},
		{Event: notifymodel.EventRenewalSuccess, TitleTpl: "实例续费成功", ContentTpl: "实例 {instance_mark} 续费成功，新到期时间 {expire_after}。", InboxOn: true, MailOn: false, Status: notifymodel.TemplateStatusActive},
		{Event: notifymodel.EventRenewalFailed, TitleTpl: "实例自动续费失败", ContentTpl: "实例 {instance_mark} 自动续费失败（{reason}），请及时处理，避免服务暂停。", InboxOn: true, MailOn: true, Status: notifymodel.TemplateStatusActive},
		{Event: notifymodel.EventInstanceExpiring, TitleTpl: "实例即将到期提醒", ContentTpl: "您的实例 {instance_mark} 将于 {expire_at} 到期（剩余 {days_left} 天），请及时续费。", InboxOn: true, MailOn: true, Status: notifymodel.TemplateStatusActive},
		{Event: notifymodel.EventTicketReplied, TitleTpl: "工单 {ticket_no} 有新回复", ContentTpl: "您的工单 {ticket_no} 有新的客服回复，请前往工单中心查看。", InboxOn: true, MailOn: false, Status: notifymodel.TemplateStatusActive},
		{Event: notifymodel.EventTicketAssigned, TitleTpl: "工单 {ticket_no} 已指派给您", ContentTpl: "工单 {ticket_no}（{title}）已指派给您，请及时跟进处理。", InboxOn: true, MailOn: false, Status: notifymodel.TemplateStatusActive},
		{Event: notifymodel.EventTicketStatus, TitleTpl: "工单 {ticket_no} 状态更新", ContentTpl: "您的工单 {ticket_no} 状态已更新为 {status}。", InboxOn: true, MailOn: false, Status: notifymodel.TemplateStatusActive},
		{Event: notifymodel.EventBalanceLow, TitleTpl: "余额不足预警", ContentTpl: "您的账户余额为 ¥{balance}，低于预警阈值 ¥{threshold}，请及时充值。", InboxOn: true, MailOn: true, Status: notifymodel.TemplateStatusActive},
		{Event: notifymodel.EventSyncFailed, TitleTpl: "上游同步失败", ContentTpl: "提供商 {provider_name} 同步失败：{reason}，请检查上游连接。", InboxOn: true, MailOn: false, Status: notifymodel.TemplateStatusActive},
		{Event: notifymodel.EventSystem, TitleTpl: "{title}", ContentTpl: "{content}", InboxOn: true, MailOn: false, Status: notifymodel.TemplateStatusActive},
	}
	for _, item := range defaults {
		var existing notifymodel.NotificationTemplate
		if err := tx.Where("event = ?", item.Event).First(&existing).Error; err == nil {
			continue
		} else if err != gorm.ErrRecordNotFound {
			return err
		}
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedSMTPConfigs 注入 SMTP 配置默认项（doc70 §6.6）。
func seedSMTPConfigs(tx *gorm.DB) error {
	defaults := []systemmodel.SystemConfig{
		{ConfigKey: "smtp_host", ConfigValue: "", ValueType: systemmodel.ValueTypeString, Group: "mail", Description: "SMTP 服务器地址", SortOrder: 1, Status: systemmodel.StatusActive},
		{ConfigKey: "smtp_port", ConfigValue: "587", ValueType: systemmodel.ValueTypeInt, Group: "mail", Description: "SMTP 端口", SortOrder: 2, Status: systemmodel.StatusActive},
		{ConfigKey: "smtp_user", ConfigValue: "", ValueType: systemmodel.ValueTypeString, Group: "mail", Description: "SMTP 用户名", SortOrder: 3, Status: systemmodel.StatusActive},
		{ConfigKey: "smtp_pass", ConfigValue: "", ValueType: systemmodel.ValueTypeString, Group: "mail", Description: "SMTP 密码", SortOrder: 4, Status: systemmodel.StatusActive},
		{ConfigKey: "smtp_from", ConfigValue: "", ValueType: systemmodel.ValueTypeString, Group: "mail", Description: "发件邮箱地址", SortOrder: 5, Status: systemmodel.StatusActive},
		{ConfigKey: "mail_channel_enabled", ConfigValue: "false", ValueType: systemmodel.ValueTypeBool, Group: "mail", Description: "是否启用邮件通知通道", SortOrder: 6, Status: systemmodel.StatusActive},
	}
	for _, config := range defaults {
		var existing systemmodel.SystemConfig
		if err := tx.Where("config_key = ?", config.ConfigKey).First(&existing).Error; err == nil {
			continue
		} else if err != gorm.ErrRecordNotFound {
			return err
		}
		if err := tx.Create(&config).Error; err != nil {
			return err
		}
	}
	return nil
}
