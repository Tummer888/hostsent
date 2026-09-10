package middleware

import (
	"context"

	"github.com/gin-gonic/gin"

	appauth "hostsent/backend/internal/pkg/auth"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// SubAccountPermissionResolver 解析子账号持有的权限码（P4-06）。
// 由 account 模块的仓储实现，避免 middleware 依赖具体模块。
type SubAccountPermissionResolver interface {
	// PermissionsOf 返回该子账号已授予的权限码。
	PermissionsOf(ctx context.Context, userID uint64) ([]string, error)
}

// RequireUserPermission 校验子账号是否持有任一权限码（P4-06）。
//   - 主账号（IsSub=false）直接放行：主账号拥有全部客户侧权限；
//   - 子账号：命中任一 code 放行，否则返回业务错误（HTTP 200 + code != 0，用户端响应风格）。
//   - resolver 为 nil 时按「拒绝子账号」处理，避免误放行（fail closed）。
func RequireUserPermission(resolver SubAccountPermissionResolver, codes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsSubAccount(c) {
			c.Next()
			return
		}
		if resolver == nil || len(codes) == 0 {
			response.Error(c, apperrors.New(40301, "当前成员无权执行该操作"))
			c.Abort()
			return
		}
		granted, err := resolver.PermissionsOf(c.Request.Context(), ActorUserID(c))
		if err != nil {
			response.Error(c, apperrors.New(50001, "权限校验失败"))
			c.Abort()
			return
		}
		for _, want := range codes {
			if want == appauth.PermSubAccountManage {
				continue // 成员管理权限永不授予子账号，此处直接忽略
			}
			for _, code := range granted {
				if code == want {
					c.Next()
					return
				}
			}
		}
		response.Error(c, apperrors.New(40301, "当前成员无权执行该操作"))
		c.Abort()
	}
}

// RejectSubAccount 硬编码拒绝子账号（P4-06）。
// 用于充值/提现/退款/实名等资金与身份类接口：不依赖权限码，兜底拒绝。
func RejectSubAccount() gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsSubAccount(c) {
			response.Error(c, apperrors.New(40301, "子账号不支持该操作，请使用主账号"))
			c.Abort()
			return
		}
		c.Next()
	}
}
