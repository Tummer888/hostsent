// Package middleware 提供跨模块复用的 gin 中间件（鉴权、审计、限流等）。
//
// 本文件为基于缓存原语的请求限流中间件（doc91 §9.1 api_rate_limit）：
// 按「主体（登录用户/管理员）+ 客户端 IP + 路由」计数，超限返回 429 + 20012。
// 配置为 0 表示不限流；缓存不可用时降级放行（与图形码降级同口径，D11）。
package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// Counter 计数端口（由 cache.Client 满足）。
type Counter interface {
	Enabled() bool
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

// LimitReader 读取每分钟限额（0=不限）。
type LimitReader func(ctx context.Context) int

// RateLimiter 限流中间件。
type RateLimiter struct {
	counter Counter
	limit   LimitReader
	// scope 用于 key 前缀，区分 admin 与 uc。
	scope string
	// warn 降级告警（同一 scope 每分钟至多一条，避免刷屏）。
	warn func(path string)
}

// RateLimitOptions 限流中间件构造参数。
type RateLimitOptions struct {
	Counter Counter
	Limit   LimitReader
	Scope   string
	Warn    func(path string)
}

// NewRateLimiter 创建限流中间件。
func NewRateLimiter(opts RateLimitOptions) *RateLimiter {
	return &RateLimiter{counter: opts.Counter, limit: opts.Limit, scope: opts.Scope, warn: opts.Warn}
}

// Handler 返回 gin 中间件。
func (m *RateLimiter) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if m == nil || m.counter == nil || m.limit == nil {
			c.Next()
			return
		}
		limit := m.limit(c.Request.Context())
		if limit <= 0 {
			c.Next()
			return
		}
		if !m.counter.Enabled() {
			// 降级：限流不生效但不阻断请求，记一次告警供事后取证。
			if m.warn != nil {
				m.warn(c.FullPath())
			}
			c.Next()
			return
		}
		key := "rl:" + m.scope + ":" + clientKey(c) + ":" + c.FullPath()
		n, err := m.counter.Incr(c.Request.Context(), key, time.Minute)
		if err != nil {
			c.Next()
			return
		}
		if n > int64(limit) {
			response.ErrorWithStatus(c, http.StatusTooManyRequests,
				apperrors.New(20012, "请求过于频繁，请稍后再试"))
			c.Abort()
			return
		}
		c.Next()
	}
}

// clientKey 取主体标识：优先登录用户，回落 IP。
func clientKey(c *gin.Context) string {
	if claims, ok := GetAdminClaims(c); ok && claims.AdminID > 0 {
		return "adm" + u64(claims.AdminID)
	}
	if claims, ok := GetUserClaims(c); ok && claims.UserID > 0 {
		return "usr" + u64(claims.UserID)
	}
	if claims, ok := GetClaims(c); ok && claims.UserID > 0 {
		return "usr" + u64(claims.UserID)
	}
	return "ip" + c.ClientIP()
}

func u64(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
