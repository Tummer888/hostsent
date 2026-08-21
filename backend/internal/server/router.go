package server

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	distributionhandler "hostsent/backend/internal/modules/distribution/handler"
	menuhandler "hostsent/backend/internal/modules/menu/handler"
	quotahandler "hostsent/backend/internal/modules/quota/handler"
	securityhandler "hostsent/backend/internal/modules/security/handler"
	"hostsent/backend/internal/modules/user/handler"
	verificationhandler "hostsent/backend/internal/modules/verification/handler"
	appauth "hostsent/backend/internal/pkg/auth"
	"hostsent/backend/internal/pkg/config"
	"hostsent/backend/internal/pkg/middleware"
)

func newRouter(cfg *config.Config, authHandler *handler.AuthHandler, userHandler *handler.UserHandler, userDetailHandler *handler.UserDetailHandler, userGroupHandler *handler.UserGroupHandler, agentLevelHandler *distributionhandler.AgentLevelHandler, agentHandler *distributionhandler.AgentHandler, subordinateHandler *distributionhandler.SubordinateHandler, commissionHandler *distributionhandler.CommissionHandler, settlementHandler *distributionhandler.SettlementHandler, roleHandler *handler.RoleHandler, permissionHandler *handler.PermissionHandler, menuHandler *menuhandler.MenuHandler, securityHandler *securityhandler.SecurityHandler, resourceQuotaHandler *quotahandler.ResourceQuotaHandler, quotaTemplateHandler *quotahandler.QuotaTemplateHandler, quotaUserLevelHandler *quotahandler.UserLevelHandler, quotaAdjustmentHandler *quotahandler.QuotaAdjustmentHandler, verificationHandler *verificationhandler.VerificationHandler, logger *zap.Logger, jwtIssuer *appauth.JWTIssuer) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery(), middleware.Logger(logger), cors.Default())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"status": "ok"}})
	})
	r.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ready", "data": gin.H{"status": "ready"}})
	})

	v1 := r.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
			auth.GET("/me", middleware.Auth(jwtIssuer, cfg.Auth.BearerPrefix), authHandler.Me)
			auth.POST("/impersonate", middleware.Auth(jwtIssuer, cfg.Auth.BearerPrefix), authHandler.Impersonate)
		}

		users := v1.Group("/users")
		users.Use(middleware.Auth(jwtIssuer, cfg.Auth.BearerPrefix))
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
		userGroups.Use(middleware.Auth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			userGroups.GET("", userGroupHandler.List)
			userGroups.POST("", userGroupHandler.Create)
			userGroups.GET("/:id", userGroupHandler.Get)
			userGroups.PUT("/:id", userGroupHandler.Update)
			userGroups.DELETE("/:id", userGroupHandler.Delete)
		}

		agentLevels := v1.Group("/distribution/agent-levels")
		agentLevels.Use(middleware.Auth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			agentLevels.GET("", agentLevelHandler.List)
			agentLevels.POST("", agentLevelHandler.Create)
			agentLevels.GET("/:id", agentLevelHandler.Get)
			agentLevels.PUT("/:id", agentLevelHandler.Update)
			agentLevels.DELETE("/:id", agentLevelHandler.Delete)
		}

		agents := v1.Group("/distribution/agents")
		agents.Use(middleware.Auth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			agents.GET("", agentHandler.List)
			agents.POST("", agentHandler.Create)
			agents.GET("/:id", agentHandler.Get)
			agents.PUT("/:id", agentHandler.Update)
			agents.DELETE("/:id", agentHandler.Delete)
		}

		subordinates := v1.Group("/distribution/subordinates")
		subordinates.Use(middleware.Auth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			subordinates.GET("", subordinateHandler.List)
			subordinates.POST("", subordinateHandler.Create)
			subordinates.GET("/:id", subordinateHandler.Get)
			subordinates.PUT("/:id", subordinateHandler.Update)
			subordinates.DELETE("/:id", subordinateHandler.Delete)
		}

		commissions := v1.Group("/distribution/commissions")
		commissions.Use(middleware.Auth(jwtIssuer, cfg.Auth.BearerPrefix))
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
		settlements.Use(middleware.Auth(jwtIssuer, cfg.Auth.BearerPrefix))
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
		roles.Use(middleware.Auth(jwtIssuer, cfg.Auth.BearerPrefix))
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
		quotas.Use(middleware.Auth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			quotas.GET("", resourceQuotaHandler.List)
			quotas.GET("/:id", resourceQuotaHandler.Get)
			quotas.GET("/users/:user_id", resourceQuotaHandler.GetByUser)
			quotas.POST("/:id/adjust", resourceQuotaHandler.Adjust)
		}

		quotaTemplates := v1.Group("/quota-templates")
		quotaTemplates.Use(middleware.Auth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			quotaTemplates.GET("", quotaTemplateHandler.List)
			quotaTemplates.POST("", quotaTemplateHandler.Create)
			quotaTemplates.GET("/:id", quotaTemplateHandler.Get)
			quotaTemplates.PUT("/:id", quotaTemplateHandler.Update)
			quotaTemplates.DELETE("/:id", quotaTemplateHandler.Delete)
		}

		userLevels := v1.Group("/user-levels")
		userLevels.Use(middleware.Auth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			userLevels.GET("", quotaUserLevelHandler.List)
			userLevels.POST("", quotaUserLevelHandler.Create)
			userLevels.GET("/:id", quotaUserLevelHandler.Get)
			userLevels.PUT("/:id", quotaUserLevelHandler.Update)
			userLevels.DELETE("/:id", quotaUserLevelHandler.Delete)
			userLevels.POST("/:id/bind-template", quotaUserLevelHandler.BindTemplate)
		}

		quotaAdjustments := v1.Group("/quota-adjustments")
		quotaAdjustments.Use(middleware.Auth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			quotaAdjustments.GET("", quotaAdjustmentHandler.List)
			quotaAdjustments.GET("/:id", quotaAdjustmentHandler.Get)
		}

		verifications := v1.Group("/verifications")
		verifications.Use(middleware.Auth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			verifications.GET("/pending", verificationHandler.ListPending)
			verifications.GET("/approved", verificationHandler.ListApproved)
			verifications.GET("/rejected", verificationHandler.ListRejected)
		}

		permissions := v1.Group("/permissions")
		permissions.Use(middleware.Auth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			permissions.GET("/tree", permissionHandler.Tree)
			permissions.POST("", permissionHandler.CreatePermission)
			permissions.PUT("/:id", permissionHandler.UpdatePermission)
			permissions.DELETE("/:id", permissionHandler.DeletePermission)
		}

		menus := v1.Group("/menus")
		menus.Use(middleware.Auth(jwtIssuer, cfg.Auth.BearerPrefix))
		{
			menus.GET("/tree", menuHandler.Tree)
			menus.POST("", menuHandler.CreateMenu)
			menus.PUT("/:id", menuHandler.UpdateMenu)
			menus.DELETE("/:id", menuHandler.DeleteMenu)
		}

		security := v1.Group("/security")
		security.Use(middleware.Auth(jwtIssuer, cfg.Auth.BearerPrefix))
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
	}

	return r
}
