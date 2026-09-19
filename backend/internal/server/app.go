package server

// Package server 由本文件聚合装配层（DI 容器）：把所有 handler/service 依赖
// 统一收敛进 App 结构体，newRouter 只接收 *App，避免逐个传参导致"加模块改三处"。
//
// 设计（见 00 规划 Phase 1 · T1.1）：
//   - App 是装配层的单一真相；server.New 负责构造依赖并 NewApp(...)，再 RegisterRoutes。
//   - 扩展新模块只需：① 在 App 加字段 ② 在 NewApp 赋值 ③ 在路由表加一行。

import (
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	finaccounthandler "hostsent/backend/internal/modules/admin/finance/account/handler"
	finbillhandler "hostsent/backend/internal/modules/admin/finance/bill/handler"
	finrechargehandler "hostsent/backend/internal/modules/admin/finance/recharge/handler"
	finwithdrawhandler "hostsent/backend/internal/modules/admin/finance/withdraw/handler"
	admininstancehandler "hostsent/backend/internal/modules/admin/instance/handler"
	lifecyclehandler "hostsent/backend/internal/modules/admin/lifecycle/handler"
	adminhandler "hostsent/backend/internal/modules/admin/manager/handler"
	adminrepo "hostsent/backend/internal/modules/admin/manager/repository"
	menuhandler "hostsent/backend/internal/modules/admin/menu/handler"
	notifyhandler "hostsent/backend/internal/modules/admin/notification/handler"
	orderhandler "hostsent/backend/internal/modules/admin/order/handler"
	cataloghandler "hostsent/backend/internal/modules/admin/product/catalog/handler"
	categoryhandler "hostsent/backend/internal/modules/admin/product/category/handler"
	discounthandler "hostsent/backend/internal/modules/admin/product/discount/handler"
	pricinghandler "hostsent/backend/internal/modules/admin/product/pricing/handler"
	promotionhandler "hostsent/backend/internal/modules/admin/product/promotion/handler"
	spechandler "hostsent/backend/internal/modules/admin/product/spec/handler"
	referralhandler "hostsent/backend/internal/modules/admin/referral/handler"
	producthandler "hostsent/backend/internal/modules/admin/resource/product/handler"
	providerhandler "hostsent/backend/internal/modules/admin/resource/provider/handler"
	reconcilehandler "hostsent/backend/internal/modules/admin/resource/reconcile/handler"
	synchandler "hostsent/backend/internal/modules/admin/resource/sync/handler"
	taskqueuehandler "hostsent/backend/internal/modules/admin/resource/taskqueue/handler"
	saleshandler "hostsent/backend/internal/modules/admin/sales/handler"
	systemhandler "hostsent/backend/internal/modules/admin/system/handler"
	tickethandler "hostsent/backend/internal/modules/admin/ticket/handler"
	"hostsent/backend/internal/modules/admin/user/account/handler"
	levelhandler "hostsent/backend/internal/modules/admin/user/level/handler"
	securityhandler "hostsent/backend/internal/modules/admin/user/security/handler"
	openhandler "hostsent/backend/internal/modules/open/handler"
	sitehandler "hostsent/backend/internal/modules/site/handler"
	usercenterhandler "hostsent/backend/internal/modules/uc/auth/handler"
	userfinancehandler "hostsent/backend/internal/modules/uc/finance/handler"
	ucinstancehandler "hostsent/backend/internal/modules/uc/instance/handler"
	memberhandler "hostsent/backend/internal/modules/uc/member/handler"
	usermenuhandler "hostsent/backend/internal/modules/uc/menu/handler"
	ucorderhandler "hostsent/backend/internal/modules/uc/order/handler"
	ucproducthandler "hostsent/backend/internal/modules/uc/product/handler"
	ucreferralhandler "hostsent/backend/internal/modules/uc/referral/handler"
	appauth "hostsent/backend/internal/pkg/auth"
	"hostsent/backend/internal/pkg/cache"
	"hostsent/backend/internal/pkg/config"
	"hostsent/backend/internal/pkg/middleware"
)

