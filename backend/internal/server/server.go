package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
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
	transmodel "hostsent/backend/internal/modules/admin/finance/transaction/model"
	fintransactionrepo "hostsent/backend/internal/modules/admin/finance/transaction/repository"
	finwithdrawhandler "hostsent/backend/internal/modules/admin/finance/withdraw/handler"
	finwithdrawrepo "hostsent/backend/internal/modules/admin/finance/withdraw/repository"
	finwithdrawservice "hostsent/backend/internal/modules/admin/finance/withdraw/service"
	instancehandler "hostsent/backend/internal/modules/admin/instance/handler"
	instancerepo "hostsent/backend/internal/modules/admin/instance/repository"
	instanceservice "hostsent/backend/internal/modules/admin/instance/service"
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
	discounthandler "hostsent/backend/internal/modules/admin/product/discount/handler"
	discountrepo "hostsent/backend/internal/modules/admin/product/discount/repository"
	discountservice "hostsent/backend/internal/modules/admin/product/discount/service"
	pricinghandler "hostsent/backend/internal/modules/admin/product/pricing/handler"
	pricingrepo "hostsent/backend/internal/modules/admin/product/pricing/repository"
	pricingservice "hostsent/backend/internal/modules/admin/product/pricing/service"
	promotionhandler "hostsent/backend/internal/modules/admin/product/promotion/handler"
	promotionrepo "hostsent/backend/internal/modules/admin/product/promotion/repository"
	promotionservice "hostsent/backend/internal/modules/admin/product/promotion/service"
	spechandler "hostsent/backend/internal/modules/admin/product/spec/handler"
	specrepo "hostsent/backend/internal/modules/admin/product/spec/repository"
	specservice "hostsent/backend/internal/modules/admin/product/spec/service"
	referralhandler "hostsent/backend/internal/modules/admin/referral/handler"
	referralrepo "hostsent/backend/internal/modules/admin/referral/repository"
	referralservice "hostsent/backend/internal/modules/admin/referral/service"
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
	userdto "hostsent/backend/internal/modules/admin/user/account/dto"
	"hostsent/backend/internal/modules/admin/user/account/handler"
	"hostsent/backend/internal/modules/admin/user/account/repository"
	"hostsent/backend/internal/modules/admin/user/account/service"
	levelhandler "hostsent/backend/internal/modules/admin/user/level/handler"
	levelrepo "hostsent/backend/internal/modules/admin/user/level/repository"
	levelservice "hostsent/backend/internal/modules/admin/user/level/service"
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
	ucsitehandler "hostsent/backend/internal/modules/uc/site/handler"
	ucsiteservice "hostsent/backend/internal/modules/uc/site/service"
	appauth "hostsent/backend/internal/pkg/auth"
	"hostsent/backend/internal/pkg/config"
	"hostsent/backend/internal/pkg/db"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/netutil"
	"hostsent/backend/internal/pkg/observability"
	"hostsent/backend/internal/pkg/pricing"
	"hostsent/backend/internal/pkg/specatom"
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
	provisionWorker    *orderservice.ProvisionWorker
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
	rbacRepo := adminrepo.NewRBACRepository(database)
	permCache := middleware.NewMemoryPermissionCache()
	adminAuditRepo := adminrepo.NewAdminAuditRepository(database, logger)
	userRepo := repository.NewUserRepository(database)
	userDetailRepo := repository.NewUserDetailRepository(database)
	userGroupRepo := repository.NewUserGroupRepository(database)
	roleRepo := repository.NewRoleRepository(database)
	permissionRepo := repository.NewPermissionRepository(database)
	menuRepo := menurepo.NewMenuRepository(database)
	securityRepo := securityrepo.NewSecurityRepository(database)
	// 订单域（仓储先行；履约适配器需依赖上游/商品服务，见下方装配）
	orderRepo := orderrepo.NewOrderRepository(database)
	orderItemRepo := orderrepo.NewOrderItemRepository(database)
	orderRefundRepo := orderrepo.NewRefundRepository(database)
	// 开通履约任务表（T5.1）：下单只投递任务，工作池异步执行上游开通（约 50s > write_timeout）。
	provisionTaskRepo := orderrepo.NewProvisionTaskRepository(database)
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
	levelRepo := levelrepo.NewUserLevelRepository(database)
	verificationRepo := verificationrepo.NewVerificationRepository(database)
	upstreamMgr := upstream.GetProviderManager()
	providerRepo := providerrepo.NewProviderRepository(database)
	poolRepo := providerrepo.NewPoolRepository(database)
	providerTypeRepo := providerrepo.NewProviderTypeRepository(database)
	productRepo := productrepo.NewProductRepository(database)
	syncRepo := syncrepo.NewSyncRepository(database)
	syncFwRepo := syncrepo.NewFrameworkRepository(database)
	adminService := adminservice.NewAdminService(adminRepo, rbacRepo, adminAuditRepo, permCache, jwtIssuer)
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
	// 默认用户组兜底：后台建号（admin user）未指定分组时归入 is_default 组。
	userService.SetDefaultGroupProvider(userGroupService)
	roleService := service.NewRoleService(roleRepo, permCache)
	permissionService := service.NewPermissionService(permissionRepo)
	menuService := menuservice.NewMenuService(menuRepo)
	securityService := securityservice.NewSecurityService(securityRepo)
	userLevelService := levelservice.NewUserLevelService(levelRepo)
	// 消费升级服务（P3-03）：订单支付成功后累加累计消费并重算等级（只升不降）。
	levelUpgradeService := levelservice.NewLevelUpgradeService(levelRepo)
	verificationService := verificationservice.NewVerificationService(verificationRepo)
	// 用户中心模块：独立的数据访问、认证服务与处理器（与后台管理模块解耦）
	userCenterRepo := usercenterrepo.NewUserRepository(database)
	userCenterService := usercenterservice.NewAuthService(userCenterRepo, jwtIssuer, ipRegionResolver, logger)
	// 默认用户组兜底：用户自助注册未指定分组时归入 is_default 组。
	userCenterService.SetDefaultGroupResolver(userGroupService)
	userCenterAuthHandler := usercenterhandler.NewAuthHandler(userCenterService)
	// 用户中心菜单：复用 menus 表（platform=user），独立 DTO 对齐前端驼峰字段
	userMenuRepo := usermenurepo.NewMenuRepository(database)
	userMenuService := usermenuservice.NewMenuService(userMenuRepo)
	userMenuHandler := usermenuhandler.NewMenuHandler(userMenuService)
	// 用户中心财务：复用账务核心/充值/账单服务，仅暴露用户自助接口；
	// 错误码映射由本装配层提供（uc 不依赖 admin 的错误变量）。
	userFinanceHandler := userfinancehandler.NewFinanceHandler(walletService, rechargeService, billService,
		func(err error) *apperrors.AppError {
			switch {
			case errors.Is(err, finaccountservice.ErrWalletNotFound):
				return apperrors.New(20002, err.Error())
			case errors.Is(err, finaccountservice.ErrInsufficientBalance):
				return apperrors.New(30001, err.Error())
			case errors.Is(err, finaccountservice.ErrStatusConflict):
				return apperrors.New(20003, err.Error())
			default:
				return apperrors.New(50001, err.Error())
			}
		},
	)
	providerService := providerservice.NewProviderService(providerRepo, poolRepo, providerTypeRepo, upstreamMgr, cfg.App.EncryptKey)
	// 渠道类型注册表（T2.2）：把已注册适配器的能力描述符回写 provider_types，
	// 使后台类型列表/动态表单以数据库为权威来源（地雷 L9 从硬编码 map 改为查表）。
	if err := providerService.SyncTypeRegistry(context.Background()); err != nil {
		logger.Warn("同步渠道类型注册表失败", zap.Error(err))
	}
	// 凭证归一（T2.3）：把迁移搬入 credentials 的旧列密文改为带 enc: 前缀的统一密文，
	// 使"解密失败必须报错"（L8）能可靠区分历史明文与密文。
	if err := providerService.NormalizeCredentials(context.Background()); err != nil {
		logger.Warn("归一渠道凭证失败", zap.Error(err))
	}
	productService := productservice.NewProductService(productRepo)
	syncEngine := syncservice.NewSyncEngine(upstreamMgr, providerService, productRepo, poolRepo, providerRepo, syncRepo, syncFwRepo, logger)
	syncService := syncservice.NewSyncService(syncRepo, syncEngine)
	scheduler := syncservice.NewScheduler(syncEngine, syncRepo, syncFwRepo, logger)
	syncFrameworkService := syncservice.NewFrameworkService(syncFwRepo, syncEngine, logger)
	adminHandler := adminhandler.NewAdminHandler(adminService)

	userDetailHandler := handler.NewUserDetailHandler(userDetailService)
	userGroupHandler := handler.NewUserGroupHandler(userGroupService)
	roleHandler := handler.NewRoleHandler(roleService)
	permissionHandler := handler.NewPermissionHandler(permissionService)
	menuHandler := menuhandler.NewMenuHandler(menuService)
	securityHandler := securityhandler.NewSecurityHandler(securityService)
	userLevelHandler := levelhandler.NewUserLevelHandler(userLevelService)
	verificationHandler := verificationhandler.NewVerificationHandler(verificationService)
	providerHandler := providerhandler.NewProviderHandler(providerService)
	productHandler := producthandler.NewProductHandler(productService)
	syncHandler := synchandler.NewSyncHandler(syncService)
	syncFrameworkHandler := synchandler.NewFrameworkHandler(syncFrameworkService)
	// 产品管理（面向终端售卖）
	prodCategoryRepo := categoryrepo.NewCategoryRepository(database)
	prodCategoryService := categoryservice.NewCategoryService(prodCategoryRepo)
	prodCategoryHandler := categoryhandler.NewCategoryHandler(prodCategoryService)
	// 商品管理（catalog 子域）
	prodCatalogRepo := catalogrepo.NewProductRepository(database)
	// 规格契约仓储/服务提前构造：catalog 服务需读取 SKU 平台绑定（T4.2/T4.6）
	// 并在克隆时登记上游规格快照（T4.3），故先于 prodCatalogService 装配。
	specContractRepo := specrepo.NewSpecContractRepository(database)
	specContractService := specservice.NewSpecContractService(specContractRepo)
	prodCatalogService := catalogservice.NewProductService(
		prodCatalogRepo, productRepo, providerRepo, specContractRepo,
		catalogservice.UpstreamSpecRegistrar(func(ctx context.Context, snap catalogservice.UpstreamSpecSnapshot) error {
			return specContractService.RegisterUpstreamSpec(ctx, specservice.UpstreamSpecSnapshot{
				ProviderID:   snap.ProviderID,
				ProviderType: snap.ProviderType,
				ExternalID:   snap.ExternalID,
				ExternalName: snap.ExternalName,
				ExternalKind: snap.ExternalKind,
				Raw:          snap.Raw,
				Normalized:   snap.Normalized,
			})
		}),
	)
	prodCatalogHandler := cataloghandler.NewProductHandler(prodCatalogService)
	// T3.4：把"已确认调价"落地到售出商品的实现注入同步引擎（写 product_history）。
	syncEngine.SetPriceApplier(prodCatalogService)
	// 推广邀请返现（独立于现金钱包）：订单完成计提、退款按比例冲减。
	referralRepo := referralrepo.NewReferralRepository(database)
	referralSvc := referralservice.NewReferralService(database, referralRepo, logger)
	// 注册链路：为新用户生成邀请码并按需绑定邀请人（失败不阻断注册）。
	userCenterService.SetInviteBinder(referralSvc)
	// 返现提现/转出：转出桥接到现金钱包，biz_type=referral_transfer，ref_no=转账号（钱包侧幂等）。
	referralWithdrawSvc := referralservice.NewWithdrawalService(database, referralRepo,
		func(ctx context.Context, userID uint64, amount float64, bizType, refNo, remark string) error {
			_, err := walletService.Change(ctx, finaccountservice.ChangeRequest{
				UserID:    userID,
				Type:      transmodel.TxTypeReferralTransfer,
				Direction: transmodel.DirectionIncome,
				Amount:    amount,
				BizType:   bizType,
				RefNo:     refNo,
				Remark:    remark,
			})
			return err
		}, logger)
	adminReferralHandler := referralhandler.NewReferralHandler(referralSvc, referralWithdrawSvc)
	ucReferralHandler := ucreferralhandler.NewReferralHandler(referralSvc, referralWithdrawSvc,
		// 错误码映射由装配层提供（uc 不依赖 admin 错误变量，与用户中心财务一致）。
		func(err error) *apperrors.AppError {
			switch {
			case errors.Is(err, referralservice.ErrWithdrawNotFound):
				return apperrors.New(20002, err.Error())
			case errors.Is(err, referralservice.ErrWithdrawStatus), errors.Is(err, referralservice.ErrReferralDisabled):
				return apperrors.New(20003, err.Error())
			case errors.Is(err, referralservice.ErrInvalidAmount), errors.Is(err, referralservice.ErrBelowMinWithdraw):
				return apperrors.New(20001, err.Error())
			case errors.Is(err, referralservice.ErrInsufficientBalance):
				return apperrors.New(30001, err.Error())
			default:
				return apperrors.New(50001, err.Error())
			}
		})
	// 订单履约：上游开通适配器（打通订单 paid/provisioning → 上游创建实例）
	orderService := orderservice.NewOrderService(orderRepo, orderItemRepo, orderRefundRepo, orderservice.NewUpstreamProvisionAdapter(buildOrderProvisionDeps(prodCatalogService, providerService, upstreamMgr, syncRepo)))
	// 异步开通（T5.1）：下单/后台重试只投递任务，工作池领取后执行上游开通（约 50s > write_timeout）。
	provisionEnqueuer := orderservice.NewProvisionEnqueuer(provisionTaskRepo, logger)
	orderService.SetProvisionQueue(provisionEnqueuer)
	orderService.SetProductSourceModeResolver(buildProductSourceModeResolver(prodCatalogService))
	// 订单开通成功 → 给邀请人计提返现（失败只记日志，不影响订单）。
	orderService.SetCashbackHook(func(ctx context.Context, orderID uint64, orderNo string, buyerUserID uint64, baseAmount float64, isRenewal bool) error {
		if err := referralSvc.AccrueForOrder(ctx, referralservice.AccrualInput{
			OrderID: orderID, OrderNo: orderNo, BuyerUserID: buyerUserID, BaseAmount: baseAmount, IsRenewal: isRenewal,
		}); err != nil {
			logger.Warn("referral cashback accrual failed",
				zap.Uint64("order_id", orderID), zap.String("order_no", orderNo),
				zap.Uint64("user_id", buyerUserID), zap.Error(err))
		}
		return nil
	})
	// 退款审核通过 → 按退款额占比冲减邀请人返现（失败只记日志，不影响退款）。
	orderService.SetRefundHook(func(ctx context.Context, orderID uint64, orderNo string, buyerUserID uint64, paidAmount, refundAmount float64, refundNo string) error {
		if err := referralSvc.ClawbackForRefund(ctx, referralservice.ClawbackInput{
			OrderID: orderID, OrderNo: orderNo, BuyerUserID: buyerUserID,
			PaidAmount: paidAmount, RefundAmount: refundAmount, RefundNo: refundNo,
		}); err != nil {
			logger.Warn("referral cashback clawback failed",
				zap.Uint64("order_id", orderID), zap.String("order_no", orderNo),
				zap.String("refund_no", refundNo), zap.Error(err))
		}
		return nil
	})
	orderHandler := orderhandler.NewOrderHandler(orderService)
	userHandler := handler.NewUserHandler(userService,
		// 为用户创建订单：余额支付并开通，或仅创建待支付
		func(ctx context.Context, userID uint64, req userdto.AdminCreateOrderRequest) (*userdto.AdminOrderBrief, error) {
			product, err := prodCatalogService.FindByID(ctx, req.ProductID)
			if err != nil {
				return nil, err
			}
			price := req.Price
			if price <= 0 {
				price = product.Price
			}
			cycle := req.BillingCycle
			if cycle == "" {
				cycle = "monthly"
			}
			payMode := req.PayMode
			if payMode == "" {
				payMode = "create"
			}
			now := time.Now()
			order := &ordermodel.Order{
				OrderNo:     fmt.Sprintf("AO%d%d%04d", userID, now.Unix(), rand.Intn(10000)),
				UserID:      userID,
				ProductID:   product.ID,
				ProductName: product.Name,
				Specs:       product.Specs,
				Quantity:    1,
				PriceModel:  cycle,
				TotalAmount: price,
				// 后台代下单不经过算价管线：原价即实付、无优惠、未命中任何规则。
				// price_snapshot 为 jsonb，空串无法写入，这里落空数组表示「无规则明细」。
				OriginalAmount: price,
				FinalAmount:    price,
				DiscountSource: "manual",
				PriceSnapshot:  "[]",
				Status:         ordermodel.OrderStatusPending,
			}
			// 余额支付：扣款 + 标记已支付 + 投递开通任务（T5.1 异步履约）。
			if payMode == "balance" {
				if _, err := walletService.Adjust(ctx, accountdto.AdjustRequest{
					UserID: userID, Type: "order", Direction: -1, Amount: price,
					BizKey: fmt.Sprintf("admin-order-%d", now.UnixNano()),
					Remark: fmt.Sprintf("后台为用户下单：%s（%s）", product.Name, cycle),
				}, userID); err != nil {
					if errors.Is(err, finaccountservice.ErrInsufficientBalance) {
						return nil, errors.New("用户余额不足")
					}
					return nil, err
				}
				order.Status = ordermodel.OrderStatusPaid
				order.PayMethod = "balance"
				order.PaidAmount = price
				order.PayTime = &now
			}
			if err := orderRepo.Create(ctx, order); err != nil {
				return nil, err
			}
			brief := &userdto.AdminOrderBrief{
				ID: order.ID, OrderNo: order.OrderNo, ProductName: product.Name,
				BillingCycle: cycle, TotalAmount: price, Status: order.Status, PayMethod: order.PayMethod,
			}
			if payMode == "balance" {
				// 异步开通：投递任务即返回 paid，前端/后台按订单状态轮询（T5.1）。
				if err := provisionEnqueuer.EnqueueProvision(ctx, order, product.SourceMode); err == nil {
					brief.Status = ordermodel.OrderStatusPaid
				}
			}
			return brief, nil
		},
	)
	refundHandler := orderhandler.NewRefundHandler(orderService)
	// 用户中心商品：复用管理端商品目录，仅暴露上架商品（用户可购）
	ucProductService := ucproductservice.NewProductService(prodCatalogService)
	ucProductHandler := ucproducthandler.NewProductHandler(ucProductService)
	// 定价与计费（pricing 子域）
	pricingRepo := pricingrepo.NewPricingRepository(database)
	pricingService := pricingservice.NewPricingService(pricingRepo)
	pricingHandler := pricinghandler.NewPricingHandler(pricingService)
	// 折扣策略（discount 子域，P5-01/P5-06）
	discountPolicyRepo := discountrepo.NewPricePolicyRepository(database)
	discountPolicyService := discountservice.NewPolicyService(discountPolicyRepo)
	discountPolicyHandler := discounthandler.NewPolicyHandler(discountPolicyService)
	// 统一算价管线（P5-03）：基础价读 product_pricing 回落 products.price；折扣来源按序注入。
	pricePipeline := pricing.NewService(pricing.Deps{
		BasePrice: func(ctx context.Context, productID uint64) (float64, uint64, error) {
			product, err := prodCatalogService.FindByID(ctx, productID)
			if err != nil {
				return 0, 0, err
			}
			unitPrice := product.Price
			// product_pricing = 这商品基础多少钱：配置了启用中的计费模板则优先采用其单价。
			if tmpl, err := pricingRepo.FindByProductID(ctx, productID); err == nil && tmpl != nil && tmpl.UnitPrice > 0 {
				unitPrice = tmpl.UnitPrice
			}
			return unitPrice, product.CategoryID, nil
		},
		// SKU 级基础价（T4.1）：请求带 spec_code 且该 SKU 有独立定价时覆盖商品级基础价；
		// 未定价 / 未选规格返回 found=false，回落商品级基础价（折扣算法不变）。
		SpecBasePrice: func(ctx context.Context, in pricing.ResolveInput) (float64, uint64, bool, error) {
			if in.SpecCode == "" {
				return 0, 0, false, nil
			}
			spec, err := prodCatalogService.FindSpec(ctx, in.ProductID, in.SpecCode)
			if err != nil || spec == nil || spec.Status != 1 || spec.Price <= 0 {
				return 0, 0, false, nil
			}
			return spec.Price, 0, true, nil
		},
		// 用户组策略 = 这客户打几折（D3 唯一折扣来源）。
		GroupRule: discountPolicyService.RuleForUserGroup,
		// 促销/优惠券暂不在管线内（缺少选券入参）。
	}, cfg.Pricing.StackMode)
	// 用户中心订单：余额支付下单 + 复用履约适配器开通上游
	// ensureOpenable：下单前校验商品能否直连开通，财务型上游（账单推送制）不支持单次开通，先拒绝避免误扣款。
	ucOrderService := ucorderservice.NewOrderService(prodCatalogService, walletService, orderRepo, orderService,
		buildEnsureOpenable(prodCatalogService, providerService),
		func(err error) bool { return errors.Is(err, finaccountservice.ErrInsufficientBalance) },
		func(userID uint64, amount float64) {
			// 异步执行：不阻塞下单；使用独立 context，避免请求结束后被取消。
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				if err := levelUpgradeService.ApplyConsume(ctx, userID, amount); err != nil {
					logger.Warn("user level: apply consume failed",
						zap.Uint64("user_id", userID), zap.Float64("amount", amount), zap.Error(err))
				}
			}()
		},
		userRepo,           // 订单「操作人」列：批量解析 operator_id → 用户名（P4-09）
		orderItemRepo,      // 订单项落库：行级折扣快照（P5-04）
		pricePipeline,      // 统一算价管线（P5-03/P5-04）
		prodCatalogService, // SKU 读取与库存增减（T4.1，按 SKU 下单）
		provisionEnqueuer,  // 异步开通任务投递（T5.1）
		provisionTaskRepo,  // 开通任务状态查询（订单列表轮询展示）
	)
	ucOrderHandler := ucorderhandler.NewOrderHandler(ucOrderService)
	// 用户中心主机管理：列表/详情/电源/VNC（复用上游适配器）
	ucInstanceRepo := ucinstancerepo.NewInstanceRepository(database)
	ucInstanceService := ucinstanceservice.NewInstanceService(ucInstanceRepo, buildProviderResolver(providerService, upstreamMgr))
	ucInstanceHandler := ucinstancehandler.NewInstanceHandler(ucInstanceService)
	// 实例运维台（管理端跨用户）：列表/详情/电源/控制台/回源/变配/销毁/流水/关联（见 docs/实施计划/61）
	instanceOpsRepo := instancerepo.NewInstanceRepository(database)
	instanceOpRepo := instancerepo.NewOperationRepository(database)
	instanceRelatedRepo := instancerepo.NewRelatedRepository(database)
	instanceOpsService := instanceservice.NewInstanceService(
		instanceOpsRepo,
		instanceOpRepo,
		instanceRelatedRepo,
		buildInstanceOpsResolver(providerService, upstreamMgr),
		logger,
	)
	instanceOpsHandler := instancehandler.NewInstanceHandler(instanceOpsService)
	// 规格管理（spec 子域）
	specTemplateRepo := specrepo.NewSpecTemplateRepository(database)
	specMappingRepo := specrepo.NewSpecMappingRepository(database)
	// specContractRepo 已在上方随 catalog 服务构造（见 prodCatalogService 处）。
	// 规格原子字典（T2.5/T2.6）：启动期从 spec_atoms 载入内存，供 Extra 校验与平台字段展示。
	if err := specrepo.LoadAtomDictionary(context.Background(), specContractRepo); err != nil {
		logger.Warn("加载规格原子字典失败", zap.Error(err))
	}
	specTemplateService := specservice.NewSpecTemplateService(specTemplateRepo)
	specMappingService := specservice.NewSpecMappingService(specMappingRepo)
	// specContractService 已在上方随 catalog 服务构造（见 prodCatalogService 处）。
	specHandler := spechandler.NewSpecHandler(specTemplateService, specMappingService, specContractService)
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
	ticketService := ticketservice.NewTicketService(ticketRepo, ticketReplyRepo, ticketCategoryRepo, nil)
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
	lifecycleRenewalSvc := lifecycleservice.NewRenewalService(database, lifecycleRenewalRepo, lifecyclePolicyRepo, lifecycleAutoRepo, lifecycleInstanceReader, lifecycleOrderWriter, walletService, pricePipeline, logger)
	// 续费接上游/平台（T5.2）：链路 A 以上游返回账期为权威，链路 B 回落本地顺延。
	lifecycleRenewalSvc.SetUpstreamRenewer(buildUpstreamRenewer(providerService, upstreamMgr))
	// 续费完成 → 给邀请人按续费比率计提返现。
	lifecycleRenewalSvc.SetCashbackHook(func(ctx context.Context, orderID uint64, orderNo string, buyerUserID uint64, amount float64, isRenewal bool) error {
		if err := referralSvc.AccrueForOrder(ctx, referralservice.AccrualInput{
			OrderID: orderID, OrderNo: orderNo, BuyerUserID: buyerUserID, BaseAmount: amount, IsRenewal: isRenewal,
		}); err != nil {
			logger.Warn("referral renewal cashback accrual failed",
				zap.Uint64("order_id", orderID), zap.String("order_no", orderNo),
				zap.Uint64("user_id", buyerUserID), zap.Error(err))
		}
		return nil
	})
	lifecycleSvc := lifecycleservice.NewLifecycleService(database, lifecyclePolicyRepo, lifecycleInstanceReader, lifecycleRenewalSvc, logger)
	lifecycleExpiringHandler := lifecyclehandler.NewExpiringHandler(lifecycleSvc)
	lifecycleAdminHandler := lifecyclehandler.NewLifecycleAdminHandler(lifecycleSvc, lifecycleRenewalSvc)
	lifecycleUserHandler := lifecyclehandler.NewLifecycleUserHandler(lifecycleSvc, lifecycleRenewalSvc)
	lifecycleScheduler := lifecycleservice.NewLifecycleScheduler(lifecycleSvc, logger)
	// 生命周期阶段推进器（T5.4）：宽限→暂停→销毁幂等落库；审计写入实例运维流水。
	lifecycleAdvancer := buildLifecycleAdvancer(
		lifecycleInstanceReader, lifecyclePolicyRepo,
		providerService, upstreamMgr,
		&stageActionRecorderBridge{svc: instanceOpsService},
		logger,
	)
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
	// 官网门户公开只读数据（公告等），无需登录；后续 site-content 也落在该模块
	ucSiteService := ucsiteservice.NewSiteService(announceSvc)
	ucSiteHandler := ucsitehandler.NewSiteHandler(ucSiteService)
	// 生命周期 Notifier 桥接：替换 noopNotifier 为通知中心
	lifecycleSvc.SetNotifier(&lifecycleNotifierBridge{notifySvc: notifySvc, logger: logger})
	// 生命周期阶段推进器注入（T5.4）：能力缺失告警复用开通失败告警通道。
	lifecycleAdvancer.SetCapabilityNotifier(buildStageCapabilityNotifier(notifySvc, logger))
	lifecycleSvc.SetStageAdvancer(lifecycleAdvancer)
	// 工单 Notifier 桥接（P2-04）：回复/状态通知用户，指派通知员工
	ticketService.SetNotifier(&ticketNotifierBridge{notifySvc: notifySvc, logger: logger})
	// 调价待确认通知桥接（T3.4）：上游改价超阈值时提醒运维处理
	syncEngine.SetNotifier(&priceChangeNotifierBridge{notifySvc: notifySvc, logger: logger})
	// 开通履约工作池（T5.1）：连续失败转人工队列时向管理端告警。
	provisionWorker := orderservice.NewProvisionWorker(
		provisionTaskRepo, orderService,
		(&provisionManualNotifier{notifySvc: notifySvc, logger: logger}).Notify,
		logger, orderservice.ProvisionWorkerOptions{},
	)
	// 订单支付成功钩子（doc60）：续费订单支付完成后联动完成续费并延长到期时间
	// doc70：支付成功后发布通知
	orderservice.OnPaid = func(ctx context.Context, order *ordermodel.Order) {
		if order == nil {
			return
		}
		observability.Inc("order_paid_total", 1)
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
	// 用户中心成员（子账号，P4）：仓储同时作为 RequireUserPermission 的权限解析器。
	memberRepo := memberrepo.NewMemberRepository(database)
	memberService := memberservice.NewMemberService(memberRepo)
	memberHandler := memberhandler.NewMemberHandler(memberService)
	// 开放平台（P6）：网关与处理器集合，单一 Bundle 字段挂 App。
	// 目录服务复用 uc 商品（上架口径）、spec 契约字典与统一算价管线（D3）；
	// 代客下单复用 UC 下单管线（余额扣 owner、SKU/库存/算价/履约投递同一条路径）。
	openBundle := buildOpenBundle(cfg, database, ucProductService, ucOrderService, specContractRepo, pricePipeline, logger)
	app := NewApp(cfg, adminHandler, userHandler, userDetailHandler, userGroupHandler, roleHandler, permissionHandler, menuHandler, securityHandler, userLevelHandler, verificationHandler, providerHandler, productHandler, syncHandler, syncFrameworkHandler, userCenterAuthHandler, userMenuHandler, prodCategoryHandler, prodCatalogHandler, specHandler, pricingHandler, discountPolicyHandler, promotionHandler, adminReferralHandler, orderHandler, refundHandler, walletHandler, rechargeHandler, withdrawHandler, billHandler, reconHandler, configHandler, userFinanceHandler, ucProductHandler, ucOrderHandler, ucInstanceHandler, instanceOpsHandler, ticketHandler, ticketCategoryHandler, userTicketHandler, lifecycleExpiringHandler, lifecycleAdminHandler, lifecycleUserHandler, notifyAdminHandler, notifyUserHandler, ucSiteHandler, ucReferralHandler, memberHandler, memberRepo, memberRepo, rbacRepo, permCache, adminAuditRepo, openBundle, logger, jwtIssuer)
	router := newRouter(app)

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
		provisionWorker:    provisionWorker,
	}, nil
}

