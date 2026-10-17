// 管理端路由——User域（自 routes_admin.go 拆分，批次三 C2）。

package server

import (
	"github.com/gin-gonic/gin"
)

func registerAdminUserRoutes(v1 *gin.RouterGroup, app *App) {
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
	auditGroup := v1.Group("/audit-logs")
	auditGroup.Use(app.adminAuth())
	{
		auditGroup.GET("", app.perm("security:audit:list"), app.adminHandler.ListAuditLogs)
	}
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
		users.POST("", app.perm("user:create"), app.userHandler.CreateUser)
		users.GET("/stats", app.perm("system:user:list"), app.userHandler.GetStats)
		// 总览页活动统计（在线用户 / 最近登录用户）。与 /stats 同权限，
		// 静态段同样必须注册在 /:id 之前。
		users.GET("/activity-overview", app.perm("system:user:list"), app.userHandler.GetActivityOverview)
		// 代下单可选项（规格/周期/选配项）：静态段须注册在 /:id 之前。
		users.GET("/order-config", app.perm("user:create"), app.userHandler.OrderConfig)
		// 子账号可选权限项（静态段，须注册在 /:id 之前）：客户侧权限固定枚举，
		// 与用户端 /uc/members 的 permission_options 同源，避免两处各写一份中文标签。
		users.GET("/member-permission-options", app.perm("user:detail"), app.userHandler.SubAccountPermissionOptions)
		users.GET(":id", app.perm("user:detail"), app.userHandler.GetUser)
		// 子账号（成员）管理（P4-10 管理面补全）：与客户侧同一套业务规则
		// （权限枚举、等级配额、唯一性校验、改密撤会话），只在归属判定上以路径上的
		// 主账号为准。写操作统一挂 user:update —— 与「改用户资料」同档权限。
		users.GET(":id/members", app.perm("user:detail"), app.userHandler.ListMembers)
		users.POST(":id/members", app.perm("user:update"), app.userHandler.CreateSubAccount)
		users.PUT(":id/members/:memberId", app.perm("user:update"), app.userHandler.UpdateSubAccount)
		users.PUT(":id/members/:memberId/permissions", app.perm("user:update"), app.userHandler.SetSubAccountPermissions)
		users.PUT(":id/members/:memberId/password", app.perm("user:update"), app.userHandler.ResetSubAccountPassword)
		users.DELETE(":id/members/:memberId", app.perm("user:update"), app.userHandler.DisableSubAccount)
		users.GET(":id/members/:memberId/logs", app.perm("user:detail"), app.userHandler.ListSubAccountLogs)
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
}
