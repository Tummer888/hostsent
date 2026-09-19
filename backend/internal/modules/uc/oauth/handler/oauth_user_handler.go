// Package handler 提供第三方登录的用户端 HTTP 处理器（doc104 §6.6）。
package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/uc/oauth/dto"
	"hostsent/backend/internal/modules/uc/oauth/service"
	"hostsent/backend/internal/pkg/middleware"
	oauthpkg "hostsent/backend/internal/pkg/oauth"
)

// UserHandler 用户端第三方登录（登录/绑定/解绑）。
type UserHandler struct{ svc service.Service }

// NewUserHandler 创建用户端处理器。
func NewUserHandler(svc service.Service) *UserHandler {
	return &UserHandler{svc: svc}
}

// Providers godoc
// @Summary 可用的第三方登录渠道
// @Description 只返回「已启用且适配器已注册」的渠道，不含任何凭证
// @Tags 用户中心-第三方登录
// @Produce json
// @Success 200 {object} dto.APIResponse[[]dto.PublicProviderInfo]
// @Router /api/v1/uc/oauth/providers [get]
func (h *UserHandler) Providers(c *gin.Context) {
	data, err := h.svc.PublicProviders(c.Request.Context())
	if err != nil {
		serverError(c, err.Error())
		return
	}
	success(c, data)
}

// Authorize godoc
// @Summary 获取第三方授权跳转地址
// @Description 免登录。mode=login 为登录/注册；invite_code 可透传注册邀请码
// @Tags 用户中心-第三方登录
// @Produce json
// @Param provider path string true "渠道标识" Enums(wechat,qq,alipay)
// @Param invite_code query string false "邀请码（透传注册归属）"
// @Success 200 {object} dto.APIResponse[map[string]string]
// @Router /api/v1/uc/oauth/{provider}/authorize [get]
func (h *UserHandler) Authorize(c *gin.Context) {
	provider := c.Param("provider")
	url, err := h.svc.AuthorizeURL(c.Request.Context(), provider, oauthpkg.ModeLogin, 0, c.Query("invite_code"))
	if err != nil {
		respondOAuthUserErr(c, err)
		return
	}
	success(c, gin.H{"authorize_url": url})
}

// Callback godoc
// @Summary 第三方授权回调
// @Description 免登录。校验 state 后处理绑定/登录，最终 302 回跳到前端并带一次性 ticket
// @Tags 用户中心-第三方登录
// @Param provider path string true "渠道标识"
// @Param code query string false "授权码"
// @Param state query string false "state"
// @Success 302
// @Router /api/v1/uc/oauth/{provider}/callback [get]
func (h *UserHandler) Callback(c *gin.Context) {
	provider := c.Param("provider")
	result, err := h.svc.Callback(c.Request.Context(), provider, c.Query("code"), c.Query("state"), c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		// 服务层已把可预期的失败转成 RedirectURL；走到这里说明是内部错误。
		serverError(c, err.Error())
		return
	}
	// 302 回跳而不是返回 JSON：这个入口是用户在浏览器里被三方重定向过来的，
	// 停在 API 的 JSON 页面上体验断裂。
	c.Redirect(http.StatusFound, result.RedirectURL)
}

// Exchange godoc
// @Summary 一次性票据换正式访问令牌
// @Description 回调回跳前端后调用；ticket 60 秒有效且一次性
// @Tags 用户中心-第三方登录
// @Accept json
// @Produce json
// @Param request body dto.ExchangeRequest true "票据"
// @Success 200 {object} dto.APIResponse[dto.ExchangeResponse]
// @Router /api/v1/uc/oauth/exchange [post]
func (h *UserHandler) Exchange(c *gin.Context) {
	var req dto.ExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	data, err := h.svc.Exchange(c.Request.Context(), req.Ticket)
	if err != nil {
		if errors.Is(err, oauthpkg.ErrInvalidState) {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 40101, "message": err.Error(), "timestamp": time.Now().Unix()})
			return
		}
		badRequest(c, err.Error())
		return
	}
	success(c, data)
}

// MyBindings godoc
// @Summary 我的第三方账号绑定
// @Description 含 can_unbind 服务端预判（解绑后必须仍有其他登录方式）
// @Tags 用户中心-第三方登录
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.APIResponse[[]dto.MyBindingInfo]
// @Router /api/v1/uc/oauth/bindings [get]
func (h *UserHandler) MyBindings(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	data, err := h.svc.MyBindings(c.Request.Context(), uid)
	if err != nil {
		serverError(c, err.Error())
		return
	}
	success(c, data)
}

// BindAuthorize godoc
// @Summary 获取绑定第三方账号的授权跳转地址
// @Description 需登录。state 里带上当前用户 ID，回调只能绑到发起时的账号
// @Tags 用户中心-第三方登录
// @Produce json
// @Security BearerAuth
// @Param provider path string true "渠道标识"
// @Success 200 {object} dto.APIResponse[map[string]string]
// @Router /api/v1/uc/oauth/{provider}/bind-authorize [get]
func (h *UserHandler) BindAuthorize(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	provider := c.Param("provider")
	url, err := h.svc.AuthorizeURL(c.Request.Context(), provider, oauthpkg.ModeBind, uid, "")
	if err != nil {
		respondOAuthUserErr(c, err)
		return
	}
	success(c, gin.H{"authorize_url": url})
}

// Unbind godoc
// @Summary 解绑第三方账号
// @Description 守卫：解绑后必须仍保留至少一种登录方式，否则返回 409
// @Tags 用户中心-第三方登录
// @Produce json
// @Security BearerAuth
// @Param provider path string true "渠道标识"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/uc/oauth/bindings/{provider} [delete]
func (h *UserHandler) Unbind(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	provider := c.Param("provider")
	if err := h.svc.Unbind(c.Request.Context(), uid, provider); err != nil {
		respondOAuthUserErr(c, err)
		return
	}
	success(c, "ok")
}

// ---------- 工具 ----------

func currentUserID(c *gin.Context) (uint64, bool) {
	claims, ok := middleware.GetUserClaims(c)
	if !ok || claims.UserID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 10001, "message": "unauthorized", "timestamp": time.Now().Unix()})
		return 0, false
	}
	return claims.UserID, true
}

func respondOAuthUserErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrProviderUnknown):
		c.JSON(http.StatusNotFound, gin.H{"code": 40401, "message": err.Error(), "timestamp": time.Now().Unix()})
	case errors.Is(err, service.ErrProviderNotConfigured), errors.Is(err, oauthpkg.ErrLastLoginMethod),
		errors.Is(err, service.ErrBindingNotFound):
		c.JSON(http.StatusConflict, gin.H{"code": 40901, "message": err.Error(), "timestamp": time.Now().Unix()})
	default:
		badRequest(c, err.Error())
	}
}

// trimQuery 读一个查询参数并去掉空白。
func trimQuery(c *gin.Context, key string) string { return strings.TrimSpace(c.Query(key)) }

var _ = trimQuery