// App 装配层依赖容器：newRouter 只接收 *App，handler/service 从字段取用。
type App struct {
	cfg       *config.Config
	logger    *zap.Logger
	jwtIssuer *appauth.JWTIssuer
	// cache 统一缓存端口（Redis + 进程内降级，doc89 §3.3）：验证码/限流/锁的共享依赖。
	cache *cache.Client
	// RBAC：员工多角色仓储 + 权限快照缓存（鉴权与权限中间件共用）。
	rbacRepo  adminrepo.RBACRepository
	permCache middleware.PermissionCache
	// auditWriter 管理端写操作审计落库（P2-06）。
	auditWriter middleware.AdminAuditWriter

	adminHandler *adminhandler.AdminHandler
	// departmentHandler 组织部门管理（S1 员工体系，doc86）。
	departmentHandler     *adminhandler.DepartmentHandler
	userHandler           *handler.UserHandler
	userDetailHandler     *handler.UserDetailHandler
	userDeletionHandler   *handler.UserDeletionHandler
	userGroupHandler      *handler.UserGroupHandler
	roleHandler           *handler.RoleHandler
	permissionHandler     *handler.PermissionHandler
	menuHandler           *menuhandler.MenuHandler
	securityHandler       *securityhandler.SecurityHandler
	userLevelHandler      *levelhandler.UserLevelHandler
	providerHandler       *providerhandler.ProviderHandler
	productHandler        *producthandler.ProductHandler
	syncHandler           *synchandler.SyncHandler
	syncFrameworkHandler  *synchandler.FrameworkHandler
	userCenterAuthHandler *usercenterhandler.AuthHandler
	userMenuHandler       *usermenuhandler.MenuHandler
	prodCategoryHandler   *categoryhandler.CategoryHandler
	prodCatalogHandler    *cataloghandler.ProductHandler
	specHandler           *spechandler.SpecHandler
	pricingHandler        *pricinghandler.PricingHandler
	priceMatrixHandler    *pricinghandler.PriceMatrixHandler
	discountPolicyHandler *discounthandler.PolicyHandler
	promotionHandler      *promotionhandler.PromotionHandler
	adminReferralHandler  *referralhandler.ReferralHandler
	// 销售体系（S4–S6，doc86）：客户归属 / 提成台账 / 提成提现 / 业绩排行。
	salesCustomerHandler    *saleshandler.CustomerHandler
	salesCommissionHandler  *saleshandler.CommissionHandler
	salesPerformanceHandler *saleshandler.PerformanceHandler
	orderHandler            *orderhandler.OrderHandler
	refundHandler           *orderhandler.RefundHandler
	walletHandler           *finaccounthandler.WalletHandler
	rechargeHandler         *finrechargehandler.RechargeHandler
	withdrawHandler         *finwithdrawhandler.WithdrawHandler
	billHandler             *finbillhandler.BillHandler
	reconHandler            *finbillhandler.ReconHandler
	configHandler           *systemhandler.ConfigHandler
	userFinanceHandler      *userfinancehandler.FinanceHandler
	ucProductHandler        *ucproducthandler.ProductHandler
	ucOrderHandler          *ucorderhandler.OrderHandler
	ucInstanceHandler       *ucinstancehandler.InstanceHandler
	instanceOpsHandler      *admininstancehandler.InstanceHandler
	// taskQueueHandler 平台动作任务队列（开通/实例动作/续费/同步）只读聚合视图（本轮 S3）。
	taskQueueHandler *taskqueuehandler.TaskQueueHandler
	// reconcileHandler 实例对账（本地售价/到期 vs 上游成本/到期）只读比对视图（本轮 S4）。
	reconcileHandler      *reconcilehandler.ReconcileHandler
	ticketHandler         *tickethandler.TicketHandler
	ticketCategoryHandler *tickethandler.CategoryHandler
	userTicketHandler     *tickethandler.UserTicketHandler
	expiringHandler       *lifecyclehandler.ExpiringHandler
	lifecycleAdminHandler *lifecyclehandler.LifecycleAdminHandler
	lifecycleUserHandler  *lifecyclehandler.LifecycleUserHandler
	notifyAdminHandler    *notifyhandler.AdminHandler
	notifyUserHandler     *notifyhandler.UserHandler
	siteHandler           *sitehandler.SiteHandler
	ucReferralHandler     *ucreferralhandler.ReferralHandler
	memberHandler         *memberhandler.MemberHandler
	// memberRepo 同时作为子账号权限解析器供 RequireUserPermission 使用（P4-06）。
	memberRepo middleware.SubAccountPermissionResolver
	// userAuditWriter 子账号写操作审计落库（P4-08），与 memberRepo 同一实现。
	userAuditWriter middleware.UserOperationLogWriter
	// open 开放平台处理器集合（P6/T6.1）：/open/v1 独立中间件链。
	open *openhandler.Bundle
	// payment 支付中心处理器集合（doc35）：admin 支付管理 + uc 收银台。
	payment *paymentBundle
	// point 积分体系处理器集合（doc36）：admin 积分中心 + uc 我的积分。
	point *pointBundle
	// captcha 验证码与二次验证体系处理器集合（doc91）：公开/管理端/用户端三组接口。
	captcha *captchaBundle
	// notify 消息中心处理器集合（doc90）：渠道/短信模板/群发/发送日志 + 投递工作器。
	notify *notifyBundle
	// logcenter 日志中心处理器集合（doc92）：统一日志浏览/导出/保留策略/清理引擎，
	// 以及上游接口采集写入器与任务运行留痕器。
	logcenter *logcenterBundle
	// content 内容中心处理器集合（doc100）：新闻/帮助/条款/隐私/分类/友情链接，
	// 以及供门户公开只读读取的适配器。
	content *contentBundle
	// verification 实名认证处理器集合（doc104 §5）：后台审核 + 用户端提交共用同一服务。
	verification *verificationBundle
	// oauth 第三方登录处理器集合（doc104 §6）：微信/QQ/支付宝。
	oauth *oauthBundle
}

