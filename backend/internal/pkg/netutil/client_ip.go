package netutil

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

// ClientIP 取客户端真实 IP。
//
// 解析交给 gin 的 ClientIP，前提是路由层已用 SetTrustedProxies 声明了可信代理
// （见 server.newRouter）。gin 的语义是：只有当**直连对端**本身可信时，才向右
// 逐跳剥离 X-Forwarded-For 里可信的地址，返回第一个不可信地址；直连对端不可信时
// 一律忽略这些头部、返回对端地址本身。
//
// 这里刻意**不再**自己读 CF-Connecting-IP / X-Forwarded-For / X-Real-IP：
// 旧实现无条件信任这三个头部，等于把「我是谁」的决定权交给调用方 —— 实测发送
// `X-Forwarded-For: 203.0.113.99` 就能让登录日志记下任意 IP。这不只是日志失真：
// 同一个值被 login_guard 用作 failIPKey 的锁定键，改个头部即可绕过 IP 维度的
// 撞库锁定（doc91 §9.1）。
//
// 顺带说明为什么不保留 CF-Connecting-IP 的特殊分支：Cloudflare 同样会写
// X-Forwarded-For，把 CF 的回源段加进 trusted_proxies 即可正常解析，
// 单独信任一个头部只会多开一个绕过口。
func ClientIP(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if ip := strings.TrimSpace(c.ClientIP()); ip != "" {
		return ip
	}
	// ClientIP 兜底：极端情况下（如 RemoteAddr 不是 host:port 形式）它可能返回空，
	// 此时退回对端地址，绝不退回任何请求头。
	host, _, err := net.SplitHostPort(strings.TrimSpace(c.Request.RemoteAddr))
	if err != nil {
		return strings.TrimSpace(c.Request.RemoteAddr)
	}
	return host
}
