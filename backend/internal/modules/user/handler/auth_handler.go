package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/user/dto"
	"hostsent/backend/internal/modules/user/service"
	"hostsent/backend/internal/pkg/middleware"
)

type AuthHandler struct {
	authService service.AuthService
}

func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Login godoc
// @Summary 用户登录
// @Description 使用用户名和密码登录并获取 JWT
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "登录参数"
// @Success 200 {object} map[string]any
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}

	resp, err := h.authService.Login(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": resp, "timestamp": time.Now().Unix()})
}

// Me godoc
// @Summary 当前用户信息
// @Description 通过 JWT 获取当前登录用户信息
// @Tags 认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/v1/auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	claims, ok := middleware.GetClaims(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 10001, "message": "unauthorized", "timestamp": time.Now().Unix()})
		return
	}

	resp, err := h.authService.Me(c.Request.Context(), claims.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": resp, "timestamp": time.Now().Unix()})
}
