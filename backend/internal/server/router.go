package server

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	appauth "hostsent/backend/internal/pkg/auth"
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
	// 管理端写操作审计（P2-06）：仅记录 POST/PUT/PATCH/DELETE，登录等白名单路径自动跳过。
	v1.Use(app.adminAudit())
	{
		auth := v1.Group("/auth")
		{
			auth.POST("/login", app.adminHandler.Login)
			auth.GET("/me", app.adminAuth(), app.adminHandler.Me)
			auth.POST("/change-password", app.adminAuth(), app.adminHandler.ChangePassword)
			// 员工管理（超管独占，81 §4.1）。保留 /auth/admins 旧路径做别名，前端逐步切到 /staff。
			auth.GET("/admins", app.adminAuth(), app.superOnly(), app.adminHandler.List)
			auth.POST("/admins", app.adminAuth(), app.superOnly(), app.adminHandler.Create)
			auth.GET("/admins/:id", app.adminAuth(), app.superOnly(), app.adminHandler.Get)
			auth.PUT("/admins/:id", app.adminAuth(), app.superOnly(), app.adminHandler.Update)
			auth.PUT("/admins/:id/roles", app.adminAuth(), app.superOnly(), app.adminHandler.SetRoles)
			auth.PATCH("/admins/:id/status", app.adminAuth(), app.superOnly(), app.adminHandler.UpdateStatus)
			auth.POST("/admins/:id/reset-password", app.adminAuth(), app.superOnly(), app.adminHandler.ResetPassword)
			auth.DELETE("/admins/:id", app.adminAuth(), app.superOnly(), app.adminHandler.Delete)
		}

		// 管理端操作审计查询（P2-06）：新表 admin_audit_logs，与用户安全审计区分
		auditGroup := v1.Group("/audit-logs")
		auditGroup.Use(app.adminAuth())
		{
			auditGroup.GET("", app.perm("security:audit:list"), app.adminHandler.ListAuditLogs)
		}

		// 员工管理（P2-01 新路径，超管独占）
		staff := v1.Group("/staff")
		staff.Use(app.adminAuth(), app.superOnly())
		{
			staff.GET("", app.adminHandler.List)
			staff.POST("", app.adminHandler.Create)
			staff.GET("/:id", app.adminHandler.Get)
			staff.PUT("/:id", app.adminHandler.Update)
			staff.PUT("/:id/roles", app.adminHandler.SetRoles)
			staff.PATCH("/:id/status", app.adminHandler.UpdateStatus)
			staff.POST("/:id/reset-password", app.adminHandler.ResetPassword)
			staff.DELETE("/:id", app.adminHandler.Delete)
		}

		users := v1.Group("/users")
		users.Use(app.adminAuth())
		{
			users.GET("", app.perm("system:user:list"), app.userHandler.ListUsers)
			users.POST("", app.perm("user:create"), app.userHandler.CreateUser)
			users.GET("/stats", app.perm("system:user:list"), app.userHandler.GetStats)
			users.GET("/region-stats", app.perm("system:user:list"), app.userHandler.GetRegionStats)
			users.GET(":id", app.perm("user:detail"), app.userHandler.GetUser)
			users.GET(":id/members", app.perm("user:detail"), app.userHandler.ListMembers)
			users.GET(":id/detail-aggregate", app.perm("user:detail"), app.userDetailHandler.GetAggregate)
			users.PUT(":id", app.perm("user:update"), app.userHandler.UpdateUser)
			users.PATCH(":id/status", app.perm("user:update_status"), app.userHandler.UpdateUserStatus)
			users.POST(":id/reset-password", app.perm("user:reset_password"), app.userHandler.ResetPassword)
			users.POST(":id/roles", app.perm("user:assign_role"), app.userHandler.AssignRoles)
			// 代登录属高危接口，仅超管
			users.POST(":id/impersonate", app.superOnly(), app.userHandler.Impersonate)
			users.POST(":id/recharge", app.perm("finance:adjust"), app.userHandler.Recharge)
			users.POST(":id/orders", app.perm("user:create"), app.userHandler.CreateOrder)
		}

		userGroups := v1.Group("/user-groups")
		userGroups.Use(app.adminAuth())
		{
			userGroups.GET("", app.perm("user:group:list"), app.userGroupHandler.List)
			userGroups.POST("", app.perm("user:group:create"), app.userGroupHandler.Create)
			userGroups.GET("/:id", app.perm("user:group:list"), app.userGroupHandler.Get)
			userGroups.PUT("/:id", app.perm("user:group:update"), app.userGroupHandler.Update)
			userGroups.DELETE("/:id", app.perm("user:group:delete"), app.userGroupHandler.Delete)
		}

		roles := v1.Group("/roles")
		roles.Use(app.adminAuth())
		{
			roles.GET("", app.perm("system:role:list"), app.roleHandler.ListRoles)
			roles.POST("", app.perm("role:create"), app.roleHandler.CreateRole)
			roles.GET("/:id", app.perm("system:role:list"), app.roleHandler.GetRole)
			roles.PUT("/:id", app.perm("role:update"), app.roleHandler.UpdateRole)
			roles.DELETE("/:id", app.perm("role:delete"), app.roleHandler.DeleteRole)
			roles.GET("/:id/permissions", app.perm("system:role:list"), app.roleHandler.GetRolePermissions)
			// 角色权限分配属高危接口，仅超管
			roles.POST("/:id/permissions", app.superOnly(), app.roleHandler.AssignPermissions)
		}

		userLevels := v1.Group("/user-levels")
		userLevels.Use(app.adminAuth())
		{
			userLevels.GET("", app.perm("level:list"), app.userLevelHandler.List)
			userLevels.POST("", app.perm("level:create"), app.userLevelHandler.Create)
			userLevels.GET("/:id", app.perm("level:list"), app.userLevelHandler.Get)
			userLevels.PUT("/:id", app.perm("level:update"), app.userLevelHandler.Update)
			userLevels.DELETE("/:id", app.perm("level:delete"), app.userLevelHandler.Delete)
		}

		verifications := v1.Group("/verifications")
		verifications.Use(app.adminAuth())
		{
			verifications.GET("/pending", app.perm("verification:list"), app.verificationHandler.ListPending)
			verifications.GET("/approved", app.perm("verification:list"), app.verificationHandler.ListApproved)
			verifications.GET("/rejected", app.perm("verification:list"), app.verificationHandler.ListRejected)
		}

		permissions := v1.Group("/permissions")
		permissions.Use(app.adminAuth())
		{
			permissions.GET("/tree", app.perm("system:permission:view"), app.permissionHandler.Tree)
			permissions.POST("", app.perm("permission:create"), app.permissionHandler.CreatePermission)
			permissions.PUT("/:id", app.perm("permission:update"), app.permissionHandler.UpdatePermission)
			permissions.DELETE("/:id", app.perm("permission:delete"), app.permissionHandler.DeletePermission)
		}

		menus := v1.Group("/menus")
		menus.Use(app.adminAuth())
		{
			// /tree 是侧边栏的数据源，服务端已按当前管理员权限过滤（FilterByPermissions），
			// 因此只需登录态即可访问；若在此再要求 system:menu，未持有该码的管理员会吃到
			// 403 并让前端回退到「不过滤」的静态菜单，等于绕过权限控制。
			menus.GET("/tree", app.menuHandler.Tree)
			menus.POST("", app.perm("menu:create"), app.menuHandler.CreateMenu)
			menus.PUT("/:id", app.perm("menu:update"), app.menuHandler.UpdateMenu)
			menus.DELETE("/:id", app.perm("menu:delete"), app.menuHandler.DeleteMenu)
		}

		security := v1.Group("/security")
		security.Use(app.adminAuth())
		{
			security.GET("/login-logs/export", app.perm("security:login-log:list"), app.securityHandler.ExportLoginLogs)
			security.GET("/login-logs", app.perm("security:login-log:list"), app.securityHandler.ListLoginLogs)
			security.GET("/login-logs/:id", app.perm("security:login-log:list"), app.securityHandler.GetLoginLog)
			security.GET("/audit-logs", app.perm("security:audit:list"), app.securityHandler.ListAuditLogs)
			security.GET("/audit-logs/:id", app.perm("security:audit:list"), app.securityHandler.GetAuditLog)
			security.GET("/audit-logs/export", app.perm("security:audit:list"), app.securityHandler.ExportAuditLogs)
			security.GET("/risk-events", app.perm("security:risk:list"), app.securityHandler.ListRiskEvents)
			security.GET("/risk-events/:id", app.perm("security:risk:list"), app.securityHandler.GetRiskEvent)
			security.POST("/risk-events/:id/ignore", app.perm("security:risk:list"), app.securityHandler.IgnoreRiskEvent)
			security.POST("/risk-events/:id/handle", app.perm("security:risk:list"), app.securityHandler.HandleRiskEvent)
			security.POST("/risk-events/:id/blacklist", app.perm("security:blacklist:manage"), app.securityHandler.CreateBlacklistFromRisk)
			security.POST("/risk-events/:id/revoke-sessions", app.perm("security:session:manage"), app.securityHandler.RevokeSessionsFromRisk)
			security.GET("/blacklists", app.perm("security:blacklist:manage"), app.securityHandler.ListBlacklists)
			security.POST("/blacklists", app.perm("security:blacklist:manage"), app.securityHandler.CreateBlacklist)
			security.GET("/blacklists/:id", app.perm("security:blacklist:manage"), app.securityHandler.GetBlacklist)
			security.PUT("/blacklists/:id", app.perm("security:blacklist:manage"), app.securityHandler.UpdateBlacklist)
			security.PATCH("/blacklists/:id/status", app.perm("security:blacklist:manage"), app.securityHandler.UpdateBlacklistStatus)
			security.DELETE("/blacklists/:id", app.perm("security:blacklist:manage"), app.securityHandler.ReleaseBlacklist)
			security.GET("/blacklists/:id/hits", app.perm("security:blacklist:manage"), app.securityHandler.ListBlacklistHits)
			security.GET("/sessions", app.perm("security:session:manage"), app.securityHandler.ListSessions)
			security.GET("/sessions/:id", app.perm("security:session:manage"), app.securityHandler.GetSession)
			security.POST("/sessions/:id/revoke", app.perm("security:session:manage"), app.securityHandler.RevokeSession)
			security.POST("/sessions/batch-revoke", app.perm("security:session:manage"), app.securityHandler.BatchRevokeSessions)
			security.POST("/sessions/revoke-user-all", app.perm("security:session:manage"), app.securityHandler.RevokeUserAllSessions)
		}

		// 资源管理（对接上游）— 第一阶段
		resourceGroup := v1.Group("/resource")
		resourceGroup.Use(app.adminAuth())
		{
			// 上游提供商
			providers := resourceGroup.Group("/providers")
			{
				providers.GET("", app.perm("resource:provider"), app.providerHandler.List)
				providers.POST("", app.perm("provider:create"), app.providerHandler.Create)
				providers.GET("/types", app.perm("resource:provider"), app.providerHandler.ListTypes)
				providers.GET("/:id", app.perm("resource:provider"), app.providerHandler.Get)
				providers.PUT("/:id", app.perm("provider:update"), app.providerHandler.Update)
				providers.DELETE("/:id", app.perm("provider:delete"), app.providerHandler.Delete)
				providers.POST("/:id/test", app.perm("provider:test"), app.providerHandler.TestConnection)
			}

			// 资源池
			pools := resourceGroup.Group("/pools")
			{
				pools.GET("", app.perm("resource:provider"), app.providerHandler.ListPools)
				pools.GET("/:id", app.perm("resource:provider"), app.providerHandler.GetPool)
			}

			// 上游商品
			products := resourceGroup.Group("/products")
			{
				products.GET("", app.perm("resource:product"), app.productHandler.List)
				products.GET("/:id", app.perm("resource:product"), app.productHandler.Get)
				products.PUT("/:id/price", app.perm("product:update_price"), app.productHandler.UpdatePrice)
				products.POST("/sync", app.perm("product:sync"), app.productHandler.Sync)
			}

			// 同步管理
			sync := resourceGroup.Group("/sync")
			{
				sync.POST("", app.perm("sync:create"), app.syncHandler.CreateTask)
				sync.GET("/tasks", app.perm("resource:sync"), app.syncHandler.ListTasks)
				sync.GET("/tasks/:id", app.perm("resource:sync"), app.syncHandler.GetTask)
				sync.GET("/logs", app.perm("sync:log"), app.syncHandler.ListLogs)
			}

			// 实例管理
			instances := resourceGroup.Group("/instances")
			{
				instances.GET("", app.perm("resource:instance"), app.syncHandler.ListInstances)
				instances.GET("/:id", app.perm("resource:instance"), app.syncHandler.GetInstance)
			}
		}

		// 生命周期管理（doc60 §7.1）
		// 到期实例 + 代续费：/admin/instances/expiring、/admin/instances/:id/renew
		// 注意：/expiring 固定路径需先于 /:id 注册，避免路由冲突
		lcInstances := v1.Group("/instances")
		lcInstances.Use(app.adminAuth())
		{
			lcInstances.GET("/expiring", app.perm("lifecycle:expiring"), app.expiringHandler.List)
			lcInstances.POST("/:id/renew", app.perm("lifecycle:renew"), app.lifecycleAdminHandler.RenewAdmin)
		}

		// 续费记录：/admin/renewals
		lcRenewals := v1.Group("/renewals")
		lcRenewals.Use(app.adminAuth())
		{
			lcRenewals.GET("", app.perm("lifecycle:renewals"), app.lifecycleAdminHandler.ListRenewals)
			lcRenewals.GET("/:id", app.perm("lifecycle:renewals"), app.lifecycleAdminHandler.GetRenewal)
		}

		// 策略与手动扫描：/admin/lifecycle/policy、/admin/lifecycle/scan
		lifecycleGroup := v1.Group("/lifecycle")
		lifecycleGroup.Use(app.adminAuth())
		{
			lifecycleGroup.GET("/policy", app.perm("lifecycle:policy"), app.lifecycleAdminHandler.GetPolicy)
			lifecycleGroup.PUT("/policy", app.perm("lifecycle:policy:update"), app.lifecycleAdminHandler.UpdatePolicy)
			lifecycleGroup.POST("/scan", app.perm("lifecycle:policy:update"), app.lifecycleAdminHandler.ScanOnce)
		}

		// 产品管理（面向终端售卖）
		productGroup := v1.Group("/product")
		productGroup.Use(app.adminAuth())
		{
			// 产品分类
			categories := productGroup.Group("/categories")
			{
				categories.GET("", app.perm("product:category"), app.prodCategoryHandler.List)
				categories.POST("", app.perm("product:category:create"), app.prodCategoryHandler.Create)
				categories.GET("/:id", app.perm("product:category"), app.prodCategoryHandler.Get)
				categories.PUT("/:id", app.perm("product:category:update"), app.prodCategoryHandler.Update)
				categories.DELETE("/:id", app.perm("product:category:delete"), app.prodCategoryHandler.Delete)
			}

			// 产品（商品管理 catalog 子域）
			prodProducts := productGroup.Group("/products")
			{
				prodProducts.GET("", app.perm("product:list"), app.prodCatalogHandler.List)
				prodProducts.POST("", app.perm("product:create"), app.prodCatalogHandler.Create)
				// 从上游商品克隆创建销售商品（对接魔方财务商品导入）。
				// 注意：静态路由需在 /:id 之前注册，避免被参数路由吞并。
				prodProducts.POST("/clone", app.perm("product:create"), app.prodCatalogHandler.CloneFromUpstream)
				prodProducts.POST("/clone/batch", app.perm("product:create"), app.prodCatalogHandler.BatchCloneFromUpstream)
				prodProducts.GET("/:id", app.perm("product:list"), app.prodCatalogHandler.Get)
				prodProducts.PUT("/:id", app.perm("product:update"), app.prodCatalogHandler.Update)
				prodProducts.DELETE("/:id", app.perm("product:delete"), app.prodCatalogHandler.Delete)
				prodProducts.POST("/:id/publish", app.perm("product:publish"), app.prodCatalogHandler.Publish)
				prodProducts.POST("/:id/unpublish", app.perm("product:publish"), app.prodCatalogHandler.Unpublish)
				prodProducts.PUT("/:id/price", app.perm("product:price:update"), app.prodCatalogHandler.UpdatePrice)
				prodProducts.POST("/:id/featured", app.perm("product:update"), app.prodCatalogHandler.SetFeatured)
				prodProducts.GET("/:id/history", app.perm("product:list"), app.prodCatalogHandler.ListHistory)
				prodProducts.GET("/:id/specs", app.perm("product:list"), app.prodCatalogHandler.ListSpecs)
			}

			// 规格管理（spec 子域）
			specGroup := productGroup.Group("/spec")
			{
				specTemplates := specGroup.Group("/templates")
				{
					specTemplates.GET("", app.perm("spec:template:list"), app.specHandler.ListTemplates)
					specTemplates.POST("", app.perm("spec:template:update"), app.specHandler.CreateTemplate)
					specTemplates.GET("/:id", app.perm("spec:template:list"), app.specHandler.GetTemplate)
					specTemplates.PUT("/:id", app.perm("spec:template:update"), app.specHandler.UpdateTemplate)
					specTemplates.DELETE("/:id", app.perm("spec:template:update"), app.specHandler.DeleteTemplate)
				}
				specMappings := specGroup.Group("/mappings")
				{
					specMappings.GET("", app.perm("spec:mapping:list"), app.specHandler.ListMappings)
					specMappings.POST("", app.perm("spec:mapping:update"), app.specHandler.CreateMapping)
					specMappings.GET("/:id", app.perm("spec:mapping:list"), app.specHandler.GetMapping)
					specMappings.PUT("/:id", app.perm("spec:mapping:update"), app.specHandler.UpdateMapping)
					specMappings.POST("/:id/bind", app.perm("spec:mapping:update"), app.specHandler.BindMapping)
					specMappings.DELETE("/:id", app.perm("spec:mapping:update"), app.specHandler.DeleteMapping)
				}
			}

			// 定价与计费（pricing 子域）
			pricingGroup := productGroup.Group("/pricing")
			{
				pricingGroup.GET("", app.perm("pricing:list"), app.pricingHandler.List)
				pricingGroup.POST("", app.perm("pricing:update"), app.pricingHandler.Create)
				pricingGroup.GET("/:id", app.perm("pricing:list"), app.pricingHandler.Get)
				pricingGroup.PUT("/:id", app.perm("pricing:update"), app.pricingHandler.Update)
				pricingGroup.DELETE("/:id", app.perm("pricing:update"), app.pricingHandler.Delete)
			}

			// 折扣策略（discount 子域，P5-01/P5-06）：用户组/代理绑定的价格策略
			discountPolicies := productGroup.Group("/discount-policies")
			{
				discountPolicies.GET("", app.perm("pricing:list"), app.discountPolicyHandler.List)
				discountPolicies.POST("", app.perm("pricing:update"), app.discountPolicyHandler.Create)
				discountPolicies.GET("/:id", app.perm("pricing:list"), app.discountPolicyHandler.Get)
				discountPolicies.PUT("/:id", app.perm("pricing:update"), app.discountPolicyHandler.Update)
				discountPolicies.DELETE("/:id", app.perm("pricing:update"), app.discountPolicyHandler.Delete)
			}

			// 促销管理（promotion 子域）
			promoGroup := productGroup.Group("/promotion")
			{
				coupons := promoGroup.Group("/coupons")
				{
					coupons.GET("", app.perm("promotion:coupon:list"), app.promotionHandler.ListCoupons)
					coupons.POST("", app.perm("promotion:coupon:update"), app.promotionHandler.CreateCoupon)
					coupons.GET("/:id", app.perm("promotion:coupon:list"), app.promotionHandler.GetCoupon)
					coupons.PUT("/:id", app.perm("promotion:coupon:update"), app.promotionHandler.UpdateCoupon)
					coupons.DELETE("/:id", app.perm("promotion:coupon:update"), app.promotionHandler.DeleteCoupon)
				}
				couponGrants := promoGroup.Group("/coupon-grants")
				{
					couponGrants.GET("", app.perm("promotion:coupon:list"), app.promotionHandler.ListCouponGrants)
					couponGrants.POST("", app.perm("promotion:coupon:update"), app.promotionHandler.CreateCouponGrants)
				}
				promotions := promoGroup.Group("/promotions")
				{
					promotions.GET("", app.perm("promotion:activity:list"), app.promotionHandler.ListPromotions)
					promotions.POST("", app.perm("promotion:activity:update"), app.promotionHandler.CreatePromotion)
					promotions.GET("/:id", app.perm("promotion:activity:list"), app.promotionHandler.GetPromotion)
					promotions.PUT("/:id", app.perm("promotion:activity:update"), app.promotionHandler.UpdatePromotion)
					promotions.DELETE("/:id", app.perm("promotion:activity:update"), app.promotionHandler.DeletePromotion)
				}
			}
		}

		// 订单管理
		orderGroup := v1.Group("/orders")
		orderGroup.Use(app.adminAuth())
		{
			orderGroup.GET("", app.perm("order:list"), app.orderHandler.List)
			orderGroup.GET("/stats", app.perm("order:stats"), app.orderHandler.Stats)
			orderGroup.GET("/:id", app.perm("order:list"), app.orderHandler.Get)
			orderGroup.POST("/:id/cancel", app.perm("order:cancel"), app.orderHandler.Cancel)
			orderGroup.PUT("/:id/remark", app.perm("order:remark"), app.orderHandler.UpdateRemark)
			orderGroup.POST("/:id/refund", app.perm("order:refund"), app.orderHandler.CreateRefund)
			orderGroup.POST("/:id/activate", app.perm("order:activate"), app.orderHandler.Activate)
		}

		// 工单支持（doc50）
		ticketGroup := v1.Group("/tickets")
		ticketGroup.Use(app.adminAuth())
		{
			ticketGroup.GET("", app.perm("ticket:list"), app.ticketHandler.List)
			ticketGroup.GET("/stats", app.perm("ticket:stats"), app.ticketHandler.Stats)
			ticketGroup.GET("/:id", app.perm("ticket:view"), app.ticketHandler.Get)
			ticketGroup.POST("/:id/reply", app.perm("ticket:reply"), app.ticketHandler.Reply)
			ticketGroup.PUT("/:id/assign", app.perm("ticket:assign"), app.ticketHandler.Assign)
			// 认领开放给有工单列表权限的员工（未分配池自助认领）
			ticketGroup.POST("/:id/claim", app.perm("ticket:list"), app.ticketHandler.Claim)
			ticketGroup.PUT("/:id/transfer", app.perm("ticket:assign"), app.ticketHandler.Transfer)
			ticketGroup.PUT("/:id/status", app.perm("ticket:update"), app.ticketHandler.UpdateStatus)
			ticketGroup.POST("/:id/close", app.perm("ticket:close"), app.ticketHandler.Close)
		}

		// 工单分类（doc50）
		ticketCategoryGroup := v1.Group("/ticket-categories")
		ticketCategoryGroup.Use(app.adminAuth())
		{
			ticketCategoryGroup.GET("", app.perm("ticket:category"), app.ticketCategoryHandler.List)
			ticketCategoryGroup.POST("", app.perm("ticket:manage"), app.ticketCategoryHandler.Create)
			ticketCategoryGroup.PUT("/:id", app.perm("ticket:manage"), app.ticketCategoryHandler.Update)
			ticketCategoryGroup.DELETE("/:id", app.perm("ticket:manage"), app.ticketCategoryHandler.Delete)
		}

		// 退款管理
		refundGroup := v1.Group("/refunds")
		refundGroup.Use(app.adminAuth())
		{
			refundGroup.GET("", app.perm("order:refunds"), app.refundHandler.List)
			refundGroup.GET("/:id", app.perm("order:refunds"), app.refundHandler.Get)
			refundGroup.POST("/:id/approve", app.perm("order:refund:audit"), app.refundHandler.Approve)
			refundGroup.POST("/:id/reject", app.perm("order:refund:audit"), app.refundHandler.Reject)
		}

		// 财务管理
		financeGroup := v1.Group("/finance")
		financeGroup.Use(app.adminAuth())
		{
			// 钱包/流水
			financeGroup.GET("/wallets/:user_id", app.perm("finance:wallet"), app.walletHandler.Balance)
			financeGroup.GET("/transactions", app.perm("finance:wallet"), app.walletHandler.ListTransactions)
			financeGroup.POST("/transactions/adjust", app.perm("finance:adjust"), app.walletHandler.Adjust)
			// 充值
			financeGroup.POST("/recharges", app.perm("finance:recharge"), app.rechargeHandler.Create)
			financeGroup.GET("/recharges", app.perm("finance:recharge"), app.rechargeHandler.List)
			financeGroup.POST("/recharges/:id/approve", app.perm("finance:recharge:approve"), app.rechargeHandler.Approve)
			// 提现
			financeGroup.GET("/withdrawals", app.perm("finance:withdraw"), app.withdrawHandler.List)
			financeGroup.POST("/withdrawals/:id/approve", app.perm("finance:withdraw:audit"), app.withdrawHandler.Approve)
			financeGroup.POST("/withdrawals/:id/reject", app.perm("finance:withdraw:audit"), app.withdrawHandler.Reject)
			// 账单/对账
			financeGroup.GET("/bills", app.perm("finance:bill"), app.billHandler.List)
			financeGroup.POST("/bills/:id/close", app.perm("finance:bill:close"), app.billHandler.Close)
			financeGroup.POST("/bills/recon", app.perm("finance:bill:recon"), app.reconHandler.Reconcile)
		}

		// 推广邀请返现（台账 / 邀请关系 / 提现审核）
		referralGroup := v1.Group("/referral")
		referralGroup.Use(app.adminAuth())
		{
			referralGroup.GET("/cashbacks", app.perm("referral:cashback:list"), app.adminReferralHandler.Cashbacks)
			referralGroup.GET("/invitees", app.perm("referral:cashback:list"), app.adminReferralHandler.Invitees)
			referralGroup.GET("/withdrawals", app.perm("referral:withdraw:list"), app.adminReferralHandler.Withdrawals)
			referralGroup.POST("/withdrawals/:id/approve", app.perm("referral:withdraw:audit"), app.adminReferralHandler.Approve)
			referralGroup.POST("/withdrawals/:id/reject", app.perm("referral:withdraw:audit"), app.adminReferralHandler.Reject)
		}

		// 系统管理（系统配置）
		systemConfigGroup := v1.Group("/system/configs")
		systemConfigGroup.Use(app.adminAuth())
		{
			systemConfigGroup.GET("", app.perm("system:config:view"), app.configHandler.List)
			systemConfigGroup.POST("", app.perm("system:config:create"), app.configHandler.Create)
			// 按分组读取（固定段 /group/:group 需先于 /:key 匹配，且二者不同段数不冲突）
			systemConfigGroup.GET("/group/:group", app.perm("system:config:view"), app.configHandler.ListByGroup)
			// 分组批量 upsert
			systemConfigGroup.POST("/batch", app.perm("system:config:update"), app.configHandler.BatchUpsert)
			systemConfigGroup.GET("/:key", app.perm("system:config:view"), app.configHandler.GetByKey)
			systemConfigGroup.PUT("/:id", app.perm("system:config:update"), app.configHandler.Update)
			systemConfigGroup.DELETE("/:id", app.perm("system:config:delete"), app.configHandler.Delete)
		}

		// 消息中心 - 公告管理（doc70 §7.1）
		annGroup := v1.Group("/announcements")
		annGroup.Use(app.adminAuth())
		{
			annGroup.GET("", app.perm("notify:announcement"), app.notifyAdminHandler.ListAnnouncements)
			annGroup.POST("", app.perm("notify:manage"), app.notifyAdminHandler.CreateAnnouncement)
			annGroup.PUT("/:id", app.perm("notify:manage"), app.notifyAdminHandler.UpdateAnnouncement)
			annGroup.POST("/:id/publish", app.perm("notify:manage"), app.notifyAdminHandler.PublishAnnouncement)
			annGroup.POST("/:id/offline", app.perm("notify:manage"), app.notifyAdminHandler.OfflineAnnouncement)
			annGroup.DELETE("/:id", app.perm("notify:manage"), app.notifyAdminHandler.DeleteAnnouncement)
		}

		// 消息中心 - 通知记录（doc70 §7.1）
		// 注意：/unread-count 和 /mail-test 固定路径需先于 /:id/resend 注册
		notifyGroup := v1.Group("/notifications")
		notifyGroup.Use(app.adminAuth())
		{
			notifyGroup.GET("", app.perm("notify:record"), app.notifyAdminHandler.ListRecords)
			notifyGroup.GET("/unread-count", app.perm("notify:record"), app.notifyAdminHandler.AdminUnreadCount)
			notifyGroup.POST("/mail-test", app.perm("notify:view"), app.notifyAdminHandler.SendMailTest)
			notifyGroup.POST("/:id/resend", app.perm("notify:view"), app.notifyAdminHandler.Resend)
		}

		// 消息中心 - 通知模板（doc70 §7.1）
		tplGroup := v1.Group("/notification-templates")
		tplGroup.Use(app.adminAuth())
		{
			tplGroup.GET("", app.perm("notify:template"), app.notifyAdminHandler.ListTemplates)
			tplGroup.PUT("/:id", app.perm("notify:manage"), app.notifyAdminHandler.UpdateTemplate)
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
	// 子账号可看账单（billing:view），但充值属资金入口，硬编码拒绝（P4-06）。
	ucFinance := r.Group("/api/v1/uc/finance")
	{
		ucFinance.GET("/balance", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermBillingView), app.userFinanceHandler.Balance)           // 我的余额
		ucFinance.GET("/transactions", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermBillingView), app.userFinanceHandler.Transactions) // 我的资金流水
		ucFinance.POST("/recharge", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.rejectSub(), app.userFinanceHandler.CreateRecharge)                        // 发起充值（子账号拒绝）
		ucFinance.GET("/bills", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermBillingView), app.userFinanceHandler.Bills)               // 我的账单
		ucFinance.POST("/recharge/callback", app.userFinanceHandler.RechargeCallback)                                                                                             // 充值回调（渠道通知）
	}

	// 用户中心推广返现：查看邀请码/邀请人/返现明细属账单域，申请提现与转入余额为资金入口（子账号拒绝）。
	ucReferral := r.Group("/api/v1/uc/referral")
	{
		ucReferral.GET("/profile", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermBillingView), app.ucReferralHandler.Profile)         // 我的推广概览
		ucReferral.GET("/invitees", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermBillingView), app.ucReferralHandler.Invitees)       // 我的邀请
		ucReferral.GET("/cashbacks", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermBillingView), app.ucReferralHandler.Cashbacks)     // 返现明细
		ucReferral.GET("/withdrawals", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermBillingView), app.ucReferralHandler.Withdrawals) // 我的提现记录
		ucReferral.POST("/withdrawals", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.rejectSub(), app.ucReferralHandler.ApplyWithdrawal)                  // 申请提现
		ucReferral.POST("/transfer", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.rejectSub(), app.ucReferralHandler.Transfer)                            // 转入现金余额
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
	// 子账号下单需 order:create，查看需 order:view（P4-06）。
	ucOrders := r.Group("/api/v1/uc/orders")
	ucOrders.Use(middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userAudit())
	{
		ucOrders.POST("", app.userPerm(appauth.PermOrderCreate), app.ucOrderHandler.Create)
		ucOrders.GET("", app.userPerm(appauth.PermOrderView), app.ucOrderHandler.List)
		// 预结算只读算价，子账号可看（P5-05，需求 order:view）。
		ucOrders.POST("/quote", app.userPerm(appauth.PermOrderView), app.ucOrderHandler.Quote)
	}

	// 用户中心主机管理：列表/详情/电源操作/VNC（需登录）
	// 查看需 instance:view，电源/VNC 需 instance:operate（P4-06）。
	ucInstances := r.Group("/api/v1/uc/instances")
	ucInstances.Use(middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userAudit())
	{
		ucInstances.GET("", app.userPerm(appauth.PermInstanceView), app.ucInstanceHandler.List)                // 我的主机列表
		ucInstances.GET("/:id", app.userPerm(appauth.PermInstanceView), app.ucInstanceHandler.Detail)          // 主机详情
		ucInstances.POST("/:id/power", app.userPerm(appauth.PermInstanceOperate), app.ucInstanceHandler.Power) // 电源操作
		ucInstances.POST("/:id/vnc", app.userPerm(appauth.PermInstanceOperate), app.ucInstanceHandler.VNC)     // 远程控制台
	}

	// 用户中心工单支持：我的工单自助管理（doc50）
	// 查看需 ticket:view，提交/回复需 ticket:submit（P4-06）。
	ucSupport := r.Group("/api/v1/uc/support")
	ucSupport.Use(app.userAudit())
	{
		ucSupport.GET("/tickets", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermTicketView), app.userTicketHandler.List)                     // 我的工单列表
		ucSupport.POST("/tickets", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermTicketSubmit), app.userTicketHandler.Create)                // 提交工单
		ucSupport.GET("/ticket-categories", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermTicketView), app.userTicketHandler.ListCategories) // 可用工单分类
		ucSupport.GET("/tickets/:id", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermTicketView), app.userTicketHandler.Get)                  // 工单详情（含回复）
		ucSupport.POST("/tickets/:id/replies", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermTicketSubmit), app.userTicketHandler.Reply)     // 追加工单回复
		ucSupport.POST("/tickets/:id/cancel", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermTicketSubmit), app.userTicketHandler.Cancel)     // 取消工单
	}

	// 用户中心成员（子账号，P4-07）：仅主账号可管理，服务层再校验一次 IsSub。
	ucMembers := r.Group("/api/v1/uc/members")
	ucMembers.Use(middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userAudit())
	{
		ucMembers.GET("", app.memberHandler.List)                           // 我的成员列表
		ucMembers.POST("", app.memberHandler.Create)                        // 新建成员
		ucMembers.PUT("/:id", app.memberHandler.Update)                     // 改备注/状态
		ucMembers.PUT("/:id/permissions", app.memberHandler.SetPermissions) // 覆盖式设置权限
		ucMembers.DELETE("/:id", app.memberHandler.Delete)                  // 删除（禁用）成员
		ucMembers.GET("/:id/logs", app.memberHandler.ListLogs)              // 成员操作日志
	}

	// 用户中心生命周期与续费（doc60 §7.2）
	// 聚合视图 /uc/instances/renewals 为固定路径，需先于 /instances/:id 注册
	ucLifecycle := r.Group("/api/v1/uc")
	ucLifecycle.Use(app.userAudit())
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
