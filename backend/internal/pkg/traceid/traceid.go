// Package traceid 提供链路追踪 ID 的中性 context 载体。
//
// 为什么单独一个包：写入方是 HTTP 中间件（internal/pkg/middleware），读取方是
// 上游适配器（internal/pkg/upstream）与日志服务。若让 upstream 直接 import
// middleware，会把 gin/模块依赖拖进纯适配器包（并可能形成环），所以把「一个字符串
// 放进 context」这件小事独立出来，两侧只依赖本包。
package traceid

import "context"

type ctxKey struct{}

// MaxLen 允许的最大长度（与 upstream_api_logs.trace_id 列宽一致）。
const MaxLen = 64

// With 把 trace id 放进 context；空串原样返回。
func With(ctx context.Context, id string) context.Context {
	if id == "" || ctx == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, id)
}

// From 从 context 取 trace id（不存在返回空串）。
func From(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(ctxKey{}).(string); ok {
		return v
	}
	return ""
}
