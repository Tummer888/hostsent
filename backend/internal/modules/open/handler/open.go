// Package handler 开放平台 HTTP 层（P6）。Bundle 聚合网关与各业务 handler，
// 由 server 装配层以单一字段注入 App，路由注册只经 Bundle 暴露的方法。
package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"hostsent/backend/internal/modules/open/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// Bundle 开放平台处理器集合。后续任务（T6.2～T6.6）的业务服务挂在同一结构上。
type Bundle struct {
	gw     *service.Gateway
	logger *zap.Logger
}

// NewBundle 构造处理器集合。
func NewBundle(gw *service.Gateway, logger *zap.Logger) *Bundle {
	return &Bundle{gw: gw, logger: logger}
}

// Audit 请求/响应审计（分组最外层）。
func (b *Bundle) Audit() gin.HandlerFunc { return b.gw.Audit() }

// Gateway 签名与应用鉴权链。
func (b *Bundle) Gateway() gin.HandlerFunc { return b.gw.Middleware() }

// RequireScope 声明路由所需能力位。
func (b *Bundle) RequireScope(scope string) gin.HandlerFunc { return b.gw.RequireScope(scope) }

// Ping 链路自检：签名通过即可访问，不占能力位。返回应用身份供联调核对。
func (b *Bundle) Ping(c *gin.Context) {
	app := service.AppFromContext(c)
	if app == nil {
		// 防御分支：Ping 挂在鉴权链之后，正常请求不会走到这里。
		response.Error(c, apperrors.New(service.CodeOpenMissingAuth, "缺少认证头"))
		return
	}
	response.Success(c, gin.H{
		"app_id":        app.App.AppID,
		"name":          app.App.Name,
		"api_version":   app.App.APIVersion,
		"owner_user_id": app.App.OwnerUserID,
		"scopes":        app.ScopeNames(),
	})
}
