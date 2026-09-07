package server

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	financehandler "hostsent/backend/internal/modules/admin/finance/handler"
	adminhandler "hostsent/backend/internal/modules/admin/manager/handler"
	menuhandler "hostsent/backend/internal/modules/admin/menu/handler"
	lifecyclehandler "hostsent/backend/internal/modules/admin/lifecycle/handler"
	notifyhandler "hostsent/backend/internal/modules/admin/notification/handler"
	orderhandler "hostsent/backend/internal/modules/admin/order/handler"
	prodhandler "hostsent/backend/internal/modules/admin/product/handler"
	producthandler "hostsent/backend/internal/modules/admin/resource/product/handler"
	providerhandler "hostsent/backend/internal/modules/admin/resource/provider/handler"
	synchandler "hostsent/backend/internal/modules/admin/resource/sync/handler"
	systemhandler "hostsent/backend/internal/modules/admin/system/handler"
	tickethandler "hostsent/backend/internal/modules/admin/ticket/handler"
	"hostsent/backend/internal/modules/admin/user/account/handler"
	distributionhandler "hostsent/backend/internal/modules/admin/user/distribution/handler"
	quotahandler "hostsent/backend/internal/modules/admin/user/quota/handler"
	securityhandler "hostsent/backend/internal/modules/admin/user/security/handler"
	verificationhandler "hostsent/backend/internal/modules/admin/user/verification/handler"
	usercenterhandler "hostsent/backend/internal/modules/user/auth/handler"
	userfinancehandler "hostsent/backend/internal/modules/user/finance/handler"
	usermenuhandler "hostsent/backend/internal/modules/user/menu/handler"
	appauth "hostsent/backend/internal/pkg/auth"
	"hostsent/backend/internal/pkg/config"
	"hostsent/backend/internal/pkg/middleware"
)

