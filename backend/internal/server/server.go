package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	accountdto "hostsent/backend/internal/modules/admin/finance/account/dto"
	finaccounthandler "hostsent/backend/internal/modules/admin/finance/account/handler"
	finaccountrepo "hostsent/backend/internal/modules/admin/finance/account/repository"
	finaccountservice "hostsent/backend/internal/modules/admin/finance/account/service"
	finbillhandler "hostsent/backend/internal/modules/admin/finance/bill/handler"
	finbillrepo "hostsent/backend/internal/modules/admin/finance/bill/repository"
	finbillservice "hostsent/backend/internal/modules/admin/finance/bill/service"
	finrechargehandler "hostsent/backend/internal/modules/admin/finance/recharge/handler"
	finrechargerepo "hostsent/backend/internal/modules/admin/finance/recharge/repository"
	finrechargeservice "hostsent/backend/internal/modules/admin/finance/recharge/service"
	fintransactionrepo "hostsent/backend/internal/modules/admin/finance/transaction/repository"
	finwithdrawhandler "hostsent/backend/internal/modules/admin/finance/withdraw/handler"
	finwithdrawrepo "hostsent/backend/internal/modules/admin/finance/withdraw/repository"
	finwithdrawservice "hostsent/backend/internal/modules/admin/finance/withdraw/service"
	lifecyclehandler "hostsent/backend/internal/modules/admin/lifecycle/handler"
	lifecyclerepo "hostsent/backend/internal/modules/admin/lifecycle/repository"
	lifecycleservice "hostsent/backend/internal/modules/admin/lifecycle/service"
	adminhandler "hostsent/backend/internal/modules/admin/manager/handler"
	adminrepo "hostsent/backend/internal/modules/admin/manager/repository"
	adminservice "hostsent/backend/internal/modules/admin/manager/service"
	menuhandler "hostsent/backend/internal/modules/admin/menu/handler"
	menurepo "hostsent/backend/internal/modules/admin/menu/repository"
	menuservice "hostsent/backend/internal/modules/admin/menu/service"
	notifydto "hostsent/backend/internal/modules/admin/notification/dto"
	notifyhandler "hostsent/backend/internal/modules/admin/notification/handler"
	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
	notifyrepo "hostsent/backend/internal/modules/admin/notification/repository"
	notifyservice "hostsent/backend/internal/modules/admin/notification/service"
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
	synchandler "hostsent/backend/internal/modules/admin/resource/sync/handler"
	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
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
	usercenterhandler "hostsent/backend/internal/modules/uc/auth/handler"
	usercenterrepo "hostsent/backend/internal/modules/uc/auth/repository"
	usercenterservice "hostsent/backend/internal/modules/uc/auth/service"
	userfinancehandler "hostsent/backend/internal/modules/uc/finance/handler"
	usermenuhandler "hostsent/backend/internal/modules/uc/menu/handler"
	usermenurepo "hostsent/backend/internal/modules/uc/menu/repository"
	usermenuservice "hostsent/backend/internal/modules/uc/menu/service"
	ucorderhandler "hostsent/backend/internal/modules/uc/order/handler"
	ucorderservice "hostsent/backend/internal/modules/uc/order/service"
	ucproducthandler "hostsent/backend/internal/modules/uc/product/handler"
	ucproductservice "hostsent/backend/internal/modules/uc/product/service"
	appauth "hostsent/backend/internal/pkg/auth"
	"hostsent/backend/internal/pkg/config"
	"hostsent/backend/internal/pkg/db"
	"hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/netutil"
	"hostsent/backend/internal/pkg/upstream"
	// 各上游适配器通过 init() 注册工厂，须在此空导入以触发注册。
	_ "hostsent/backend/internal/pkg/upstream/mofangfinance"
	_ "hostsent/backend/internal/pkg/upstream/mofangyun"
)