// NewApp 构造装配容器（DI 单一接线点）。
// 参数顺序与旧 newRouter 一致：cfg → 各 handler → logger → jwtIssuer。
func NewApp(
	cfg *config.Config,
	adminHandler *adminhandler.AdminHandler,
	departmentHandler *adminhandler.DepartmentHandler,
	userHandler *handler.UserHandler,
	userDetailHandler *handler.UserDetailHandler,
	userDeletionHandler *handler.UserDeletionHandler,
	userGroupHandler *handler.UserGroupHandler,
	roleHandler *handler.RoleHandler,
	permissionHandler *handler.PermissionHandler,
	menuHandler *menuhandler.MenuHandler,
	securityHandler *securityhandler.SecurityHandler,
	userLevelHandler *levelhandler.UserLevelHandler,
	providerHandler *providerhandler.ProviderHandler,
	productHandler *producthandler.ProductHandler,
	syncHandler *synchandler.SyncHandler,
	syncFrameworkHandler *synchandler.FrameworkHandler,
	userCenterAuthHandler *usercenterhandler.AuthHandler,
	userMenuHandler *usermenuhandler.MenuHandler,
	prodCategoryHandler *categoryhandler.CategoryHandler,
	prodCatalogHandler *cataloghandler.ProductHandler,
	specHandler *spechandler.SpecHandler,
	pricingHandler *pricinghandler.PricingHandler,
	priceMatrixHandler *pricinghandler.PriceMatrixHandler,
	discountPolicyHandler *discounthandler.PolicyHandler,
	promotionHandler *promotionhandler.PromotionHandler,
	adminReferralHandler *referralhandler.ReferralHandler,
	salesCustomerHandler *saleshandler.CustomerHandler,
	salesCommissionHandler *saleshandler.CommissionHandler,
	salesPerformanceHandler *saleshandler.PerformanceHandler,
	orderHandler *orderhandler.OrderHandler,
	refundHandler *orderhandler.RefundHandler,
	walletHandler *finaccounthandler.WalletHandler,
	rechargeHandler *finrechargehandler.RechargeHandler,
	withdrawHandler *finwithdrawhandler.WithdrawHandler,
	billHandler *finbillhandler.BillHandler,
	reconHandler *finbillhandler.ReconHandler,
	configHandler *systemhandler.ConfigHandler,
	userFinanceHandler *userfinancehandler.FinanceHandler,
	ucProductHandler *ucproducthandler.ProductHandler,
	ucOrderHandler *ucorderhandler.OrderHandler,
	ucInstanceHandler *ucinstancehandler.InstanceHandler,
	instanceOpsHandler *admininstancehandler.InstanceHandler,
	taskQueueHandler *taskqueuehandler.TaskQueueHandler,
	reconcileHandler *reconcilehandler.ReconcileHandler,
	ticketHandler *tickethandler.TicketHandler,
	ticketCategoryHandler *tickethandler.CategoryHandler,
	userTicketHandler *tickethandler.UserTicketHandler,
	expiringHandler *lifecyclehandler.ExpiringHandler,
	lifecycleAdminHandler *lifecyclehandler.LifecycleAdminHandler,
	lifecycleUserHandler *lifecyclehandler.LifecycleUserHandler,
	notifyAdminHandler *notifyhandler.AdminHandler,
	notifyUserHandler *notifyhandler.UserHandler,
	siteHandler *sitehandler.SiteHandler,
	ucReferralHandler *ucreferralhandler.ReferralHandler,
	memberHandler *memberhandler.MemberHandler,
	memberRepo middleware.SubAccountPermissionResolver,
	userAuditWriter middleware.UserOperationLogWriter,
	rbacRepo adminrepo.RBACRepository,
	permCache middleware.PermissionCache,
	auditWriter middleware.AdminAuditWriter,
	open *openhandler.Bundle,
	payment *paymentBundle,
	point *pointBundle,
	captchaBundle *captchaBundle,
	notifyBundle *notifyBundle,
	logcenterBundle *logcenterBundle,
	contentBundle *contentBundle,
	verificationBundle *verificationBundle,
	oauthBundle *oauthBundle,
	cacheClient *cache.Client,
	logger *zap.Logger,
	jwtIssuer *appauth.JWTIssuer,
) *App {
	return &App{
		cfg:                     cfg,
		logger:                  logger,
		jwtIssuer:               jwtIssuer,
		cache:                   cacheClient,
		rbacRepo:                rbacRepo,
		permCache:               permCache,
		auditWriter:             auditWriter,
		adminHandler:            adminHandler,
		departmentHandler:       departmentHandler,
		userHandler:             userHandler,
		userDetailHandler:       userDetailHandler,
		userDeletionHandler:     userDeletionHandler,
		userGroupHandler:        userGroupHandler,
		roleHandler:             roleHandler,
		permissionHandler:       permissionHandler,
		menuHandler:             menuHandler,
		securityHandler:         securityHandler,
		userLevelHandler:        userLevelHandler,
		providerHandler:         providerHandler,
		productHandler:          productHandler,
		syncHandler:             syncHandler,
		syncFrameworkHandler:    syncFrameworkHandler,
		userCenterAuthHandler:   userCenterAuthHandler,
		userMenuHandler:         userMenuHandler,
		prodCategoryHandler:     prodCategoryHandler,
		prodCatalogHandler:      prodCatalogHandler,
		specHandler:             specHandler,
		pricingHandler:          pricingHandler,
		priceMatrixHandler:      priceMatrixHandler,
		discountPolicyHandler:   discountPolicyHandler,
		promotionHandler:        promotionHandler,
		adminReferralHandler:    adminReferralHandler,
		salesCustomerHandler:    salesCustomerHandler,
		salesCommissionHandler:  salesCommissionHandler,
		salesPerformanceHandler: salesPerformanceHandler,
		orderHandler:            orderHandler,
		refundHandler:           refundHandler,
		walletHandler:           walletHandler,
		rechargeHandler:         rechargeHandler,
		withdrawHandler:         withdrawHandler,
		billHandler:             billHandler,
		reconHandler:            reconHandler,
		configHandler:           configHandler,
		userFinanceHandler:      userFinanceHandler,
		ucProductHandler:        ucProductHandler,
		ucOrderHandler:          ucOrderHandler,
		ucInstanceHandler:       ucInstanceHandler,
		instanceOpsHandler:      instanceOpsHandler,
		taskQueueHandler:        taskQueueHandler,
		reconcileHandler:        reconcileHandler,
		ticketHandler:           ticketHandler,
		ticketCategoryHandler:   ticketCategoryHandler,
		userTicketHandler:       userTicketHandler,
		expiringHandler:         expiringHandler,
		lifecycleAdminHandler:   lifecycleAdminHandler,
		lifecycleUserHandler:    lifecycleUserHandler,
		notifyAdminHandler:      notifyAdminHandler,
		notifyUserHandler:       notifyUserHandler,
		siteHandler:             siteHandler,
		ucReferralHandler:       ucReferralHandler,
		memberHandler:           memberHandler,
		memberRepo:              memberRepo,
		userAuditWriter:         userAuditWriter,
		open:                    open,
		payment:                 payment,
		point:                   point,
		captcha:                 captchaBundle,
		notify:                  notifyBundle,
		logcenter:               logcenterBundle,
		content:                 contentBundle,
		verification:            verificationBundle,
		oauth:                   oauthBundle,
	}
}

