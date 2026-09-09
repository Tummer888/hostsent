// Package observability 提供极简、依赖-free的指标观测能力（Prometheus 文本格式）。
//
// 用途（见 00 规划 Phase 5 · T5.3）：为上游调用、订单状态机、财务对账等关键路径提供
// 计数器与耗时累计，通过 GET /metrics 以文本暴露。不引入 prometheus client，保持轻量。
package observability

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Registry 极简计数器注册表。
type Registry struct {
	mu   sync.Mutex
	vals map[string]int64
}

var reg = &Registry{vals: map[string]int64{}}

// Inc 将名称为 name 的计数器增加 delta。
func Inc(name string, delta int64) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.vals[name] += delta
}

// Timed 通过一个耗时计数维度记录 fn 的执行（毫秒累计 + 错误数）。
// name 用作指标前缀：`<name>_duration_ms`、`<name>_errors_total`。
func Timed(name string, fn func() error) error {
	start := time.Now()
	err := fn()
	Inc(name+"_duration_ms", time.Since(start).Milliseconds())
	if err != nil {
		Inc(name+"_errors_total", 1)
	}
	return err
}

// Snapshot 返回 Prometheus 文本格式的指标快照（按名称排序）。
func Snapshot() string {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	names := make([]string, 0, len(reg.vals))
	for n := range reg.vals {
		names = append(names, n)
	}
	sort.Strings(names)

	var b strings.Builder
	for _, n := range names {
		b.WriteString(n)
		b.WriteByte(' ')
		b.WriteString(strconv.FormatInt(reg.vals[n], 10))
		b.WriteByte('\n')
	}
	return b.String()
}