type Server struct {
	cfg                *config.Config
	logger             *zap.Logger
	http               *http.Server
	scheduler          *syncservice.Scheduler
	lifecycleScheduler *lifecycleservice.LifecycleScheduler
	cancel             context.CancelFunc
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
	// 订单域（仓储先行；履约适配器需依赖上游/商品服务，见下方装配）
	orderRepo := orderrepo.NewOrderRepository(database)
	orderItemRepo := orderrepo.NewOrderItemRepository(database)
	orderRefundRepo := orderrepo.NewRefundRepository(database)
	// 财务域
	walletTxRepo := fintransactionrepo.NewTransactionRepository(database)
	walletRepo := finaccountrepo.NewWalletRepository(database)
	walletService := finaccountservice.NewWalletService(database, walletRepo, walletTxRepo)
	rechargeRepo := finrechargerepo.NewRechargeRepository(database)
	rechargeService := finrechargeservice.NewRechargeService(rechargeRepo, walletService)
	withdrawRepo := finwithdrawrepo.NewWithdrawRepository(database)
	withdrawService := finwithdrawservice.NewWithdrawService(withdrawRepo, walletService)
	billRepo := finbillrepo.NewBillRepository(database)
	billService := finbillservice.NewBillService(billRepo, walletTxRepo)
	reconService := finbillservice.NewReconService(walletRepo, walletTxRepo)
	walletHandler := finaccounthandler.NewWalletHandler(walletService)
	rechargeHandler := finrechargehandler.NewRechargeHandler(rechargeService)
	withdrawHandler := finwithdrawhandler.NewWithdrawHandler(withdrawService)
	billHandler := finbillhandler.NewBillHandler(billService)
	reconHandler := finbillhandler.NewReconHandler(reconService)
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
	userService := service.NewUserService(
		userRepo,
		jwtIssuer,
		func(ctx context.Context, userID uint64, amount float64, remark string, operatorID uint64) error {
			_, err := walletService.Adjust(ctx, accountdto.AdjustRequest{
				UserID:    userID,
				Type:      "adjust",
				Direction: 1, // 收入
				Amount:    amount,
				BizKey:    fmt.Sprintf("admin-recharge-%d", userID) + fmt.Sprintf("-%d", time.Now().UnixNano()),
				Remark:    remark,
			}, operatorID)
			return err
		},
	)
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
	prodCategoryRepo := categoryrepo.NewCategoryRepository(database)
	prodCategoryService := categoryservice.NewCategoryService(prodCategoryRepo)
	prodCategoryHandler := categoryhandler.NewCategoryHandler(prodCategoryService)
	// 商品管理（catalog 子域）
	prodCatalogRepo := catalogrepo.NewProductRepository(database)
	prodCatalogService := catalogservice.NewProductService(prodCatalogRepo, productRepo)
	prodCatalogHandler := cataloghandler.NewProductHandler(prodCatalogService)
	// 订单履约：上游开通适配器（打通订单 paid/provisioning → 上游创建实例）
	orderService := orderservice.NewOrderService(orderRepo, orderItemRepo, orderRefundRepo, orderservice.NewUpstreamProvisionAdapter(orderservice.ProvisionDeps{
		BuildProvisionRequest: func(ctx context.Context, productID uint64, name string) (interface{}, error) {
			return prodCatalogService.BuildProvisionRequest(ctx, productID, name)
		},
		BuildProviderConfig: providerService.BuildProviderConfig,
		CreateInstance: func(ctx context.Context, cfg *upstream.ProviderConfig, req *model.CreateInstanceRequest) (*model.StandardInstance, error) {
			provider, err := upstreamMgr.Build(cfg.Type, cfg)
			if err != nil {
				return nil, err
			}
			return provider.CreateInstance(ctx, req)
		},
		RecordInstance: func(ctx context.Context, inst *model.StandardInstance, order *ordermodel.Order) error {
			row := buildRecordedInstance(inst, order)
			if row.InstanceID == "" {
				return nil
			}
			return syncRepo.UpsertInstances(ctx, []syncmodel.Instance{row})
		},
		HasInstance: func(ctx context.Context, userID, productID uint64) (bool, error) {
			n, err := syncRepo.CountInstancesByProductUser(ctx, userID, productID)
			return n > 0, err
		},
	}))
	orderHandler := orderhandler.NewOrderHandler(orderService)
	refundHandler := orderhandler.NewRefundHandler(orderService)
	// 用户中心商品：复用管理端商品目录，仅暴露上架商品（用户可购）
	ucProductService := ucproductservice.NewProductService(prodCatalogService)
	ucProductHandler := ucproducthandler.NewProductHandler(ucProductService)
	// 用户中心订单：余额支付下单 + 复用履约适配器开通上游
	// ensureOpenable：下单前校验商品能否直连开通，财务型上游（账单推送制）不支持单次开通，先拒绝避免误扣款。
	ucOrderService := ucorderservice.NewOrderService(prodCatalogService, walletService, orderRepo, orderService,
		func(ctx context.Context, productID uint64) error {
			preq, err := prodCatalogService.BuildProvisionRequest(ctx, productID, "")
			if err != nil {
				return err
			}
			if preq == nil || preq.ProviderID == 0 {
				return errors.New("该商品未绑定上游，暂不可在线购买")
			}
			cfg, err := providerService.BuildProviderConfig(ctx, preq.ProviderID)
			if err != nil {
				return errors.New("该商品上游配置不可用，暂不可购买")
			}
			if cfg.Type == "mofangfinance" {
				return errors.New("该商品由魔方财务上架，需走订单推送流程，暂不支持在线直接开通")
			}
			return nil
		},
	)
	ucOrderHandler := ucorderhandler.NewOrderHandler(ucOrderService)
	// 规格管理（spec 子域）
	specTemplateRepo := specrepo.NewSpecTemplateRepository(database)
	specMappingRepo := specrepo.NewSpecMappingRepository(database)
	specTemplateService := specservice.NewSpecTemplateService(specTemplateRepo)
	specMappingService := specservice.NewSpecMappingService(specMappingRepo)
	specHandler := spechandler.NewSpecHandler(specTemplateService, specMappingService)
	// 定价与计费（pricing 子域）
	pricingRepo := pricingrepo.NewPricingRepository(database)
	pricingService := pricingservice.NewPricingService(pricingRepo)
	pricingHandler := pricinghandler.NewPricingHandler(pricingService)
	// 促销管理（promotion 子域）
	couponRepo := promotionrepo.NewCouponRepository(database)
	couponGrantRepo := promotionrepo.NewCouponGrantRepository(database)
	promotionRepo := promotionrepo.NewPromotionRepository(database)
	couponService := promotionservice.NewCouponService(couponRepo, couponGrantRepo)
	couponGrantService := promotionservice.NewCouponGrantService(couponGrantRepo, couponRepo)
	promotionService := promotionservice.NewPromotionService(promotionRepo)
	promotionHandler := promotionhandler.NewPromotionHandler(couponService, couponGrantService, promotionService)
	// 工单支持域
	ticketRepo := ticketrepo.NewTicketRepository(database)
	ticketReplyRepo := ticketrepo.NewReplyRepository(database)
	ticketCategoryRepo := ticketrepo.NewCategoryRepository(database)
	ticketService := ticketservice.NewTicketService(ticketRepo, ticketReplyRepo, ticketCategoryRepo)
	ticketCategoryService := ticketservice.NewCategoryService(ticketCategoryRepo, ticketRepo)
	ticketHandler := tickethandler.NewTicketHandler(ticketService)
	ticketCategoryHandler := tickethandler.NewCategoryHandler(ticketCategoryService)
	userTicketHandler := tickethandler.NewUserTicketHandler(ticketService, ticketCategoryService)
	// 生命周期与续费域（doc60）
	lifecycleRenewalRepo := lifecyclerepo.NewRenewalRepository(database)
	lifecyclePolicyRepo := lifecyclerepo.NewPolicyRepository(database)
	lifecycleAutoRepo := lifecyclerepo.NewAutoRenewRepository(database)
	lifecycleInstanceReader := lifecyclerepo.NewInstanceReader(database)
	lifecycleOrderWriter := lifecyclerepo.NewOrderWriter(database)
	lifecycleRenewalSvc := lifecycleservice.NewRenewalService(database, lifecycleRenewalRepo, lifecyclePolicyRepo, lifecycleAutoRepo, lifecycleInstanceReader, lifecycleOrderWriter, walletService, logger)
	lifecycleSvc := lifecycleservice.NewLifecycleService(database, lifecyclePolicyRepo, lifecycleInstanceReader, lifecycleRenewalSvc, logger)
	lifecycleExpiringHandler := lifecyclehandler.NewExpiringHandler(lifecycleSvc)
	lifecycleAdminHandler := lifecyclehandler.NewLifecycleAdminHandler(lifecycleSvc, lifecycleRenewalSvc)
	lifecycleUserHandler := lifecyclehandler.NewLifecycleUserHandler(lifecycleSvc, lifecycleRenewalSvc)
	lifecycleScheduler := lifecycleservice.NewLifecycleScheduler(lifecycleSvc, logger)
	// 通知与消息中心（doc70）
	notifyRepo := notifyrepo.NewNotificationRepository(database)
	notifyTplRepo := notifyrepo.NewTemplateRepository(database)
	notifyPrefRepo := notifyrepo.NewPreferenceRepository(database)
	notifyAnnRepo := notifyrepo.NewAnnouncementRepository(database)
	notifyMailChannel := notifyservice.NewMailChannel(database, configRepo, notifyRepo, logger)
	notifySvc := notifyservice.NewNotificationService(database, notifyRepo, notifyTplRepo, notifyPrefRepo, notifyAnnRepo, notifyMailChannel, logger)
	announceSvc := notifyservice.NewAnnouncementService(notifyAnnRepo)
	templateSvc := notifyservice.NewTemplateService(notifyTplRepo)
	preferenceSvc := notifyservice.NewPreferenceService(notifyPrefRepo, notifyTplRepo)
	notifyAdminHandler := notifyhandler.NewAdminHandler(notifySvc, announceSvc, templateSvc, preferenceSvc)
	notifyUserHandler := notifyhandler.NewUserHandler(notifySvc, announceSvc, preferenceSvc)
	// 生命周期 Notifier 桥接：替换 noopNotifier 为通知中心
	lifecycleSvc.SetNotifier(&lifecycleNotifierBridge{notifySvc: notifySvc, logger: logger})
	// 订单支付成功钩子（doc60）：续费订单支付完成后联动完成续费并延长到期时间
	// doc70：支付成功后发布通知
	orderservice.OnPaid = func(ctx context.Context, order *ordermodel.Order) {
		if order == nil {
			return
		}
		if order.RenewalID != 0 {
			if err := lifecycleRenewalSvc.CompleteRenewalByOrderID(ctx, order.ID); err != nil {
				logger.Error("lifecycle: complete renewal by order failed", zap.Uint64("order_id", order.ID), zap.Error(err))
			}
		}
		// 发布支付成功通知（doc70）
		_ = notifySvc.Publish(ctx, notifydto.PublishInput{
			Event:        notifymodel.EventOrderPaid,
			UserID:       order.UserID,
			Target:       notifymodel.TargetUser,
			Vars:         map[string]string{"order_no": order.OrderNo, "amount": fmt.Sprintf("%.2f", order.PaidAmount)},
			SourceModule: "order",
			SourceID:     order.OrderNo,
		})
	}
	router := newRouter(cfg, adminHandler, userHandler, userDetailHandler, userGroupHandler, agentLevelHandler, agentHandler, subordinateHandler, commissionHandler, settlementHandler, roleHandler, permissionHandler, menuHandler, securityHandler, resourceQuotaHandler, quotaTemplateHandler, quotaUserLevelHandler, quotaAdjustmentHandler, verificationHandler, providerHandler, productHandler, syncHandler, userCenterAuthHandler, userMenuHandler, prodCategoryHandler, prodCatalogHandler, specHandler, pricingHandler, promotionHandler, orderHandler, refundHandler, walletHandler, rechargeHandler, withdrawHandler, billHandler, reconHandler, configHandler, userFinanceHandler, ucProductHandler, ucOrderHandler, ticketHandler, ticketCategoryHandler, userTicketHandler, lifecycleExpiringHandler, lifecycleAdminHandler, lifecycleUserHandler, notifyAdminHandler, notifyUserHandler, logger, jwtIssuer)

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
		scheduler:          scheduler,
		lifecycleScheduler: lifecycleScheduler,
	}, nil
}

