package server

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	menuhandler "hostsent/backend/internal/modules/menu/handler"
	"hostsent/backend/internal/modules/user/handler"
	appauth "hostsent/backend/internal/pkg/auth"
	"hostsent/backend/internal/pkg/config"
	"hostsent/backend/internal/pkg/middleware"
)

func newRouter(cfg *config.Config, authHandler *handler.AuthHandler, userHandler *handler.UserHandler, roleHandler *handler.RoleHandler, permissionHandler *handler.PermissionHandler, menuHandler *menuhandler.MenuHandler, logger *zap.Logger, jwtIssuer *appauth.JWTIssuer) *gin.Engine {
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
			users.GET(":id", userHandler.GetUser)
			users.PUT(":id", userHandler.UpdateUser)
			users.PATCH(":id/status", userHandler.UpdateUserStatus)
			users.POST(":id/reset-password", userHandler.ResetPassword)
			users.POST(":id/roles", userHandler.AssignRoles)
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