func (s *Server) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	defer cancel()
	s.scheduler.Start(ctx)
	s.lifecycleScheduler.Start(ctx)
	s.provisionWorker.Start(ctx)
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

// ticketNotifierBridge 桥接工单模块的 Notifier 接口到通知中心（P2-04）。
// target=user 时通知工单提交人，target=admin 时通知被指派员工。
type ticketNotifierBridge struct {
	notifySvc notifyservice.NotificationService
	logger    *zap.Logger
}

func (b *ticketNotifierBridge) Publish(ctx context.Context, event, target string, recipientID uint64, vars map[string]string, sourceID string) error {
	if target == "" {
		target = notifymodel.TargetUser
	}
	err := b.notifySvc.Publish(ctx, notifydto.PublishInput{
		Event:        event,
		UserID:       recipientID,
		Target:       target,
		Vars:         vars,
		SourceModule: "ticket",
		SourceID:     sourceID,
	})
	if err != nil {
		b.logger.Warn("ticket: publish notification failed",
			zap.String("event", event), zap.Uint64("recipient_id", recipientID), zap.Error(err))
	}
	return err
}

// priceChangeNotifierBridge 桥接同步引擎的调价提醒到通知中心（T3.4）。
// 上游改价超阈值进入待确认队列时，向管理端发一条站内信（模板 event=sync_failed 之外
// 复用 system 事件，避免为一个提示新增模板）。
type priceChangeNotifierBridge struct {
	notifySvc notifyservice.NotificationService
	logger    *zap.Logger
}

