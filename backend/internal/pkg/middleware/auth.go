package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	appauth "hostsent/backend/internal/pkg/auth"
	apperrors "hostsent/backend/internal/pkg/errors"
)

const (
	claimsContextKey      = "claims"
	adminClaimsContextKey = "admin_claims"
	userClaimsContextKey  = "user_claims"
)

// Auth 兼容旧接口的管理员鉴权：解析 Claims（UserType 未设置时视为管理员）。
func Auth(jwtIssuer *appauth.JWTIssuer, bearerPrefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, ok := extractBearerToken(c, bearerPrefix)
		if !ok {
			unauthorized(c)
			return
		}

		claims, err := jwtIssuer.Parse(tokenStr)
		if err != nil {
			unauthorized(c)
			return
		}

		c.Set(claimsContextKey, claims)
		c.Next()
	}
}

// AdminAuth 管理员专用鉴权：解析 AdminClaims，并按 admin_roles 加载真实角色与权限快照。
// resolver/cache 允许为 nil（此时仅按 token 内的 role 字符串兼容处理）。
func AdminAuth(jwtIssuer *appauth.JWTIssuer, bearerPrefix string, resolver PermissionResolver, cache PermissionCache) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, ok := extractBearerToken(c, bearerPrefix)
		if !ok {
			unauthorized(c)
			return
		}

		var adminID uint64
		var username, role string
		if adminClaims, err := jwtIssuer.ParseAdmin(tokenStr); err == nil {
			adminID, username, role = adminClaims.AdminID, adminClaims.Username, adminClaims.Role
			c.Set(adminClaimsContextKey, adminClaims)
		} else {
			// 兼容旧管理员 token（UserType 为空）
			claims, perr := jwtIssuer.Parse(tokenStr)
			if perr != nil {
				unauthorized(c)
				return
			}
			adminID, username, role = claims.UserID, claims.Username, claims.Role
			if role == "" && len(claims.Roles) > 0 {
				role = claims.Roles[0]
			}
			c.Set(adminClaimsContextKey, &appauth.AdminClaims{AdminID: adminID, Username: username, Role: role})
		}

		// 鉴权以 admin_roles 为准：加载真实角色与权限快照，替代 token 里的单 role 字符串。
		roleCodes := stringSlice(role)
		if resolver != nil {
			grant, err := LoadAdminGrant(c.Request.Context(), cache, resolver, adminID)
			if err == nil {
				if !grant.Active {
					// 员工被禁用：旧 token 立即失效
					unauthorized(c)
					return
				}
				if len(grant.Roles) > 0 {
					roleCodes = grant.Roles
					if role == "" {
						role = grant.Roles[0]
					}
				}
				c.Set(adminGrantContextKey, grant)
			}
		}

		// 转成兼容的 Claims，供现有 handler 读取 UserID/Role/Roles
		c.Set(claimsContextKey, &appauth.Claims{
			UserID:   adminID,
			Username: username,
			Role:     role,
			Roles:    roleCodes,
			UserType: appauth.UserTypeAdmin,
		})
		c.Next()
	}
}

// adminIDFromContext 从上下文或 bearer token 解析当前管理员 ID（供权限中间件独立使用）。
func adminIDFromContext(c *gin.Context, issuer *appauth.JWTIssuer, bearerPrefix string) (uint64, bool) {
	if claims, ok := GetAdminClaims(c); ok {
		return claims.AdminID, true
	}
	if claims, ok := GetClaims(c); ok && claims.UserID > 0 {
		return claims.UserID, true
	}
	if issuer == nil {
		return 0, false
	}
	tokenStr, ok := extractBearerToken(c, bearerPrefix)
	if !ok {
		return 0, false
	}
	if adminClaims, err := issuer.ParseAdmin(tokenStr); err == nil {
		return adminClaims.AdminID, true
	}
	if claims, err := issuer.Parse(tokenStr); err == nil {
		return claims.UserID, true
	}
	return 0, false
}

// SessionChecker 会话有效性校验端口（由 pkg/sessionguard 实现）。
//
// 抽成接口而不是直接依赖 sessionguard：middleware 不该知道会话是怎么查的，
// 测试里也就能塞一个「全放行/全拒绝」的假实现。
type SessionChecker interface {
	IsActive(ctx context.Context, sessionID string) (bool, error)
}

