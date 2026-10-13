package server

import (
	"github.com/gin-gonic/gin"
)

// registerAdminRoutes 注册管理端 /api/v1/admin 全部路由（自 router.go 原样拆出，批次四 4a）。
// 组内注册顺序与拆分前保持一致；写操作审计、全局限流挂组级中间件。
// 第三方登录同时覆盖管理端与用户端两端路由，故在函数尾部一并注册。
func registerAdminRoutes(r *gin.Engine, app *App) {
	v1 := r.Group("/api/v1/admin")
	// 管理端写操作审计（P2-06）：仅记录 POST/PUT/PATCH/DELETE，登录等白名单路径自动跳过。
	v1.Use(app.adminAudit())
	// 全局 API 限流（doc91 §9.1 api_rate_limit）：默认 0=不限；缓存不可用时放行并告警。
	if app.captcha != nil {
		v1.Use(app.captcha.limiter.Handler())
	}
	{
		auth := v1.Group("/auth")
		{
			auth.POST("/login", app.adminHandler.Login)
			// 登录二次验证（doc91 §5.2）：凭 otp_token 换正式令牌。
			auth.POST("/login/verify-otp", app.adminHandler.VerifyLoginOTP)
			auth.GET("/me", app.adminAuth(), app.adminHandler.Me)
			auth.POST("/change-password", app.adminAuth(), app.adminHandler.ChangePassword)
			// 员工管理（超管独占，81 §4.1）。保留 /auth/admins 旧路径做别名，前端逐步切到 /staff。
			auth.GET("/admins", app.adminAuth(), app.superOnly(), app.adminHandler.List)
			auth.POST("/admins", app.adminAuth(), app.superOnly(), app.adminHandler.Create)
			auth.GET("/admins/:id", app.adminAuth(), app.superOnly(), app.adminHandler.Get)
			auth.PUT("/admins/:id", app.adminAuth(), app.superOnly(), app.adminHandler.Update)
			auth.PUT("/admins/:id/roles", app.adminAuth(), app.superOnly(), app.adminRequireVerification("admin_grant_change"), app.adminHandler.SetRoles)
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

		// 员工管理（P2-01 新路径）
		// S1 员工体系：由 superOnly() 改为逐路由 perm(...)，语义等价（超管靠通配），
		// 便于后续给 support_lead 开 staff:view（doc86 §2.1）。
		staff := v1.Group("/staff")
		staff.Use(app.adminAuth())
		{
			staff.GET("", app.perm("staff:list"), app.adminHandler.List)
			staff.POST("", app.perm("staff:create"), app.adminHandler.Create)
			staff.GET("/:id", app.perm("staff:view"), app.adminHandler.Get)
			staff.PUT("/:id", app.perm("staff:update"), app.adminHandler.Update)
			staff.PUT("/:id/roles", app.perm("staff:assign_role"), app.adminHandler.SetRoles)
			staff.PATCH("/:id/status", app.perm("staff:update"), app.adminHandler.UpdateStatus)
			staff.POST("/:id/reset-password", app.perm("staff:reset_password"), app.adminHandler.ResetPassword)
			staff.POST("/:id/resign", app.perm("staff:update"), app.adminHandler.Resign)
			staff.DELETE("/:id", app.perm("staff:delete"), app.adminHandler.Delete)
		}

		// 组织部门（S1 员工体系，doc86 §2.1）
		departments := v1.Group("/departments")
		departments.Use(app.adminAuth())
		{
			departments.GET("", app.perm("department:list"), app.departmentHandler.List)
			departments.POST("", app.perm("department:create"), app.departmentHandler.Create)
			departments.GET("/:id", app.perm("department:list"), app.departmentHandler.Get)
			departments.PUT("/:id", app.perm("department:update"), app.departmentHandler.Update)
			departments.DELETE("/:id", app.perm("department:delete"), app.departmentHandler.Delete)
		}

		users := v1.Group("/users")
		users.Use(app.adminAuth())
		{
			users.GET("", app.perm("system:user:list"), app.userHandler.ListUsers)
			// 导出必须与列表同权限：能看列表就能导出，且导出走同一套筛选条件。
			// 静态段 /export 注册在 /:id 之前，避免被参数路由吞掉。
			users.GET("/export", app.perm("system:user:list"), app.userHandler.ExportUsers)
			users.POST("", app.perm("user:create"), app.userHandler.CreateUser)
			users.GET("/stats", app.perm("system:user:list"), app.userHandler.GetStats)
			// 总览页活动统计（在线用户 / 最近登录用户）。与 /stats 同权限，
			// 静态段同样必须注册在 /:id 之前。
			users.GET("/activity-overview", app.perm("system:user:list"), app.userHandler.GetActivityOverview)
			// 代下单可选项（规格/周期/选配项）：静态段须注册在 /:id 之前。
			users.GET("/order-config", app.perm("user:create"), app.userHandler.OrderConfig)
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
			// 算价预览：与创建订单同口径但不落库，供弹窗展示默认原价（客服填自定义价前先看到基准）。
			users.POST(":id/orders/quote", app.perm("user:create"), app.userHandler.OrderQuote)
			// —— 注销 / 恢复 / 留存期清理（doc104 §4）——
			// 注意：批量路由必须注册在 ":id" 之前语义上才不冲突，gin 的路由树
			// 对静态段与参数段能正确分流（/users/batch-delete 不会命中 /users/:id）。
			users.GET(":id/deletion-check", app.perm("user:delete"), app.userDeletionHandler.CheckDeletion)
			users.DELETE(":id", app.perm("user:delete"), app.userDeletionHandler.DeleteUser)
			users.POST(":id/restore", app.perm("user:restore"), app.userDeletionHandler.RestoreUser)
			users.POST("/batch-delete", app.perm("user:delete"), app.userDeletionHandler.BatchDeleteUsers)
			users.POST("/batch-restore", app.perm("user:restore"), app.userDeletionHandler.BatchRestoreUsers)
			// 留存期清理是硬删除，属不可逆高危操作，仅超管。
			users.POST("/purge", app.superOnly(), app.userDeletionHandler.PurgeUsers)
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

		// 代理等级（doc108）：折扣的唯一来源。矩阵与阶梯填充是「批量写折扣」的入口，
		// 写操作用 update 权限；只读视图用 list。
		agentLevels := v1.Group("/agent-levels")
		agentLevels.Use(app.adminAuth())
		{
			agentLevels.GET("", app.perm("agent_level:list"), app.agentLevelHandler.List)
			agentLevels.POST("", app.perm("agent_level:create"), app.agentLevelHandler.Create)
			// 静态段必须注册在 /:id 之前，否则 "matrix" 会被当成 id 命中。
			agentLevels.GET("/matrix", app.perm("agent_level:list"), app.agentLevelHandler.Matrix)
			agentLevels.POST("/ladder/preview", app.perm("agent_level:list"), app.agentLevelHandler.PreviewLadder)
			agentLevels.POST("/ladder/apply", app.perm("agent_level:update"), app.agentLevelHandler.ApplyLadder)
			// 商品分组 × 折扣组（doc108 §8I）：录入组织层，应用时展开进逐格矩阵。
			// 静态段全部注册在 /:id 之前。
			productGroups := agentLevels.Group("/product-groups")
			{
				productGroups.GET("", app.perm("agent_level:list"), app.schemeHandler.ListGroups)
				productGroups.POST("", app.perm("agent_level:create"), app.schemeHandler.CreateGroup)
				productGroups.PUT("/:id", app.perm("agent_level:update"), app.schemeHandler.UpdateGroup)
				productGroups.DELETE("/:id", app.perm("agent_level:delete"), app.schemeHandler.DeleteGroup)
			}
			discountSchemes := agentLevels.Group("/discount-schemes")
			{
				discountSchemes.GET("", app.perm("agent_level:list"), app.schemeHandler.ListSchemes)
				discountSchemes.POST("", app.perm("agent_level:create"), app.schemeHandler.CreateScheme)
				discountSchemes.POST("/:id/apply", app.perm("agent_level:update"), app.schemeHandler.ApplyScheme)
				discountSchemes.PUT("/:id", app.perm("agent_level:update"), app.schemeHandler.UpdateScheme)
				discountSchemes.DELETE("/:id", app.perm("agent_level:delete"), app.schemeHandler.DeleteScheme)
			}

			// 单格折扣更新：静态段必须注册在 /:id 之前。
			agentLevels.PUT("/discount", app.perm("agent_level:update"), app.agentLevelHandler.UpdateCell)
			// 代理分组成员（归属管理）：把账号纳入/移出某个代理分组。
			// 纳入 = 设置 users.agent_level_id，移出 = 置空（取消代理身份）。
			agentLevels.GET("/:id/members", app.perm("agent_level:list"), app.agentLevelHandler.ListMembers)
			agentLevels.POST("/:id/members", app.perm("agent_level:update"), app.agentLevelHandler.AssignMembers)
			agentLevels.GET("/:id", app.perm("agent_level:list"), app.agentLevelHandler.Get)
			agentLevels.PUT("/:id", app.perm("agent_level:update"), app.agentLevelHandler.Update)
			agentLevels.DELETE("/:id", app.perm("agent_level:delete"), app.agentLevelHandler.Delete)
		}

		// 实名认证（doc104 §5.7）：整单审核 + 三方核验 + 服务商与配置维护。
		// 审核类接口要求 verification:audit（通过/驳回/撤销/三方核验都改状态），
		// 只读列表与详情用 verification:list/detail；服务商与策略配置用 verification:config。
		if app.verification != nil {
			verifications := v1.Group("/verifications")
			verifications.Use(app.adminAuth())
			{
				verifications.GET("/pending", app.perm("verification:list"), app.verification.adminHandler.ListPending)
				verifications.GET("/approved", app.perm("verification:list"), app.verification.adminHandler.ListApproved)
				verifications.GET("/rejected", app.perm("verification:list"), app.verification.adminHandler.ListRejected)

				verifications.GET("/configs", app.perm("verification:config"), app.verification.adminHandler.ListConfigs)
				verifications.POST("/configs", app.perm("verification:config"), app.verification.adminHandler.UpsertConfig)
				verifications.DELETE("/configs/:id", app.perm("verification:config"), app.verification.adminHandler.DeleteConfig)
				verifications.GET("/provider-types", app.perm("verification:config"), app.verification.adminHandler.ProviderTypes)
				verifications.GET("/providers", app.perm("verification:config"), app.verification.adminHandler.ListProviders)
				verifications.POST("/providers", app.perm("verification:config"), app.verification.adminHandler.UpsertProvider)
				verifications.DELETE("/providers/:id", app.perm("verification:config"), app.verification.adminHandler.DeleteProvider)
				// 连通性测试：无论成功失败都返回 HTTP 200 + {ok,message}，
				// 让前端把失败原因直接展示给运营，而不是落进通用错误提示。
				verifications.POST("/providers/:id/test", app.perm("verification:config"), app.verification.adminHandler.TestProvider)
				// 审核员下载材料（doc104 §5.7）：与用户端同一条服务逻辑，
				// 只是 manageView=true 允许取任意申请下的材料。
				verifications.GET("/documents/:id/download", app.perm("verification:detail"), app.verification.adminHandler.DownloadDocument)

				// 静态段与动态段在 Gin 里是兄弟节点，注册顺序不影响匹配；
				// 这里仍按「静态在前、动态在后」排列，便于人工核对路径全集。
				verifications.GET("/:id", app.perm("verification:detail"), app.verification.adminHandler.Detail)
				verifications.GET("/:id/logs", app.perm("verification:detail"), app.verification.adminHandler.ReviewLogs)
				verifications.POST("/:id/approve", app.perm("verification:audit"), app.verification.adminHandler.Approve)
				verifications.POST("/:id/reject", app.perm("verification:audit"), app.verification.adminHandler.Reject)
				verifications.POST("/:id/revoke", app.perm("verification:audit"), app.verification.adminHandler.Revoke)
				verifications.POST("/:id/provider-check", app.perm("verification:audit"), app.verification.adminHandler.ProviderCheck)
			}
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
			// 只读：菜单树是 seed（db.SeedMenus）的投影，管理端写接口已下线（doc102 M0），
			// 运营改菜单名/层级需改代码并发版。
			// /tree 只要求登录态，不加 system:menu —— 它是所有管理员侧边栏的数据源，
			// 服务端已按当前管理员权限过滤（service.FilterByPermissions），不会返回无权限的
			// 菜单项；若在此再要求 system:menu，未持有该码的角色（ops_admin / finance_admin /
			// support 等）会吃到 403，前端随即降级渲染「不过滤」的静态 navMenu，
			// 反而把全部菜单暴露出来（doc102 §4.3 的原建议即为此，已按实测推翻）。
			menus.GET("/tree", app.menuHandler.Tree)
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
			security.GET("/risk-events/stats", app.perm("security:risk:list"), app.securityHandler.CountRiskEvents)
			security.GET("/risk-events/:id", app.perm("security:risk:list"), app.securityHandler.GetRiskEvent)
			security.POST("/risk-events/:id/ignore", app.perm("security:risk:list"), app.securityHandler.IgnoreRiskEvent)
			security.POST("/risk-events/:id/handle", app.perm("security:risk:list"), app.securityHandler.HandleRiskEvent)
			security.POST("/risk-events/:id/level", app.perm("security:risk:list"), app.securityHandler.UpdateRiskEventLevel)
			security.POST("/risk-events/:id/blacklist", app.perm("security:blacklist:manage"), app.securityHandler.CreateBlacklistFromRisk)
			security.POST("/risk-events/:id/revoke-sessions", app.perm("security:session:manage"), app.securityHandler.RevokeSessionsFromRisk)
			// 处置时间线：详情抽屉显示「谁在什么时候做了什么」。
			security.GET("/risk-events/:id/actions", app.perm("security:risk:list"), app.securityHandler.ListRiskEventActions)
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
				// 关联数据详情：删除前弹窗列出该渠道下的商品与资源池，让运营看清删的是什么。
				providers.GET("/:id/associations", app.perm("provider:delete"), app.providerHandler.Associations)
				// 连带删除（渠道 + 未被销售商品引用的商品 + 资源池）；有引用时拒绝并说明是哪些。
				providers.DELETE("/:id/cascade", app.perm("provider:delete"), app.providerHandler.DeleteCascade)
				providers.POST("/:id/test", app.perm("provider:test"), app.providerHandler.TestConnection)
				providers.POST("/:id/sync/resume", app.perm("provider:update"), app.providerHandler.ResumeSync)
				// 平台可售资源目录（区域/节点/存储/镜像）：自营规格模板配置平台参数用。
				providers.GET("/:id/resources", app.perm("resource:provider"), app.providerHandler.PlatformResources)
			}

			// 资源池
			pools := resourceGroup.Group("/pools")
			{
				pools.GET("", app.perm("resource:provider"), app.providerHandler.ListPools)
				pools.GET("/:id", app.perm("resource:provider"), app.providerHandler.GetPool)
				// 删除用于清理演示/历史脏数据；在线渠道的池会在下次同步重新出现，前端已提示。
				pools.DELETE("/:id", app.perm("resource:provider"), app.providerHandler.DeletePool)
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
			// 对账差异台账（doc120）：把上面的比对结论持久化，供预警与持续时长追踪。
			// /findings 固定段先于可能的通配段注册。
			reconcileGroup := resourceGroup.Group("/reconcile/findings")
			{
				reconcileGroup.GET("", app.perm("resource:instance"), app.reconcileFindingHandler.ListFindings)
				reconcileGroup.PUT("/:id", app.perm("resource:instance"), app.reconcileFindingHandler.UpdateFinding)
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

		// 实例运维台（管理端跨用户）：/admin/instances（见 docs/实施计划/61-实例运维管理台实施计划.md）
		// 同样遵循"静态段先于 /:id 注册"，故本组置于生命周期组之后。
		instanceOps := v1.Group("/instances")
		instanceOps.Use(app.adminAuth())
		{
			instanceOps.GET("", app.perm("resource:instance"), app.instanceOpsHandler.List)
			instanceOps.GET("/stats", app.perm("resource:instance"), app.instanceOpsHandler.Stats)
			// 全局操作流水（跨实例审计视图）：静态段须先于 /:id。
			instanceOps.GET("/operations", app.perm("resource:instance"), app.instanceOpsHandler.OperationLogs)
			instanceOps.GET("/:id", app.perm("resource:instance"), app.instanceOpsHandler.Detail)
			instanceOps.GET("/:id/operations", app.perm("resource:instance"), app.instanceOpsHandler.Operations)
			instanceOps.GET("/:id/related", app.perm("resource:instance"), app.instanceOpsHandler.Related)
			instanceOps.POST("/:id/sync", app.perm("instance:action"), app.instanceOpsHandler.Sync)
			// 重新开通（待开通占位实例专用）：重投来源订单的履约任务，
			// 上游充值后点一次即可复用已生成账单完成开通，不会重复下单。
			instanceOps.POST("/:id/provision-retry", app.perm("instance:action"), app.instanceOpsHandler.RetryProvision)
			instanceOps.POST("/:id/power", app.perm("instance:action"), app.instanceOpsHandler.Power)
			// 批量运维（doc61 P1）：静态段 /batch 须先于 /:id 注册。
			instanceOps.POST("/batch", app.perm("instance:action"), app.instanceOpsHandler.BatchAction)
			// 批量销毁：独立权限码 + 二次验证（与单实例销毁同场景），
			// 且服务层要求逐台确认文本 —— "勾了全选"不能变成"一键删机器"。
			instanceOps.POST("/batch-destroy", app.perm("instance:destroy"), app.adminRequireVerification("instance_destroy"), app.instanceOpsHandler.BatchDestroy)
			instanceOps.POST("/:id/suspend", app.perm("instance:action"), app.instanceOpsHandler.Suspend)
			instanceOps.POST("/:id/unsuspend", app.perm("instance:action"), app.instanceOpsHandler.Unsuspend)
			instanceOps.PUT("/:id/remark", app.perm("instance:action"), app.instanceOpsHandler.SetRemark)
			instanceOps.POST("/:id/vnc", app.perm("instance:console"), app.instanceOpsHandler.VNC)
			instanceOps.POST("/:id/resize", app.perm("instance:resize"), app.adminRequireVerification("instance_resize"), app.instanceOpsHandler.Resize)
			// 维护类操作（平台接口实测存在）：各拆独立权限码，避免"能重启就能重装"。
			// 会丢失数据的动作（重装格式化 / 快照恢复）另加二次验证。
			instanceOps.POST("/:id/reinstall", app.perm("instance:reinstall"), app.adminRequireVerification("instance_reinstall"), app.instanceOpsHandler.Reinstall)
			instanceOps.POST("/:id/reset-password", app.perm("instance:password"), app.instanceOpsHandler.ResetPassword)
			instanceOps.POST("/:id/rescue", app.perm("instance:rescue"), app.instanceOpsHandler.Rescue)
			instanceOps.POST("/:id/exit-rescue", app.perm("instance:rescue"), app.instanceOpsHandler.ExitRescue)
			instanceOps.GET("/:id/snapshots", app.perm("instance:snapshot"), app.instanceOpsHandler.Snapshots)
			instanceOps.POST("/:id/snapshots", app.perm("instance:snapshot"), app.instanceOpsHandler.CreateSnapshot)
			instanceOps.DELETE("/:id/snapshots/:snapshotId", app.perm("instance:snapshot"), app.instanceOpsHandler.DeleteSnapshot)
			instanceOps.POST("/:id/snapshots/restore", app.perm("instance:snapshot"), app.adminRequireVerification("instance_snapshot_restore"), app.instanceOpsHandler.RestoreSnapshot)
			instanceOps.PUT("/:id/bandwidth", app.perm("instance:hardware"), app.instanceOpsHandler.SetBandwidth)
			instanceOps.POST("/:id/ips", app.perm("instance:hardware"), app.instanceOpsHandler.AddIP)
			instanceOps.POST("/:id/disks", app.perm("instance:hardware"), app.instanceOpsHandler.AttachDataDisk)
			// 账期自定义（本地口径，不推送上游——上游无改到期时间/周期接口）。
			// 高危：改动直接影响续费/到期处置，需二次验证（复用 instance_resize 场景以外的独立场景码）。
			instanceOps.PUT("/:id/billing", app.perm("instance:billing"), app.adminRequireVerification("instance_billing"), app.instanceOpsHandler.AdjustBilling)
			// 账期调整上下文：可选周期以上游能力为准（避免选中上游不卖的周期）+ 各周期上游价/我方价。
			instanceOps.GET("/:id/billing-context", app.perm("instance:billing"), app.instanceOpsHandler.AdjustContext)
			// 按上游实例号重新关联（上游换机/换号）：改上游号 = 改变后续所有上游动作的作用对象，
			// 因此需独立权限码 + 二次验证，与销毁同档。
			instanceOps.POST("/:id/relink", app.perm("instance:relink"), app.adminRequireVerification("instance_relink"), app.instanceOpsHandler.RelinkUpstream)
			// 更换绑定的本地商品 / 上游镜像（上游换产品）：直接决定续费算价与配置口径，与账期同档。
			instanceOps.POST("/:id/rebind-product", app.perm("instance:billing"), app.instanceOpsHandler.RebindProduct)
			// 查看明文密码：只读但属敏感信息访问，独立权限码便于收窄。
			instanceOps.POST("/:id/credential", app.perm("instance:credential"), app.instanceOpsHandler.RevealCredential)
			instanceOps.DELETE("/:id", app.perm("instance:destroy"), app.adminRequireVerification("instance_destroy"), app.instanceOpsHandler.Destroy)
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
			// 到期处置（doc61 §8.4）：预演只读；手动单实例执行属高危写，需二次验证。
			// 复用既有 instance_destroy 场景（处置可能含销毁），不新增验证场景码。
			lifecycleGroup.GET("/enforcement/preview", app.perm("lifecycle:enforce"), app.lifecycleAdminHandler.PreviewEnforcement)
			lifecycleGroup.POST("/enforcement/:id/run", app.perm("lifecycle:enforce"), app.adminRequireVerification("instance_destroy"), app.lifecycleAdminHandler.EnforceInstance)
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
				// 上游商品页调价：设定加价规则并重算其已导入商品的售价（静态路由须在 /:id 之前）。
				prodProducts.POST("/upstream-markup", app.perm("product:price:update"), app.prodCatalogHandler.ApplyUpstreamMarkup)
				// 批量操作（产品列表页多选）：逐条复用单条逻辑，失败逐条回报。
				// 上架门禁的显式放行项（仅透传）：单独开关 + 留痕，避免用改商品的其他字段顺带打开。
				prodProducts.POST("/:id/spec-passthrough", app.perm("product:update"), app.prodCatalogHandler.SetSpecPassthrough)
				prodProducts.POST("/batch-publish", app.perm("product:publish"), app.prodCatalogHandler.BatchPublishProducts)
				prodProducts.POST("/batch-unpublish", app.perm("product:publish"), app.prodCatalogHandler.BatchUnpublishProducts)
				prodProducts.POST("/batch-delete", app.perm("product:delete"), app.prodCatalogHandler.BatchDeleteProducts)
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
				// 自营链路打通：按规格模板生成 SKU 并同步落平台绑定（模板含平台写参数）。
				prodProducts.POST("/:id/specs/generate", app.perm("product:update"), app.prodCatalogHandler.GenerateSpecFromTemplate)
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

				// 平台配置项目录与取值库（T4.5 规格配置化）：
				// 按对接平台（魔方云）声明可配置项，运营改默认值/必选并维护可选值（含镜像批量入库）。
				// 静态路由须排在 /option-catalog/:id 之前，避免被参数路由吞并。
				specGroup.GET("/option-catalog", app.perm("spec:option:list"), app.specHandler.ListOptionCatalog)
				specGroup.POST("/option-catalog", app.perm("spec:option:update"), app.specHandler.CreateOptionSpec)
				specGroup.POST("/option-catalog/sync", app.perm("spec:option:update"), app.specHandler.SyncOptionCatalog)
				specGroup.PUT("/option-catalog/:id", app.perm("spec:option:update"), app.specHandler.UpdateOptionSpec)
				specGroup.DELETE("/option-catalog/:id", app.perm("spec:option:update"), app.specHandler.DeleteOptionSpec)
				specGroup.GET("/option-values", app.perm("spec:option:list"), app.specHandler.ListOptionValues)
				specGroup.POST("/option-values", app.perm("spec:option:update"), app.specHandler.UpsertOptionValue)
				specGroup.POST("/option-values/import", app.perm("spec:option:update"), app.specHandler.ImportOptionValues)
				specGroup.POST("/option-values/refresh", app.perm("spec:option:update"), app.specHandler.RefreshOptionValues)
				specGroup.DELETE("/option-values/:id", app.perm("spec:option:update"), app.specHandler.DeleteOptionValue)
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
				// 限时活动折扣（doc108 §8J）：面向**普通用户**的营销折扣，真实参与算价。
				// 与代理拿货折扣（agent-levels）是两条互不叠加的线：代理用户不参与活动折扣。
				flashDiscounts := promoGroup.Group("/flash-discounts")
				{
					flashDiscounts.GET("", app.perm("promotion:activity:list"), app.flashDiscountHandler.List)
					flashDiscounts.POST("", app.perm("promotion:activity:update"), app.flashDiscountHandler.Create)
					flashDiscounts.GET("/:id", app.perm("promotion:activity:list"), app.flashDiscountHandler.Get)
					flashDiscounts.PUT("/:id", app.perm("promotion:activity:update"), app.flashDiscountHandler.Update)
					flashDiscounts.DELETE("/:id", app.perm("promotion:activity:update"), app.flashDiscountHandler.Delete)
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

		// 工单支持（doc50；S2/S3 部门数据范围与双人复核）
		ticketGroup := v1.Group("/tickets")
		ticketGroup.Use(app.adminAuth())
		{
			ticketGroup.GET("", app.perm("ticket:list"), app.ticketHandler.List)
			// 复核队列必须先于 /:id 注册，否则被通配路径吞掉。
			ticketGroup.GET("/reviews", app.perm("ticket:review"), app.ticketHandler.ListReviews)
			ticketGroup.GET("/stats", app.perm("ticket:stats"), app.ticketHandler.Stats)
			ticketGroup.GET("/attachments/:id/download", app.perm("ticket:view"), app.ticketHandler.DownloadAttachment)
			ticketGroup.POST("/replies/:replyId/review", app.perm("ticket:review"), app.ticketHandler.ReviewReply)
			ticketGroup.GET("/:id", app.perm("ticket:view"), app.ticketHandler.Get)
			ticketGroup.POST("/:id/reply", app.perm("ticket:reply"), app.ticketHandler.Reply)
			ticketGroup.POST("/:id/attachments", app.perm("ticket:reply"), app.ticketHandler.UploadAttachment)
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
			// 财务统计与参数（本轮整理）：总览与报表共用同一聚合口径，避免各自前端累加；
			// 参数接口按白名单读写财务键，财务角色无需 system:config:*。
			financeGroup.GET("/stats", app.perm("finance:wallet"), app.financeStatsHandler.Stats)
			financeGroup.GET("/settings", app.perm("finance:config"), app.financeSettingsHandler.List)
			financeGroup.PUT("/settings", app.perm("finance:config"), app.financeSettingsHandler.Update)
			// 钱包/流水
			financeGroup.GET("/wallets/:user_id", app.perm("finance:wallet"), app.walletHandler.Balance)
			financeGroup.GET("/transactions", app.perm("finance:wallet"), app.walletHandler.ListTransactions)
			// 明细导出（CSV）：与列表同筛选条件，条数封顶 20000。
			financeGroup.GET("/transactions/export", app.perm("finance:wallet"), app.walletHandler.ExportTransactions)
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

		// 成本管理（doc111）：月度成本/利润核算、成本项配置、上游余额台账
		costGroup := v1.Group("/cost")
		costGroup.Use(app.adminAuth())
		{
			// 成本总览：月度收入/成本/利润与利润率、成本构成、近 12 月趋势
			costGroup.GET("/overview", app.perm("finance:cost:overview"), app.costHandler.Overview)
			// 成本项配置（母机月费 / 人力 / 机房带宽…）
			costGroup.GET("/items", app.perm("finance:cost:item"), app.costHandler.ListItems)
			costGroup.POST("/items", app.perm("finance:cost:item"), app.costHandler.CreateItem)
			costGroup.PUT("/items/:id", app.perm("finance:cost:item"), app.costHandler.UpdateItem)
			costGroup.DELETE("/items/:id", app.perm("finance:cost:item"), app.costHandler.DeleteItem)
			// 上游余额台账（doc111 §5）：余额=自动抓取快照，消耗/充值=上游账本流水，全部自动取数
			costGroup.GET("/balances", app.perm("finance:cost:balance"), app.costHandler.Ledger)
			// 一键抓取渠道余额（适配器未实现 AccountReader 时返回 30008）
			costGroup.POST("/balances/fetch", app.perm("finance:cost:balance"), app.costHandler.FetchBalance)
			// 上游账本（doc111 §5.2）：消费/充值流水同步与明细、余额水位告警
			costGroup.POST("/balances/sync-ledger", app.perm("finance:cost:balance"), app.costHandler.SyncLedger)
			costGroup.GET("/balances/entries", app.perm("finance:cost:balance"), app.costHandler.ListLedgerEntries)
			costGroup.GET("/balances/alerts", app.perm("finance:cost:balance"), app.costHandler.BalanceAlerts)
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

		// 销售（客户归属 / 提成台账 / 提成提现 / 业绩排行，doc86 §2.3）
		salesGroup := v1.Group("/sales")
		salesGroup.Use(app.adminAuth())
		{
			salesGroup.GET("/customers", app.perm("sales:customer:list"), app.salesCustomerHandler.List)
			salesGroup.GET("/customers/unassigned", app.perm("sales:customer:assign"), app.salesCustomerHandler.Unassigned)
			salesGroup.POST("/customers/assign", app.perm("sales:customer:assign"), app.salesCustomerHandler.Assign)
			salesGroup.POST("/customers/release", app.perm("sales:customer:assign"), app.salesCustomerHandler.Release)
			salesGroup.GET("/customers/:userID/relations", app.perm("sales:customer:list"), app.salesCustomerHandler.Relations)
			salesGroup.GET("/sales-candidates", app.perm("sales:customer:assign"), app.salesCustomerHandler.Candidates)

			salesGroup.GET("/commissions", app.perm("sales:commission:list"), app.salesCommissionHandler.List)
			salesGroup.GET("/commissions/summary", app.perm("sales:commission:list"), app.salesCommissionHandler.Summary)
			salesGroup.GET("/withdrawals", app.perm("sales:commission:list"), app.salesCommissionHandler.Withdrawals)
			salesGroup.POST("/withdrawals", app.perm("sales:commission:list"), app.salesCommissionHandler.ApplyWithdrawal)
			salesGroup.POST("/withdrawals/:id/approve", app.perm("sales:commission:audit"), app.salesCommissionHandler.ApproveWithdrawal)
			salesGroup.POST("/withdrawals/:id/reject", app.perm("sales:commission:audit"), app.salesCommissionHandler.RejectWithdrawal)
			salesGroup.POST("/withdrawals/:id/pay", app.perm("sales:commission:settle"), app.adminRequireVerification("payout_apply"), app.salesCommissionHandler.PayWithdrawal)

			salesGroup.GET("/performance/ranking", app.perm("sales:performance:view"), app.salesPerformanceHandler.Ranking)
			salesGroup.GET("/performance/targets", app.perm("sales:performance:view"), app.salesPerformanceHandler.Targets)
			salesGroup.PUT("/performance/targets", app.perm("sales:performance:view"), app.salesPerformanceHandler.UpsertTargets)
			salesGroup.GET("/performance/me", app.perm("sales:commission:list"), app.salesPerformanceHandler.Me)
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

		// 内容中心（doc100 §7.1）：文章 / 分类 / 友情链接。
		// 与公告分开的原因：公告带「平台定向 / 等级 / 弹窗」的站内消息语义，归消息中心；
		// 这里只服务门户侧展示型内容。两套权限码也分开，避免「能发公告」隐含「能改条款」。
		if app.content != nil {
			contentArticles := v1.Group("/content/articles")
			contentArticles.Use(app.adminAuth(), app.adminAudit())
			{
				contentArticles.GET("", app.perm("content:article:list"), app.content.articleHandler.List)
				contentArticles.POST("", app.perm("content:article:manage"), app.content.articleHandler.Create)
				contentArticles.GET("/:id", app.perm("content:article:list"), app.content.articleHandler.Get)
				contentArticles.PUT("/:id", app.perm("content:article:manage"), app.content.articleHandler.Update)
				contentArticles.POST("/:id/publish", app.perm("content:article:manage"), app.content.articleHandler.Publish)
				contentArticles.POST("/:id/offline", app.perm("content:article:manage"), app.content.articleHandler.Offline)
				contentArticles.DELETE("/:id", app.perm("content:article:manage"), app.content.articleHandler.Delete)
			}

			contentCategories := v1.Group("/content/categories")
			contentCategories.Use(app.adminAuth(), app.adminAudit())
			{
				contentCategories.GET("", app.perm("content:category:list"), app.content.categoryHandler.List)
				contentCategories.POST("", app.perm("content:category:manage"), app.content.categoryHandler.Create)
				contentCategories.PUT("/:id", app.perm("content:category:manage"), app.content.categoryHandler.Update)
				contentCategories.DELETE("/:id", app.perm("content:category:manage"), app.content.categoryHandler.Delete)
			}

			contentLinks := v1.Group("/content/links")
			contentLinks.Use(app.adminAuth(), app.adminAudit())
			{
				contentLinks.GET("", app.perm("content:link:list"), app.content.linkHandler.List)
				contentLinks.POST("", app.perm("content:link:manage"), app.content.linkHandler.Create)
				contentLinks.PUT("/:id", app.perm("content:link:manage"), app.content.linkHandler.Update)
				contentLinks.DELETE("/:id", app.perm("content:link:manage"), app.content.linkHandler.Delete)
			}
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

		// 验证码配置（doc91 §8.1/§10.2）：服务商 / 场景策略 / 统计。
		// 固定路径（policies、stats、providers）与 /providers/:id 不同段数，无匹配歧义。
		captchaGroup := v1.Group("/captcha")
		captchaGroup.Use(app.adminAuth())
		{
			captchaGroup.GET("/providers/types", app.perm("captcha:config"), app.captcha.adminHandler.ListProviderTypes)
			captchaGroup.GET("/providers", app.perm("captcha:config"), app.captcha.adminHandler.ListProviders)
			captchaGroup.POST("/providers", app.perm("captcha:config:manage"), app.captcha.adminHandler.CreateProvider)
			captchaGroup.PUT("/providers/:id", app.perm("captcha:config:manage"), app.captcha.adminHandler.UpdateProvider)
			captchaGroup.DELETE("/providers/:id", app.perm("captcha:config:manage"), app.captcha.adminHandler.DeleteProvider)
			captchaGroup.POST("/providers/:id/test", app.perm("captcha:config"), app.captcha.adminHandler.TestProvider)
			captchaGroup.GET("/policies", app.perm("captcha:config"), app.captcha.adminHandler.ListPolicies)
			captchaGroup.PUT("/policies/:scene", app.perm("captcha:config:manage"), app.captcha.adminHandler.UpdatePolicy)
			captchaGroup.GET("/stats", app.perm("captcha:config"), app.captcha.adminHandler.Stats)
		}

		// 消息中心 - 渠道 / 短信模板 / 群发 / 发送日志（doc90 §3.4）。
		//
		// 既有三个平级前缀（/announcements、/notifications、/notification-templates）保持不变，
		// 本轮新增接口统一挂 /notification/*；7 个固定段必须注册在任何通配段之前，
		// 否则 Gin 会因通配冲突 panic（如 /channels 与 /channels/:id 的先后）。
		adminNotify := v1.Group("/notification")
		adminNotify.Use(app.adminAuth())
		{
			// 渠道类型与渠道实例
			adminNotify.GET("/channel-types", app.perm("notify:channel"), app.notify.channelHandler.ListTypes)
			adminNotify.GET("/channels", app.perm("notify:channel"), app.notify.channelHandler.List)
			adminNotify.POST("/channels", app.perm("notify:channel:manage"), app.notify.channelHandler.Create)
			adminNotify.GET("/channels/:id", app.perm("notify:channel"), app.notify.channelHandler.Get)
			adminNotify.PUT("/channels/:id", app.perm("notify:channel:manage"), app.notify.channelHandler.Update)
			adminNotify.PATCH("/channels/:id/status", app.perm("notify:channel:manage"), app.notify.channelHandler.UpdateStatus)
			adminNotify.POST("/channels/:id/test", app.perm("notify:channel"), app.notify.channelHandler.Test)
			// 短信 / 邮箱测试发送（统一入口，频控 10 次/分钟/管理员）
			adminNotify.POST("/test-send", app.perm("notify:channel"), app.notify.channelHandler.TestSend)

			// 短信模板
			adminNotify.POST("/sms-templates/preview", app.perm("notify:sms-template"), app.notify.smsTemplateHandler.Preview)
			adminNotify.GET("/sms-templates", app.perm("notify:sms-template"), app.notify.smsTemplateHandler.List)
			adminNotify.POST("/sms-templates", app.perm("notify:sms-template:manage"), app.notify.smsTemplateHandler.Create)
			adminNotify.GET("/sms-templates/:id", app.perm("notify:sms-template"), app.notify.smsTemplateHandler.Get)
			adminNotify.PUT("/sms-templates/:id", app.perm("notify:sms-template:manage"), app.notify.smsTemplateHandler.Update)
			adminNotify.DELETE("/sms-templates/:id", app.perm("notify:sms-template:manage"), app.notify.smsTemplateHandler.Delete)

			// 模板变量注册表（D7）
			adminNotify.GET("/template-vars", app.perm("notify:template"), app.notify.smsTemplateHandler.ListVars)
			adminNotify.POST("/template-vars", app.perm("notify:template"), app.notify.smsTemplateHandler.CreateVar)
			adminNotify.PUT("/template-vars/:id", app.perm("notify:template"), app.notify.smsTemplateHandler.UpdateVar)
			adminNotify.DELETE("/template-vars/:id", app.perm("notify:template"), app.notify.smsTemplateHandler.DeleteVar)

			// 消息群发（目标检索 / 预览 / 发送）
			adminNotify.GET("/broadcast/targets", app.perm("notify:broadcast"), app.notify.broadcastHandler.Targets)
			adminNotify.POST("/broadcast/preview", app.perm("notify:broadcast"), app.notify.broadcastHandler.Preview)
			adminNotify.POST("/broadcast", app.perm("notify:broadcast"), app.notify.broadcastHandler.Send)

			// 发送日志（投递记录）：batch-retry 必须注册在 /:id/retry 之前
			adminNotify.POST("/deliveries/batch-retry", app.perm("notify:delivery"), app.notify.deliveryHandler.BatchRetry)
			adminNotify.GET("/deliveries", app.perm("notify:delivery"), app.notify.deliveryHandler.List)
			adminNotify.GET("/deliveries/:id", app.perm("notify:delivery"), app.notify.deliveryHandler.Get)
			adminNotify.POST("/deliveries/:id/retry", app.perm("notify:delivery"), app.notify.deliveryHandler.Retry)
		}

		// 日志中心（doc92 §9.2）：统一日志浏览 / 导出 / 保留策略 / 清理任务。
		//
		// 26 个源共用一套页面，靠 GET /logs/catalog 下发的列元数据驱动渲染。
		// 权限码与文档一致：log:center 读、log:export 导出下载、log:cleanup 清理、log:policy 策略。
		// 路由顺序硬约束：所有固定段（catalog/stats/query/export/export-files/
		// policies/cleanup/cleanup-jobs）必须先注册，最后才注册带通配段的
		// /cleanup-jobs/:id 与 /:source/detail/:id，否则 Gin 通配冲突 panic。
		if app.logcenter != nil {
			logs := v1.Group("/logs")
			logs.Use(app.adminAuth())
			{
				// 目录 / 统计 / 查询（固定段）
				logs.GET("/catalog", app.perm("log:center"), app.logcenter.handler.Catalog)
				logs.GET("/stats", app.perm("log:center"), app.logcenter.handler.Stats)
				logs.GET("/query", app.perm("log:center"), app.logcenter.handler.Query)

				// 导出（导出即留痕；下载同权限，写操作由全局审计中间件记录）
				logs.POST("/export", app.perm("log:export"), app.logcenter.handler.Export)
				logs.GET("/export-files", app.perm("log:center"), app.logcenter.handler.ListExportFiles)
				logs.GET("/export-files/:id/download", app.perm("log:export"), app.logcenter.handler.DownloadExportFile)
				logs.DELETE("/export-files/:id", app.perm("log:cleanup"), app.logcenter.handler.DeleteExportFile)

				// 保留策略
				logs.GET("/policies", app.perm("log:policy"), app.logcenter.handler.ListPolicies)
				logs.PUT("/policies/:source", app.perm("log:cleanup"), app.logcenter.handler.UpdatePolicy)

				// 清理：预演与执行分离，执行需 DELETE 确认文本
				logs.POST("/cleanup/preview", app.perm("log:cleanup"), app.logcenter.handler.PreviewCleanup)
				logs.POST("/cleanup", app.perm("log:cleanup"), app.logcenter.handler.RunCleanup)
				logs.GET("/cleanup-jobs", app.perm("log:center"), app.logcenter.handler.ListCleanupJobs)

				// 通配段收尾注册
				logs.GET("/cleanup-jobs/:id", app.perm("log:center"), app.logcenter.handler.GetCleanupJob)
				logs.POST("/cleanup-jobs/:id/cancel", app.perm("log:cleanup"), app.logcenter.handler.CancelCleanupJob)
				logs.GET("/:source/detail/:id", app.perm("log:center"), app.logcenter.handler.Detail)
			}
		}
	}

	// 第三方登录（doc104 §6）：登录页授权入口与免登录回调在 registerOAuthRoutes 内注册，
	// 该函数同时注册管理端 /admin/oauth 与用户端 /api/v1/uc/oauth 两端路由。
	registerOAuthRoutes(v1, r.Group("/api/v1/uc"), app.oauth, app)
}