func (b *priceChangeNotifierBridge) NotifyPriceChangePending(ctx context.Context, providerID uint64, providerName string, pending int) error {
	if pending <= 0 {
		return nil
	}
	err := b.notifySvc.Publish(ctx, notifydto.PublishInput{
		Event:  notifymodel.EventSystem,
		Target: notifymodel.TargetAdmin,
		Vars: map[string]string{
			"title":   "上游调价待确认",
			"content": fmt.Sprintf("提供商 %s（#%d）有 %d 条成本价变动超过阈值，售价未自动调整，请前往「同步与调度 → 待确认调价」处理。", providerName, providerID, pending),
		},
		SourceModule: "sync",
		SourceID:     fmt.Sprintf("price-pending-%d-%d", providerID, time.Now().Unix()/3600),
	})
	if err != nil {
		b.logger.Warn("publish price change notification failed",
			zap.Uint64("provider_id", providerID), zap.Error(err))
	}
	return err
}

// buildProductSourceModeResolver 返回商品链路判据解析（T5.1 任务留痕用）：
// 读不到商品时返回空串，不影响开通投递。
func buildProductSourceModeResolver(catalog catalogservice.ProductService) func(ctx context.Context, productID uint64) string {
	return func(ctx context.Context, productID uint64) string {
		product, err := catalog.FindByID(ctx, productID)
		if err != nil || product == nil {
			return ""
		}
		return product.SourceMode
	}
}

