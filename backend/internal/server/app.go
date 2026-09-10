package server

// Package server 由本文件聚合装配层（DI 容器）：把所有 handler/service 依赖
// 统一收敛进 App 结构体，newRouter 只接收 *App，避免逐个传参导致"加模块改三处"。
//
// 设计（见 00 规划 Phase 1 · T1.1）：
//   - App 是装配层的单一真相；server.New 负责构造依赖并 NewApp(...)，再 RegisterRoutes。
//   - 扩展新模块只需：① 在 App 加字段 ② 在 NewApp 赋值 ③ 在路由表加一行。

import (
	"go.uber.org/zap"

	finaccounthandler "hostsent/backend/internal/modules/admin/finance/account/handler"
	finbillhandler "hostsent/backend/internal/modules/admin/finance/bill/handler"
	finrechargehandler "hostsent/backend/internal/modules/admin/finance/recharge/handler"
	finwithdrawhandler "hostsent/backend/internal/modules/admin/finance/withdraw/handler"
	lifecyclehandler "hostsent/backend/internal/modules/admin/lifecycle/handler"
	adminhandler "hostsent/backend/internal/modules/admin/manager/handler"
	menuhandler "hostsent/backend/internal/modules/admin/menu/handler"
	notifyhandler "hostsent/backend/internal/modules/admin/notification/handler"
	orderhandler "hostsent/backend/internal/modules/admin/order/handler"
	cataloghandler "hostsent/backend/internal/modules/admin/product/catalog/handler"
	categoryhandler "hostsent/backend/internal/modules/admin/product/category/handler"
	pricinghandler "hostsent/backend/internal/modules/admin/product/pricing/handler"
	promotionhandler "hostsent/backend/internal/modules/admin/product/promotion/handler"
	spechandler "hostsent/backend/internal/modules/admin/product/spec/handler"
	producthandler "hostsent/backend/internal/modules/admin/resource/product/handler"
	providerhandler "hostsent/backend/internal/modules/admin/resource/provider/handler"
	synchandler "hostsent/backend/internal/modules/admin/resource/sync/handler"
	systemhandler "hostsent/backend/internal/modules/admin/system/handler"
	tickethandler "hostsent/backend/internal/modules/admin/ticket/handler"
	"hostsent/backend/internal/modules/admin/user/account/handler"
	distributionhandler "hostsent/backend/internal/modules/admin/user/distribution/handler"
	levelhandler "hostsent/backend/internal/modules/admin/user/level/handler"
	securityhandler "hostsent/backend/internal/modules/admin/user/security/handler"
	verificationhandler "hostsent/backend/internal/modules/admin/user/verification/handler"
	usercenterhandler "hostsent/backend/internal/modules/uc/auth/handler"
	userfinancehandler "hostsent/backend/internal/modules/uc/finance/handler"
	ucinstancehandler "hostsent/backend/internal/modules/uc/instance/handler"
	usermenuhandler "hostsent/backend/internal/modules/uc/menu/handler"
	ucorderhandler "hostsent/backend/internal/modules/uc/order/handler"
	ucproducthandler "hostsent/backend/internal/modules/uc/product/handler"
	ucsitehandler "hostsent/backend/internal/modules/uc/site/handler"
	appauth "hostsent/backend/internal/pkg/auth"
	"hostsent/backend/internal/pkg/config"
)

// App 装配层依赖容器：newRouter 只接收 *App，handler/service 从字段取用。
type App struct {
	cfg       *config.Config
	logger    *zap.Logger
	jwtIssuer *appauth.JWTIssuer

	adminHandler          *adminhandler.AdminHandler
	userHandler           *handler.UserHandler
	userDetailHandler     *handler.UserDetailHandler
	userGroupHandler      *handler.UserGroupHandler
	agentLevelHandler     *distributionhandler.AgentLevelHandler
	agentHandler          *distributionhandler.AgentHandler
	subordinateHandler    *distributionhandler.SubordinateHandler
	commissionHandler     *distributionhandler.CommissionHandler
	settlementHandler     *distributionhandler.SettlementHandler
	roleHandler           *handler.RoleHandler
	permissionHandler     *handler.PermissionHandler
	menuHandler           *menuhandler.MenuHandler
	securityHandler       *securityhandler.SecurityHandler
	userLevelHandler      *levelhandler.UserLevelHandler
	verificationHandler   *verificationhandler.VerificationHandler
	providerHandler       *providerhandler.ProviderHandler
	productHandler        *producthandler.ProductHandler
	syncHandler           *synchandler.SyncHandler
	userCenterAuthHandler *usercenterhandler.AuthHandler
	userMenuHandler       *usermenuhandler.MenuHandler
	prodCategoryHandler   *categoryhandler.CategoryHandler
	prodCatalogHandler    *cataloghandler.ProductHandler
	specHandler           *spechandler.SpecHandler
	pricingHandler        *pricinghandler.PricingHandler
	promotionHandler      *promotionhandler.PromotionHandler
	orderHandler          *orderhandler.OrderHandler
	refundHandler         *orderhandler.RefundHandler
	walletHandler         *finaccounthandler.WalletHandler
	rechargeHandler       *finrechargehandler.RechargeHandler
	withdrawHandler       *finwithdrawhandler.WithdrawHandler
	billHandler           *finbillhandler.BillHandler
	reconHandler          *finbillhandler.ReconHandler
	configHandler         *systemhandler.ConfigHandler
	userFinanceHandler    *userfinancehandler.FinanceHandler
	ucProductHandler      *ucproducthandler.ProductHandler
	ucOrderHandler        *ucorderhandler.OrderHandler
	ucInstanceHandler     *ucinstancehandler.InstanceHandler
	ticketHandler         *tickethandler.TicketHandler
	ticketCategoryHandler *tickethandler.CategoryHandler
	userTicketHandler     *tickethandler.UserTicketHandler
	expiringHandler       *lifecyclehandler.ExpiringHandler
	lifecycleAdminHandler *lifecyclehandler.LifecycleAdminHandler
	lifecycleUserHandler  *lifecyclehandler.LifecycleUserHandler
	notifyAdminHandler    *notifyhandler.AdminHandler
	notifyUserHandler     *notifyhandler.UserHandler
	ucSiteHandler         *ucsitehandler.SiteHandler
}

