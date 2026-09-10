package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	membermodel "hostsent/backend/internal/modules/uc/member/model"
)

// UserOperationLogWriter 用户侧操作日志落库能力（由 uc/member 仓储实现，P4-08）。
type UserOperationLogWriter interface {
	CreateOperationLog(ctx context.Context, log *membermodel.OperationLog) error
}

// userAuditExcludedPaths 不记录的高噪/敏感路径（前缀匹配）。
var userAuditExcludedPaths = []string{
	"/api/v1/uc/auth/login",
	"/api/v1/uc/auth/register",
	"/api/v1/uc/finance/recharge/callback",
	"/api/v1/auth/login",
	"/api/v1/auth/register",
}

// UserOperationAudit 记录子账号的写操作到 user_operation_logs（P4-08）。
//   - 仅在已通过 UserAuth 后使用：actor 从 UserClaims 取（真实操作人）；
//   - 只记录子账号（IsSub=true）的写操作：主账号自身操作由管理端审计覆盖；
//   - 落库失败不影响主流程。
func UserOperationAudit(writer UserOperationLogWriter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if writer == nil || !isAuditMethod(c.Request.Method) {
			c.Next()
			return
		}
		for _, prefix := range userAuditExcludedPaths {
			if strings.HasPrefix(c.Request.URL.Path, prefix) {
				c.Next()
				return
			}
		}

		c.Next()

		// 鉴权中间件可能注册在本中间件之后，claims 需在 c.Next() 返回后再读；
		// 且只记录子账号的写操作（主账号操作由管理端审计覆盖）。
		if !IsSubAccount(c) {
			return
		}
		actorID := ActorUserID(c)
		accountID := EffectiveUserID(c)
		if actorID == 0 || accountID == 0 {
			return
		}
		module, action := parseUserAuditPath(c.Request.URL.Path, c.Request.Method)
		_ = writer.CreateOperationLog(c.Request.Context(), &membermodel.OperationLog{
			ActorUserID:   actorID,
			ActorName:     ActorUsername(c),
			AccountUserID: accountID,
			Module:        module,
			Action:        action,
			Target:        truncate(c.Request.URL.Path, 128),
			IP:            c.ClientIP(),
		})
	}
}

// parseUserAuditPath 从 /api/v1/uc/{module}/... 解析模块与动作。
func parseUserAuditPath(path, method string) (module, action string) {
	trimmed := strings.TrimPrefix(path, "/api/v1/uc/")
	trimmed = strings.TrimPrefix(trimmed, "/api/v1/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) > 0 && parts[0] != "" {
		module = parts[0]
	} else {
		module = "uc"
	}
	switch method {
	case "POST":
		action = "create"
	case "PUT", "PATCH":
		action = "update"
	case "DELETE":
		action = "delete"
	default:
		action = strings.ToLower(method)
	}
	return module, action
}
