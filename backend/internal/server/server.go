package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	financehandler "hostsent/backend/internal/modules/admin/finance/handler"
	financerepo "hostsent/backend/internal/modules/admin/finance/repository"
	financeservice "hostsent/backend/internal/modules/admin/finance/service"
	adminhandler "hostsent/backend/internal/modules/admin/manager/handler"
	adminrepo "hostsent/backend/internal/modules/admin/manager/repository"
	adminservice "hostsent/backend/internal/modules/admin/manager/service"
	menuhandler "hostsent/backend/internal/modules/admin/menu/handler"
	menurepo "hostsent/backend/internal/modules/admin/menu/repository"
	menuservice "hostsent/backend/internal/modules/admin/menu/service"
	orderhandler "hostsent/backend/internal/modules/admin/order/handler"
	orderrepo "hostsent/backend/internal/modules/admin/order/repository"
	orderservice "hostsent/backend/internal/modules/admin/order/service"
	prodhandler "hostsent/backend/internal/modules/admin/product/handler"
	prodrepo "hostsent/backend/internal/modules/admin/product/repository"
	prodservice "hostsent/backend/internal/modules/admin/product/service"
	producthandler "hostsent/backend/internal/modules/admin/resource/product/handler"
	productrepo "hostsent/backend/internal/modules/admin/resource/product/repository"
	productservice "hostsent/backend/internal/modules/admin/resource/product/service"
	providerhandler "hostsent/backend/internal/modules/admin/resource/provider/handler"
	providerrepo "hostsent/backend/internal/modules/admin/resource/provider/repository"
	providerservice "hostsent/backend/internal/modules/admin/resource/provider/service"
	synchandler "hostsent/backend/internal/modules/admin/resource/sync/handler"
	syncrepo "hostsent/backend/internal/modules/admin/resource/sync/repository"
	syncservice "hostsent/backend/internal/modules/admin/resource/sync/service"
	systemhandler "hostsent/backend/internal/modules/admin/system/handler"
	systemrepo "hostsent/backend/internal/modules/admin/system/repository"
	systemservice "hostsent/backend/internal/modules/admin/system/service"
	tickethandler "hostsent/backend/internal/modules/admin/ticket/handler"
	ticketrepo "hostsent/backend/internal/modules/admin/ticket/repository"
	ticketservice "hostsent/backend/internal/modules/admin/ticket/service"
	"hostsent/backend/internal/modules/admin/user/account/handler"
	"hostsent/backend/internal/modules/admin/user/account/repository"
	"hostsent/backend/internal/modules/admin/user/account/service"
	distributionhandler "hostsent/backend/internal/modules/admin/user/distribution/handler"
	distributionrepo "hostsent/backend/internal/modules/admin/user/distribution/repository"
	distributionservice "hostsent/backend/internal/modules/admin/user/distribution/service"
	quotahandler "hostsent/backend/internal/modules/admin/user/quota/handler"
	quotarepo "hostsent/backend/internal/modules/admin/user/quota/repository"
	quotaservice "hostsent/backend/internal/modules/admin/user/quota/service"
	securityhandler "hostsent/backend/internal/modules/admin/user/security/handler"
	securityrepo "hostsent/backend/internal/modules/admin/user/security/repository"
	securityservice "hostsent/backend/internal/modules/admin/user/security/service"
	verificationhandler "hostsent/backend/internal/modules/admin/user/verification/handler"
	verificationrepo "hostsent/backend/internal/modules/admin/user/verification/repository"
	verificationservice "hostsent/backend/internal/modules/admin/user/verification/service"
	usercenterhandler "hostsent/backend/internal/modules/user/auth/handler"
	usercenterrepo "hostsent/backend/internal/modules/user/auth/repository"
	usercenterservice "hostsent/backend/internal/modules/user/auth/service"
	userfinancehandler "hostsent/backend/internal/modules/user/finance/handler"
	usermenuhandler "hostsent/backend/internal/modules/user/menu/handler"
	usermenurepo "hostsent/backend/internal/modules/user/menu/repository"
	usermenuservice "hostsent/backend/internal/modules/user/menu/service"
	appauth "hostsent/backend/internal/pkg/auth"
	"hostsent/backend/internal/pkg/config"
	"hostsent/backend/internal/pkg/db"
	"hostsent/backend/internal/pkg/netutil"
	"hostsent/backend/internal/pkg/upstream"
)