// provisionManualNotifier 桥接开通任务连续失败告警到通知中心（T5.1）。
// 复用 system 事件避免为一个提示新增模板（与调价待确认一致）。
type provisionManualNotifier struct {
	notifySvc notifyservice.NotificationService
	logger    *zap.Logger
}

// Notify 发布"开通失败转人工"管理端站内信；发布失败只记日志，不影响工作池。
func (b *provisionManualNotifier) Notify(ctx context.Context, task *ordermodel.ProvisionTask) {
	if task == nil {
		return
	}
	err := b.notifySvc.Publish(ctx, notifydto.PublishInput{
		Event:  notifymodel.EventSystem,
		Target: notifymodel.TargetAdmin,
		Vars: map[string]string{
			"title": "订单开通失败待人工处理",
			"content": fmt.Sprintf("订单 #%d（用户 #%d）连续 %d 次开通失败，已转入人工队列：%s。请前往「订单管理」重试或联系上游排查。",
				task.OrderID, task.UserID, task.Attempts, task.LastError),
		},
		SourceModule: "order",
		SourceID:     fmt.Sprintf("provision-manual-%d", task.OrderID),
	})
	if err != nil {
		b.logger.Warn("publish provision manual notification failed",
			zap.Uint64("order_id", task.OrderID), zap.Error(err))
	}
}