// UserAuth 普通用户专用鉴权：仅接受 UserClaims，并校验令牌绑定的会话仍有效。
//
// 会话校验是「强制下线」生效的前提：JWT 无状态，撤销只能靠服务端查证令牌里的
// sid。checker 为 nil 时**不放行**（返回 401 并告警）—— 安全原语「校验不了」
// 必须等于「不通过」，否则忘装配就成了全站绕过。这与图形码降级即放行的取舍相反。
func UserAuth(jwtIssuer *appauth.JWTIssuer, bearerPrefix string, checker SessionChecker, logger *zap.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = zap.NewNop()
	}
	return func(c *gin.Context) {
		tokenStr, ok := extractBearerToken(c, bearerPrefix)
		if !ok {
			unauthorized(c)
			return
		}

		userClaims, err := jwtIssuer.ParseUser(tokenStr)
		if err != nil {
			unauthorized(c)
			return
		}

		if checker == nil {
			// 装配缺失属于部署事故，必须留下明确日志而不是静默 401。
			logger.Error("user auth: session checker not configured, rejecting request",
				zap.String("path", c.FullPath()))
			unauthorized(c)
			return
		}
		// 令牌必须带会话句柄：旧令牌（升级前签发）没有 sid，一律拒绝，
		// 让所有在线用户在部署后重新登录一次，而不是留下一条永久可用的旁路。
		if userClaims.SessionID == "" {
			unauthorized(c)
			return
		}
		active, err := checker.IsActive(c.Request.Context(), userClaims.SessionID)
		if err != nil {
			// 查库失败与「会话无效」对客户端是同一个结果，但日志里要能区分。
			logger.Warn("user auth: session check failed",
				zap.String("session_id", userClaims.SessionID), zap.Error(err))
			unauthorized(c)
			return
		}
		if !active {
			// 会话已被撤销/过期/清理：令牌立即失效（doc91 §9 强制下线）。
			unauthorized(c)
			return
		}

		c.Set(userClaimsContextKey, userClaims)
		c.Set(claimsContextKey, &appauth.Claims{
			UserID:   userClaims.UserID,
			Username: userClaims.Username,
			UserType: appauth.UserTypeUser,
		})
		c.Next()
	}
}

func extractBearerToken(c *gin.Context, bearerPrefix string) (string, bool) {
	authorization := c.GetHeader("Authorization")
	parts := strings.SplitN(authorization, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], bearerPrefix) {
		return "", false
	}
	return parts[1], true
}

func unauthorized(c *gin.Context) {
	err := apperrors.New(10001, "unauthorized")
	c.JSON(http.StatusUnauthorized, gin.H{"code": err.Code, "message": err.Message, "timestamp": timestamp()})
	c.Abort()
}

func toAdminClaims(claims *appauth.Claims) *appauth.AdminClaims {
	role := claims.Role
	if role == "" && len(claims.Roles) > 0 {
		role = claims.Roles[0]
	}
	return &appauth.AdminClaims{
		AdminID:  claims.UserID,
		Username: claims.Username,
		Role:     role,
	}
}

func stringSlice(r string) []string {
	if r == "" {
		return nil
	}
	return []string{r}
}

// GetClaims 兼容旧接口，返回统一 Claims。
func GetClaims(c *gin.Context) (*appauth.Claims, bool) {
	value, ok := c.Get(claimsContextKey)
	if !ok {
		return nil, false
	}
	claims, ok := value.(*appauth.Claims)
	return claims, ok
}

// GetAdminClaims 返回管理员声明。
func GetAdminClaims(c *gin.Context) (*appauth.AdminClaims, bool) {
	value, ok := c.Get(adminClaimsContextKey)
	if !ok {
		return nil, false
	}
	claims, ok := value.(*appauth.AdminClaims)
	return claims, ok
}

// GetUserClaims 返回普通用户声明。
func GetUserClaims(c *gin.Context) (*appauth.UserClaims, bool) {
	value, ok := c.Get(userClaimsContextKey)
	if !ok {
		return nil, false
	}
	claims, ok := value.(*appauth.UserClaims)
	return claims, ok
}
