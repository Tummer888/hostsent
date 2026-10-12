// Package safego 提供 goroutine 的 panic 防护。
//
// 约定（CONTRIBUTING §5）：每个 go func() 入口必须 defer Recover(logger)，
// 保证 panic 被记录而不是击穿整个进程；长生命周期调度循环还应在每轮任务
// 执行处单独 Recover，单轮异常只记日志、不停摆调度。
package safego

import (
	"fmt"
	"os"
	"runtime/debug"

	"go.uber.org/zap"
)

// Recover 捕获当前 goroutine 的 panic 并记录错误日志（含堆栈），
// 返回是否发生了 panic，便于调用方做收尾（如向结果通道写入错误）。
// 仅用于 defer：`defer safego.Recover(logger)`。
//
// logger 为 nil 时降级输出到 stderr（基础库 goroutine 未注入 logger 的场景）。
func Recover(logger *zap.Logger) (recovered bool) {
	r := recover()
	if r == nil {
		return false
	}
	stack := debug.Stack()
	if logger == nil {
		fmt.Fprintf(os.Stderr, "safego: panic recovered in goroutine: %v\n%s", r, stack)
		return true
	}
	logger.Error("panic recovered in goroutine",
		zap.Any("panic", r),
		zap.ByteString("stack", stack))
	return true
}

// Go 以受保护的方式启动 goroutine：fn 的 panic 会被 Recover 记录而非击穿进程。
func Go(logger *zap.Logger, fn func()) {
	go func() {
		defer Recover(logger)
		fn()
	}()
}