func newRouter(cfg *config.Config, adminHandler *adminhandler.AdminHandler, userHandler *handler.UserHandler, userDetailHandler *handler.UserDetailHandler, userGroupHandler *handler.UserGroupHandler, agentLevelHandler *distributionhandler.AgentLevelHandler, agentHandler *distributionhandler.AgentHandler, subordinateHandler *distributionhandler.SubordinateHandler, commissionHandler *distributionhandler.CommissionHandler, settlementHandler *distributionhandler.SettlementHandler, roleHandler *handler.RoleHandler, permissionHandler *handler.PermissionHandler, menuHandler *menuhandler.MenuHandler, securityHandler *securityhandler.SecurityHandler, resourceQuotaHandler *quotahandler.ResourceQuotaHandler, quotaTemplateHandler *quotahandler.QuotaTemplateHandler, quotaUserLevelHandler *quotahandler.UserLevelHandler, quotaAdjustmentHandler *quotahandler.QuotaAdjustmentHandler, verificationHandler *verificationhandler.VerificationHandler, providerHandler *providerhandler.ProviderHandler, productHandler *producthandler.ProductHandler, syncHandler *synchandler.SyncHandler, userCenterAuthHandler *usercenterhandler.AuthHandler, userMenuHandler *usermenuhandler.MenuHandler, prodCategoryHandler *prodhandler.CategoryHandler, prodProductHandler *prodhandler.ProductHandler, orderHandler *orderhandler.OrderHandler, refundHandler *orderhandler.RefundHandler, walletHandler *financehandler.WalletHandler, rechargeHandler *financehandler.RechargeHandler, withdrawHandler *financehandler.WithdrawHandler, billHandler *financehandler.BillHandler, reconHandler *financehandler.ReconHandler, configHandler *systemhandler.ConfigHandler, userFinanceHandler *userfinancehandler.FinanceHandler, ticketHandler *tickethandler.TicketHandler, ticketCategoryHandler *tickethandler.CategoryHandler, userTicketHandler *tickethandler.UserTicketHandler, expiringHandler *lifecyclehandler.ExpiringHandler, lifecycleAdminHandler *lifecyclehandler.LifecycleAdminHandler, lifecycleUserHandler *lifecyclehandler.LifecycleUserHandler, notifyAdminHandler *notifyhandler.AdminHandler, notifyUserHandler *notifyhandler.UserHandler, logger *zap.Logger, jwtIssuer *appauth.JWTIssuer) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery(), middleware.Logger(logger), cors.Default())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"status": "ok"}})
	})
	r.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ready", "data": gin.H{"status": "ready"}})
	})

	v1 := r.Group("/api/v1/admin")
	{
		auth := v1.Group("/auth")
		{
			auth.POST("/login", adminHandler.Login)
			auth.GET("/me", middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix), adminHandler.Me)
			auth.GET("/admins", middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix), adminHandler.List)
			auth.POST("/admins", middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix), adminHandler.Create)
			auth.GET("/admins/:id", middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix), adminHandler.Get)
			auth.PUT("/admins/:id", middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix), adminHandler.Update)
			auth.PATCH("/admins/:id/status", middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix), adminHandler.UpdateStatus)
			auth.POST("/admins/:id/reset-password", middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix), adminHandler.ResetPassword)
			auth.DELETE("/admins/:id", middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix), adminHandler.Delete)
		}

		users := v1.Group("/users")
		users.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			users.GET("", userHandler.ListUsers)
			users.POST("", userHandler.CreateUser)
			users.GET("/stats", userHandler.GetStats)
			users.GET("/region-stats", userHandler.GetRegionStats)
			users.GET(":id", userHandler.GetUser)
			users.GET(":id/detail-aggregate", userDetailHandler.GetAggregate)
			users.PUT(":id", userHandler.UpdateUser)
			users.PATCH(":id/status", userHandler.UpdateUserStatus)
			users.POST(":id/reset-password", userHandler.ResetPassword)
			users.POST(":id/roles", userHandler.AssignRoles)
		}

		userGroups := v1.Group("/user-groups")
		userGroups.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			userGroups.GET("", userGroupHandler.List)
			userGroups.POST("", userGroupHandler.Create)
			userGroups.GET("/:id", userGroupHandler.Get)
			userGroups.PUT("/:id", userGroupHandler.Update)
			userGroups.DELETE("/:id", userGroupHandler.Delete)
		}

		agentLevels := v1.Group("/distribution/agent-levels")
		agentLevels.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			agentLevels.GET("", agentLevelHandler.List)
			agentLevels.POST("", agentLevelHandler.Create)
			agentLevels.GET("/:id", agentLevelHandler.Get)
			agentLevels.PUT("/:id", agentLevelHandler.Update)
			agentLevels.DELETE("/:id", agentLevelHandler.Delete)
		}

		agents := v1.Group("/distribution/agents")
		agents.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			agents.GET("", agentHandler.List)
			agents.POST("", agentHandler.Create)
			agents.GET("/:id", agentHandler.Get)
			agents.PUT("/:id", agentHandler.Update)
			agents.DELETE("/:id", agentHandler.Delete)
		}

		subordinates := v1.Group("/distribution/subordinates")
		subordinates.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			subordinates.GET("", subordinateHandler.List)
			subordinates.POST("", subordinateHandler.Create)
			subordinates.GET("/:id", subordinateHandler.Get)
			subordinates.PUT("/:id", subordinateHandler.Update)
			subordinates.DELETE("/:id", subordinateHandler.Delete)
		}

		commissions := v1.Group("/distribution/commissions")
		commissions.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			commissions.GET("", commissionHandler.List)
			commissions.POST("", commissionHandler.Create)
			commissions.GET("/:id", commissionHandler.Get)
			commissions.PUT("/:id", commissionHandler.Update)
			commissions.POST("/:id/freeze", commissionHandler.Freeze)
			commissions.POST("/:id/unfreeze", commissionHandler.Unfreeze)
			commissions.POST("/:id/cancel", commissionHandler.Cancel)
			commissions.DELETE("/:id", commissionHandler.Delete)
		}

		settlements := v1.Group("/distribution/settlements")
		settlements.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			settlements.GET("", settlementHandler.List)
			settlements.POST("", settlementHandler.Create)
			settlements.GET("/:id", settlementHandler.Get)
			settlements.PUT("/:id", settlementHandler.Update)
			settlements.POST("/:id/confirm", settlementHandler.Confirm)
			settlements.POST("/:id/pay", settlementHandler.Pay)
			settlements.POST("/:id/cancel", settlementHandler.Cancel)
			settlements.DELETE("/:id", settlementHandler.Delete)
		}

		roles := v1.Group("/roles")
		roles.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			roles.GET("", roleHandler.ListRoles)
			roles.POST("", roleHandler.CreateRole)
			roles.GET("/:id", roleHandler.GetRole)
			roles.PUT("/:id", roleHandler.UpdateRole)
			roles.DELETE("/:id", roleHandler.DeleteRole)
			roles.GET("/:id/permissions", roleHandler.GetRolePermissions)
			roles.POST("/:id/permissions", roleHandler.AssignPermissions)
		}

		quotas := v1.Group("/quotas")
		quotas.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			quotas.GET("", resourceQuotaHandler.List)
			quotas.GET("/:id", resourceQuotaHandler.Get)
			quotas.GET("/users/:user_id", resourceQuotaHandler.GetByUser)
			quotas.POST("/:id/adjust", resourceQuotaHandler.Adjust)
		}

		quotaTemplates := v1.Group("/quota-templates")
		quotaTemplates.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			quotaTemplates.GET("", quotaTemplateHandler.List)
			quotaTemplates.POST("", quotaTemplateHandler.Create)
			quotaTemplates.GET("/:id", quotaTemplateHandler.Get)
			quotaTemplates.PUT("/:id", quotaTemplateHandler.Update)
			quotaTemplates.DELETE("/:id", quotaTemplateHandler.Delete)
		}

		userLevels := v1.Group("/user-levels")
		userLevels.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			userLevels.GET("", quotaUserLevelHandler.List)
			userLevels.POST("", quotaUserLevelHandler.Create)
			userLevels.GET("/:id", quotaUserLevelHandler.Get)
			userLevels.PUT("/:id", quotaUserLevelHandler.Update)
			userLevels.DELETE("/:id", quotaUserLevelHandler.Delete)
			userLevels.POST("/:id/bind-template", quotaUserLevelHandler.BindTemplate)
		}

		quotaAdjustments := v1.Group("/quota-adjustments")
		quotaAdjustments.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			quotaAdjustments.GET("", quotaAdjustmentHandler.List)
			quotaAdjustments.GET("/:id", quotaAdjustmentHandler.Get)
		}

		verifications := v1.Group("/verifications")
		verifications.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			verifications.GET("/pending", verificationHandler.ListPending)
			verifications.GET("/approved", verificationHandler.ListApproved)
			verifications.GET("/rejected", verificationHandler.ListRejected)
		}

		permissions := v1.Group("/permissions")
		permissions.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			permissions.GET("/tree", permissionHandler.Tree)
			permissions.POST("", permissionHandler.CreatePermission)
			permissions.PUT("/:id", permissionHandler.UpdatePermission)
			permissions.DELETE("/:id", permissionHandler.DeletePermission)
		}

		menus := v1.Group("/menus")
		menus.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			menus.GET("/tree", menuHandler.Tree)
			menus.POST("", menuHandler.CreateMenu)
			menus.PUT("/:id", menuHandler.UpdateMenu)
			menus.DELETE("/:id", menuHandler.DeleteMenu)
		}

		security := v1.Group("/security")
		security.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			security.GET("/login-logs/export", securityHandler.ExportLoginLogs)
			security.GET("/login-logs", securityHandler.ListLoginLogs)
			security.GET("/login-logs/:id", securityHandler.GetLoginLog)
			security.GET("/audit-logs", securityHandler.ListAuditLogs)
			security.GET("/audit-logs/:id", securityHandler.GetAuditLog)
			security.GET("/audit-logs/export", securityHandler.ExportAuditLogs)
			security.GET("/risk-events", securityHandler.ListRiskEvents)
			security.GET("/risk-events/:id", securityHandler.GetRiskEvent)
			security.POST("/risk-events/:id/ignore", securityHandler.IgnoreRiskEvent)
			security.POST("/risk-events/:id/handle", securityHandler.HandleRiskEvent)
			security.POST("/risk-events/:id/blacklist", securityHandler.CreateBlacklistFromRisk)
			security.POST("/risk-events/:id/revoke-sessions", securityHandler.RevokeSessionsFromRisk)
			security.GET("/blacklists", securityHandler.ListBlacklists)
			security.POST("/blacklists", securityHandler.CreateBlacklist)
			security.GET("/blacklists/:id", securityHandler.GetBlacklist)
			security.PUT("/blacklists/:id", securityHandler.UpdateBlacklist)
			security.PATCH("/blacklists/:id/status", securityHandler.UpdateBlacklistStatus)
			security.DELETE("/blacklists/:id", securityHandler.ReleaseBlacklist)
			security.GET("/blacklists/:id/hits", securityHandler.ListBlacklistHits)
			security.GET("/sessions", securityHandler.ListSessions)
			security.GET("/sessions/:id", securityHandler.GetSession)
			security.POST("/sessions/:id/revoke", securityHandler.RevokeSession)
			security.POST("/sessions/batch-revoke", securityHandler.BatchRevokeSessions)
			security.POST("/sessions/revoke-user-all", securityHandler.RevokeUserAllSessions)
		}

		// 资源管理（对接上游）— 第一阶段
		resourceGroup := v1.Group("/resource")
		resourceGroup.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			// 上游提供商
			providers := resourceGroup.Group("/providers")
			{
				providers.GET("", providerHandler.List)
				providers.POST("", providerHandler.Create)
				providers.GET("/types", providerHandler.ListTypes)
				providers.GET("/:id", providerHandler.Get)
				providers.PUT("/:id", providerHandler.Update)
				providers.DELETE("/:id", providerHandler.Delete)
				providers.POST("/:id/test", providerHandler.TestConnection)
			}

			// 资源池
			pools := resourceGroup.Group("/pools")
			{
				pools.GET("", providerHandler.ListPools)
				pools.GET("/:id", providerHandler.GetPool)
			}

			// 上游商品
			products := resourceGroup.Group("/products")
			{
				products.GET("", productHandler.List)
				products.GET("/:id", productHandler.Get)
				products.PUT("/:id/price", productHandler.UpdatePrice)
				products.POST("/sync", productHandler.Sync)
			}

			// 同步管理
			sync := resourceGroup.Group("/sync")
			{
				sync.POST("", syncHandler.CreateTask)
				sync.GET("/tasks", syncHandler.ListTasks)
				sync.GET("/tasks/:id", syncHandler.GetTask)
				sync.GET("/logs", syncHandler.ListLogs)
			}

			// 实例管理
			instances := resourceGroup.Group("/instances")
			{
				instances.GET("", syncHandler.ListInstances)
				instances.GET("/:id", syncHandler.GetInstance)
			}
		}

		// 生命周期管理（doc60 §7.1）
		// 到期实例 + 代续费：/admin/instances/expiring、/admin/instances/:id/renew
		// 注意：/expiring 固定路径需先于 /:id 注册，避免路由冲突
		lcInstances := v1.Group("/instances")
		lcInstances.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			lcInstances.GET("/expiring", expiringHandler.List)
			lcInstances.POST("/:id/renew", lifecycleAdminHandler.RenewAdmin)
		}

		// 续费记录：/admin/renewals
		lcRenewals := v1.Group("/renewals")
		lcRenewals.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			lcRenewals.GET("", lifecycleAdminHandler.ListRenewals)
			lcRenewals.GET("/:id", lifecycleAdminHandler.GetRenewal)
		}

		// 策略与手动扫描：/admin/lifecycle/policy、/admin/lifecycle/scan
		lifecycleGroup := v1.Group("/lifecycle")
		lifecycleGroup.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			lifecycleGroup.GET("/policy", lifecycleAdminHandler.GetPolicy)
			lifecycleGroup.PUT("/policy", lifecycleAdminHandler.UpdatePolicy)
			lifecycleGroup.POST("/scan", lifecycleAdminHandler.ScanOnce)
		}

		// 产品管理（面向终端售卖）
		productGroup := v1.Group("/product")
		productGroup.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			// 产品分类
			categories := productGroup.Group("/categories")
			{
				categories.GET("", prodCategoryHandler.List)
				categories.POST("", prodCategoryHandler.Create)
				categories.GET("/:id", prodCategoryHandler.Get)
				categories.PUT("/:id", prodCategoryHandler.Update)
				categories.DELETE("/:id", prodCategoryHandler.Delete)
			}

			// 产品
			prodProducts := productGroup.Group("/products")
			{
				prodProducts.GET("", prodProductHandler.List)
				prodProducts.POST("", prodProductHandler.Create)
				prodProducts.GET("/:id", prodProductHandler.Get)
				prodProducts.PUT("/:id", prodProductHandler.Update)
				prodProducts.DELETE("/:id", prodProductHandler.Delete)
				prodProducts.POST("/:id/publish", prodProductHandler.Publish)
				prodProducts.POST("/:id/unpublish", prodProductHandler.Unpublish)
				prodProducts.PUT("/:id/price", prodProductHandler.UpdatePrice)
				prodProducts.GET("/:id/history", prodProductHandler.ListHistory)
				prodProducts.GET("/:id/specs", prodProductHandler.ListSpecs)
			}
		}

		// 订单管理
		orderGroup := v1.Group("/orders")
		orderGroup.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			orderGroup.GET("", orderHandler.List)
			orderGroup.GET("/stats", orderHandler.Stats)
			orderGroup.GET("/:id", orderHandler.Get)
			orderGroup.POST("/:id/cancel", orderHandler.Cancel)
			orderGroup.PUT("/:id/remark", orderHandler.UpdateRemark)
			orderGroup.POST("/:id/refund", orderHandler.CreateRefund)
			orderGroup.POST("/:id/activate", orderHandler.Activate)
		}

		// 工单支持（doc50）
		ticketGroup := v1.Group("/tickets")
		ticketGroup.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			ticketGroup.GET("", ticketHandler.List)
			ticketGroup.GET("/stats", ticketHandler.Stats)
			ticketGroup.GET("/:id", ticketHandler.Get)
			ticketGroup.POST("/:id/reply", ticketHandler.Reply)
			ticketGroup.PUT("/:id/assign", ticketHandler.Assign)
			ticketGroup.PUT("/:id/status", ticketHandler.UpdateStatus)
			ticketGroup.POST("/:id/close", ticketHandler.Close)
		}

		// 工单分类（doc50）
		ticketCategoryGroup := v1.Group("/ticket-categories")
		ticketCategoryGroup.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			ticketCategoryGroup.GET("", ticketCategoryHandler.List)
			ticketCategoryGroup.POST("", ticketCategoryHandler.Create)
			ticketCategoryGroup.PUT("/:id", ticketCategoryHandler.Update)
			ticketCategoryGroup.DELETE("/:id", ticketCategoryHandler.Delete)
		}

		// 退款管理
		refundGroup := v1.Group("/refunds")
		refundGroup.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			refundGroup.GET("", refundHandler.List)
			refundGroup.GET("/:id", refundHandler.Get)
			refundGroup.POST("/:id/approve", refundHandler.Approve)
			refundGroup.POST("/:id/reject", refundHandler.Reject)
		}

		// 财务管理
		financeGroup := v1.Group("/finance")
		financeGroup.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			// 钱包/流水
			financeGroup.GET("/wallets/:user_id", walletHandler.Balance)
			financeGroup.GET("/transactions", walletHandler.ListTransactions)
			financeGroup.POST("/transactions/adjust", walletHandler.Adjust)
			// 充值
			financeGroup.POST("/recharges", rechargeHandler.Create)
			financeGroup.GET("/recharges", rechargeHandler.List)
			financeGroup.POST("/recharges/:id/approve", rechargeHandler.Approve)
			// 提现
			financeGroup.GET("/withdrawals", withdrawHandler.List)
			financeGroup.POST("/withdrawals/:id/approve", withdrawHandler.Approve)
			financeGroup.POST("/withdrawals/:id/reject", withdrawHandler.Reject)
			// 账单/对账
			financeGroup.GET("/bills", billHandler.List)
			financeGroup.POST("/bills/:id/close", billHandler.Close)
			financeGroup.POST("/bills/recon", reconHandler.Reconcile)
		}

		// 系统管理（系统配置）
		systemConfigGroup := v1.Group("/system/configs")
		systemConfigGroup.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			systemConfigGroup.GET("", configHandler.List)
			systemConfigGroup.POST("", configHandler.Create)
			systemConfigGroup.GET("/:key", configHandler.GetByKey)
			systemConfigGroup.PUT("/:id", configHandler.Update)
			systemConfigGroup.DELETE("/:id", configHandler.Delete)
		}

		// 消息中心 - 公告管理（doc70 §7.1）
		annGroup := v1.Group("/announcements")
		annGroup.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			annGroup.GET("", notifyAdminHandler.ListAnnouncements)
			annGroup.POST("", notifyAdminHandler.CreateAnnouncement)
			annGroup.PUT("/:id", notifyAdminHandler.UpdateAnnouncement)
			annGroup.POST("/:id/publish", notifyAdminHandler.PublishAnnouncement)
			annGroup.POST("/:id/offline", notifyAdminHandler.OfflineAnnouncement)
			annGroup.DELETE("/:id", notifyAdminHandler.DeleteAnnouncement)
		}

		// 消息中心 - 通知记录（doc70 §7.1）
		// 注意：/unread-count 和 /mail-test 固定路径需先于 /:id/resend 注册
		notifyGroup := v1.Group("/notifications")
		notifyGroup.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			notifyGroup.GET("", notifyAdminHandler.ListRecords)
			notifyGroup.GET("/unread-count", notifyAdminHandler.AdminUnreadCount)
			notifyGroup.POST("/mail-test", notifyAdminHandler.SendMailTest)
			notifyGroup.POST("/:id/resend", notifyAdminHandler.Resend)
		}

		// 消息中心 - 通知模板（doc70 §7.1）
		tplGroup := v1.Group("/notification-templates")
		tplGroup.Use(middleware.AdminAuth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			tplGroup.GET("", notifyAdminHandler.ListTemplates)
			tplGroup.PUT("/:id", notifyAdminHandler.UpdateTemplate)
		}
	}

	// 用户中心（普通用户自助）：独立模块 internal/modules/user/auth
	ucAuth := r.Group("/api/v1/uc/auth")
	{
		ucAuth.POST("/login", userCenterAuthHandler.Login)                                                                   // 登录
		ucAuth.POST("/register", userCenterAuthHandler.Register)                                                             // 注册
		ucAuth.POST("/logout", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userCenterAuthHandler.Logout)          // 登出
		ucAuth.GET("/userinfo", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userCenterAuthHandler.UserInfo)       // 用户信息
		ucAuth.PUT("/profile", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userCenterAuthHandler.UpdateProfile)   // 更新资料
		ucAuth.PUT("/password", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userCenterAuthHandler.ChangePassword) // 修改密码
	}

	// 用户中心菜单：普通用户控制台侧边栏（platform=user）
	ucMenu := r.Group("/api/v1/uc/menus")
	ucMenu.Use(middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix))
	{
		ucMenu.GET("/tree", userMenuHandler.Tree) // 菜单树
	}

	// 兼容 frontend-user 项目 baseURL=/api/v1 时的 /auth 路径（同处理器，避免前端改动）
	ucAuthCompat := r.Group("/api/v1/auth")
	{
		ucAuthCompat.POST("/login", userCenterAuthHandler.Login)
		ucAuthCompat.POST("/register", userCenterAuthHandler.Register)
		ucAuthCompat.POST("/logout", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userCenterAuthHandler.Logout)
		ucAuthCompat.GET("/userinfo", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userCenterAuthHandler.UserInfo)
		ucAuthCompat.PUT("/profile", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userCenterAuthHandler.UpdateProfile)   // 更新资料
		ucAuthCompat.PUT("/password", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userCenterAuthHandler.ChangePassword) // 修改密码
	}

	// 兼容 frontend-user 项目 baseURL=/api/v1 时的 /menus/tree 路径（同处理器）
	ucMenuCompat := r.Group("/api/v1/menus")
	ucMenuCompat.Use(middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix))
	{
		ucMenuCompat.GET("/tree", userMenuHandler.Tree) // 菜单树
	}

	// 用户中心财务：用户自助查看余额 / 流水 / 账单并发起充值
	ucFinance := r.Group("/api/v1/uc/finance")
	{
		ucFinance.GET("/balance", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userFinanceHandler.Balance)           // 我的余额
		ucFinance.GET("/transactions", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userFinanceHandler.Transactions) // 我的资金流水
		ucFinance.POST("/recharge", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userFinanceHandler.CreateRecharge)  // 发起充值
		ucFinance.GET("/bills", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userFinanceHandler.Bills)               // 我的账单
		ucFinance.POST("/recharge/callback", userFinanceHandler.RechargeCallback)                                              // 充值回调（渠道通知）
	}

	// 用户中心工单支持：我的工单自助管理（doc50）
	ucSupport := r.Group("/api/v1/uc/support")
	{
		ucSupport.GET("/tickets", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userTicketHandler.List)                     // 我的工单列表
		ucSupport.POST("/tickets", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userTicketHandler.Create)                  // 提交工单
		ucSupport.GET("/ticket-categories", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userTicketHandler.ListCategories) // 可用工单分类
		ucSupport.GET("/tickets/:id", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userTicketHandler.Get)                  // 工单详情（含回复）
		ucSupport.POST("/tickets/:id/replies", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userTicketHandler.Reply)       // 追加工单回复
		ucSupport.POST("/tickets/:id/cancel", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userTicketHandler.Cancel)       // 取消工单
	}

	// 用户中心生命周期与续费（doc60 §7.2）
	// 聚合视图 /uc/instances/renewals 为固定路径，需先于 /instances/:id 注册
	ucLifecycle := r.Group("/api/v1/uc")
	{
		ucLifecycle.GET("/instances/renewals", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), lifecycleUserHandler.RenewalsView)       // 续费管理聚合视图
		ucLifecycle.POST("/instances/:id/renew", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), lifecycleUserHandler.Renew)            // 手动续费
		ucLifecycle.PUT("/instances/:id/auto-renew", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), lifecycleUserHandler.ToggleAutoRenew) // 自动续费开关
		ucLifecycle.GET("/renewals", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), lifecycleUserHandler.Records)                      // 我的续费记录
		ucLifecycle.GET("/renewals/:id", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), lifecycleUserHandler.Detail)                   // 续费记录详情
		}

	// 用户中心消息中心（doc70 §7.2）
	ucNotify := r.Group("/api/v1/uc")
	{
		ucNotify.GET("/notifications/unread-count", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), notifyUserHandler.UserUnreadCount)
		ucNotify.GET("/notifications", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), notifyUserHandler.UserList)
		ucNotify.GET("/notifications/:id", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), notifyUserHandler.UserDetail)
		ucNotify.POST("/notifications/read-all", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), notifyUserHandler.UserReadAll)
		ucNotify.GET("/announcements", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), notifyUserHandler.UserAnnouncements)
		ucNotify.GET("/notification-preferences", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), notifyUserHandler.UserGetPrefs)
		ucNotify.PUT("/notification-preferences", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), notifyUserHandler.UserUpdatePrefs)
	}

	return r
}
