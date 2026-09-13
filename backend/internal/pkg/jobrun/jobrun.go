// Package jobrun 是任务运行留痕的中性端点（doc92 §7.1/§7.2）。
//
// 设计动机：业务调度器（sync/lifecycle/sales/order/open/notification）不能 import
// admin/logcenter 模块 —— 那会把日志中心的仓储、模型与 admin 依赖拖进业务包，
// 形成反向依赖。因此这里只放一个接口 + 一个包级单例（与 internal/pkg/upstream
// 的 Recorder 同款做法），由 internal/server 在装配时注入日志中心的 Runner。
//
// 未注入时 Run 直接执行任务体，零开销、零行为变化。
package jobrun

import (
	"context"
	"sync/atomic"
)

// 触发方式（与 logmodel.Trigger* 同值，此处不 import 以保持零依赖）。
const (
	TriggerScheduled = "scheduled"
	TriggerManual    = "manual"
)

// HighFrequencyJobs 轮询间隔 ≤ 60 秒的任务：本轮无处理对象时不落 job_run_logs。
//
// 不这样做的代价：notify_delivery 15 秒一轮 = 5760 行/天，180 天约 104 万行，
// 只为记录「本轮扫了 0 条」（doc92 §7.2）。
var HighFrequencyJobs = map[string]bool{
	"sales_release":       true,
	"open_notify_deliver": true,
	"notify_delivery":     true,
}

// ShouldSkipEmpty 报告该任务是否属于「空轮不落库」的高频任务。
func ShouldSkipEmpty(jobName string) bool { return HighFrequencyJobs[jobName] }

// Func 任务体：返回 (扫描数, 处理数, 摘要)。摘要为 nil 时不写。
type Func func(ctx context.Context) (scanned, affected int, summary map[string]any)

// TypedFunc 带 error 的任务体，与 logcenter Runner 的签名对齐。
type TypedFunc func(ctx context.Context) (scanned, affected int, summary map[string]any, err error)

// Runner 任务留痕执行器。实现必须保证自身异常绝不影响任务体执行。
type Runner interface {
	Run(ctx context.Context, jobName, jobGroup, triggerType string, fn TypedFunc)
}

var global atomic.Value // Runner

// SetRunner 注入执行器（装配层调用一次）。
func SetRunner(r Runner) { global.Store(r) }

// current 当前执行器（可能为 nil）。
func current() Runner {
	v := global.Load()
	if v == nil {
		return nil
	}
	r, _ := v.(Runner)
	return r
}

// Run 执行任务并留痕。runner 未注入时直接执行任务体。
//
// 本函数只做转发，不含业务判断：是否落库（空轮跳过、单日上限）由注入的
// Runner 决定，业务调度器不必关心。
func Run(ctx context.Context, jobName, jobGroup, triggerType string, fn Func) {
	if fn == nil {
		return
	}
	r := current()
	if r == nil {
		fn(ctx)
		return
	}
	r.Run(ctx, jobName, jobGroup, triggerType, func(ctx context.Context) (int, int, map[string]any, error) {
		scanned, affected, summary := fn(ctx)
		return scanned, affected, summary, nil
	})
}

// RunErr 同 Run，但任务体可返回 error（用于本来就把 err 记进日志的调度器）。
func RunErr(ctx context.Context, jobName, jobGroup, triggerType string, fn TypedFunc) {
	if fn == nil {
		return
	}
	r := current()
	if r == nil {
		_, _, _, _ = fn(ctx)
		return
	}
	r.Run(ctx, jobName, jobGroup, triggerType, fn)
}