func (s *Server) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	defer cancel()
	s.scheduler.Start(ctx)
	s.lifecycleScheduler.Start(ctx)
	s.logger.Info("server starting", zap.String("addr", s.http.Addr), zap.String("name", s.cfg.App.Name))
	return s.http.ListenAndServe()
}

// lifecycleNotifierBridge 桥接生命周期模块的 Notifier 接口到通知中心。
// 生命周期提醒（到期/续费结果）通过 Publish 发布到通知中心。
type lifecycleNotifierBridge struct {
	notifySvc notifyservice.NotificationService
	logger    *zap.Logger
}

func (b *lifecycleNotifierBridge) Publish(ctx context.Context, userID uint64, title, content string) error {
	return b.notifySvc.Publish(ctx, notifydto.PublishInput{
		Event:        notifymodel.EventInstanceExpiring,
		UserID:       userID,
		Target:       notifymodel.TargetUser,
		Vars:         map[string]string{"title": title, "content": content},
		SourceModule: "lifecycle",
		SourceID:     fmt.Sprintf("remind-%d", userID),
	})
}

// buildRecordedInstance 将标准实例转换为同步实例记录（订单履约开通成功后落库）。
func buildRecordedInstance(inst *model.StandardInstance, order *ordermodel.Order) syncmodel.Instance {
	rawJSON, _ := json.Marshal(inst.RawData)
	row := syncmodel.Instance{
		InstanceID:  inst.UpstreamID,
		UserID:      order.UserID,
		ProductID:   order.ProductID,
		Name:        firstNonEmpty(inst.Name, order.ProductName),
		CPU:         inst.Specs.CPU,
		Memory:      inst.Specs.Memory,
		Disk:        inst.Specs.Disk,
		DiskType:    inst.Specs.DiskType,
		Bandwidth:   inst.Specs.Bandwidth,
		OS:          inst.Specs.OS,
		Region:      inst.Region,
		Zone:        inst.Zone,
		Status:      string(inst.Status),
		PublicIP:    inst.PublicIP,
		PrivateIP:   inst.PrivateIP,
		RawData:     string(rawJSON),
		BillingMode: order.PriceModel,
	}
	if !inst.ExpireAt.IsZero() {
		exp := inst.ExpireAt
		row.ExpireAt = &exp
	}
	return row
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