type Server struct {
	cfg       *config.Config
	logger    *zap.Logger
	http      *http.Server
	scheduler *syncservice.Scheduler
	cancel    context.CancelFunc
}

func New(cfg *config.Config, logger *zap.Logger) (*Server, error) {
	database, err := db.New(cfg.Database)
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(database); err != nil {
		return nil, err
	}
	if err := db.Seed(database); err != nil {
		return nil, err
	}

	jwtIssuer := appauth.NewJWTIssuer(cfg.Auth.JWTSecret, cfg.Auth.JWTIssuer, time.Duration(cfg.Auth.JWTExpireHours)*time.Hour)
	ipRegionResolver := netutil.NewHTTPIPRegionResolver()
	adminRepo := adminrepo.NewAdminRepository(database)
	userRepo := repository.NewUserRepository(database)
	userDetailRepo := repository.NewUserDetailRepository(database)
	userGroupRepo := repository.NewUserGroupRepository(database)
	agentLevelRepo := distributionrepo.NewAgentLevelRepository(database)
	agentRepo := distributionrepo.NewAgentRepository(database)
	subordinateRepo := distributionrepo.NewSubordinateRepository(database)
	commissionRepo := distributionrepo.NewCommissionRepository(database)
	settlementRepo := distributionrepo.NewSettlementRepository(database)
	roleRepo := repository.NewRoleRepository(database)
	permissionRepo := repository.NewPermissionRepository(database)
	menuRepo := menurepo.NewMenuRepository(database)
	securityRepo := securityrepo.NewSecurityRepository(database)
	// 订单域
	orderRepo := orderrepo.NewOrderRepository(database)
	orderItemRepo := orderrepo.NewOrderItemRepository(database)
	orderRefundRepo := orderrepo.NewRefundRepository(database)
	orderService := orderservice.NewOrderService(orderRepo, orderItemRepo, orderRefundRepo, orderservice.NewDefaultProvisionAdapter())
	orderHandler := orderhandler.NewOrderHandler(orderService)
	refundHandler := orderhandler.NewRefundHandler(orderService)
	// 财务域
	walletRepo := financerepo.NewWalletRepository(database)
	walletTxRepo := financerepo.NewTransactionRepository(database)
	walletService := financeservice.NewWalletService(database, walletRepo, walletTxRepo)
	rechargeRepo := financerepo.NewRechargeRepository(database)
	rechargeService := financeservice.NewRechargeService(rechargeRepo, walletService)
	withdrawRepo := financerepo.NewWithdrawRepository(database)
	withdrawService := financeservice.NewWithdrawService(withdrawRepo, walletService)
	billRepo := financerepo.NewBillRepository(database)
	billService := financeservice.NewBillService(billRepo, walletTxRepo)
	reconService := financeservice.NewReconService(walletRepo, walletTxRepo)
	walletHandler := financehandler.NewWalletHandler(walletService)
	rechargeHandler := financehandler.NewRechargeHandler(rechargeService)
	withdrawHandler := financehandler.NewWithdrawHandler(withdrawService)
	billHandler := financehandler.NewBillHandler(billService)
	reconHandler := financehandler.NewReconHandler(reconService)
	// 系统配置
	configRepo := systemrepo.NewConfigRepository(database)
	configService := systemservice.NewConfigService(configRepo)
	configHandler := systemhandler.NewConfigHandler(configService)
	resourceQuotaRepo := quotarepo.NewResourceQuotaRepository(database)
	quotaTemplateRepo := quotarepo.NewQuotaTemplateRepository(database)
	quotaUserLevelRepo := quotarepo.NewUserLevelRepository(database)
	quotaAdjustmentRepo := quotarepo.NewQuotaAdjustmentRepository(database)
	verificationRepo := verificationrepo.NewVerificationRepository(database)
	upstreamMgr := upstream.GetProviderManager()
	providerRepo := providerrepo.NewProviderRepository(database)
	poolRepo := providerrepo.NewPoolRepository(database)
	productRepo := productrepo.NewProductRepository(database)
	syncRepo := syncrepo.NewSyncRepository(database)
	adminService := adminservice.NewAdminService(adminRepo, jwtIssuer)
	userService := service.NewUserService(userRepo)
	userDetailService := service.NewUserDetailService(userRepo, userDetailRepo)
	userGroupService := service.NewUserGroupService(userGroupRepo)
	agentLevelService := distributionservice.NewAgentLevelService(agentLevelRepo)
	agentService := distributionservice.NewAgentService(agentRepo, userRepo, agentLevelRepo)
	subordinateService := distributionservice.NewSubordinateService(subordinateRepo, agentRepo, userRepo)
	commissionService := distributionservice.NewCommissionService(commissionRepo, agentRepo, subordinateRepo, userRepo)
	settlementService := distributionservice.NewSettlementService(settlementRepo, agentRepo, userRepo, commissionRepo)
	roleService := service.NewRoleService(roleRepo)
	permissionService := service.NewPermissionService(permissionRepo)
	menuService := menuservice.NewMenuService(menuRepo)
	securityService := securityservice.NewSecurityService(securityRepo)
	resourceQuotaService := quotaservice.NewResourceQuotaService(resourceQuotaRepo, quotaAdjustmentRepo)
	quotaTemplateService := quotaservice.NewQuotaTemplateService(quotaTemplateRepo)
	quotaUserLevelService := quotaservice.NewUserLevelService(quotaUserLevelRepo)
	quotaAdjustmentService := quotaservice.NewQuotaAdjustmentService(quotaAdjustmentRepo)
	verificationService := verificationservice.NewVerificationService(verificationRepo)
	// 用户中心模块：独立的数据访问、认证服务与处理器（与后台管理模块解耦）
	userCenterRepo := usercenterrepo.NewUserRepository(database)
	userCenterService := usercenterservice.NewAuthService(userCenterRepo, jwtIssuer, ipRegionResolver)
	userCenterAuthHandler := usercenterhandler.NewAuthHandler(userCenterService)
	// 用户中心菜单：复用 menus 表（platform=user），独立 DTO 对齐前端驼峰字段
	userMenuRepo := usermenurepo.NewMenuRepository(database)
	userMenuService := usermenuservice.NewMenuService(userMenuRepo)
	userMenuHandler := usermenuhandler.NewMenuHandler(userMenuService)
	// 用户中心财务：复用账务核心/充值/账单服务，仅暴露用户自助接口
	userFinanceHandler := userfinancehandler.NewFinanceHandler(walletService, rechargeService, billService)
	providerService := providerservice.NewProviderService(providerRepo, poolRepo, upstreamMgr, cfg.App.EncryptKey)
	productService := productservice.NewProductService(productRepo)
	syncEngine := syncservice.NewSyncEngine(upstreamMgr, providerService, productRepo, poolRepo, providerRepo, syncRepo, logger)
	syncService := syncservice.NewSyncService(syncRepo, syncEngine)
	scheduler := syncservice.NewScheduler(syncEngine, syncRepo, logger)
	adminHandler := adminhandler.NewAdminHandler(adminService)
	userHandler := handler.NewUserHandler(userService)
	userDetailHandler := handler.NewUserDetailHandler(userDetailService)
	userGroupHandler := handler.NewUserGroupHandler(userGroupService)
	agentLevelHandler := distributionhandler.NewAgentLevelHandler(agentLevelService)
	agentHandler := distributionhandler.NewAgentHandler(agentService)
	subordinateHandler := distributionhandler.NewSubordinateHandler(subordinateService)
	commissionHandler := distributionhandler.NewCommissionHandler(commissionService)
	settlementHandler := distributionhandler.NewSettlementHandler(settlementService)
	roleHandler := handler.NewRoleHandler(roleService)
	permissionHandler := handler.NewPermissionHandler(permissionService)
	menuHandler := menuhandler.NewMenuHandler(menuService)
	securityHandler := securityhandler.NewSecurityHandler(securityService)
	resourceQuotaHandler := quotahandler.NewResourceQuotaHandler(resourceQuotaService)
	quotaTemplateHandler := quotahandler.NewQuotaTemplateHandler(quotaTemplateService)
	quotaUserLevelHandler := quotahandler.NewUserLevelHandler(quotaUserLevelService)
	quotaAdjustmentHandler := quotahandler.NewQuotaAdjustmentHandler(quotaAdjustmentService)
	verificationHandler := verificationhandler.NewVerificationHandler(verificationService)
	providerHandler := providerhandler.NewProviderHandler(providerService)
	productHandler := producthandler.NewProductHandler(productService)
	syncHandler := synchandler.NewSyncHandler(syncService)
	// 产品管理（面向终端售卖）
	prodCategoryRepo := prodrepo.NewCategoryRepository(database)
	prodProductRepo := prodrepo.NewProductRepository(database)
	prodCategoryService := prodservice.NewCategoryService(prodCategoryRepo)
	prodProductService := prodservice.NewProductService(prodProductRepo)
	prodCategoryHandler := prodhandler.NewCategoryHandler(prodCategoryService)
	prodProductHandler := prodhandler.NewProductHandler(prodProductService)
	// 工单支持域
	ticketRepo := ticketrepo.NewTicketRepository(database)
	ticketReplyRepo := ticketrepo.NewReplyRepository(database)
	ticketCategoryRepo := ticketrepo.NewCategoryRepository(database)
	ticketService := ticketservice.NewTicketService(ticketRepo, ticketReplyRepo, ticketCategoryRepo)
	ticketCategoryService := ticketservice.NewCategoryService(ticketCategoryRepo, ticketRepo)
	ticketHandler := tickethandler.NewTicketHandler(ticketService)
	ticketCategoryHandler := tickethandler.NewCategoryHandler(ticketCategoryService)
	userTicketHandler := tickethandler.NewUserTicketHandler(ticketService, ticketCategoryService)
	router := newRouter(cfg, adminHandler, userHandler, userDetailHandler, userGroupHandler, agentLevelHandler, agentHandler, subordinateHandler, commissionHandler, settlementHandler, roleHandler, permissionHandler, menuHandler, securityHandler, resourceQuotaHandler, quotaTemplateHandler, quotaUserLevelHandler, quotaAdjustmentHandler, verificationHandler, providerHandler, productHandler, syncHandler, userCenterAuthHandler, userMenuHandler, prodCategoryHandler, prodProductHandler, orderHandler, refundHandler, walletHandler, rechargeHandler, withdrawHandler, billHandler, reconHandler, configHandler, userFinanceHandler, ticketHandler, ticketCategoryHandler, userTicketHandler, logger, jwtIssuer)

	addr := fmt.Sprintf("%s:%d", cfg.App.Host, cfg.App.Port)

	return &Server{
		cfg:    cfg,
		logger: logger,
		http: &http.Server{
			Addr:         addr,
			Handler:      router,
			ReadTimeout:  time.Duration(cfg.App.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(cfg.App.WriteTimeout) * time.Second,
		},
		scheduler: scheduler,
	}, nil
}

func (s *Server) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	defer s.cancel()
	s.scheduler.Start(ctx)
	s.logger.Info("server starting", zap.String("addr", s.http.Addr), zap.String("name", s.cfg.App.Name))
	return s.http.ListenAndServe()
}
