package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	appauth "hostsent/backend/internal/pkg/auth"
)

const adminGrantContextKey = "admin_grant"

func forbidden(c *gin.Context, message string) {
	if message == "" {
		message = "forbidden"
	}
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 40300, "message": message, "timestamp": timestamp()})
}

// GetAdminGrant 返回鉴权中间件写入的管理员权限快照。
func GetAdminGrant(c *gin.Context) (*AdminGrant, bool) {
	value, ok := c.Get(adminGrantContextKey)
	if !ok {
		return nil, false
	}
	grant, ok := value.(*AdminGrant)
	return grant, ok
}

// resolveRequestGrant 取当前请求管理员的权限快照：
// 优先复用 AdminAuth 已加载的结果，缺失时按 bearer token 现场解析（支持中间件独立使用）。
func resolveRequestGrant(c *gin.Context, issuer *appauth.JWTIssuer, bearerPrefix string, cache PermissionCache, resolver PermissionResolver) (*AdminGrant, bool) {
	if grant, ok := GetAdminGrant(c); ok {
		return grant, true
	}
	if resolver == nil {
		return nil, false
	}
	adminID, ok := adminIDFromContext(c, issuer, bearerPrefix)
	if !ok {
		return nil, false
	}
	grant, err := LoadAdminGrant(c.Request.Context(), cache, resolver, adminID)
	if err != nil {
		return nil, false
	}
	if issuer != nil {
		c.Set(adminGrantContextKey, grant)
	}
	return grant, true
}

// RequirePermission 校验当前管理员是否持有任一所需权限码。
// 超级管理员（权限集合含 "*"）直接放行；失败返回 HTTP 403（管理端裸响应风格）。
func RequirePermission(issuer *appauth.JWTIssuer, bearerPrefix string, cache PermissionCache, resolver PermissionResolver, codes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		grant, ok := resolveRequestGrant(c, issuer, bearerPrefix, cache, resolver)
		if !ok || !grant.Active {
			unauthorized(c)
			return
		}
		if grant.IsSuper() || grant.HasAny(codes...) {
			c.Next()
			return
		}
		forbidden(c, "无权执行该操作")
	}
}

// RequireSuperAdmin 仅放行超级管理员（角色含 super_admin 或权限集合含通配 "*"）。
// 用于管理员增删改、角色权限分配、代登录等高危接口（81 §4.1）。
func RequireSuperAdmin(issuer *appauth.JWTIssuer, bearerPrefix string, cache PermissionCache, resolver PermissionResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		grant, ok := resolveRequestGrant(c, issuer, bearerPrefix, cache, resolver)
		if !ok || !grant.Active {
			unauthorized(c)
			return
		}
		if grant.IsSuper() || appauth.HasRoleCode(grant.Roles, appauth.SuperAdminRoleCode) {
			c.Next()
			return
		}
		forbidden(c, "仅超级管理员可操作")
	}
}
