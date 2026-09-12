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
				providers.POST("/:id/sync/resume", app.perm("provider:update"), app.providerHandler.ResumeSync)
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
				// ---- P3 同步框架（T3.2/T3.4/T3.5）----
				sync.GET("/scopes", app.perm("resource:sync"), app.syncFrameworkHandler.ScopeMeta)
				sync.GET("/schedules", app.perm("resource:sync"), app.syncFrameworkHandler.ListSchedules)
				sync.PUT("/schedules/:id", app.perm("sync:schedule"), app.syncFrameworkHandler.UpdateSchedule)
				sync.GET("/price-changes", app.perm("sync:price"), app.syncFrameworkHandler.ListPriceChanges)
				sync.POST("/price-changes/handle", app.perm("sync:price:confirm"), app.syncFrameworkHandler.HandlePriceChanges)
				sync.GET("/diffs", app.perm("resource:sync"), app.syncFrameworkHandler.ListDiffs)
				sync.GET("/diffs/summary", app.perm("resource:sync"), app.syncFrameworkHandler.DiffSummary)
			}

			// 实例管理
			instances := resourceGroup.Group("/instances")
			{
				instances.GET("", app.perm("resource:instance"), app.syncHandler.ListInstances)
				instances.GET("/:id", app.perm("resource:instance"), app.syncHandler.GetInstance)
			}

			// 任务队列（本轮 S3）：开通履约 / 实例动作 / 续费 / 上游同步四类平台动作的「是否到达上游」。
			// 只读聚合视图，无写接口；/categories 静态段先于其它段注册。
			taskQueue := resourceGroup.Group("/task-queue")
			{
				taskQueue.GET("", app.perm("resource:sync"), app.taskQueueHandler.List)
				taskQueue.GET("/categories", app.perm("resource:sync"), app.taskQueueHandler.Categories)
			}

			// 实例对账（本轮 S4）：已开通的上游链路实例，本地售价/到期 vs 上游成本/到期。
			// 只读比对视图，修正动作仍在商品/实例详情页完成。
			resourceGroup.GET("/reconcile", app.perm("resource:instance"), app.reconcileHandler.List)
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

		// 实例运维台（管理端跨用户）：/admin/instances（见 docs/实施计划/61-实例运维管理台实施计划.md）
		// 同样遵循"静态段先于 /:id 注册"，故本组置于生命周期组之后。
		instanceOps := v1.Group("/instances")
		instanceOps.Use(app.adminAuth())
		{
			instanceOps.GET("", app.perm("resource:instance"), app.instanceOpsHandler.List)
			instanceOps.GET("/stats", app.perm("resource:instance"), app.instanceOpsHandler.Stats)
			instanceOps.GET("/:id", app.perm("resource:instance"), app.instanceOpsHandler.Detail)
			instanceOps.GET("/:id/operations", app.perm("resource:instance"), app.instanceOpsHandler.Operations)
			instanceOps.GET("/:id/related", app.perm("resource:instance"), app.instanceOpsHandler.Related)
			instanceOps.POST("/:id/sync", app.perm("instance:action"), app.instanceOpsHandler.Sync)
			instanceOps.POST("/:id/power", app.perm("instance:action"), app.instanceOpsHandler.Power)
			instanceOps.POST("/:id/suspend", app.perm("instance:action"), app.instanceOpsHandler.Suspend)
			instanceOps.POST("/:id/unsuspend", app.perm("instance:action"), app.instanceOpsHandler.Unsuspend)
			instanceOps.PUT("/:id/remark", app.perm("instance:action"), app.instanceOpsHandler.SetRemark)
			instanceOps.POST("/:id/vnc", app.perm("instance:console"), app.instanceOpsHandler.VNC)
			instanceOps.POST("/:id/resize", app.perm("instance:resize"), app.instanceOpsHandler.Resize)
			instanceOps.DELETE("/:id", app.perm("instance:destroy"), app.instanceOpsHandler.Destroy)
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
				// SKU 规格变体维护（T4.1）：商品下挂规格矩阵，下单按 SKU 计价与开通。
				prodProducts.POST("/:id/specs", app.perm("product:update"), app.prodCatalogHandler.CreateSpec)
				prodProducts.PUT("/:id/specs/:specId", app.perm("product:update"), app.prodCatalogHandler.UpdateSpec)
				prodProducts.DELETE("/:id/specs/:specId", app.perm("product:update"), app.prodCatalogHandler.DeleteSpec)
				// 可配置项维护（T4.4）：source=upstream 走上游配置 id，source=self 直接下发平台参数名。
				prodProducts.GET("/:id/config-options", app.perm("product:list"), app.prodCatalogHandler.ListConfigOptions)
				prodProducts.PUT("/:id/config-options", app.perm("product:update"), app.prodCatalogHandler.SaveConfigOptions)
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
				// 规格契约（P2/T2.5）：原子字典 / 校验 / 外部规格快照 / 绑定
				specGroup.GET("/atoms", app.perm("spec:contract:list"), app.specHandler.ListAtoms)
				specGroup.POST("/validate", app.perm("spec:contract:list"), app.specHandler.ValidateSpec)
				specGroup.GET("/external-specs", app.perm("spec:contract:list"), app.specHandler.ListExternalSpecs)
				specGroup.POST("/external-specs", app.perm("spec:contract:update"), app.specHandler.UpsertExternalSpec)
				specGroup.GET("/bindings", app.perm("spec:contract:list"), app.specHandler.ListBindings)
				specGroup.POST("/bindings", app.perm("spec:contract:update"), app.specHandler.UpsertBinding)
				specGroup.POST("/bindings/:id/confirm", app.perm("spec:contract:update"), app.specHandler.ConfirmBinding)
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

			// 周期价格矩阵（doc25）：商品 × 规格 × 周期，GET 取矩阵 / PUT 整表保存。
			prices := productGroup.Group("/prices")
			{
				prices.GET("", app.perm("pricing:list"), app.priceMatrixHandler.Get)
				prices.PUT("", app.perm("pricing:update"), app.priceMatrixHandler.Save)
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
			// 打款闭环（doc34 F-02）：审核通过后登记打款完成/失败，成功才结算冻结资金。
			financeGroup.POST("/withdrawals/:id/mark-paid", app.perm("finance:withdraw:audit"), app.withdrawHandler.MarkPaid)
			financeGroup.POST("/withdrawals/:id/mark-failed", app.perm("finance:withdraw:audit"), app.withdrawHandler.MarkFailed)
			// 账单/对账
			financeGroup.GET("/bills", app.perm("finance:bill"), app.billHandler.List)
			// 手动生成账单（doc36 §7-2）：按用户 + 账期归集消费/退款并分类。
			financeGroup.POST("/bills/generate", app.perm("finance:bill:close"), app.billHandler.Generate)
			financeGroup.POST("/bills/:id/close", app.perm("finance:bill:close"), app.billHandler.Close)
			financeGroup.POST("/bills/recon", app.perm("finance:bill:recon"), app.reconHandler.Reconcile)
			// 发票管理（doc36 §3.3）：查看申请 → 开票/驳回；渠道与外部单号已预埋。
			financeGroup.GET("/invoices", app.perm("finance:invoice"), app.billHandler.Invoices)
			financeGroup.POST("/invoices/:id/issue", app.perm("finance:invoice:issue"), app.billHandler.IssueInvoice)
			financeGroup.POST("/invoices/:id/reject", app.perm("finance:invoice:issue"), app.billHandler.RejectInvoice)
		}

		// 支付中心（doc35）：渠道配置 / 支付单 / 回调 / 退款 / 打款 / 对账 / 支付方式
		paymentGroup := v1.Group("/payment")
		paymentGroup.Use(app.adminAuth())
		{
			// 渠道类型与渠道实例
			paymentGroup.GET("/channel-types", app.perm("payment:channel"), app.payment.channelHandler.ListTypes)
			paymentGroup.GET("/channels", app.perm("payment:channel"), app.payment.channelHandler.List)
			paymentGroup.POST("/channels", app.perm("payment:channel:manage"), app.payment.channelHandler.Create)
			paymentGroup.GET("/channels/:id", app.perm("payment:channel"), app.payment.channelHandler.Get)
			paymentGroup.PUT("/channels/:id", app.perm("payment:channel:manage"), app.payment.channelHandler.Update)
			paymentGroup.PATCH("/channels/:id/status", app.perm("payment:channel:manage"), app.payment.channelHandler.UpdateStatus)
			paymentGroup.POST("/channels/:id/test", app.perm("payment:channel"), app.payment.channelHandler.Test)
			// 支付单
			paymentGroup.GET("/orders", app.perm("payment:order"), app.payment.orderHandler.List)
			paymentGroup.GET("/orders/:id", app.perm("payment:order"), app.payment.orderHandler.Get)
			paymentGroup.POST("/orders/:id/confirm", app.perm("payment:order:operate"), app.payment.orderHandler.Confirm)
			paymentGroup.POST("/orders/:id/close", app.perm("payment:order:operate"), app.payment.orderHandler.Close)
			paymentGroup.POST("/orders/:id/sync", app.perm("payment:order:operate"), app.payment.orderHandler.Sync)
			// 回调日志
			paymentGroup.GET("/callbacks", app.perm("payment:callback"), app.payment.callbackHandler.List)
			// 渠道退款单
			paymentGroup.GET("/refunds", app.perm("payment:refund"), app.payment.refundHandler.List)
			// 打款单
			paymentGroup.GET("/payouts", app.perm("payment:payout"), app.payment.payoutHandler.List)
			paymentGroup.POST("/payouts/:id/mark-paid", app.perm("payment:payout:operate"), app.payment.payoutHandler.MarkPaid)
			paymentGroup.POST("/payouts/:id/retry", app.perm("payment:payout:operate"), app.payment.payoutHandler.Retry)
			// 渠道对账
			paymentGroup.GET("/recon", app.perm("payment:recon"), app.payment.reconHandler.List)
			paymentGroup.POST("/recon", app.perm("payment:recon"), app.payment.reconHandler.Reconcile)
			// 支付方式路由（场景可用渠道与优先级）
			paymentGroup.GET("/methods", app.perm("payment:method"), app.payment.methodHandler.Options)
		}

		// 积分中心（doc36）：独立于资金账本的积分体系，仅规则/账户/流水管理，
		// 不暴露任何「积分抵扣/提现」入口 —— 积分绝不可作为支付方式。
		pointGroup := v1.Group("/points")
		pointGroup.Use(app.adminAuth())
		{
			pointGroup.GET("/overview", app.perm("point:account"), app.point.adminHandler.Overview)
			pointGroup.GET("/rules", app.perm("point:rule"), app.point.adminHandler.Rules)
			pointGroup.POST("/rules", app.perm("point:rule:manage"), app.point.adminHandler.CreateRule)
			pointGroup.PUT("/rules/:id", app.perm("point:rule:manage"), app.point.adminHandler.UpdateRule)
			pointGroup.GET("/accounts", app.perm("point:account"), app.point.adminHandler.Accounts)
			pointGroup.GET("/accounts/:user_id", app.perm("point:account"), app.point.adminHandler.Account)
			pointGroup.POST("/accounts/adjust", app.perm("point:account:adjust"), app.point.adminHandler.Adjust)
			pointGroup.GET("/transactions", app.perm("point:transaction"), app.point.adminHandler.Transactions)
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
		ucFinance.GET("/recharges", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermBillingView), app.userFinanceHandler.Recharges)       // 我的充值单（doc34 F-07）
		ucFinance.GET("/bills", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermBillingView), app.userFinanceHandler.Bills)               // 我的账单
		// 发票（doc36 §3.3）：查看与申请均属账单域，子账号可读可申请（不涉及资金出入）。
		ucFinance.GET("/invoices", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermBillingView), app.userFinanceHandler.MyInvoices) // 我的发票申请
		ucFinance.POST("/invoices", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.rejectSub(), app.userFinanceHandler.ApplyInvoice)                    // 申请开票（子账号拒绝）
		// 发票取件与邮件下发（doc36 §3.3 预埋）：下载已可用（取文件地址），邮件下发本轮返回明确未接入。
		ucFinance.GET("/invoices/:id/download", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermBillingView), app.userFinanceHandler.InvoiceDownload)
		ucFinance.POST("/invoices/:id/email", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermBillingView), app.userFinanceHandler.InvoiceEmail)
		// 注：原未鉴权充值回调 POST /uc/finance/recharge/callback 已下线（doc34 F-01）。
		// 充值到账改由支付中心统一回调 /api/v1/payment/notify/:channel_code 验签后驱动。
	}

	// 用户中心支付：收银台 / 支付方式偏好 / 收款账户 / 提现申请（子账号拒绝资金入口）。
	ucPayment := r.Group("/api/v1/uc/payment")
	{
		// 查看类（账单域权限即可）
		ucPayment.GET("/methods", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermBillingView), app.payment.userPaymentHandler.Methods)
		ucPayment.GET("/preferences", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermBillingView), app.payment.userPaymentHandler.Preferences)
		ucPayment.PUT("/preferences", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermBillingView), app.payment.userPaymentHandler.SavePreferences)
		ucPayment.GET("/accounts", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermBillingView), app.payment.userPaymentHandler.Accounts)
		ucPayment.GET("/withdrawals", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermBillingView), app.payment.userPaymentHandler.Withdrawals)
		ucPayment.GET("/orders/:payment_no", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermBillingView), app.payment.userPaymentHandler.Order)
		// 资金入口（子账号硬拒绝，P4-06）
		ucPayment.POST("/accounts", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.rejectSub(), app.payment.userPaymentHandler.CreateAccount)
		ucPayment.PUT("/accounts/:id/default", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.rejectSub(), app.payment.userPaymentHandler.SetDefaultAccount)
		ucPayment.POST("/recharge", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.rejectSub(), app.payment.userPaymentHandler.Recharge)
		ucPayment.POST("/bills/pay", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.rejectSub(), app.payment.userPaymentHandler.PayBill)
		ucPayment.POST("/withdrawals", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.rejectSub(), app.payment.userPaymentHandler.Withdraw)
	}

	// 用户中心积分（doc36）：只读展示，积分不可抵扣、不可提现、不可转入余额。
	// 复用账单域权限（billing:view），子账号可读（积分归主账号）。
	ucPoints := r.Group("/api/v1/uc/points")
	{
		ucPoints.GET("", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermBillingView), app.point.userHandler.Overview)                  // 我的积分
		ucPoints.GET("/transactions", middleware.UserAuth(app.jwtIssuer, app.cfg.Auth.BearerPrefix), app.userPerm(appauth.PermBillingView), app.point.userHandler.Transactions) // 我的积分流水
	}

	// 支付渠道异步回调（免登录，唯一信任来源为渠道验签；取代原未鉴权充值回调）。
	paymentNotify := r.Group("/api/v1/payment/notify")
	{
		paymentNotify.POST("/:channel_code", app.payment.notifyHandler.Notify)
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

	// 开放平台（P6/T6.1）：对外前缀 /open/v1，独立中间件链：
	// 审计（最外层，鉴权失败同样留痕）→ 签名 → 应用校验 → IP 白名单 → nonce 防重放 → 限流。
	openV1 := r.Group("/open/v1")
	openV1.Use(app.open.Audit(), app.open.Gateway())
	{
		// 链路自检：签名通过即可访问，不占能力位。POST 用于验证带 body 的签名。
		openV1.GET("/ping", app.open.Ping)
		openV1.POST("/ping", app.open.Ping)
		// 只读目录（T6.2，能力位 catalog:read）：标准契约，内部异构不出门（doc16 §8.1）。
		openV1.GET("/spec-atoms", app.open.RequireScope("catalog:read"), app.open.ListSpecAtoms)
		openV1.GET("/products", app.open.RequireScope("catalog:read"), app.open.ListProducts)
		openV1.GET("/products/:id", app.open.RequireScope("catalog:read"), app.open.GetProduct)
		openV1.GET("/regions", app.open.RequireScope("catalog:read"), app.open.ListRegions)
		openV1.GET("/images", app.open.RequireScope("catalog:read"), app.open.ListImages)
		openV1.POST("/quote", app.open.RequireScope("catalog:read"), app.open.Quote)
		// 代客下单（T6.3，能力位 order:create）：幂等键走 X-Client-Request-Id 头（doc16 §8.5）。
		openV1.POST("/orders", app.open.RequireScope("order:create"), app.open.CreateOrder)
		// 实例接口（T6.4）：查询/续费/电源/暂停恢复，按能力位细分；
		// D2 安全边界：DELETE 显式注册并固定返回 40009「不支持」（开放平台无销毁能力）。
		openV1.GET("/instances", app.open.RequireScope("instance:read"), app.open.ListInstances)
		openV1.GET("/instances/:id", app.open.RequireScope("instance:read"), app.open.GetInstance)
		openV1.POST("/instances/:id/renew", app.open.RequireScope("instance:renew"), app.open.RenewInstance)
		openV1.POST("/instances/:id/power", app.open.RequireScope("instance:power"), app.open.PowerInstance)
		openV1.POST("/instances/:id/suspend", app.open.RequireScope("instance:suspend"), app.open.SuspendInstance)
		openV1.POST("/instances/:id/unsuspend", app.open.RequireScope("instance:suspend"), app.open.UnsuspendInstance)
		openV1.DELETE("/instances/:id", app.open.DeleteInstance)
		// 对账（T6.6，能力位 audit:read）：幂等请求与代客下单订单。
		openV1.GET("/audit/requests", app.open.RequireScope("audit:read"), app.open.ListAuditRequests)
		openV1.GET("/audit/orders", app.open.RequireScope("audit:read"), app.open.ListAuditOrders)
	}

	return r
}
