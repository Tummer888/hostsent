// Package handler 提供验证码与二次验证体系的 HTTP 处理器（doc91 §3.3/§4.1/§4.5/§8）。
package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/uc/captcha/dto"
	"hostsent/backend/internal/modules/uc/captcha/model"
	"hostsent/backend/internal/modules/uc/captcha/service"
	captchapkg "hostsent/backend/internal/pkg/captcha"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/netutil"
	"hostsent/backend/internal/pkg/response"
)

// PublicHandler 公开接口（登录前使用，无需鉴权）。
type PublicHandler struct {
	svc *service.Service
}

// NewPublicHandler 创建公开处理器。
func NewPublicHandler(svc *service.Service) *PublicHandler {
	return &PublicHandler{svc: svc}
}

// AuthConfig godoc
// @Summary 登录方式配置
// @Tags 前台公开
// @Security BearerAuth
// @Success 200 {string} string "ok"
// @Router /api/v1/public/auth-config [get]
// AuthConfig `/public/auth-config`：前端据此决定是否渲染验证码。
//
// 这个接口是必需的：没有它，前端要么永远渲染验证码（默认关时丑且多余），
// 要么写死（改策略就要重新发版）。
func (h *PublicHandler) AuthConfig(c *gin.Context) {
	ctx := c.Request.Context()
	scenes := []string{
		model.SceneAdminLogin, model.SceneUserLogin, model.SceneUserLoginSMS,
		model.SceneUserLoginEmail, model.SceneUserRegister, model.ScenePasswordReset,
	}
	out := map[string]dto.ScenePolicyInfo{}
	for _, scene := range scenes {
		eff, err := h.svc.Policy().EffectivePolicy(ctx, scene, service.Subject{})
		if err != nil || eff == nil {
			out[scene] = dto.ScenePolicyInfo{}
			continue
		}
		out[scene] = dto.ScenePolicyInfo{
			ImageRequired: eff.ImageRequired,
			OTPRequired:   eff.OTPRequired,
			OTPChannel:    eff.OTPChannel,
		}
	}
	resp := dto.AuthConfigResponse{
		CaptchaEnabled: h.svc.Switches().CaptchaEnabled(ctx),
		Scenes:         out,
		ImageProvider:  h.svc.Switches().CaptchaProvider(ctx),
	}
	// 第三方图形码前端 SDK 参数（native 时为空）。
	if params, ok := h.svc.ThirdPartyFrontendParams(ctx); ok {
		resp.ImageProvider = params.Provider
		resp.ThirdParty = dto.ThirdPartyConfig{Provider: params.Provider, CaptchaID: params.CaptchaID}
	}
	response.Success(c, resp)
}

