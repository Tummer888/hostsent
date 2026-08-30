package server

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	adminhandler "hostsent/backend/internal/modules/admin/manager/handler"
	menuhandler "hostsent/backend/internal/modules/admin/menu/handler"
	providerhandler "hostsent/backend/internal/modules/admin/resource/provider/handler"
	producthandler "hostsent/backend/internal/modules/admin/resource/product/handler"
	synchandler "hostsent/backend/internal/modules/admin/resource/sync/handler"
	"hostsent/backend/internal/modules/admin/user/account/handler"
	distributionhandler "hostsent/backend/internal/modules/admin/user/distribution/handler"
	quotahandler "hostsent/backend/internal/modules/admin/user/quota/handler"
	securityhandler "hostsent/backend/internal/modules/admin/user/security/handler"
	verificationhandler "hostsent/backend/internal/modules/admin/user/verification/handler"
	usercenterhandler "hostsent/backend/internal/modules/user/auth/handler"
	usermenuhandler "hostsent/backend/internal/modules/user/menu/handler"
	appauth "hostsent/backend/internal/pkg/auth"
	"hostsent/backend/internal/pkg/config"
	"hostsent/backend/internal/pkg/middleware"
)

func newRouter(cfg *config.Config, adminHandler *adminhandler.AdminHandler, userHandler *handler.UserHandler, userDetailHandler *handler.UserDetailHandler, userGroupHandler *handler.UserGroupHandler, agentLevelHandler *distributionhandler.AgentLevelHandler, agentHandler *distributionhandler.AgentHandler, subordinateHandler *distributionhandler.SubordinateHandler, commissionHandler *distributionhandler.CommissionHandler, settlementHandler *distributionhandler.SettlementHandler, roleHandler *handler.RoleHandler, permissionHandler *handler.PermissionHandler, menuHandler *menuhandler.MenuHandler, securityHandler *securityhandler.SecurityHandler, resourceQuotaHandler *quotahandler.ResourceQuotaHandler, quotaTemplateHandler *quotahandler.QuotaTemplateHandler, quotaUserLevelHandler *quotahandler.UserLevelHandler, quotaAdjustmentHandler *quotahandler.QuotaAdjustmentHandler, verificationHandler *verificationhandler.VerificationHandler, providerHandler *providerhandler.ProviderHandler, productHandler *producthandler.ProductHandler, syncHandler *synchandler.SyncHandler, userCenterAuthHandler *usercenterhandler.AuthHandler, userMenuHandler *usermenuhandler.MenuHandler, logger *zap.Logger, jwtIssuer *appauth.JWTIssuer) *gin.Engine {
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
	}

	// 用户中心（普通用户自助）：独立模块 internal/modules/user/auth
	ucAuth := r.Group("/api/v1/uc/auth")
	{
		ucAuth.POST("/login", userCenterAuthHandler.Login)    // 登录
		ucAuth.POST("/register", userCenterAuthHandler.Register) // 注册
		ucAuth.POST("/logout", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userCenterAuthHandler.Logout) // 登出
		ucAuth.GET("/userinfo", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userCenterAuthHandler.UserInfo) // 用户信息
		ucAuth.PUT("/profile", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userCenterAuthHandler.UpdateProfile) // 更新资料
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
		ucAuthCompat.PUT("/profile", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userCenterAuthHandler.UpdateProfile) // 更新资料
		ucAuthCompat.PUT("/password", middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix), userCenterAuthHandler.ChangePassword) // 修改密码
	}

	// 兼容 frontend-user 项目 baseURL=/api/v1 时的 /menus/tree 路径（同处理器）
	ucMenuCompat := r.Group("/api/v1/menus")
	ucMenuCompat.Use(middleware.UserAuth(jwtIssuer, cfg.Auth.BearerPrefix))
	{
		ucMenuCompat.GET("/tree", userMenuHandler.Tree) // 菜单树
	}

	return r
}
