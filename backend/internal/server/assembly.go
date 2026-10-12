// 装配层核心（批次四 4b）：asm 装配上下文 + newAsm 地基 + 跨域接线。
// 各域的 buildX 方法见同目录 assembly_<域>.go；路由注册见 routes_*.go。

package server

import (
	"context"
	"fmt"
	finaccounthandler "hostsent/backend/internal/modules/admin/finance/account/handler"
	finrechargehandler "hostsent/backend/internal/modules/admin/finance/account/recharge/handler"
	finrechargerepo "hostsent/backend/internal/modules/admin/finance/account/recharge/repository"
	finrechargeservice "hostsent/backend/internal/modules/admin/finance/account/recharge/service"
	finaccountrepo "hostsent/backend/internal/modules/admin/finance/account/repository"
	finaccountservice "hostsent/backend/internal/modules/admin/finance/account/service"
	finbillhandler "hostsent/backend/internal/modules/admin/finance/bill/handler"
	finbillrepo "hostsent/backend/internal/modules/admin/finance/bill/repository"
	finbillservice "hostsent/backend/internal/modules/admin/finance/bill/service"
	referralhandler "hostsent/backend/internal/modules/admin/finance/referral/handler"
	referralrepo "hostsent/backend/internal/modules/admin/finance/referral/repository"
	referralservice "hostsent/backend/internal/modules/admin/finance/referral/service"
	finwithdrawhandler "hostsent/backend/internal/modules/admin/finance/referral/walletwithdraw/handler"
	finwithdrawrepo "hostsent/backend/internal/modules/admin/finance/referral/walletwithdraw/repository"
	finwithdrawservice "hostsent/backend/internal/modules/admin/finance/referral/walletwithdraw/service"
	finsettingshandler "hostsent/backend/internal/modules/admin/finance/settings/handler"
	finsettingsservice "hostsent/backend/internal/modules/admin/finance/settings/service"
	finstatshandler "hostsent/backend/internal/modules/admin/finance/stats/handler"
	finstatsservice "hostsent/backend/internal/modules/admin/finance/stats/service"
	fintransactionrepo "hostsent/backend/internal/modules/admin/finance/transaction/repository"
	instancehandler "hostsent/backend/internal/modules/admin/instance/handler"
	lifecyclehandler "hostsent/backend/internal/modules/admin/instance/lifecycle/handler"
	lifecyclerepo "hostsent/backend/internal/modules/admin/instance/lifecycle/repository"
	lifecycleservice "hostsent/backend/internal/modules/admin/instance/lifecycle/service"
	instancerepo "hostsent/backend/internal/modules/admin/instance/repository"
	instanceservice "hostsent/backend/internal/modules/admin/instance/service"
	adminhandler "hostsent/backend/internal/modules/admin/manager/handler"
	adminrepo "hostsent/backend/internal/modules/admin/manager/repository"
	adminservice "hostsent/backend/internal/modules/admin/manager/service"
	menuhandler "hostsent/backend/internal/modules/admin/menu/handler"
	menurepo "hostsent/backend/internal/modules/admin/menu/repository"
	menuservice "hostsent/backend/internal/modules/admin/menu/service"
	orderhandler "hostsent/backend/internal/modules/admin/order/handler"
	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	orderrepo "hostsent/backend/internal/modules/admin/order/repository"
	orderservice "hostsent/backend/internal/modules/admin/order/service"
	cataloghandler "hostsent/backend/internal/modules/admin/product/catalog/handler"
	catalogrepo "hostsent/backend/internal/modules/admin/product/catalog/repository"
	catalogservice "hostsent/backend/internal/modules/admin/product/catalog/service"
	categoryhandler "hostsent/backend/internal/modules/admin/product/category/handler"
	categoryrepo "hostsent/backend/internal/modules/admin/product/category/repository"
	categoryservice "hostsent/backend/internal/modules/admin/product/category/service"
	discounthandler "hostsent/backend/internal/modules/admin/product/discount/handler"
	discountrepo "hostsent/backend/internal/modules/admin/product/discount/repository"
	discountservice "hostsent/backend/internal/modules/admin/product/discount/service"
	flashdiscounthandler "hostsent/backend/internal/modules/admin/product/flashdiscount/handler"
	flashdiscountrepo "hostsent/backend/internal/modules/admin/product/flashdiscount/repository"
	flashdiscountservice "hostsent/backend/internal/modules/admin/product/flashdiscount/service"
	pricinghandler "hostsent/backend/internal/modules/admin/product/pricing/handler"
	pricingrepo "hostsent/backend/internal/modules/admin/product/pricing/repository"
	pricingservice "hostsent/backend/internal/modules/admin/product/pricing/service"
	promotionhandler "hostsent/backend/internal/modules/admin/product/promotion/handler"
	promotionrepo "hostsent/backend/internal/modules/admin/product/promotion/repository"
	promotionservice "hostsent/backend/internal/modules/admin/product/promotion/service"
	spechandler "hostsent/backend/internal/modules/admin/product/spec/handler"
	specrepo "hostsent/backend/internal/modules/admin/product/spec/repository"
	specservice "hostsent/backend/internal/modules/admin/product/spec/service"
	producthandler "hostsent/backend/internal/modules/admin/resource/product/handler"
	productrepo "hostsent/backend/internal/modules/admin/resource/product/repository"
	productservice "hostsent/backend/internal/modules/admin/resource/product/service"
	providerhandler "hostsent/backend/internal/modules/admin/resource/provider/handler"
	providerrepo "hostsent/backend/internal/modules/admin/resource/provider/repository"
	providerservice "hostsent/backend/internal/modules/admin/resource/provider/service"
	reconcilehandler "hostsent/backend/internal/modules/admin/resource/reconcile/handler"
	synchandler "hostsent/backend/internal/modules/admin/resource/sync/handler"
	syncrepo "hostsent/backend/internal/modules/admin/resource/sync/repository"
	syncservice "hostsent/backend/internal/modules/admin/resource/sync/service"
	taskqueuehandler "hostsent/backend/internal/modules/admin/resource/taskqueue/handler"
	systemhandler "hostsent/backend/internal/modules/admin/system/config/handler"
	systemrepo "hostsent/backend/internal/modules/admin/system/config/repository"
	systemservice "hostsent/backend/internal/modules/admin/system/config/service"
	notifydto "hostsent/backend/internal/modules/admin/system/notification/dto"
	notifyhandler "hostsent/backend/internal/modules/admin/system/notification/handler"
	notifymodel "hostsent/backend/internal/modules/admin/system/notification/model"
	notifyrepo "hostsent/backend/internal/modules/admin/system/notification/repository"
	notifyservice "hostsent/backend/internal/modules/admin/system/notification/service"
	tickethandler "hostsent/backend/internal/modules/admin/ticket/handler"
	ticketrepo "hostsent/backend/internal/modules/admin/ticket/repository"
	ticketservice "hostsent/backend/internal/modules/admin/ticket/service"
	"hostsent/backend/internal/modules/admin/user/account/handler"
	levelhandler "hostsent/backend/internal/modules/admin/user/account/level/handler"
	levelrepo "hostsent/backend/internal/modules/admin/user/account/level/repository"
	levelservice "hostsent/backend/internal/modules/admin/user/account/level/service"
	"hostsent/backend/internal/modules/admin/user/account/repository"
	"hostsent/backend/internal/modules/admin/user/account/service"
	agentlevelhandler "hostsent/backend/internal/modules/admin/user/agentlevel/handler"
	agentlevelrepo "hostsent/backend/internal/modules/admin/user/agentlevel/repository"
	agentlevelservice "hostsent/backend/internal/modules/admin/user/agentlevel/service"
	securityhandler "hostsent/backend/internal/modules/admin/user/security/handler"
	securityrepo "hostsent/backend/internal/modules/admin/user/security/repository"
	securityservice "hostsent/backend/internal/modules/admin/user/security/service"
	openhandler "hostsent/backend/internal/modules/open/handler"
	openrepo "hostsent/backend/internal/modules/open/repository"
	openservice "hostsent/backend/internal/modules/open/service"
	sitehandler "hostsent/backend/internal/modules/site/handler"
	siteservice "hostsent/backend/internal/modules/site/service"
	usercenterhandler "hostsent/backend/internal/modules/uc/auth/handler"
	usercenterrepo "hostsent/backend/internal/modules/uc/auth/repository"
	usercenterservice "hostsent/backend/internal/modules/uc/auth/service"
	userfinancehandler "hostsent/backend/internal/modules/uc/finance/handler"
	ucinstancehandler "hostsent/backend/internal/modules/uc/instance/handler"
	ucinstancerepo "hostsent/backend/internal/modules/uc/instance/repository"
	ucinstanceservice "hostsent/backend/internal/modules/uc/instance/service"
	memberhandler "hostsent/backend/internal/modules/uc/member/handler"
	memberrepo "hostsent/backend/internal/modules/uc/member/repository"
	memberservice "hostsent/backend/internal/modules/uc/member/service"
	usermenuhandler "hostsent/backend/internal/modules/uc/menu/handler"
	usermenurepo "hostsent/backend/internal/modules/uc/menu/repository"
	usermenuservice "hostsent/backend/internal/modules/uc/menu/service"
	ucorderhandler "hostsent/backend/internal/modules/uc/order/handler"
	ucorderservice "hostsent/backend/internal/modules/uc/order/service"
	ucproducthandler "hostsent/backend/internal/modules/uc/product/handler"
	ucproductservice "hostsent/backend/internal/modules/uc/product/service"
	ucreferralhandler "hostsent/backend/internal/modules/uc/referral/handler"
	appauth "hostsent/backend/internal/pkg/auth"
	"hostsent/backend/internal/pkg/cache"
	"hostsent/backend/internal/pkg/config"
	"hostsent/backend/internal/pkg/db"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/observability"
	"hostsent/backend/internal/pkg/pricing"
	"hostsent/backend/internal/pkg/publishsched"
	"hostsent/backend/internal/pkg/revalidate"
	"hostsent/backend/internal/pkg/safego"
	"hostsent/backend/internal/pkg/sessionguard"
	"hostsent/backend/internal/pkg/storage"
	"hostsent/backend/internal/pkg/upstream"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// asm 装配上下文：New() 的全部中间产物按域挂载，buildX 方法经它传递依赖（批次四 4b）。
type asm struct {
	// 地基（newAsm 构造）
	cfg                *config.Config
	logger             *zap.Logger
	database           *gorm.DB
	cacheClient        *cache.Client
	jwtIssuer          *appauth.JWTIssuer
	sessionGuard       *sessionguard.Guard
	adminRepo          adminrepo.AdminRepository
	rbacRepo           adminrepo.RBACRepository
	departmentRepo     adminrepo.DepartmentRepository
	permCache          *middleware.MemoryPermissionCache
	adminAuditRepo     adminrepo.AdminAuditRepository
	userRepo           repository.UserRepository
	userDetailRepo     repository.UserDetailRepository
	userGroupRepo      repository.UserGroupRepository
	roleRepo           repository.RoleRepository
	permissionRepo     repository.PermissionRepository
	menuRepo           menurepo.MenuRepository
	securityRepo       securityrepo.SecurityRepository
	orderRepo          orderrepo.OrderRepository
	orderItemRepo      orderrepo.OrderItemRepository
	orderRefundRepo    orderrepo.RefundRepository
	provisionTaskRepo  orderrepo.ProvisionTaskRepository
	salesBundle        *salesBundle
	openEventPublisher *openservice.EventPublisher
	// 其余各域产物（按名称排序）
	adminHandler             *adminhandler.AdminHandler
	adminReferralHandler     *referralhandler.ReferralHandler
	adminService             adminservice.AdminService
	agentLevelHandler        *agentlevelhandler.AgentLevelHandler
	agentLevelRepo           agentlevelrepo.AgentLevelRepository
	agentLevelService        agentlevelservice.AgentLevelService
	announceSvc              notifyservice.AnnouncementService
	attachmentStore          *storage.LocalStore
	billHandler              *finbillhandler.BillHandler
	billRepo                 finbillrepo.BillRepository
	billService              finbillservice.BillService
	blacklistGuard           *securityservice.BlacklistGuard
	captchaBundle            *captchaBundle
	configHandler            *systemhandler.ConfigHandler
	configRepo               systemrepo.ConfigRepository
	configService            systemservice.ConfigService
	configValueReader        func(ctx context.Context, key string) (string, bool, error)
	contentBundle            *contentBundle
	costBundle               costBundle
	couponGrantRepo          promotionrepo.CouponGrantRepository
	couponGrantService       promotionservice.CouponGrantService
	couponRepo               promotionrepo.CouponRepository
	couponService            promotionservice.CouponService
	departmentHandler        *adminhandler.DepartmentHandler
	departmentService        adminservice.DepartmentService
	discountPolicyHandler    *discounthandler.PolicyHandler
	discountPolicyRepo       discountrepo.PricePolicyRepository
	discountPolicyService    discountservice.PolicyService
	discountSchemeService    agentlevelservice.DiscountSchemeService
	financeSettingsHandler   *finsettingshandler.SettingsHandler
	financeSettingsService   finsettingsservice.SettingsService
	financeStatsHandler      *finstatshandler.StatsHandler
	financeStatsService      finstatsservice.StatsService
	flashDiscountHandler     *flashdiscounthandler.Handler
	flashDiscountRepo        flashdiscountrepo.Repository
	flashDiscountService     flashdiscountservice.Service
	instanceOpRepo           instancerepo.OperationRepository
	instanceOpsHandler       *instancehandler.InstanceHandler
	instanceOpsRepo          instancerepo.InstanceRepository
	instanceOpsService       instanceservice.InstanceService
	instanceRelatedRepo      instancerepo.RelatedRepository
	levelRepo                levelrepo.UserLevelRepository
	levelUpgradeService      levelservice.LevelUpgradeService
	lifecycleAdminHandler    *lifecyclehandler.LifecycleAdminHandler
	lifecycleAdvancer        *lifecycleservice.LifecycleAdvancer
	lifecycleAutoRepo        lifecyclerepo.AutoRenewRepository
	lifecycleExpiringHandler *lifecyclehandler.ExpiringHandler
	lifecycleInstanceReader  lifecyclerepo.InstanceReader
	lifecycleOrderWriter     lifecyclerepo.OrderWriter
	lifecyclePolicyRepo      lifecyclerepo.PolicyRepository
	lifecycleRenewalRepo     lifecyclerepo.RenewalRepository
	lifecycleRenewalSvc      lifecycleservice.RenewalService
	lifecycleScheduler       *lifecycleservice.LifecycleScheduler
	lifecycleSvc             lifecycleservice.LifecycleService
	lifecycleUserHandler     *lifecyclehandler.LifecycleUserHandler
	logcenterBundle          *logcenterBundle
	memberHandler            *memberhandler.MemberHandler
	memberRepo               memberrepo.MemberRepository
	memberService            memberservice.MemberService
	menuHandler              *menuhandler.MenuHandler
	menuService              menuservice.MenuService
	notifyAdminHandler       *notifyhandler.AdminHandler
	notifyAnnRepo            notifyrepo.AnnouncementRepository
	notifyBundleInst         *notifyBundle
	notifyMailChannel        notifyservice.MailChannel
	notifyPrefRepo           notifyrepo.PreferenceRepository
	notifyRepo               notifyrepo.NotificationRepository
	notifySvc                notifyservice.NotificationService
	notifyTplRepo            notifyrepo.TemplateRepository
	notifyUserHandler        *notifyhandler.UserHandler
	notifyWorker             *openservice.NotifyDeliveryWorker
	oauthBundle              *oauthBundle
	openBundle               *openhandler.Bundle
	orderHandler             *orderhandler.OrderHandler
	orderService             orderservice.OrderService
	paymentBundle            *paymentBundle
	pendingExpireScheduler   *ucorderservice.PendingExpireScheduler
	permissionHandler        *handler.PermissionHandler
	permissionService        service.PermissionService
	pointBundle              *pointBundle
	poolRepo                 providerrepo.PoolRepository
	portalRevalidator        *revalidate.HTTPNotifier
	preferenceSvc            notifyservice.PreferenceService
	priceMatrixHandler       *pricinghandler.PriceMatrixHandler
	priceMatrixRepo          pricingrepo.PriceMatrixRepository
	priceMatrixService       pricingservice.PriceMatrixService
	pricePipeline            *pricing.Service
	pricingHandler           *pricinghandler.PricingHandler
	pricingRepo              pricingrepo.PricingRepository
	pricingService           pricingservice.PricingService
	prodCatalogHandler       *cataloghandler.ProductHandler
	prodCatalogRepo          catalogrepo.ProductRepository
	prodCatalogService       catalogservice.ProductService
	prodCategoryHandler      *categoryhandler.CategoryHandler
	prodCategoryRepo         categoryrepo.CategoryRepository
	prodCategoryService      categoryservice.CategoryService
	productGroupService      agentlevelservice.ProductGroupService
	productHandler           *producthandler.ProductHandler
	productRepo              productrepo.ProductRepository
	productService           productservice.ProductService
	promotionHandler         *promotionhandler.PromotionHandler
	promotionRepo            promotionrepo.PromotionRepository
	promotionService         promotionservice.PromotionService
	providerHandler          *providerhandler.ProviderHandler
	providerRepo             providerrepo.ProviderRepository
	providerService          providerservice.ProviderService
	providerTypeRepo         providerrepo.ProviderTypeRepository
	provisionEnqueuer        *orderservice.ProvisionEnqueuer
	provisionWorker          *orderservice.ProvisionWorker
	publishScheduler         *publishsched.Scheduler
	rechargeHandler          *finrechargehandler.RechargeHandler
	rechargeRepo             finrechargerepo.RechargeRepository
	rechargeService          finrechargeservice.RechargeService
	reconHandler             *finbillhandler.ReconHandler
	reconService             finbillservice.ReconService
	reconcileHandler         *reconcilehandler.ReconcileHandler
	referralRepo             referralrepo.ReferralRepository
	referralSvc              referralInvitePort
	referralWithdrawSvc      referralservice.WithdrawalService
	refundHandler            *orderhandler.RefundHandler
	riskEngine               *securityservice.RiskEngine
	riskRepo                 securityrepo.RiskRepository
	roleHandler              *handler.RoleHandler
	roleService              service.RoleService
	scheduler                *syncservice.Scheduler
	schemeHandler            *agentlevelhandler.SchemeHandler
	schemeRepo               agentlevelrepo.SchemeRepository
	securityHandler          *securityhandler.SecurityHandler
	securityService          securityservice.SecurityService
	sessionExpireScheduler   *securityservice.SessionExpireScheduler
	siteHandler              *sitehandler.SiteHandler
	siteSvc                  siteservice.SiteService
	specContractRepo         specrepo.SpecContractRepository
	specContractService      specservice.SpecContractService
	specHandler              *spechandler.SpecHandler
	specMappingRepo          specrepo.SpecMappingRepository
	specMappingService       specservice.SpecMappingService
	specOptionRepo           specrepo.SpecOptionRepository
	specOptionService        specservice.SpecOptionService
	specTemplateRepo         specrepo.SpecTemplateRepository
	specTemplateService      specservice.SpecTemplateService
	syncEngine               *syncservice.SyncEngine
	syncFrameworkHandler     *synchandler.FrameworkHandler
	syncFrameworkService     syncservice.FrameworkService
	syncFwRepo               syncrepo.FrameworkRepository
	syncHandler              *synchandler.SyncHandler
	syncRepo                 syncrepo.SyncRepository
	syncService              syncservice.SyncService
	taskQueueHandler         *taskqueuehandler.TaskQueueHandler
	templateSvc              notifyservice.TemplateService
	ticketAttachmentRepo     ticketrepo.AttachmentRepository
	ticketAttachmentService  ticketservice.AttachmentService
	ticketCategoryHandler    *tickethandler.CategoryHandler
	ticketCategoryRepo       ticketrepo.CategoryRepository
	ticketCategoryService    ticketservice.CategoryService
	ticketHandler            *tickethandler.TicketHandler
	ticketPreconditionRepo   ticketrepo.PreconditionChecker
	ticketReplyRepo          ticketrepo.ReplyRepository
	ticketRepo               ticketrepo.TicketRepository
	ticketService            ticketservice.TicketService
	ucInstanceHandler        *ucinstancehandler.InstanceHandler
	ucInstanceRepo           ucinstancerepo.InstanceRepository
	ucInstanceService        ucinstanceservice.InstanceService
	ucOrderHandler           *ucorderhandler.OrderHandler
	ucOrderService           ucorderservice.OrderService
	ucProductHandler         *ucproducthandler.ProductHandler
	ucProductService         ucproductservice.ProductService
	ucReferralHandler        *ucreferralhandler.ReferralHandler
	upstreamMgr              *upstream.ProviderManager
	userCenterAuthHandler    *usercenterhandler.AuthHandler
	userCenterRepo           usercenterrepo.UserRepository
	userCenterService        usercenterservice.AuthService
	userDeletionHandler      *handler.UserDeletionHandler
	userDeletionService      service.DeletionService
	userDetailHandler        *handler.UserDetailHandler
	userDetailService        service.UserDetailService
	userFinanceHandler       *userfinancehandler.FinanceHandler
	userGroupHandler         *handler.UserGroupHandler
	userGroupService         service.UserGroupService
	userHandler              *handler.UserHandler
	userLevelHandler         *levelhandler.UserLevelHandler
	userLevelService         levelservice.UserLevelService
	userMenuHandler          *usermenuhandler.MenuHandler
	userMenuRepo             usermenurepo.MenuRepository
	userMenuService          usermenuservice.MenuService
	userPurgeScheduler       *service.PurgeScheduler
	userService              service.UserService
	userTicketHandler        *tickethandler.UserTicketHandler
	verificationBundle       *verificationBundle
	walletHandler            *finaccounthandler.WalletHandler
	walletRepo               finaccountrepo.WalletRepository
	walletService            finaccountservice.WalletService
	walletTxRepo             fintransactionrepo.TransactionRepository
	withdrawHandler          *finwithdrawhandler.WithdrawHandler
	withdrawRepo             finwithdrawrepo.WithdrawRepository
	withdrawService          finwithdrawservice.WithdrawService
	pointEarner              PointEarner
}

// newAsm 构造装配地基：DB/迁移/种子、缓存、门户缓存失效、JWT/会话守卫、基础仓储与销售/开放平台发布器。
func newAsm(cfg *config.Config, logger *zap.Logger) (*asm, error) {
	a := &asm{cfg: cfg, logger: logger}
	var err error
	a.database, err = db.New(a.cfg.Database)
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(a.database); err != nil {
		return nil, err
	}
	if err := db.Seed(a.database); err != nil {
		return nil, err
	}

	// 缓存地基（doc89 §3 / doc91 §2.2）：Redis 是加速器而非依赖。
	// Required=false 时连不通只降级（Warn + 进程内 LRU + 业务侧 DB 兜底），不阻断启动。
	a.cacheClient, err = cache.New(a.cfg.Redis, a.cfg.Redis.Required, a.logger)
	if err != nil {
		if a.cfg.Redis.Required {
			return nil, fmt.Errorf("redis required but unavailable: %w", err)
		}
		a.logger.Warn("redis unavailable, running in degraded mode", zap.Error(err))
		a.cacheClient = cache.NewDisabled(a.logger)
	}
	a.logger.Info("cache initialized", zap.Bool("redis_enabled", a.cacheClient.Enabled()))

	// 门户主动缓存失效（doc80 §10.1）：后台保存站点配置/发布内容后回调门户清 Nitro 缓存。
	// 未配地址时整体空转 —— 纯后端环境不该因为门户没起而刷告警。
	a.portalRevalidator = revalidate.NewHTTPNotifier(a.cfg.Revalidate.PortalURL, a.cfg.Revalidate.Token, a.logger)
	revalidate.SetNotifier(a.portalRevalidator)
	if a.portalRevalidator.Enabled() {
		a.logger.Info("portal revalidate enabled", zap.String("portal_url", a.cfg.Revalidate.PortalURL))
	}

	a.jwtIssuer = appauth.NewJWTIssuer(a.cfg.Auth.JWTSecret, a.cfg.Auth.JWTIssuer, time.Duration(a.cfg.Auth.JWTExpireHours)*time.Hour)
	// 会话有效性守卫：令牌里的 sid → user_sessions 状态，带缓存 + 撤销时显式失效。
	// 这是「强制下线/登出/改密撤销」能真正生效的唯一环节（JWT 本身不可撤回）。
	a.sessionGuard = sessionguard.New(sessionguard.NewGormStore(a.database), a.cacheClient, a.logger)
	a.adminRepo = adminrepo.NewAdminRepository(a.database)
	a.rbacRepo = adminrepo.NewRBACRepository(a.database)
	// departmentRepo 组织部门（S1 员工体系）：同时作为员工服务的部门名解析器。
	a.departmentRepo = adminrepo.NewDepartmentRepository(a.database)
	a.permCache = middleware.NewMemoryPermissionCache()
	a.adminAuditRepo = adminrepo.NewAdminAuditRepository(a.database, a.logger)
	a.userRepo = repository.NewUserRepository(a.database)
	a.userDetailRepo = repository.NewUserDetailRepository(a.database)
	a.userGroupRepo = repository.NewUserGroupRepository(a.database)
	a.roleRepo = repository.NewRoleRepository(a.database)
	a.permissionRepo = repository.NewPermissionRepository(a.database)
	a.menuRepo = menurepo.NewMenuRepository(a.database)
	a.securityRepo = securityrepo.NewSecurityRepository(a.database)
	// 订单域（仓储先行；履约适配器需依赖上游/商品服务，见下方装配）
	a.orderRepo = orderrepo.NewOrderRepository(a.database)
	a.orderItemRepo = orderrepo.NewOrderItemRepository(a.database)
	a.orderRefundRepo = orderrepo.NewRefundRepository(a.database)
	// 开通履约任务表（T5.1）：下单只投递任务，工作池异步执行上游开通（约 50s > write_timeout）。
	a.provisionTaskRepo = orderrepo.NewProvisionTaskRepository(a.database)
	// 销售体系（doc86 S4–S6）：客户归属 / 提成账本 / 提成提现 / 业绩排行。
	// 只依赖 database 与 logger，故尽早构造：员工离职转派、订单/续费钩子都要用。
	// 打款端口在支付中心装配完成后回填（见 wireSalesPayout）。
	a.salesBundle = buildSalesBundle(a.database, a.logger)
	// 开放平台事件发布器（P6/T6.5）：事件先落 open_notify_deliveries，投递由工作池异步执行；
	// 只依赖仓储，构造可早于通知中心。
	a.openEventPublisher = openservice.NewEventPublisher(
		openrepo.NewNotifyRepository(a.database),
		openrepo.NewAppRepository(a.database),
		a.logger,
	)
	return a, nil
}

// referralInvitePort 返现邀请端口（批次四 4b）：referralservice.referralService 为未导出类型，
// asm 字段以组合接口收窄依赖面——InviteService 供 handler 实参传递，计提/回冲供订单钩子调用。
type referralInvitePort interface {
	referralservice.InviteService
	// AccrueForOrder 订单支付后返现计提。
	AccrueForOrder(ctx context.Context, in referralservice.AccrualInput) error
	// ClawbackForRefund 退款后返现回冲。
	ClawbackForRefund(ctx context.Context, in referralservice.ClawbackInput) error
}

// buildBridges 跨域桥接：生命周期/工单/调价通知桥、退款实例联动与开通履约工作池。
func (a *asm) buildBridges() {
	// 续费完成事件（P6/T6.5）：instance.renewed → 开放平台回调。
	a.lifecycleRenewalSvc.SetRenewedHook(buildRenewedNotifier(a.openEventPublisher))
	// 生命周期 Notifier 桥接：替换 noopNotifier 为通知中心
	a.lifecycleSvc.SetNotifier(&lifecycleNotifierBridge{notifySvc: a.notifySvc, logger: a.logger})
	// 生命周期阶段推进器注入（T5.4）：能力缺失告警复用开通失败告警通道。
	a.lifecycleAdvancer.SetCapabilityNotifier(buildStageCapabilityNotifier(a.notifySvc, a.logger))
	a.lifecycleSvc.SetStageAdvancer(a.lifecycleAdvancer)
	// 退款 → 实例联动（doc61 §8 第 2 条）：策略从生命周期策略读（默认 none），
	// 处置动作走实例运维台（能力分派/流水/事件一并具备）。装配在此处因为需要
	// instanceOpsService 与 lifecyclePolicyRepo 同时在作用域内。
	a.instanceOpsService.SetRefundPolicyReader(buildRefundPolicyReader(a.lifecyclePolicyRepo, a.logger))
	a.orderService.SetInstanceRefundHook(buildRefundInstanceHook(a.instanceOpsService, a.logger))
	// 工单 Notifier 桥接（P2-04）：回复/状态通知用户，指派通知员工
	a.ticketService.SetNotifier(&ticketNotifierBridge{notifySvc: a.notifySvc, logger: a.logger})
	// 调价待确认通知桥接（T3.4）：上游改价超阈值时提醒运维处理
	a.syncEngine.SetNotifier(&priceChangeNotifierBridge{notifySvc: a.notifySvc, logger: a.logger})
	// 开通履约工作池（T5.1）：连续失败转人工队列时向管理端告警。
	a.provisionWorker = orderservice.NewProvisionWorker(
		a.provisionTaskRepo, a.orderService,
		(&provisionManualNotifier{notifySvc: a.notifySvc, logger: a.logger}).Notify,
		a.logger, orderservice.ProvisionWorkerOptions{},
	)
}

// buildBundles 构造积分/验证码/日志中心/实名/OAuth Bundle 并回填登录安全与风控端口。
func (a *asm) buildBundles() {
	// 积分体系（doc36）：独立账本，不参与任何资金口径；发放挂在下单/续费/账单结清事件上。
	a.pointBundle, a.pointEarner = buildPointBundle(a.database, a.logger)
	// 验证码与二次验证体系（doc91）：公开/管理端/用户端三组接口 + 登录守卫 + 限流。
	// 配置读取走 system_configs 的逐键查询（读不到按默认值，绝不因配置缺失阻断登录）。
	// OTP 下发走消息中心渠道路由（doc90 切换点），未接渠道表的部署自动回退历史 SMTP。
	a.captchaBundle = buildCaptchaBundle(a.cfg, a.database, a.cacheClient, a.jwtIssuer, a.configValueReader, a.notifyBundleInst.resolver, a.logger)
	// 日志中心（doc92 L1–L7）：上游采集写入器 + 任务留痕 + 26 源统一浏览/导出/清理。
	// 采集端点在装配时即注册（upstream.SetRecorder），业务侧适配器调用自动留痕。
	a.logcenterBundle = buildLogcenterBundle(logcenterBundleDeps{
		DB:           a.database,
		ConfigReader: a.configValueReader,
		ConfigIntOr:  logcenterConfigIntOr(a.configValueReader),
		ConfigBoolOr: logcenterConfigBoolOr(a.configValueReader),
		StorageRoot:  a.cfg.Storage.Root,
		Locker:       &logcenterRedisLocker{cache: a.cacheClient},
		AuditWriter:  a.adminAuditRepo,
		Logger:       a.logger,
	})
	// 登录安全端口回填（doc91 C3）：管理端与用户端认证服务都只依赖中性端口，
	// 不 import 验证码模块，admin → uc 的反向依赖不存在。
	if a.captchaBundle != nil && a.captchaBundle.port != nil {
		a.adminService.SetSecurityDeps(a.captchaBundle.port)
		a.userCenterService.SetSecurityPort(a.captchaBundle.port)
	}
	// 风控回填（doc06）：黑名单与规则引擎同样只经中性接口注入，两个认证服务
	// 都不 import 安全模块。未装配时黑名单恒放行、规则引擎不上报 —— 缺能力
	// 不会演变成「谁都登不上」或「页面刷满告警」。
	a.userCenterService.SetBlacklistChecker(a.blacklistGuard)
	a.userCenterService.SetLoginObserver(a.riskEngine)
	a.adminService.SetBlacklistChecker(a.blacklistGuard)
	a.adminService.SetLoginObserver(a.riskEngine)
	// 实名认证（doc104 §5）：整单审核状态机 + 三方核验 + 用户端提交。
	// 二次验证经 VerifyPort 端口接验证码策略，验证码模块不可用时不阻断实名提交；
	// 材料上传复用与工单附件同一个本地存储实例（attachmentStore 可能为 nil）。
	a.verificationBundle = buildVerificationBundle(a.cfg, a.database, a.captchaBundle, a.attachmentStore, a.logger)
	// 第三方登录（doc104 §6）：微信/QQ/支付宝的授权、绑定、自动注册与回调。
	// 回调基地址与前端回跳地址优先读系统配置（部署域名各异，写死等于每次部署改代码），
	// 这里给的只是配置缺失时的兜底值。
	a.oauthBundle = buildOAuthBundle(a.cfg, a.database, a.cacheClient, a.jwtIssuer, a.configValueReader,
		a.referralSvc, a.userGroupService, a.userCenterRepo,
		fmt.Sprintf("http://127.0.0.1:%d/api/v1/uc/oauth", a.cfg.App.Port),
		"/oauth/callback", a.logger)
}

// buildWire 收尾接线：积分发放钩子、OnPaid、子账号、开放平台与支付中心、收银台端口与各调度器。
func (a *asm) buildWire() {
	// 续费完成积分：lifecycle 侧独立发放，幂等键为订单号。
	a.lifecycleRenewalSvc.SetPointEarner(func(ctx context.Context, orderID uint64, orderNo string, userID uint64, amount float64) {
		a.pointEarner(ctx, userID, "order", orderNo, amount, true)
	})
	// 账单结清积分：规则默认关闭，避免与消费积分重复计发。
	a.billService.SetPointEarner(func(ctx context.Context, userID uint64, billNo string, amount float64) {
		a.pointEarner(ctx, userID, "bill", billNo, amount, false)
	})
	// 订单支付成功钩子（doc60）：续费订单支付完成后联动完成续费并延长到期时间
	// doc70：支付成功后发布通知；doc36：非续费订单在此发放积分（续费积分走 lifecycle 路径，避免重复）。
	//
	// 本钩子只在「渠道支付到账」路径触发（MarkPaidByChannel 调用）：余额支付在下单时即扣款，
	// 其累计消费在下单流程内记账，不会走到这里，因此这里的 ApplyConsume 不会重复累计。
	orderservice.OnPaid = func(ctx context.Context, order *ordermodel.Order) {
		if order == nil {
			return
		}
		observability.Inc("order_paid_total", 1)
		if order.RenewalID != 0 {
			if err := a.lifecycleRenewalSvc.CompleteRenewalByOrderID(ctx, order.ID); err != nil {
				a.logger.Error("lifecycle: complete renewal by order failed", zap.Uint64("order_id", order.ID), zap.Error(err))
			}
		} else {
			// 渠道支付的订单到账才累计消费并重算等级（下单时未收款，不能提前计入）。
			go func(userID uint64, amount float64) {
				defer safego.Recover(a.logger)
				cctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				if err := a.levelUpgradeService.ApplyConsume(cctx, userID, amount); err != nil {
					a.logger.Warn("user level: apply consume failed",
						zap.Uint64("user_id", userID), zap.Float64("amount", amount), zap.Error(err))
				}
			}(order.UserID, order.PaidAmount)
			a.pointEarner(ctx, order.UserID, "order", order.OrderNo, order.PaidAmount, false)
		}
		// 发布支付成功通知（doc70）
		_ = a.notifySvc.Publish(ctx, notifydto.PublishInput{
			Event:        notifymodel.EventOrderPaid,
			UserID:       order.UserID,
			Target:       notifymodel.TargetUser,
			Vars:         map[string]string{"order_no": order.OrderNo, "amount": fmt.Sprintf("%.2f", order.PaidAmount)},
			SourceModule: "order",
			SourceID:     order.OrderNo,
		})
	}
	// 用户中心成员（子账号，P4）：仓储同时作为 RequireUserPermission 的权限解析器。
	a.memberRepo = memberrepo.NewMemberRepository(a.database)
	a.memberService = memberservice.NewMemberService(a.memberRepo)
	a.memberHandler = memberhandler.NewMemberHandler(a.memberService)
	// 开放平台（P6）：网关与处理器集合，单一 Bundle 字段挂 App。
	// 目录服务复用 uc 商品（上架口径）、spec 契约字典与统一算价管线（D3）；
	// 代客下单复用 UC 下单管线（余额扣 owner、SKU/库存/算价/履约投递同一条路径）；
	// 实例接口复用实例运维台（电源/暂停/审计）与生命周期续费（双链路，T5.2）。
	a.openBundle = buildOpenBundle(a.cfg, a.database, a.ucProductService, a.ucOrderService, a.instanceOpsService, a.lifecycleRenewalSvc, a.specContractRepo, a.pricePipeline, a.logger)
	// 支付中心（doc35）：渠道管理 + 支付单 + 收银台，并注入提现打款与订单渠道退款钩子。
	a.paymentBundle = buildPaymentBundle(a.cfg, a.database, a.walletService, a.rechargeService, a.billService, a.orderService, a.withdrawService, a.salesBundle.payoutSettler(), a.logger)
	// 用户中心订单 ← 支付中心（doc88 §6.2）：待支付订单发起收银台支付；
	// 取消/超时关单时同步关闭未支付支付单，避免用户付款打到已作废的订单上。
	a.ucOrderService.SetCashierPorts(a.paymentBundle.orderService, a.paymentBundle.orderService)
	// 选配项校验与计价（T4.5）：同一个 catalog 服务实例，保证与算价管线同一套规则。
	a.ucOrderService.SetOptionPricingPort(a.prodCatalogService)
	// 待支付订单过期关单调度器：有效期每轮从系统配置 order_expire_minutes 重读（改配置无需重启）。
	a.pendingExpireScheduler = ucorderservice.NewPendingExpireScheduler(a.ucOrderService, func(ctx context.Context) int {
		item, err := a.configService.GetByKey(ctx, "order_expire_minutes")
		if err != nil || item == nil {
			return 0
		}
		minutes, err := strconv.Atoi(strings.TrimSpace(item.ConfigValue))
		if err != nil || minutes <= 0 {
			return 0
		}
		return minutes
	}, a.logger)
	// 销售提现打款出口：复用支付中心打款单，biz_type=sales_withdraw（doc86 §2.5）。
	a.salesBundle.wireSalesPayout(a.paymentBundle.payoutService)
	// 内容定时发布：管理端「定时发布时间」只在写入时判一次到点，到点后没人推进；
	// 这条调度把 draft + 已到 publish_at 的公告与文章翻成 published（doc100 §10 第三期 23）。
	a.publishScheduler = publishsched.NewScheduler(a.notifyAnnRepo, a.contentBundle.articleRepo, a.logger)
}

// firstNonEmpty 返回第一个非空字符串。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
