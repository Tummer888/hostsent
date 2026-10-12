package server

import (
	"github.com/gin-gonic/gin"
)

// registerOpenRoutes 注册开放平台 /open/v1 路由（P6/T6.1）。
// 自 router.go 原样拆出（批次四 4a），组内注册顺序与拆分前保持一致。
func registerOpenRoutes(r *gin.Engine, app *App) {
	// 开放平台（P6/T6.1）：对外前缀 /open/v1，独立中间件链：
	// 审计（最外层，鉴权失败同样留痕）→ 签名 → 应用校验 → IP 白名单 → nonce 防重放 → 限流。
	openV1 := r.Group("/open/v1")
	openV1.Use(app.open.Audit(), app.open.Gateway())
	{
		// 链路自检：签名通过即可访问，不占能力位。POST 用于验证带 body 的签名。
		openV1.GET("/ping", app.open.Ping)
		openV1.POST("/ping", app.open.Ping)
		// 只读目录（T6.2，能力位 catalog:read）：标准契约，内部异构不出门（doc16 §8.1）。
		openV1.GET("/spec-atoms", app.open.RequireScope("catalog:read"), app.open.ListSpecAtoms)
		openV1.GET("/products", app.open.RequireScope("catalog:read"), app.open.ListProducts)
		openV1.GET("/products/:id", app.open.RequireScope("catalog:read"), app.open.GetProduct)
		openV1.GET("/regions", app.open.RequireScope("catalog:read"), app.open.ListRegions)
		openV1.GET("/images", app.open.RequireScope("catalog:read"), app.open.ListImages)
		openV1.POST("/quote", app.open.RequireScope("catalog:read"), app.open.Quote)
		// 代客下单（T6.3，能力位 order:create）：幂等键走 X-Client-Request-Id 头（doc16 §8.5）。
		openV1.POST("/orders", app.open.RequireScope("order:create"), app.open.CreateOrder)
		// 实例接口（T6.4）：查询/续费/电源/暂停恢复，按能力位细分；
		// D2 安全边界：DELETE 显式注册并固定返回 40009「不支持」（开放平台无销毁能力）。
		openV1.GET("/instances", app.open.RequireScope("instance:read"), app.open.ListInstances)
		openV1.GET("/instances/:id", app.open.RequireScope("instance:read"), app.open.GetInstance)
		openV1.POST("/instances/:id/renew", app.open.RequireScope("instance:renew"), app.open.RenewInstance)
		openV1.POST("/instances/:id/power", app.open.RequireScope("instance:power"), app.open.PowerInstance)
		openV1.POST("/instances/:id/suspend", app.open.RequireScope("instance:suspend"), app.open.SuspendInstance)
		openV1.POST("/instances/:id/unsuspend", app.open.RequireScope("instance:suspend"), app.open.UnsuspendInstance)
		openV1.DELETE("/instances/:id", app.open.DeleteInstance)
		// 对账（T6.6，能力位 audit:read）：幂等请求与代客下单订单。
		openV1.GET("/audit/requests", app.open.RequireScope("audit:read"), app.open.ListAuditRequests)
		openV1.GET("/audit/orders", app.open.RequireScope("audit:read"), app.open.ListAuditOrders)
	}
}
