package server

import (
	"github.com/gin-gonic/gin"
)

// registerUpstreamPushRoutes 注册上游推送接收端（doc120）。
//
// 路径刻意保持在 /api/... 顶层、不带 /v1 前缀：这是**上游魔方财务的既定协议**
// （源码 pushHostInfo → POST {downstream_url}/api/host/sync；pushTicketReply →
// POST {downstream_url}/api/ticket_reply/sync），路径由对方写死，我方无权改。
// 因此这组路由不能跟着我方 /api/v1/admin 或 /api/v1/uc 的版本前缀走。
//
// 鉴权：不挂任何登录中间件——调用方是上游服务器，没有我方令牌。信任来源是
// body 内的 signature（用 orders.downstream_token 验签，见 upstreampush/service）。
// 这是免登录可写入口，因此：
//   - 必须验签（服务层硬校验，失败一律拒绝）；
//   - 单独限流，避免被当成放大器刷爆；
//   - 响应只用统一结构，不透露"这个 order_id 存不存在"。
func registerUpstreamPushRoutes(r *gin.Engine, app *App) {
	if app.upstreamPushHandler == nil {
		return
	}
	// 单独一张限流桶：与 uc/admin 的额度互不占用（上游重试是机器行为，
	// 不能因为它的突发把真实用户的请求挤掉）。
	limiter := app.upstreamPushLimiter()
	if limiter != nil {
		pushGroup := r.Group("/api")
		pushGroup.Use(limiter.Handler())
		{
			pushGroup.POST("/host/sync", app.upstreamPushHandler.HostSync)
			pushGroup.POST("/ticket_reply/sync", app.upstreamPushHandler.TicketReplySync)
		}
		return
	}

	// 限流组件不可用（验证码模块未装配）时仍要接住推送，否则上游 404 会重试 5 次后放弃。
	r.POST("/api/host/sync", app.upstreamPushHandler.HostSync)
	r.POST("/api/ticket_reply/sync", app.upstreamPushHandler.TicketReplySync)
}
