package server

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/observability"
)

func newRouter(app *App) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery(), middleware.Logger(app.logger), cors.Default())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"status": "ok"}})
	})
	r.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ready", "data": gin.H{"status": "ready"}})
	})
	r.GET("/metrics", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/plain; version=0.0.4", []byte(observability.Snapshot()))
	})

	v1 := r.Group("/api/v1/admin")
	{
		auth := v1.Group("/auth")
		{
			auth.POST("/login", app.adminHandler.Login)
			auth.GET("/me", middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.adminHandler.Me)
			auth.GET("/admins", middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.adminHandler.List)
			auth.POST("/admins", middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.adminHandler.Create)
			auth.GET("/admins/:id", middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.adminHandler.Get)
			auth.PUT("/admins/:id", middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.adminHandler.Update)
			auth.PATCH("/admins/:id/status", middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.adminHandler.UpdateStatus)
			auth.POST("/admins/:id/reset-password", middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.adminHandler.ResetPassword)
			auth.DELETE("/admins/:id", middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.adminHandler.Delete)
		}

		users := v1.Group("/users")
		users.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			users.GET("", app.userHandler.ListUsers)
			users.POST("", app.userHandler.CreateUser)
			users.GET("/stats", app.userHandler.GetStats)
			users.GET("/region-stats", app.userHandler.GetRegionStats)
			users.GET(":id", app.userHandler.GetUser)
			users.GET(":id/detail-aggregate", app.userDetailHandler.GetAggregate)
			users.PUT(":id", app.userHandler.UpdateUser)
			users.PATCH(":id/status", app.userHandler.UpdateUserStatus)
			users.POST(":id/reset-password", app.userHandler.ResetPassword)
			users.POST(":id/roles", app.userHandler.AssignRoles)
			users.POST(":id/impersonate", app.userHandler.Impersonate)
			users.POST(":id/recharge", app.userHandler.Recharge)
			users.POST(":id/orders", app.userHandler.CreateOrder)
		}

		userGroups := v1.Group("/user-groups")
		userGroups.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			userGroups.GET("", app.userGroupHandler.List)
			userGroups.POST("", app.userGroupHandler.Create)
			userGroups.GET("/:id", app.userGroupHandler.Get)
			userGroups.PUT("/:id", app.userGroupHandler.Update)
			userGroups.DELETE("/:id", app.userGroupHandler.Delete)
		}

		agentLevels := v1.Group("/distribution/agent-levels")
		agentLevels.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			agentLevels.GET("", app.agentLevelHandler.List)
			agentLevels.POST("", app.agentLevelHandler.Create)
			agentLevels.GET("/:id", app.agentLevelHandler.Get)
			agentLevels.PUT("/:id", app.agentLevelHandler.Update)
			agentLevels.DELETE("/:id", app.agentLevelHandler.Delete)
		}

		agents := v1.Group("/distribution/agents")
		agents.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			agents.GET("", app.agentHandler.List)
			agents.POST("", app.agentHandler.Create)
			agents.GET("/:id", app.agentHandler.Get)
			agents.PUT("/:id", app.agentHandler.Update)
			agents.DELETE("/:id", app.agentHandler.Delete)
		}

		subordinates := v1.Group("/distribution/subordinates")
		subordinates.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			subordinates.GET("", app.subordinateHandler.List)
			subordinates.POST("", app.subordinateHandler.Create)
			subordinates.GET("/:id", app.subordinateHandler.Get)
			subordinates.PUT("/:id", app.subordinateHandler.Update)
			subordinates.DELETE("/:id", app.subordinateHandler.Delete)
		}

		commissions := v1.Group("/distribution/commissions")
		commissions.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			commissions.GET("", app.commissionHandler.List)
			commissions.POST("", app.commissionHandler.Create)
			commissions.GET("/:id", app.commissionHandler.Get)
			commissions.PUT("/:id", app.commissionHandler.Update)
			commissions.POST("/:id/freeze", app.commissionHandler.Freeze)
			commissions.POST("/:id/unfreeze", app.commissionHandler.Unfreeze)
			commissions.POST("/:id/cancel", app.commissionHandler.Cancel)
			commissions.DELETE("/:id", app.commissionHandler.Delete)
		}

		settlements := v1.Group("/distribution/settlements")
		settlements.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			settlements.GET("", app.settlementHandler.List)
			settlements.POST("", app.settlementHandler.Create)
			settlements.GET("/:id", app.settlementHandler.Get)
			settlements.PUT("/:id", app.settlementHandler.Update)
			settlements.POST("/:id/confirm", app.settlementHandler.Confirm)
			settlements.POST("/:id/pay", app.settlementHandler.Pay)
			settlements.POST("/:id/cancel", app.settlementHandler.Cancel)
			settlements.DELETE("/:id", app.settlementHandler.Delete)
		}

		roles := v1.Group("/roles")
		roles.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			roles.GET("", app.roleHandler.ListRoles)
			roles.POST("", app.roleHandler.CreateRole)
			roles.GET("/:id", app.roleHandler.GetRole)
			roles.PUT("/:id", app.roleHandler.UpdateRole)
			roles.DELETE("/:id", app.roleHandler.DeleteRole)
			roles.GET("/:id/permissions", app.roleHandler.GetRolePermissions)
			roles.POST("/:id/permissions", app.roleHandler.AssignPermissions)
		}

		userLevels := v1.Group("/user-levels")
		userLevels.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			userLevels.GET("", app.userLevelHandler.List)
			userLevels.POST("", app.userLevelHandler.Create)
			userLevels.GET("/:id", app.userLevelHandler.Get)
			userLevels.PUT("/:id", app.userLevelHandler.Update)
			userLevels.DELETE("/:id", app.userLevelHandler.Delete)
		}

		verifications := v1.Group("/verifications")
		verifications.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			verifications.GET("/pending", app.verificationHandler.ListPending)
			verifications.GET("/approved", app.verificationHandler.ListApproved)
			verifications.GET("/rejected", app.verificationHandler.ListRejected)
		}

		permissions := v1.Group("/permissions")
		permissions.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			permissions.GET("/tree", app.permissionHandler.Tree)
			permissions.POST("", app.permissionHandler.CreatePermission)
			permissions.PUT("/:id", app.permissionHandler.UpdatePermission)
			permissions.DELETE("/:id", app.permissionHandler.DeletePermission)
		}

		menus := v1.Group("/menus")
		menus.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			menus.GET("/tree", app.menuHandler.Tree)
			menus.POST("", app.menuHandler.CreateMenu)
			menus.PUT("/:id", app.menuHandler.UpdateMenu)
			menus.DELETE("/:id", app.menuHandler.DeleteMenu)
		}

		security := v1.Group("/security")
		security.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			security.GET("/login-logs/export", app.securityHandler.ExportLoginLogs)
			security.GET("/login-logs", app.securityHandler.ListLoginLogs)
			security.GET("/login-logs/:id", app.securityHandler.GetLoginLog)
			security.GET("/audit-logs", app.securityHandler.ListAuditLogs)
			security.GET("/audit-logs/:id", app.securityHandler.GetAuditLog)
			security.GET("/audit-logs/export", app.securityHandler.ExportAuditLogs)
			security.GET("/risk-events", app.securityHandler.ListRiskEvents)
			security.GET("/risk-events/:id", app.securityHandler.GetRiskEvent)
			security.POST("/risk-events/:id/ignore", app.securityHandler.IgnoreRiskEvent)
			security.POST("/risk-events/:id/handle", app.securityHandler.HandleRiskEvent)
			security.POST("/risk-events/:id/blacklist", app.securityHandler.CreateBlacklistFromRisk)
			security.POST("/risk-events/:id/revoke-sessions", app.securityHandler.RevokeSessionsFromRisk)
			security.GET("/blacklists", app.securityHandler.ListBlacklists)
			security.POST("/blacklists", app.securityHandler.CreateBlacklist)
			security.GET("/blacklists/:id", app.securityHandler.GetBlacklist)
			security.PUT("/blacklists/:id", app.securityHandler.UpdateBlacklist)
			security.PATCH("/blacklists/:id/status", app.securityHandler.UpdateBlacklistStatus)
			security.DELETE("/blacklists/:id", app.securityHandler.ReleaseBlacklist)
			security.GET("/blacklists/:id/hits", app.securityHandler.ListBlacklistHits)
			security.GET("/sessions", app.securityHandler.ListSessions)
			security.GET("/sessions/:id", app.securityHandler.GetSession)
			security.POST("/sessions/:id/revoke", app.securityHandler.RevokeSession)
			security.POST("/sessions/batch-revoke", app.securityHandler.BatchRevokeSessions)
			security.POST("/sessions/revoke-user-all", app.securityHandler.RevokeUserAllSessions)
		}

		// 资源管理（对接上游）— 第一阶段
		resourceGroup := v1.Group("/resource")
		resourceGroup.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			// 上游提供商
			providers := resourceGroup.Group("/providers")
			{
				providers.GET("", app.providerHandler.List)
				providers.POST("", app.providerHandler.Create)
				providers.GET("/types", app.providerHandler.ListTypes)
				providers.GET("/:id", app.providerHandler.Get)
				providers.PUT("/:id", app.providerHandler.Update)
				providers.DELETE("/:id", app.providerHandler.Delete)
				providers.POST("/:id/test", app.providerHandler.TestConnection)
			}

			// 资源池
			pools := resourceGroup.Group("/pools")
			{
				pools.GET("", app.providerHandler.ListPools)
				pools.GET("/:id", app.providerHandler.GetPool)
			}

			// 上游商品
			products := resourceGroup.Group("/products")
			{
				products.GET("", app.productHandler.List)
				products.GET("/:id", app.productHandler.Get)
				products.PUT("/:id/price", app.productHandler.UpdatePrice)
				products.POST("/sync", app.productHandler.Sync)
			}

			// 同步管理
			sync := resourceGroup.Group("/sync")
			{
				sync.POST("", app.syncHandler.CreateTask)
				sync.GET("/tasks", app.syncHandler.ListTasks)
				sync.GET("/tasks/:id", app.syncHandler.GetTask)
				sync.GET("/logs", app.syncHandler.ListLogs)
			}

			// 实例管理
			instances := resourceGroup.Group("/instances")
			{
				instances.GET("", app.syncHandler.ListInstances)
				instances.GET("/:id", app.syncHandler.GetInstance)
			}
		}

		// 生命周期管理（doc60 §7.1）
		// 到期实例 + 代续费：/admin/instances/expiring、/admin/instances/:id/renew
		// 注意：/expiring 固定路径需先于 /:id 注册，避免路由冲突
		lcInstances := v1.Group("/instances")
		lcInstances.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			lcInstances.GET("/expiring", app.expiringHandler.List)
			lcInstances.POST("/:id/renew", app.lifecycleAdminHandler.RenewAdmin)
		}

		// 续费记录：/admin/renewals
		lcRenewals := v1.Group("/renewals")
		lcRenewals.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			lcRenewals.GET("", app.lifecycleAdminHandler.ListRenewals)
			lcRenewals.GET("/:id", app.lifecycleAdminHandler.GetRenewal)
		}

		// 策略与手动扫描：/admin/lifecycle/policy、/admin/lifecycle/scan
		lifecycleGroup := v1.Group("/lifecycle")
		lifecycleGroup.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			lifecycleGroup.GET("/policy", app.lifecycleAdminHandler.GetPolicy)
			lifecycleGroup.PUT("/policy", app.lifecycleAdminHandler.UpdatePolicy)
			lifecycleGroup.POST("/scan", app.lifecycleAdminHandler.ScanOnce)
		}

		// 产品管理（面向终端售卖）
		productGroup := v1.Group("/product")
		productGroup.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			// 产品分类
			categories := productGroup.Group("/categories")
			{
				categories.GET("", app.prodCategoryHandler.List)
				categories.POST("", app.prodCategoryHandler.Create)
				categories.GET("/:id", app.prodCategoryHandler.Get)
				categories.PUT("/:id", app.prodCategoryHandler.Update)
				categories.DELETE("/:id", app.prodCategoryHandler.Delete)
			}

			// 产品（商品管理 catalog 子域）
			prodProducts := productGroup.Group("/products")
			{
				prodProducts.GET("", app.prodCatalogHandler.List)
				prodProducts.POST("", app.prodCatalogHandler.Create)
				// 从上游商品克隆创建销售商品（对接魔方财务商品导入）。
				// 注意：静态路由需在 /:id 之前注册，避免被参数路由吞并。
				prodProducts.POST("/clone", app.prodCatalogHandler.CloneFromUpstream)
				prodProducts.POST("/clone/batch", app.prodCatalogHandler.BatchCloneFromUpstream)
				prodProducts.GET("/:id", app.prodCatalogHandler.Get)
				prodProducts.PUT("/:id", app.prodCatalogHandler.Update)
				prodProducts.DELETE("/:id", app.prodCatalogHandler.Delete)
				prodProducts.POST("/:id/publish", app.prodCatalogHandler.Publish)
				prodProducts.POST("/:id/unpublish", app.prodCatalogHandler.Unpublish)
				prodProducts.PUT("/:id/price", app.prodCatalogHandler.UpdatePrice)
				prodProducts.POST("/:id/featured", app.prodCatalogHandler.SetFeatured)
				prodProducts.GET("/:id/history", app.prodCatalogHandler.ListHistory)
				prodProducts.GET("/:id/specs", app.prodCatalogHandler.ListSpecs)
			}

			// 规格管理（spec 子域）
			specGroup := productGroup.Group("/spec")
			{
				specTemplates := specGroup.Group("/templates")
				{
					specTemplates.GET("", app.specHandler.ListTemplates)
					specTemplates.POST("", app.specHandler.CreateTemplate)
					specTemplates.GET("/:id", app.specHandler.GetTemplate)
					specTemplates.PUT("/:id", app.specHandler.UpdateTemplate)
					specTemplates.DELETE("/:id", app.specHandler.DeleteTemplate)
				}
				specMappings := specGroup.Group("/mappings")
				{
					specMappings.GET("", app.specHandler.ListMappings)
					specMappings.POST("", app.specHandler.CreateMapping)
					specMappings.GET("/:id", app.specHandler.GetMapping)
					specMappings.PUT("/:id", app.specHandler.UpdateMapping)
					specMappings.POST("/:id/bind", app.specHandler.BindMapping)
					specMappings.DELETE("/:id", app.specHandler.DeleteMapping)
				}
			}

			// 定价与计费（pricing 子域）
			pricingGroup := productGroup.Group("/pricing")
			{
				pricingGroup.GET("", app.pricingHandler.List)
				pricingGroup.POST("", app.pricingHandler.Create)
				pricingGroup.GET("/:id", app.pricingHandler.Get)
				pricingGroup.PUT("/:id", app.pricingHandler.Update)
				pricingGroup.DELETE("/:id", app.pricingHandler.Delete)
			}

			// 促销管理（promotion 子域）
			promoGroup := productGroup.Group("/promotion")
			{
				coupons := promoGroup.Group("/coupons")
				{
					coupons.GET("", app.promotionHandler.ListCoupons)
					coupons.POST("", app.promotionHandler.CreateCoupon)
					coupons.GET("/:id", app.promotionHandler.GetCoupon)
					coupons.PUT("/:id", app.promotionHandler.UpdateCoupon)
					coupons.DELETE("/:id", app.promotionHandler.DeleteCoupon)
				}
				couponGrants := promoGroup.Group("/coupon-grants")
				{
					couponGrants.GET("", app.promotionHandler.ListCouponGrants)
					couponGrants.POST("", app.promotionHandler.CreateCouponGrants)
				}
				promotions := promoGroup.Group("/promotions")
				{
					promotions.GET("", app.promotionHandler.ListPromotions)
					promotions.POST("", app.promotionHandler.CreatePromotion)
					promotions.GET("/:id", app.promotionHandler.GetPromotion)
					promotions.PUT("/:id", app.promotionHandler.UpdatePromotion)
					promotions.DELETE("/:id", app.promotionHandler.DeletePromotion)
				}
			}
		}

		// 订单管理
		orderGroup := v1.Group("/orders")
		orderGroup.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			orderGroup.GET("", app.orderHandler.List)
			orderGroup.GET("/stats", app.orderHandler.Stats)
			orderGroup.GET("/:id", app.orderHandler.Get)
			orderGroup.POST("/:id/cancel", app.orderHandler.Cancel)
			orderGroup.PUT("/:id/remark", app.orderHandler.UpdateRemark)
			orderGroup.POST("/:id/refund", app.orderHandler.CreateRefund)
			orderGroup.POST("/:id/activate", app.orderHandler.Activate)
		}

		// 工单支持（doc50）
		ticketGroup := v1.Group("/tickets")
		ticketGroup.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			ticketGroup.GET("", app.ticketHandler.List)
			ticketGroup.GET("/stats", app.ticketHandler.Stats)
			ticketGroup.GET("/:id", app.ticketHandler.Get)
			ticketGroup.POST("/:id/reply", app.ticketHandler.Reply)
			ticketGroup.PUT("/:id/assign", app.ticketHandler.Assign)
			ticketGroup.PUT("/:id/status", app.ticketHandler.UpdateStatus)
			ticketGroup.POST("/:id/close", app.ticketHandler.Close)
		}

		// 工单分类（doc50）
		ticketCategoryGroup := v1.Group("/ticket-categories")
		ticketCategoryGroup.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			ticketCategoryGroup.GET("", app.ticketCategoryHandler.List)
			ticketCategoryGroup.POST("", app.ticketCategoryHandler.Create)
			ticketCategoryGroup.PUT("/:id", app.ticketCategoryHandler.Update)
			ticketCategoryGroup.DELETE("/:id", app.ticketCategoryHandler.Delete)
		}

		// 退款管理
		refundGroup := v1.Group("/refunds")
		refundGroup.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			refundGroup.GET("", app.refundHandler.List)
			refundGroup.GET("/:id", app.refundHandler.Get)
			refundGroup.POST("/:id/approve", app.refundHandler.Approve)
			refundGroup.POST("/:id/reject", app.refundHandler.Reject)
		}

		// 财务管理
		financeGroup := v1.Group("/finance")
		financeGroup.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			// 钱包/流水
			financeGroup.GET("/wallets/:user_id", app.walletHandler.Balance)
			financeGroup.GET("/transactions", app.walletHandler.ListTransactions)
			financeGroup.POST("/transactions/adjust", app.walletHandler.Adjust)
			// 充值
			financeGroup.POST("/recharges", app.rechargeHandler.Create)
			financeGroup.GET("/recharges", app.rechargeHandler.List)
			financeGroup.POST("/recharges/:id/approve", app.rechargeHandler.Approve)
			// 提现
			financeGroup.GET("/withdrawals", app.withdrawHandler.List)
			financeGroup.POST("/withdrawals/:id/approve", app.withdrawHandler.Approve)
			financeGroup.POST("/withdrawals/:id/reject", app.withdrawHandler.Reject)
			// 账单/对账
			financeGroup.GET("/bills", app.billHandler.List)
			financeGroup.POST("/bills/:id/close", app.billHandler.Close)
			financeGroup.POST("/bills/recon", app.reconHandler.Reconcile)
		}

		// 系统管理（系统配置）
		systemConfigGroup := v1.Group("/system/configs")
		systemConfigGroup.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			systemConfigGroup.GET("", app.configHandler.List)
			systemConfigGroup.POST("", app.configHandler.Create)
			// 按分组读取（固定段 /group/:group 需先于 /:key 匹配，且二者不同段数不冲突）
			systemConfigGroup.GET("/group/:group", app.configHandler.ListByGroup)
			// 分组批量 upsert
			systemConfigGroup.POST("/batch", app.configHandler.BatchUpsert)
			systemConfigGroup.GET("/:key", app.configHandler.GetByKey)
			systemConfigGroup.PUT("/:id", app.configHandler.Update)
			systemConfigGroup.DELETE("/:id", app.configHandler.Delete)
		}

		// 消息中心 - 公告管理（doc70 §7.1）
		annGroup := v1.Group("/announcements")
		annGroup.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			annGroup.GET("", app.notifyAdminHandler.ListAnnouncements)
			annGroup.POST("", app.notifyAdminHandler.CreateAnnouncement)
			annGroup.PUT("/:id", app.notifyAdminHandler.UpdateAnnouncement)
			annGroup.POST("/:id/publish", app.notifyAdminHandler.PublishAnnouncement)
			annGroup.POST("/:id/offline", app.notifyAdminHandler.OfflineAnnouncement)
			annGroup.DELETE("/:id", app.notifyAdminHandler.DeleteAnnouncement)
		}

		// 消息中心 - 通知记录（doc70 §7.1）
		// 注意：/unread-count 和 /mail-test 固定路径需先于 /:id/resend 注册
		notifyGroup := v1.Group("/notifications")
		notifyGroup.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			notifyGroup.GET("", app.notifyAdminHandler.ListRecords)
			notifyGroup.GET("/unread-count", app.notifyAdminHandler.AdminUnreadCount)
			notifyGroup.POST("/mail-test", app.notifyAdminHandler.SendMailTest)
			notifyGroup.POST("/:id/resend", app.notifyAdminHandler.Resend)
		}

		// 消息中心 - 通知模板（doc70 §7.1）
		tplGroup := v1.Group("/notification-templates")
		tplGroup.Use(middleware.AdminAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
		{
			tplGroup.GET("", app.notifyAdminHandler.ListTemplates)
			tplGroup.PUT("/:id", app.notifyAdminHandler.UpdateTemplate)
		}
	}

	// 用户中心（普通用户自助）：独立模块 internal/modules/uc/auth
	ucAuth := r.Group("/api/v1/uc/auth")
	{
		ucAuth.POST("/login", app.userCenterAuthHandler.Login)                                                                           // 登录
		ucAuth.POST("/register", app.userCenterAuthHandler.Register)                                                                     // 注册
		ucAuth.POST("/logout", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userCenterAuthHandler.Logout)          // 登出
		ucAuth.GET("/userinfo", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userCenterAuthHandler.UserInfo)       // 用户信息
		ucAuth.PUT("/profile", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userCenterAuthHandler.UpdateProfile)   // 更新资料
		ucAuth.PUT("/password", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userCenterAuthHandler.ChangePassword) // 修改密码
	}

	// 用户中心菜单：普通用户控制台侧边栏（platform=user）
	ucMenu := r.Group("/api/v1/uc/menus")
	ucMenu.Use(middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
	{
		ucMenu.GET("/tree", app.userMenuHandler.Tree) // 菜单树
	}

	// 兼容 frontend-user 项目 baseURL=/api/v1 时的 /auth 路径（同处理器，避免前端改动）
	ucAuthCompat := r.Group("/api/v1/auth")
	{
		ucAuthCompat.POST("/login", app.userCenterAuthHandler.Login)
		ucAuthCompat.POST("/register", app.userCenterAuthHandler.Register)
		ucAuthCompat.POST("/logout", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userCenterAuthHandler.Logout)
		ucAuthCompat.GET("/userinfo", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userCenterAuthHandler.UserInfo)
		ucAuthCompat.PUT("/profile", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userCenterAuthHandler.UpdateProfile)   // 更新资料
		ucAuthCompat.PUT("/password", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userCenterAuthHandler.ChangePassword) // 修改密码
	}

	// 兼容 frontend-user 项目 baseURL=/api/v1 时的 /menus/tree 路径（同处理器）
	ucMenuCompat := r.Group("/api/v1/menus")
	ucMenuCompat.Use(middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
	{
		ucMenuCompat.GET("/tree", app.userMenuHandler.Tree) // 菜单树
	}

	// 用户中心财务：用户自助查看余额 / 流水 / 账单并发起充值
	ucFinance := r.Group("/api/v1/uc/finance")
	{
		ucFinance.GET("/balance", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userFinanceHandler.Balance)           // 我的余额
		ucFinance.GET("/transactions", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userFinanceHandler.Transactions) // 我的资金流水
		ucFinance.POST("/recharge", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userFinanceHandler.CreateRecharge)  // 发起充值
		ucFinance.GET("/bills", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userFinanceHandler.Bills)               // 我的账单
		ucFinance.POST("/recharge/callback", app.userFinanceHandler.RechargeCallback)                                                      // 充值回调（渠道通知）
	}

	// 用户中心商品：上架商品公开可浏览（无需登录）
	ucProducts := r.Group("/api/v1/uc/products")
	{
		ucProducts.GET("", app.ucProductHandler.List)
		ucProducts.GET("/:id", app.ucProductHandler.Get)
	}

	// 官网门户公开只读接口（无需登录，字段已脱敏，响应声明可共享缓存）
	publicSite := r.Group("/api/v1/public")
	{
		publicSite.GET("/announcements", app.ucSiteHandler.Announcements) // 已发布公告
	}

	// 用户中心订单：下单（余额支付开通）+ 我的订单（需登录）
	ucOrders := r.Group("/api/v1/uc/orders")
	ucOrders.Use(middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
	{
		ucOrders.POST("", app.ucOrderHandler.Create)
		ucOrders.GET("", app.ucOrderHandler.List)
	}

	// 用户中心主机管理：列表/详情/电源操作/VNC（需登录）
	ucInstances := r.Group("/api/v1/uc/instances")
	ucInstances.Use(middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix))
	{
		ucInstances.GET("", app.ucInstanceHandler.List)             // 我的主机列表
		ucInstances.GET("/:id", app.ucInstanceHandler.Detail)       // 主机详情
		ucInstances.POST("/:id/power", app.ucInstanceHandler.Power) // 电源操作 on/off/reboot/hard_off/hard_reboot
		ucInstances.POST("/:id/vnc", app.ucInstanceHandler.VNC)     // 远程控制台
	}

	// 用户中心工单支持：我的工单自助管理（doc50）
	ucSupport := r.Group("/api/v1/uc/support")
	{
		ucSupport.GET("/tickets", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userTicketHandler.List)                     // 我的工单列表
		ucSupport.POST("/tickets", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userTicketHandler.Create)                  // 提交工单
		ucSupport.GET("/ticket-categories", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userTicketHandler.ListCategories) // 可用工单分类
		ucSupport.GET("/tickets/:id", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userTicketHandler.Get)                  // 工单详情（含回复）
		ucSupport.POST("/tickets/:id/replies", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userTicketHandler.Reply)       // 追加工单回复
		ucSupport.POST("/tickets/:id/cancel", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userTicketHandler.Cancel)       // 取消工单
	}

	// 用户中心生命周期与续费（doc60 §7.2）
	// 聚合视图 /uc/instances/renewals 为固定路径，需先于 /instances/:id 注册
	ucLifecycle := r.Group("/api/v1/uc")
	{
		ucLifecycle.GET("/instances/renewals", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.lifecycleUserHandler.RenewalsView)          // 续费管理聚合视图
		ucLifecycle.POST("/instances/:id/renew", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.lifecycleUserHandler.Renew)               // 手动续费
		ucLifecycle.PUT("/instances/:id/auto-renew", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.lifecycleUserHandler.ToggleAutoRenew) // 自动续费开关
		ucLifecycle.GET("/renewals", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.lifecycleUserHandler.Records)                         // 我的续费记录
		ucLifecycle.GET("/renewals/:id", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.lifecycleUserHandler.Detail)                      // 续费记录详情
	}

	// 用户中心消息中心（doc70 §7.2）
	ucNotify := r.Group("/api/v1/uc")
	{
		ucNotify.GET("/notifications/unread-count", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.notifyUserHandler.UserUnreadCount)
		ucNotify.GET("/notifications", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.notifyUserHandler.UserList)
		ucNotify.GET("/notifications/:id", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.notifyUserHandler.UserDetail)
		ucNotify.POST("/notifications/read-all", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.notifyUserHandler.UserReadAll)
		ucNotify.GET("/announcements", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.notifyUserHandler.UserAnnouncements)
		ucNotify.GET("/notification-preferences", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.notifyUserHandler.UserGetPrefs)
		ucNotify.PUT("/notification-preferences", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.notifyUserHandler.UserUpdatePrefs)
	}

	return r
}
