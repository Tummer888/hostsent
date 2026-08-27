package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	adminhandler "hostsent/backend/internal/modules/admin/manager/handler"
	adminrepo "hostsent/backend/internal/modules/admin/manager/repository"
	adminservice "hostsent/backend/internal/modules/admin/manager/service"
	menuhandler "hostsent/backend/internal/modules/admin/menu/handler"
	menurepo "hostsent/backend/internal/modules/admin/menu/repository"
	menuservice "hostsent/backend/internal/modules/admin/menu/service"
	providerhandler "hostsent/backend/internal/modules/admin/upstream/provider/handler"
	providerrepo "hostsent/backend/internal/modules/admin/upstream/provider/repository"
	providerservice "hostsent/backend/internal/modules/admin/upstream/provider/service"
	producthandler "hostsent/backend/internal/modules/admin/upstream/product/handler"
	productrepo "hostsent/backend/internal/modules/admin/upstream/product/repository"
	productservice "hostsent/backend/internal/modules/admin/upstream/product/service"
	synchandler "hostsent/backend/internal/modules/admin/upstream/sync/handler"
	syncrepo "hostsent/backend/internal/modules/admin/upstream/sync/repository"
	syncservice "hostsent/backend/internal/modules/admin/upstream/sync/service"
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
	authService := service.NewAuthService(userRepo, jwtIssuer, ipRegionResolver)
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
	providerService := providerservice.NewProviderService(providerRepo, poolRepo, upstreamMgr, cfg.App.EncryptKey)
	productService := productservice.NewProductService(productRepo)
	syncEngine := syncservice.NewSyncEngine(upstreamMgr, providerService, productRepo, poolRepo, providerRepo, syncRepo, logger)
	syncService := syncservice.NewSyncService(syncRepo, syncEngine)
	scheduler := syncservice.NewScheduler(syncEngine, syncRepo, logger)
	adminHandler := adminhandler.NewAdminHandler(adminService)
	authHandler := handler.NewAuthHandler(authService)
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
	router := newRouter(cfg, adminHandler, authHandler, userHandler, userDetailHandler, userGroupHandler, agentLevelHandler, agentHandler, subordinateHandler, commissionHandler, settlementHandler, roleHandler, permissionHandler, menuHandler, securityHandler, resourceQuotaHandler, quotaTemplateHandler, quotaUserLevelHandler, quotaAdjustmentHandler, verificationHandler, providerHandler, productHandler, syncHandler, logger, jwtIssuer)

	addr := fmt.Sprintf("%s:%d", cfg.App.Host, cfg.App.Port)

	return &Server{
		cfg:       cfg,
		logger:    logger,
		http:      &http.Server{
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