// buildStageCapabilityNotifier 阶段推进遇上游缺能力时的显式告警（T5.4/T5.5）。
//
// 语义：平台确实不支持暂停/销毁时，推进器只落阶段并告警，必须让运营可见，
// 避免"看起来已暂停"的静默假完成。告警复用 system 事件，不新增模板。
func buildStageCapabilityNotifier(notifySvc notifyservice.NotificationService, logger *zap.Logger) lifecycleservice.StageCapabilityNotifier {
	return func(ctx context.Context, inst *syncmodel.Instance, stage string, cause error) {
		if inst == nil {
			return
		}
		err := notifySvc.Publish(ctx, notifydto.PublishInput{
			Event:  notifymodel.EventSystem,
			Target: notifymodel.TargetAdmin,
			Vars: map[string]string{
				"title": "实例到期处置缺少上游能力",
				"content": fmt.Sprintf("实例 #%d（用户 #%d，服务商 #%d）进入阶段 %s 时上游不支持对应操作，已仅落库标记，需人工跟进：%v。",
					inst.ID, inst.UserID, inst.ProviderID, stage, cause),
			},
			SourceModule: "lifecycle",
			SourceID:     fmt.Sprintf("lifecycle-capability-%d-%s", inst.ID, stage),
		})
		if err != nil {
			logger.Warn("publish lifecycle capability notification failed",
				zap.Uint64("instance_id", inst.ID), zap.String("stage", stage), zap.Error(err))
		}
	}
}

