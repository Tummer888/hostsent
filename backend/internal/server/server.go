package server

// server 是装配层入口：newRouter 组装引擎骨架，New 按依赖序装配各域并启动。
// 路由注册拆分在 routes_*.go，各域装配拆分在 assembly_*.go（批次四 4b）。

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	costservice "hostsent/backend/internal/modules/admin/finance/cost/service"
	lifecycleservice "hostsent/backend/internal/modules/admin/instance/lifecycle/service"
	orderservice "hostsent/backend/internal/modules/admin/order/service"
	reconcileservice "hostsent/backend/internal/modules/admin/resource/reconcile/service"
	syncservice "hostsent/backend/internal/modules/admin/resource/sync/service"
	notifyservice "hostsent/backend/internal/modules/admin/system/notification/service"
	salesservice "hostsent/backend/internal/modules/admin/system/sales/service"
	"hostsent/backend/internal/modules/admin/user/account/service"
	securityservice "hostsent/backend/internal/modules/admin/user/security/service"
	openservice "hostsent/backend/internal/modules/open/service"
	ucorderservice "hostsent/backend/internal/modules/uc/order/service"
	"hostsent/backend/internal/pkg/config"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/observability"
	"hostsent/backend/internal/pkg/publishsched"
)

type Server struct {
	cfg                *config.Config
	logger             *zap.Logger
	http               *http.Server
	scheduler          *syncservice.Scheduler
	lifecycleScheduler *lifecycleservice.LifecycleScheduler
	// pendingExpireScheduler 待支付订单过期关单（doc88 §6.2）。
	pendingExpireScheduler *ucorderservice.PendingExpireScheduler
	salesScheduler         *salesservice.ReleaseScheduler
	// costScheduler 上游余额快照每日自动抓取（doc111 §5）。
	costScheduler *costservice.SnapshotScheduler
	// userPurgeScheduler 用户留存期清理（doc104 §4.6）：到期硬删除已注销用户。
	userPurgeScheduler *service.PurgeScheduler
	// sessionExpireScheduler 会话过期状态回写：expired_at 到点后把 status 收成 expired。
	sessionExpireScheduler *securityservice.SessionExpireScheduler
	provisionWorker        *orderservice.ProvisionWorker
	notifyWorker           *openservice.NotifyDeliveryWorker
	// deliveryWorker 通知投递队列工作器（doc90 N4）：邮件/短信排队投递与重试。
	deliveryWorker *notifyservice.DeliveryWorker
	// logcenter 日志中心后台组件（doc92）：上游采集写入器 + 统一清理调度器。
	logcenter *logcenterBundle
	// publishScheduler 内容定时发布（公告与内容文章 draft → published）。
	publishScheduler *publishsched.Scheduler
	// findingScheduler 对账差异扫描（doc120）：本地比对，周期落台账并触发预警。
	findingScheduler *reconcileservice.FindingScheduler
	cancel           context.CancelFunc
}

// newRouter 构造 gin 引擎：引擎 setup + 可信代理 + 全局中间件 + 健康检查端点。
// 路由注册按端拆分到 routes_admin/uc/site/open 四个文件（批次四 4a），
// 本函数只保留跨端共享的骨架，不再承载具体业务路由。
func newRouter(app *App) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	// 可信代理声明：决定 ClientIP 是否采信 X-Forwarded-For。
	// 不声明时 gin 走内置默认（信任全部私有网段），在容器网络里会把
	// docker 网关地址当代理、把真实公网 IP 丢掉。详见 netutil.ClientIP 的注释：
	// IP 不只是日志字段，还是登录失败锁定（failIPKey）的键。
	if err := r.SetTrustedProxies(app.cfg.App.TrustedProxies); err != nil {
		// 配置写错不该让服务起不来，但必须留下明确告警：静默降级会让所有用户
		// 在日志里显示成同一个代理 IP，排查时毫无线索。
		app.logger.Warn("invalid trusted_proxies, falling back to loopback only",
			zap.Error(err), zap.Strings("configured", app.cfg.App.TrustedProxies))
		_ = r.SetTrustedProxies([]string{"127.0.0.1/8", "::1"})
	}
	r.Use(gin.Recovery(), middleware.Logger(app.logger), cors.Default())
	// 用户中心限流（doc91 §9.1）：仅对 /api/v1/uc 前缀生效，挂全局以保证不漏挂。
	r.Use(app.ucRateLimit())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"status": "ok"}})
	})
	r.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ready", "data": gin.H{"status": "ready"}})
	})
	r.GET("/metrics", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/plain; version=0.0.4", []byte(observability.Snapshot()))
	})

	// 路由注册按端拆分：管理端 / 用户中心 / 门户公开 / 开放平台 / 上游推送。
	registerAdminRoutes(r, app)
	registerUCRoutes(r, app)
	registerSiteRoutes(r, app)
	registerOpenRoutes(r, app)
	// 上游推送接收端（doc120）：/api/host/sync 与 /api/ticket_reply/sync，
	// 路径由上游协议写死（无 /v1 前缀），不挂登录中间件、单独限流。
	registerUpstreamPushRoutes(r, app)

	return r
}

