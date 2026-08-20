package server

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	distributionhandler "hostsent/backend/internal/modules/distribution/handler"
	menuhandler "hostsent/backend/internal/modules/menu/handler"
	"hostsent/backend/internal/modules/user/handler"
	appauth "hostsent/backend/internal/pkg/auth"
	"hostsent/backend/internal/pkg/config"
	"hostsent/backend/internal/pkg/middleware"
)

func newRouter(cfg *config.Config, authHandler *handler.AuthHandler, userHandler *handler.UserHandler, userDetailHandler *handler.UserDetailHandler, userGroupHandler *handler.UserGroupHandler, agentLevelHandler *distributionhandler.AgentLevelHandler, agentHandler *distributionhandler.AgentHandler, subordinateHandler *distributionhandler.SubordinateHandler, commissionHandler *distributionhandler.CommissionHandler, settlementHandler *distributionhandler.SettlementHandler, roleHandler *handler.RoleHandler, permissionHandler *handler.PermissionHandler, menuHandler *menuhandler.MenuHandler, logger *zap.Logger, jwtIssuer *appauth.JWTIssuer) *gin.Engine {
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
	}

	return r
}
