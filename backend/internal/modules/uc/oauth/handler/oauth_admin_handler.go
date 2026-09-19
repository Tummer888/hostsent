// Package handler 提供第三方登录的管理端 HTTP 处理器（doc104 §6.6）。
package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/uc/oauth/dto"
	"hostsent/backend/internal/modules/uc/oauth/service"
	oauthpkg "hostsent/backend/internal/pkg/oauth"
)

// AdminHandler 管理端第三方登录配置与绑定排查。
type AdminHandler struct{ svc service.Service }

// NewAdminHandler 创建管理端处理器。
func NewAdminHandler(svc service.Service) *AdminHandler {
	return &AdminHandler{svc: svc}
}

// ProviderTypes godoc
// @Summary 第三方登录渠道类型
// @Description 描述符驱动前端动态凭证表单
// @Tags 系统设置-第三方登录
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.APIResponse[[]dto.ProviderTypeInfo]
// @Router /api/v1/admin/oauth/provider-types [get]
func (h *AdminHandler) ProviderTypes(c *gin.Context) {
	success(c, h.svc.ProviderTypes())
}

// ListProviders godoc
// @Summary 第三方登录渠道配置列表
// @Description 未落库的渠道返回 ID=0 的待配置卡片（凭证为空）；已配置的凭证只回脱敏值
// @Tags 系统设置-第三方登录
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.APIResponse[[]dto.ProviderInfo]
// @Router /api/v1/admin/oauth/providers [get]
func (h *AdminHandler) ListProviders(c *gin.Context) {
	data, err := h.svc.ListProviders(c.Request.Context())
	if err != nil {
		serverError(c, err.Error())
		return
	}
	success(c, data)
}

// UpdateProvider godoc
// @Summary 更新第三方登录渠道配置
// @Description 按渠道名 upsert（微信/QQ/支付宝三家固定）。凭证回传脱敏值表示未修改
// @Tags 系统设置-第三方登录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param provider path string true "渠道标识" Enums(wechat,qq,alipay)
// @Param request body dto.ProviderUpdateRequest true "渠道配置"
// @Success 200 {object} dto.APIResponse[dto.ProviderInfo]
// @Router /api/v1/admin/oauth/providers/{provider} [put]
func (h *AdminHandler) UpdateProvider(c *gin.Context) {
	provider := c.Param("provider")
	var req dto.ProviderUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	data, err := h.svc.UpdateProvider(c.Request.Context(), provider, req)
	if err != nil {
		respondOAuthErr(c, err)
		return
	}
	success(c, data)
}

// TestProvider godoc
// @Summary 第三方登录渠道连通性测试
// @Description 失败时 HTTP 仍为 200，由 ok/message 表达结果
// @Tags 系统设置-第三方登录
// @Produce json
// @Security BearerAuth
// @Param provider path string true "渠道标识"
// @Success 200 {object} dto.APIResponse[map[string]any]
// @Router /api/v1/admin/oauth/providers/{provider}/test [post]
func (h *AdminHandler) TestProvider(c *gin.Context) {
	provider := c.Param("provider")
	if err := h.svc.TestProvider(c.Request.Context(), provider); err != nil {
		success(c, gin.H{"ok": false, "message": err.Error()})
		return
	}
	success(c, gin.H{"ok": true, "message": "连通性正常"})
}

// ListBindings godoc
// @Summary 第三方登录绑定排查
// @Description 按渠道/用户/关键词筛选绑定关系（含已注销用户，便于排查历史绑定）
// @Tags 系统设置-第三方登录
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param provider query string false "渠道"
// @Param user_id query int false "用户ID"
// @Param status query string false "绑定状态"
// @Param keyword query string false "关键词（用户名/昵称/openid）"
// @Success 200 {object} dto.APIResponse[dto.BindingListResponse]
// @Router /api/v1/admin/oauth/bindings [get]
func (h *AdminHandler) ListBindings(c *gin.Context) {
	var query dto.BindingListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		badRequest(c, err.Error())
		return
	}
	data, err := h.svc.ListBindings(c.Request.Context(), query)
	if err != nil {
		serverError(c, err.Error())
		return
	}
	success(c, data)
}

// ---------- 工具 ----------

func respondOAuthErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrProviderUnknown):
		c.JSON(http.StatusNotFound, gin.H{"code": 40401, "message": err.Error(), "timestamp": time.Now().Unix()})
	case errors.Is(err, service.ErrProviderNotConfigured):
		c.JSON(http.StatusConflict, gin.H{"code": 40901, "message": err.Error(), "timestamp": time.Now().Unix()})
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": 40401, "message": "记录不存在", "timestamp": time.Now().Unix()})
	default:
		serverError(c, err.Error())
	}
}

func parseUintParam(c *gin.Context, name string) (uint64, bool) {
	raw := c.Param(name)
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		badRequest(c, name+" 不合法")
		return 0, false
	}
	return id, true
}

func badRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": message, "timestamp": time.Now().Unix()})
}

func serverError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": message, "timestamp": time.Now().Unix()})
}

func success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}

// 确保 oauthpkg 被引用（错误类型在 service 层已用，这里仅作编译期锚点）。
var _ = oauthpkg.ModeBind