// buildRecordedInstance 将标准实例转换为同步实例记录（订单履约开通成功后落库）。
// sourceMode 为商品链路判据（D6），upstreamProductID 为代理商品对应的上游资源商品 ID。
func buildRecordedInstance(inst *model.StandardInstance, order *ordermodel.Order, sourceMode string, upstreamProductID uint64) syncmodel.Instance {
	rawJSON, _ := json.Marshal(inst.RawData)
	// 规格回落（T4.1）：上游详情拉取失败时适配器只返回最小实例（规格为空），
	// 此时用订单规格快照（SKU 原子取值 JSON）补齐，保证实例列表能显示下单所选配置。
	specs := inst.Specs
	if specs.CPU == 0 && specs.Memory == 0 && specs.Disk == 0 {
		if snapshot := specsFromOrder(order); snapshot != nil && (snapshot.CPU > 0 || snapshot.Memory > 0 || snapshot.Disk > 0) {
			specs = *snapshot
		}
	}
	if sourceMode == "" {
		sourceMode = syncmodel.SourceModeSelf
	}
	row := syncmodel.Instance{
		InstanceID:  inst.UpstreamID,
		ProviderID:  uint64(inst.ProviderID),
		UserID:      order.UserID,
		ProductID:   order.ProductID,
		Name:        firstNonEmpty(inst.Name, order.ProductName),
		CPU:         specs.CPU,
		Memory:      specs.Memory,
		Disk:        specs.Disk,
		DiskType:    specs.DiskType,
		Bandwidth:   specs.Bandwidth,
		OS:          specs.OS,
		Region:      firstNonEmpty(inst.Region, specs.Region),
		Zone:        firstNonEmpty(inst.Zone, specs.Zone),
		Status:      string(inst.Status),
		PublicIP:    inst.PublicIP,
		PrivateIP:   inst.PrivateIP,
		RawData:     string(rawJSON),
		BillingMode: order.PriceModel,
		// 子账号下单时 order.OperatorID 为真实操作人，落到实例「操作人」列（P4-09）。
		ActorUserID: order.OperatorID,
		// 记录来源订单，实例运维台据此展示关联订单（见 61 实施计划 §4.1）。
		OrderID: order.ID,
		// 双链路语义（P1/T1.2，D6）：判据取自商品 source_mode，而非"是否经订单开通"
		//——代理商品（upstream）经订单开通后仍是上游链路实例，续费/暂停/销毁须调上游。
		// sell_product_id 恒为我方售出的 products.id（续费算价与展示的权威来源）；
		// 上游链路另记 upstream_product_id（resource_products.id）便于对账与回溯源。
		SourceMode:         sourceMode,
		SellProductID:      order.ProductID,
		UpstreamProductID:  upstreamProductID,
		ProviderInstanceID: inst.UpstreamID,
	}
	if !inst.ExpireAt.IsZero() {
		exp := inst.ExpireAt
		row.ExpireAt = &exp
	}
	return row
}

// specsFromOrder 解析订单的规格快照（SKU 原子取值 JSON）为标准规格；解析失败返回 nil。
func specsFromOrder(order *ordermodel.Order) *model.StandardProductSpec {
	raw := strings.TrimSpace(order.Specs)
	if raw == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	spec := specatom.FromMap(m)
	return &spec
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