// ImageChallenge godoc
// @Summary 图形验证码
// @Tags 前台公开
// @Security BearerAuth
// @Param request body dto.SendCodeRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/public/captcha/image [get]
// ImageChallenge `GET /public/captcha/image`：服务端生成图形码。
func (h *PublicHandler) ImageChallenge(c *gin.Context) {
	scene := c.DefaultQuery("scene", model.SceneUserLogin)
	level := c.Query("level")
	ip := netutil.ClientIP(c)
	resp, err := h.svc.ImageChallenge(c.Request.Context(), scene, level, ip)
	if err != nil {
		var limited *service.ErrRateLimited
		if errors.As(err, &limited) {
			response.ErrorWithStatus(c, http.StatusTooManyRequests,
				apperrors.New(service.CodeImageRateLimited, "验证码获取过于频繁，请稍后再试"))
			return
		}
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// publicSendScenes 公开下发接口允许的场景（登录前的场景集合）。
//
// 已登录场景（password_change / withdraw_apply / instance_destroy 等）走
// /uc/security/verification/send——那里目标由账号绑定决定，前端传不进来。
// 若公开接口不设场景白名单，任何人都能对任意邮箱/手机号发起 OTP，
// 把「验证码下发」变成垃圾邮件/短信投递通道。
var publicSendScenes = map[string]bool{
	model.SceneUserRegister:   true,
	model.ScenePasswordReset:  true,
	model.SceneUserLoginSMS:   true,
	model.SceneUserLoginEmail: true,
	model.SceneUserLogin:      true,
	model.SceneRealnameSubmit: true,
}

// SendCode godoc
// @Summary 下发验证码
// @Tags 前台公开
// @Security BearerAuth
// @Param request body dto.SendCodeRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/public/verify-code/send [post]
// SendCode `POST /public/verify-code/send`：下发 OTP（强频控）。
func (h *PublicHandler) SendCode(c *gin.Context) {
	var req dto.SendCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	if !publicSendScenes[strings.TrimSpace(req.Scene)] {
		response.Error(c, apperrors.New(service.CodeSceneDisabled, service.ErrSceneDisabled.Error()))
		return
	}
	resp, err := h.svc.SendCode(c.Request.Context(), req, service.Subject{},
		0, netutil.ClientIP(c), c.GetHeader("User-Agent"))
	if err != nil {
		writeCaptchaError(c, err)
		return
	}
	response.Success(c, resp)
}

// writeCaptchaError 统一错误码映射（doc91 §3.4/§4.1）。
func writeCaptchaError(c *gin.Context, err error) {
	var limited *service.ErrRateLimited
	switch {
	case errors.As(err, &limited):
		if limited.Daily {
			response.Error(c, apperrors.New(service.CodeSendDailyLimit, limited.Error()))
			return
		}
		response.Error(c, apperrors.New(service.CodeSendTooFrequent, limited.Error()))
	case errors.Is(err, service.ErrSceneDisabled):
		response.Error(c, apperrors.New(service.CodeSceneDisabled, err.Error()))
	case errors.Is(err, service.ErrTargetUnbound):
		response.Error(c, apperrors.New(service.CodeTargetUnbound, err.Error()))
	case errors.Is(err, captchapkg.ErrInvalid), errors.Is(err, captchapkg.ErrTooManyAttempts):
		response.Error(c, apperrors.New(service.CodeCaptchaInvalid, "验证码错误或已过期"))
	default:
		response.Error(c, apperrors.New(50001, err.Error()))
	}
}

// AdminHandler 管理端验证码配置（服务商 / 场景策略 / 统计）。
type AdminHandler struct {
	svc *service.AdminService
}

// NewAdminHandler 创建管理端处理器。
func NewAdminHandler(svc *service.AdminService) *AdminHandler {
	return &AdminHandler{svc: svc}
}

// ListProviderTypes godoc
// @Summary 验证码渠道列表
// @Tags 管理端-验证码安全
// @Security BearerAuth
// @Param request body dto.ProviderUpsertRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/admin/captcha/providers/types [get]
// ListProviderTypes 可选服务商类型（描述符驱动前端动态表单）。
func (h *AdminHandler) ListProviderTypes(c *gin.Context) {
	response.Success(c, h.svc.ProviderTypes())
}

// ListProviders 服务商列表（凭证只回脱敏值）。
func (h *AdminHandler) ListProviders(c *gin.Context) {
	items, err := h.svc.ListProviders(c.Request.Context())
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, items)
}

// CreateProvider godoc
// @Summary 新建验证码渠道
// @Tags 管理端-验证码安全
// @Security BearerAuth
// @Param request body dto.ProviderUpsertRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/admin/captcha/providers [post]
// CreateProvider 新建服务商。
func (h *AdminHandler) CreateProvider(c *gin.Context) {
	var req dto.ProviderUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	item, err := h.svc.CreateProvider(c.Request.Context(), req)
	if err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	response.Success(c, item)
}

// UpdateProvider godoc
// @Summary 更新验证码渠道
// @Tags 管理端-验证码安全
// @Security BearerAuth
// @Param id path int true "id"
// @Param request body dto.ProviderUpsertRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/admin/captcha/providers/{id} [put]
// UpdateProvider 更新服务商。
func (h *AdminHandler) UpdateProvider(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.ProviderUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	item, err := h.svc.UpdateProvider(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	response.Success(c, item)
}

// DeleteProvider 删除服务商（内置 native 不可删除）。
// @Router /api/v1/admin/captcha/providers/{id} [delete]
func (h *AdminHandler) DeleteProvider(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteProvider(c.Request.Context(), id); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	response.SuccessMessage(c, "删除成功")
}

// TestProvider 连通性/凭证测试（占位 provider 返回「待接入」，不是 500）。
func (h *AdminHandler) TestProvider(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.ProviderTestRequest
	_ = c.ShouldBindJSON(&req)
	if err := h.svc.TestProvider(c.Request.Context(), id); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	response.SuccessMessage(c, "测试通过")
}

// ListPolicies godoc
// @Summary 验证码策略列表
// @Tags 管理端-验证码安全
// @Security BearerAuth
// @Param request body dto.PolicyUpdateRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/admin/captcha/policies [get]
// ListPolicies 场景策略列表。
func (h *AdminHandler) ListPolicies(c *gin.Context) {
	items, err := h.svc.ListPolicies(c.Request.Context())
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, items)
}

// UpdatePolicy godoc
// @Summary 更新验证码策略
// @Tags 管理端-验证码安全
// @Security BearerAuth
// @Param scene path string true "scene"
// @Param request body dto.PolicyUpdateRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/admin/captcha/policies/{scene} [put]
// UpdatePolicy 更新场景策略。
func (h *AdminHandler) UpdatePolicy(c *gin.Context) {
	scene := strings.TrimSpace(c.Param("scene"))
	if scene == "" {
		response.Error(c, apperrors.New(20001, "场景不能为空"))
		return
	}
	var req dto.PolicyUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	item, err := h.svc.UpdatePolicy(c.Request.Context(), scene, req)
	if err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	response.Success(c, item)
}

// Stats godoc
// @Summary 验证码统计
// @Tags 管理端-验证码安全
// @Security BearerAuth
// @Param request body dto.SecuritySettingsUpdateRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/admin/captcha/stats [get]
// Stats 统计（按场景聚合下发量/通过率/费用）。
func (h *AdminHandler) Stats(c *gin.Context) {
	from := parseTimeQuery(c.Query("from"), time.Now().AddDate(0, 0, -7))
	to := parseTimeQuery(c.Query("to"), time.Now().Add(24*time.Hour))
	resp, err := h.svc.Stats(c.Request.Context(), from, to)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// UserHandler 用户端安全设置（需登录）。
type UserHandler struct {
	svc *service.UserService
}

// NewUserHandler 创建用户端处理器。
func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// GetSettings godoc
// @Summary 验证码设置详情
// @Tags 用户端-验证码安全
// @Security BearerAuth
// @Param request body dto.SecuritySettingsUpdateRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/uc/security/settings [get]
// GetSettings 读取用户安全设置（只回 effective）。
func (h *UserHandler) GetSettings(c *gin.Context) {
	claims, ok := middleware.GetUserClaims(c)
	if !ok {
		response.ErrorWithStatus(c, 401, apperrors.New(10001, "unauthorized"))
		return
	}
	resp, err := h.svc.GetSettings(c.Request.Context(), claims.UserID)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// UpdateSettings godoc
// @Summary 更新验证码设置
// @Tags 用户端-验证码安全
// @Security BearerAuth
// @Param request body dto.SecuritySettingsUpdateRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/uc/security/settings [put]
// UpdateSettings 更新用户安全设置（只表达加严；平台强制项被忽略并回显真实值）。
func (h *UserHandler) UpdateSettings(c *gin.Context) {
	claims, ok := middleware.GetUserClaims(c)
	if !ok {
		response.ErrorWithStatus(c, 401, apperrors.New(10001, "unauthorized"))
		return
	}
	var req dto.SecuritySettingsUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.UpdateSettings(c.Request.Context(), claims.UserID, req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// SendVerification godoc
// @Summary 下发验证码
// @Tags 用户端-验证码安全
// @Security BearerAuth
// @Param request body dto.SendCodeRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/uc/security/verification/send [post]
// SendVerification 用户端下发验证码（已登录，目标取账号绑定）。
func (h *UserHandler) SendVerification(c *gin.Context) {
	claims, ok := middleware.GetUserClaims(c)
	if !ok {
		response.ErrorWithStatus(c, 401, apperrors.New(10001, "unauthorized"))
		return
	}
	var req dto.SendCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.SendVerification(c.Request.Context(), claims.UserID, req, netutil.ClientIP(c), c.GetHeader("User-Agent"))
	if err != nil {
		writeCaptchaError(c, err)
		return
	}
	response.Success(c, resp)
}

// VerifyCode godoc
// @Summary 校验关键操作验证
// @Tags 用户端-验证码安全
// @Security BearerAuth
// @Param request body dto.VerifyCodeRequest true "请求参数"
// @Success 200 {string} string "ok"
// @Router /api/v1/uc/security/verification/verify [post]
// VerifyCode 用户端校验验证码并换取票据。
func (h *UserHandler) VerifyCode(c *gin.Context) {
	claims, ok := middleware.GetUserClaims(c)
	if !ok {
		response.ErrorWithStatus(c, 401, apperrors.New(10001, "unauthorized"))
		return
	}
	var req dto.VerifyCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.svc.VerifyCode(c.Request.Context(), claims.UserID, req)
	if err != nil {
		writeCaptchaError(c, err)
		return
	}
	response.Success(c, resp)
}

// OpenVerification godoc
// @Summary 关键操作验证要求
// @Tags 用户端-验证码安全
// @Security BearerAuth
// @Success 200 {string} string "ok"
// @Router /api/v1/uc/security/verification/requirement [get]
// OpenVerification 无票据时查询该场景的验证要求（前端据此弹验证框）。
func (h *UserHandler) OpenVerification(c *gin.Context) {
	claims, ok := middleware.GetUserClaims(c)
	if !ok {
		response.ErrorWithStatus(c, 401, apperrors.New(10001, "unauthorized"))
		return
	}
	scene := c.Query("scene")
	resp, err := h.svc.VerificationRequirement(c.Request.Context(), claims.UserID, scene)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

func parseID(c *gin.Context) (uint64, bool) {
	var id uint64
	for _, ch := range c.Param("id") {
		if ch < '0' || ch > '9' {
			response.Error(c, apperrors.New(20001, "ID 不合法"))
			return 0, false
		}
		id = id*10 + uint64(ch-'0')
	}
	if id == 0 {
		response.Error(c, apperrors.New(20001, "ID 不合法"))
		return 0, false
	}
	return id, true
}

func parseTimeQuery(raw string, def time.Time) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t
		}
	}
	return def
}