// New 构造 Server：按依赖序调用各域 buildX（批次四 4b），本函数只做编排、
// App 字面量填充与 Server 组装；具体语句见 assembly_*.go。
func New(cfg *config.Config, logger *zap.Logger) (*Server, error) {
	a, err := newAsm(cfg, logger)
	if err != nil {
		return nil, err
	}
	a.buildFinance()
	a.buildConfig()
	a.buildResource()
	a.buildAdminCore()
	a.buildUserDomain()
	a.buildUCAuth()
	a.buildCatalog()
	a.buildReferralCost()
	a.buildOrder()
	a.buildInstanceOps()
	a.buildTicket()
	a.buildLifecycle()
	a.buildNotify()
	a.buildContent()
	a.buildSite()
	a.buildBridges()
	a.buildBundles()
	a.buildWire()

	app := &App{
		cfg:                     a.cfg,
		logger:                  a.logger,
		jwtIssuer:               a.jwtIssuer,
		cache:                   a.cacheClient,
		sessionGuard:            a.sessionGuard,
		rbacRepo:                a.rbacRepo,
		permCache:               a.permCache,
		auditWriter:             a.adminAuditRepo,
		adminHandler:            a.adminHandler,
		departmentHandler:       a.departmentHandler,
		userHandler:             a.userHandler,
		userDetailHandler:       a.userDetailHandler,
		userDeletionHandler:     a.userDeletionHandler,
		userGroupHandler:        a.userGroupHandler,
		roleHandler:             a.roleHandler,
		permissionHandler:       a.permissionHandler,
		menuHandler:             a.menuHandler,
		securityHandler:         a.securityHandler,
		userLevelHandler:        a.userLevelHandler,
		agentLevelHandler:       a.agentLevelHandler,
		schemeHandler:           a.schemeHandler,
		providerHandler:         a.providerHandler,
		productHandler:          a.productHandler,
		syncHandler:             a.syncHandler,
		syncFrameworkHandler:    a.syncFrameworkHandler,
		userCenterAuthHandler:   a.userCenterAuthHandler,
		userMenuHandler:         a.userMenuHandler,
		prodCategoryHandler:     a.prodCategoryHandler,
		prodCatalogHandler:      a.prodCatalogHandler,
		specHandler:             a.specHandler,
		pricingHandler:          a.pricingHandler,
		priceMatrixHandler:      a.priceMatrixHandler,
		discountPolicyHandler:   a.discountPolicyHandler,
		promotionHandler:        a.promotionHandler,
		flashDiscountHandler:    a.flashDiscountHandler,
		adminReferralHandler:    a.adminReferralHandler,
		salesCustomerHandler:    a.salesBundle.customerHandler,
		salesCommissionHandler:  a.salesBundle.commissionHandler,
		salesPerformanceHandler: a.salesBundle.performanceHandler,
		orderHandler:            a.orderHandler,
		refundHandler:           a.refundHandler,
		walletHandler:           a.walletHandler,
		rechargeHandler:         a.rechargeHandler,
		withdrawHandler:         a.withdrawHandler,
		billHandler:             a.billHandler,
		reconHandler:            a.reconHandler,
		financeStatsHandler:     a.financeStatsHandler,
		financeSettingsHandler:  a.financeSettingsHandler,
		costHandler:             a.costBundle.handler,
		configHandler:           a.configHandler,
		userFinanceHandler:      a.userFinanceHandler,
		ucProductHandler:        a.ucProductHandler,
		ucOrderHandler:          a.ucOrderHandler,
		ucInstanceHandler:       a.ucInstanceHandler,
		instanceOpsHandler:      a.instanceOpsHandler,
		taskQueueHandler:        a.taskQueueHandler,
		reconcileHandler:        a.reconcileHandler,
		reconcileFindingHandler: a.reconcileFindingHandler,
		upstreamPushHandler:     a.upstreamPushHandler,
		ticketHandler:           a.ticketHandler,
		ticketCategoryHandler:   a.ticketCategoryHandler,
		userTicketHandler:       a.userTicketHandler,
		expiringHandler:         a.lifecycleExpiringHandler,
		lifecycleAdminHandler:   a.lifecycleAdminHandler,
		lifecycleUserHandler:    a.lifecycleUserHandler,
		notifyAdminHandler:      a.notifyAdminHandler,
		notifyUserHandler:       a.notifyUserHandler,
		siteHandler:             a.siteHandler,
		ucReferralHandler:       a.ucReferralHandler,
		memberHandler:           a.memberHandler,
		memberRepo:              a.memberRepo,
		userAuditWriter:         a.memberRepo,
		open:                    a.openBundle,
		payment:                 a.paymentBundle,
		point:                   a.pointBundle,
		captcha:                 a.captchaBundle,
		notify:                  a.notifyBundleInst,
		logcenter:               a.logcenterBundle,
		content:                 a.contentBundle,
		verification:            a.verificationBundle,
		oauth:                   a.oauthBundle,
	}
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
		scheduler:              a.scheduler,
		lifecycleScheduler:     a.lifecycleScheduler,
		pendingExpireScheduler: a.pendingExpireScheduler,
		salesScheduler:         a.salesBundle.scheduler,
		costScheduler:          a.costBundle.scheduler,
		userPurgeScheduler:     a.userPurgeScheduler,
		sessionExpireScheduler: a.sessionExpireScheduler,
		provisionWorker:        a.provisionWorker,
		notifyWorker:           a.notifyWorker,
		deliveryWorker:         a.notifyBundleInst.worker,
		logcenter:              a.logcenterBundle,
		publishScheduler:       a.publishScheduler,
		findingScheduler:       a.findingScheduler,
	}, nil
}

