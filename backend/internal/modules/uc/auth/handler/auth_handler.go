package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/uc/auth/dto"
	"hostsent/backend/internal/modules/uc/auth/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/netutil"
	"hostsent/backend/internal/pkg/response"
	"hostsent/backend/internal/pkg/security"
)

// AuthHandler 用户中心认证 HTTP 处理器。
// 处理用户自助场景下的注册、登录、登出、信息查询等请求。
type AuthHandler struct {
	authService service.AuthService
}

// NewAuthHandler 创建用户中心认证处理器。
func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Login 用户登录
// @Summary 用户中心登录
// @Description 支持 password（默认）/ sms / email 三种登录方式；策略要求二次验证时返回 need_otp
// @Tags 用户中心-认证
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "登录参数"
// @Success 200 {object} response.Body{data=dto.LoginResponse}
// @Failure 400 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/uc/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}

	resp, err := h.authService.Login(c.Request.Context(), req, netutil.ClientIP(c), c.GetHeader("User-Agent"))
	if err != nil {
		writeSecurityError(c, err)
		return
	}

	response.Success(c, resp)
}

// VerifyLoginOTP 登录二次验证
// @Summary 登录二次验证
// @Description 用登录返回的 otp_token 换取正式访问令牌（doc91 §4.6）
// @Tags 用户中心-认证
// @Accept json
// @Produce json
// @Param request body dto.VerifyOTPRequest true "验证参数"
// @Success 200 {object} response.Body{data=dto.LoginResponse}
// @Router /api/v1/uc/auth/login/verify-otp [post]
func (h *AuthHandler) VerifyLoginOTP(c *gin.Context) {
	var req dto.VerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}

	resp, err := h.authService.VerifyLoginOTP(c.Request.Context(), req, netutil.ClientIP(c), c.GetHeader("User-Agent"))
	if err != nil {
		writeSecurityError(c, err)
		return
	}

	response.Success(c, resp)
}

// ForgotPassword 忘记密码
// @Summary 忘记密码（下发验证码）
// @Description 向账号绑定的邮箱/手机下发验证码；无论账号是否存在都返回 sent=true（防枚举）
// @Tags 用户中心-认证
// @Accept json
// @Produce json
// @Param request body dto.ForgotPasswordRequest true "找回参数"
// @Success 200 {object} response.Body{data=dto.ForgotPasswordResponse}
// @Router /api/v1/uc/auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req dto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}

	resp, err := h.authService.ForgotPassword(c.Request.Context(), req, netutil.ClientIP(c), c.GetHeader("User-Agent"))
	if err != nil {
		writeSecurityError(c, err)
		return
	}

	response.Success(c, resp)
}

// ResetPassword 重置密码
// @Summary 重置密码
// @Description 校验验证码后写入新密码并撤销历史会话
// @Tags 用户中心-认证
// @Accept json
// @Produce json
// @Param request body dto.ResetPasswordRequest true "重置参数"
// @Success 200 {object} response.Body
// @Router /api/v1/uc/auth/reset-password [post]
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}

	if err := h.authService.ResetPassword(c.Request.Context(), req, netutil.ClientIP(c), c.GetHeader("User-Agent")); err != nil {
		writeSecurityError(c, err)
		return
	}

	response.SuccessMessage(c, "密码重置成功")
}

// writeSecurityError 把安全域错误映射为业务错误码（20010/20011/20014/20015/20016 等）。
//
// 验证码错误、账号锁定属于正常业务反馈，用 200 业务码返回，避免前端与监控
// 把「用户输错验证码」当成服务故障（doc91 §3.4）。
func writeSecurityError(c *gin.Context, err error) {
	if code := security.CodeOf(err); code != 0 {
		response.Error(c, apperrors.New(code, err.Error()))
		return
	}
	response.Error(c, apperrors.New(50001, err.Error()))
}

// Register 用户注册
// @Summary 用户中心注册
// @Description 普通用户自助注册（图形码 + 邮箱验证码），返回新用户 ID
// @Tags 用户中心-认证
// @Accept json
// @Produce json
// @Param request body dto.RegisterRequest true "注册参数"
// @Success 200 {object} response.Body{data=map[string]uint64}
// @Failure 400 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/uc/auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}

	id, err := h.authService.Register(c.Request.Context(), req, netutil.ClientIP(c), c.GetHeader("User-Agent"))
	if err != nil {
		writeSecurityError(c, err)
		return
	}

	response.Success(c, gin.H{"id": id})
}

// UserInfo 获取当前登录用户信息
// @Summary 获取用户信息
// @Description 通过 JWT 获取当前登录用户的详细信息
// @Tags 用户中心-认证
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Body{data=dto.UserInfo}
// @Failure 401 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/uc/auth/userinfo [get]
func (h *AuthHandler) UserInfo(c *gin.Context) {
	claims, ok := middleware.GetUserClaims(c)
	if !ok {
		response.ErrorWithStatus(c, 401, apperrors.New(10001, "unauthorized"))
		return
	}

	userInfo, err := h.authService.UserInfo(c.Request.Context(), claims.UserID)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}

	response.Success(c, userInfo)
}

// Logout 用户登出
// @Summary 用户登出
// @Description JWT 为无状态，登出由前端清除 token，后端仅返回成功确认
// @Tags 用户中心-认证
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Body
// @Router /api/v1/uc/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	response.SuccessMessage(c, "登出成功")
}

// UpdateProfile 更新当前用户资料
// @Summary 更新用户资料
// @Description 更新当前登录用户的显示名、邮箱、手机、头像
// @Tags 用户中心-认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.UpdateProfileRequest true "资料参数"
// @Success 200 {object} response.Body{data=dto.UserInfo}
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/uc/auth/profile [put]
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	claims, ok := middleware.GetUserClaims(c)
	if !ok {
		response.ErrorWithStatus(c, 401, apperrors.New(10001, "unauthorized"))
		return
	}

	var req dto.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}

	userInfo, err := h.authService.UpdateProfile(c.Request.Context(), claims.UserID, req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}

	response.Success(c, userInfo)
}

// ChangePassword 修改当前用户密码
// @Summary 修改密码
// @Description 校验原密码后更新为新密码（新密码最短 6 位）
// @Tags 用户中心-认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.ChangePasswordRequest true "密码参数"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 401 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/uc/auth/password [put]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	claims, ok := middleware.GetUserClaims(c)
	if !ok {
		response.ErrorWithStatus(c, 401, apperrors.New(10001, "unauthorized"))
		return
	}

	var req dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}

	if err := h.authService.ChangePassword(c.Request.Context(), claims.UserID, req); err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}

	response.SuccessMessage(c, "密码修改成功")
}
