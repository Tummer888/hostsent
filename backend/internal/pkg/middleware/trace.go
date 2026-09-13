package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/pkg/traceid"
)

// TraceHeader 链路追踪请求/响应头。
const TraceHeader = "X-Trace-Id"

// traceIDKey gin.Context 中的 trace id 键。
const traceIDKey = "trace_id"

// TraceID 注入链路追踪 ID（doc92 §3.2）。
//
// 没有 trace 的日志只是流水，有 trace 的日志才能排查一次故障：审计日志
// （admin_audit_logs.trace_id）、上游接口日志、任务日志都靠这个字段串起来。
//
// 上游（网关/前端）传 X-Trace-Id 则复用，否则生成；无论来源如何都会写回响应头，
// 便于用户报障时直接给出 trace id。
func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader(TraceHeader))
		if id == "" || len(id) > traceid.MaxLen {
			id = newTraceID()
		}
		c.Set(traceIDKey, id)
		c.Request = c.Request.WithContext(traceid.With(c.Request.Context(), id))
		c.Writer.Header().Set(TraceHeader, id)
		c.Next()
	}
}

// GetTraceID 从 gin.Context 取 trace id（不存在返回空串）。
func GetTraceID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if v, ok := c.Get(traceIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// TraceIDFromContext 从 context 取 trace id（不存在返回空串）。
func TraceIDFromContext(ctx context.Context) string {
	return traceid.From(ctx)
}

// newTraceID 生成 16 字节十六进制 trace id。
func newTraceID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "t" + strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(buf)
}