// adminAuth 后台管理路由统一鉴权：解析 token 并加载真实角色/权限快照。
func (a *App) adminAuth() gin.HandlerFunc {
	return middleware.AdminAuth(a.jwtIssuer, a.cfg.Auth.BearerPrefix, a.rbacRepo, a.permCache)
}

// userAuth 用户中心路由统一鉴权（普通用户令牌）。
//
// 与 adminAuth 对称：装配层里的模块化 Bundle（如 assembly_oauth.go）需要挂
// 用户端鉴权时不必各自重复拼 middleware.UserAuth 的参数。
func (a *App) userAuth() gin.HandlerFunc {
	return middleware.UserAuth(a.jwtIssuer, a.cfg.Auth.BearerPrefix)
}

// perm 要求任一权限码即可访问（超管 "*" 恒通过）。
func (a *App) perm(codes ...string) gin.HandlerFunc {
	return middleware.RequirePermission(a.jwtIssuer, a.cfg.Auth.BearerPrefix, a.permCache, a.rbacRepo, codes...)
}

// superOnly 仅超级管理员可访问（管理员增删改、角色权限分配、代登录）。
func (a *App) superOnly() gin.HandlerFunc {
	return middleware.RequireSuperAdmin(a.jwtIssuer, a.cfg.Auth.BearerPrefix, a.permCache, a.rbacRepo)
}