// NewApp 构造装配容器（DI 单一接线点）。
// 参数顺序与旧 newRouter 一致：cfg → 各 handler → logger → jwtIssuer。
func NewApp(
	cfg *config.Config,
	adminHandler *adminhandler.AdminHandler,
	userHandler *handler.UserHandler,
	userDetailHandler *handler.UserDetailHandler,
	userGroupHandler *handler.UserGroupHandler,
	agentLevelHandler *distributionhandler.AgentLevelHandler,
	agentHandler *distributionhandler.AgentHandler,
	subordinateHandler *distributionhandler.SubordinateHandler,
	commissionHandler *distributionhandler.CommissionHandler,
	settlementHandler *distributionhandler.SettlementHandler,
	roleHandler *handler.RoleHandler,
	permissionHandler *handler.PermissionHandler,
	menuHandler *menuhandler.MenuHandler,
	securityHandler *securityhandler.SecurityHandler,
	userLevelHandler *levelhandler.UserLevelHandler,
	verificationHandler *verificationhandler.VerificationHandler,
	providerHandler *providerhandler.ProviderHandler,
	productHandler *producthandler.ProductHandler,
	syncHandler *synchandler.SyncHandler,
	userCenterAuthHandler *usercenterhandler.AuthHandler,
	userMenuHandler *usermenuhandler.MenuHandler,
	prodCategoryHandler *categoryhandler.CategoryHandler,
	prodCatalogHandler *cataloghandler.ProductHandler,
	specHandler *spechandler.SpecHandler,
	pricingHandler *pricinghandler.PricingHandler,
	promotionHandler *promotionhandler.PromotionHandler,
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
	ticketHandler *tickethandler.TicketHandler,
	ticketCategoryHandler *tickethandler.CategoryHandler,
	userTicketHandler *tickethandler.UserTicketHandler,
	expiringHandler *lifecyclehandler.ExpiringHandler,
	lifecycleAdminHandler *lifecyclehandler.LifecycleAdminHandler,
	lifecycleUserHandler *lifecyclehandler.LifecycleUserHandler,
	notifyAdminHandler *notifyhandler.AdminHandler,
	notifyUserHandler *notifyhandler.UserHandler,
	ucSiteHandler *ucsitehandler.SiteHandler,
	logger *zap.Logger,
	jwtIssuer *appauth.JWTIssuer,
) *App {
	return &App{
		cfg:                   cfg,
		logger:                logger,
		jwtIssuer:             jwtIssuer,
		adminHandler:          adminHandler,
		userHandler:           userHandler,
		userDetailHandler:     userDetailHandler,
		userGroupHandler:      userGroupHandler,
		agentLevelHandler:     agentLevelHandler,
		agentHandler:          agentHandler,
		subordinateHandler:    subordinateHandler,
		commissionHandler:     commissionHandler,
		settlementHandler:     settlementHandler,
		roleHandler:           roleHandler,
		permissionHandler:     permissionHandler,
		menuHandler:           menuHandler,
		securityHandler:       securityHandler,
		userLevelHandler:      userLevelHandler,
		verificationHandler:   verificationHandler,
		providerHandler:       providerHandler,
		productHandler:        productHandler,
		syncHandler:           syncHandler,
		userCenterAuthHandler: userCenterAuthHandler,
		userMenuHandler:       userMenuHandler,
		prodCategoryHandler:   prodCategoryHandler,
		prodCatalogHandler:    prodCatalogHandler,
		specHandler:           specHandler,
		pricingHandler:        pricingHandler,
		promotionHandler:      promotionHandler,
		orderHandler:          orderHandler,
		refundHandler:         refundHandler,
		walletHandler:         walletHandler,
		rechargeHandler:       rechargeHandler,
		withdrawHandler:       withdrawHandler,
		billHandler:           billHandler,
		reconHandler:          reconHandler,
		configHandler:         configHandler,
		userFinanceHandler:    userFinanceHandler,
		ucProductHandler:      ucProductHandler,
		ucOrderHandler:        ucOrderHandler,
		ucInstanceHandler:     ucInstanceHandler,
		ticketHandler:         ticketHandler,
		ticketCategoryHandler: ticketCategoryHandler,
		userTicketHandler:     userTicketHandler,
		expiringHandler:       expiringHandler,
		lifecycleAdminHandler: lifecycleAdminHandler,
		lifecycleUserHandler:  lifecycleUserHandler,
		notifyAdminHandler:    notifyAdminHandler,
		notifyUserHandler:     notifyUserHandler,
		ucSiteHandler:         ucSiteHandler,
	}
}
