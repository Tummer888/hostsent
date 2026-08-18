package server

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"hostsent/backend/internal/modules/user/handler"
	"hostsent/backend/internal/pkg/config"
	"hostsent/backend/internal/pkg/middleware"
)

func newRouter(cfg *config.Config, authHandler *handler.AuthHandler, logger *zap.Logger) *gin.Engine {
	r := gin.New()
	r.Use(middleware.Recovery(), middleware.Logger(logger))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"status": "ok"}, "timestamp": time.Now().Unix()})
	})

	r.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"status": "ready"}, "timestamp": time.Now().Unix()})
	})

	api := r.Group("/api/v1")
	authGroup := api.Group("/auth")
	authGroup.POST("/login", authHandler.Login)
	authGroup.GET("/me", middleware.Auth(cfg.Auth.MockToken), authHandler.Me)

	return r
}