// adminAudit 记录后台写操作（POST/PUT/PATCH/DELETE）到管理端审计表（P2-06）。
func (a *App) adminAudit() gin.HandlerFunc {
	return middleware.AdminAudit(a.auditWriter)
}

// userPerm 校验子账号是否持有任一权限码（主账号直接放行，P4-06）。
func (a *App) userPerm(codes ...string) gin.HandlerFunc {
	return middleware.RequireUserPermission(a.memberRepo, codes...)
}

// rejectSub 硬编码拒绝子账号（充值/提现/退款/实名等资金与身份类接口）。
func (a *App) rejectSub() gin.HandlerFunc {
	return middleware.RejectSubAccount()
}

// userAudit 记录子账号写操作到 user_operation_logs（P4-08）。
func (a *App) userAudit() gin.HandlerFunc {
	return middleware.UserOperationAudit(a.userAuditWriter)
}

// invalidateAdmin 清理指定管理员的权限缓存（角色/状态变更后调用）。
func (a *App) invalidateAdmin(adminID uint64) {
	if a.permCache != nil {
		a.permCache.InvalidateAdmin(adminID)
	}
}

// ucRateLimit 仅对 /api/v1/uc 前缀生效的限流（doc91 §9.1 api_rate_limit）。
//
// 用户中心路由分散在多个 Group（auth/finance/orders/instances/support/...），
// 逐组挂载容易漏；这里用前缀判断统一挂一条，语义等价且不可能漏挂。
func (a *App) ucRateLimit() gin.HandlerFunc {
	if a.captcha == nil {
		return func(c *gin.Context) { c.Next() }
	}
	handler := a.captcha.userLimiter.Handler()
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/api/v1/uc") {
			c.Next()
			return
		}
		handler(c)
	}
}

// userRequireVerification 用户端关键操作二次验证中间件（doc91 §6.1）。
//
// 策略不要求 OTP 时直接放行，因此默认（captcha_enabled=false）挂上也不改变任何
// 现有行为；运营逐场景开启后生效。
func (a *App) userRequireVerification(scene string) gin.HandlerFunc {
	if a.captcha == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return a.captcha.verifyTicket(scene, false)
}

// adminRequireVerification 管理端关键操作二次验证中间件（doc91 §6.2），
// subject 取 adminID，票据 key 前缀为 auth:verify_ticket:admin:{id}:{scene}。
func (a *App) adminRequireVerification(scene string) gin.HandlerFunc {
	if a.captcha == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return a.captcha.verifyTicket(scene, true)
}