func (s *Server) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	defer cancel()
	s.scheduler.Start(ctx)
	s.lifecycleScheduler.Start(ctx)
	s.pendingExpireScheduler.Start(ctx)
	s.salesScheduler.Start(ctx)
	if s.costScheduler != nil {
		s.costScheduler.Start(ctx)
	}
	s.userPurgeScheduler.Start(ctx)
	s.sessionExpireScheduler.Start(ctx)
	s.provisionWorker.Start(ctx)
	go s.notifyWorker.Start(ctx)
	s.publishScheduler.Start(ctx)
	// 对账差异扫描（doc120）：纯本地比对，不依赖任何渠道连接，未装配时静默跳过。
	if s.findingScheduler != nil {
		s.findingScheduler.Start(ctx)
	}
	if s.deliveryWorker != nil {
		go s.deliveryWorker.Start(ctx)
	}
	// 日志中心（doc92）：上游采集写入器与统一清理调度器。
	// 二者都以 ctx 控制生命周期，退出前写入器会尽力落盘一次。
	if s.logcenter != nil {
		if s.logcenter.recorder != nil {
			go s.logcenter.recorder.Start(ctx)
		}
		if s.logcenter.scheduler != nil {
			s.logcenter.scheduler.Start(ctx)
		}
		// 进程中断遗留的 running 任务行会让对应 job 永久被判重入而停摆，
		// 启动时统一收成 failed（doc92 §7.1 重入保护的补丁）。
		if s.logcenter.Runner != nil {
			if _, err := s.logcenter.Runner.ReconcileStaleRunning(ctx); err != nil {
				s.logger.Warn("logcenter: reconcile stale running job rows failed", zap.Error(err))
			}
		}
		// 采集与保留期配置周期重读：改配置不重启即生效（doc89 §9.1）。
		s.logcenter.startCaptureRefresh(ctx)
	}
	s.logger.Info("server starting", zap.String("addr", s.http.Addr), zap.String("name", s.cfg.App.Name))
	return s.http.ListenAndServe()
}

// lifecycleNotifierBridge 桥接生命周期模块的 Notifier 接口到通知中心。
