// 管理端路由装配（自 router.go 拆出，批次四 4a；批次三 C2 再按域拆分）。

package server

import (
	"github.com/gin-gonic/gin"
)

// registerAdminRoutes 注册管理端 /api/v1/admin 全部路由。
// 组内注册顺序与拆分前保持一致；写操作审计、全局限流挂组级中间件。
// 各域路由分别在 routes_admin_<域>.go 注册；第三方登录同时覆盖管理端
// 与用户端两端路由，故在函数尾部一并注册。
func registerAdminRoutes(r *gin.Engine, app *App) {
	v1 := r.Group("/api/v1/admin")
	// 管理端写操作审计（P2-06）：仅记录 POST/PUT/PATCH/DELETE，登录等白名单路径自动跳过。
	v1.Use(app.adminAudit())
	// 全局 API 限流（doc91 §9.1 api_rate_limit）：默认 0=不限；缓存不可用时放行并告警。
	if app.captcha != nil {
		v1.Use(app.captcha.limiter.Handler())
	}

	registerAdminUserRoutes(v1, app)
	registerAdminResourceRoutes(v1, app)
	registerAdminProductRoutes(v1, app)
	registerAdminOrderRoutes(v1, app)
	registerAdminFinanceRoutes(v1, app)
	registerAdminSystemRoutes(v1, app)
	registerAdminLogRoutes(v1, app)

	// 第三方登录（doc104 §6）：登录页授权入口与免登录回调在 registerOAuthRoutes 内注册，
	// 该函数同时注册管理端 /admin/oauth 与用户端 /api/v1/uc/oauth 两端路由。
	registerOAuthRoutes(v1, r.Group("/api/v1/uc"), app.oauth, app)
}
