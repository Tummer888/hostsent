package upstream

// 本文件是「上游调用可观测」的埋点端（doc92 §3.1）。
//
// 约束：internal/pkg/upstream 是纯适配器包，不 import 任何 DB/仓储包。所以这里只定义
// 数据结构 + 包级接收器钩子 + 采样开关，真正落库的实现由 logcenter 模块在装配时注入
// （SetRecorder）。未注入时是零开销空实现，适配器代码不需要任何分支判断。
//
// 埋点方式（关键设计）：适配器的 httpDo/httpForm 只有 *http.Request，拿不到业务 op 名。
// 因此业务方法（call/callShell）通过 context 放一个 *Trace，httpDo 用同一个 ctx 把
// 「一次 HTTP 往返」投进 Trace；业务方法在返回时用 defer 结束 Trace 并回填业务判定结果
// （success/error_code/message）。这样既不破坏既有函数签名，又能让登录请求
// （无 Trace）与业务请求（有 Trace、可能重试）都得到记录。
//
// trace_id 用来把审计日志、上游日志、任务日志串成一条链路：来源是请求头注入的
// X-Trace-Id（middleware.TraceID），适配器不做生成。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"math/rand"
	"net/url"
	"strings"
	"sync"
	"time"

	"hostsent/backend/internal/pkg/mask"
	"hostsent/backend/internal/pkg/observability"
)

// Record 一条上游调用记录（已脱敏、已截断）。
type Record struct {
	ProviderID   uint
	ProviderName string
	ProviderType string
	Op           string
	Method       string
	URL          string

	RequestDigest    string
	RequestBody      string
	RequestBytes     int
	RequestTruncated bool

	StatusCode        int
	ResponseDigest    string
	ResponseBody      string
	ResponseBytes     int
	ResponseTruncated bool

	Success      bool
	ErrorCode    string
	ErrorMessage string
	DurationMS   int
	RetryIndex   int
	TraceID      string
	CreatedAt    time.Time
}

// Recorder 上游调用记录接收器。实现必须非阻塞（内部缓冲），
// 因为它是在请求主链路上被同步调用的 —— 把日志系统的问题变成业务故障是不可接受的。
type Recorder interface {
	Record(Record)
}

// nopRecorder 未注入实现时的空接收器。
type nopRecorder struct{}

func (nopRecorder) Record(Record) {}

var recorder Recorder = nopRecorder{}

// SetRecorder 注入记录接收器；传 nil 恢复空实现。
func SetRecorder(r Recorder) {
	if r == nil {
		recorder = nopRecorder{}
		return
	}
	recorder = r
}

// CaptureOptions 上游日志采集参数（来自 system_configs，由装配层周期刷新）。
type CaptureOptions struct {
	// Enabled 对应 log_upstream_capture：false 时完全不写日志。
	Enabled bool
	// SampleRate 百分比（0–100）；100 表示全量，0 表示不采集。
	SampleRate int
	// BodyMaxBytes 正文截断上限（对应 log_upstream_body_max_bytes）。
	BodyMaxBytes int
}

var (
	optsMu     sync.RWMutex
	captureOpt = CaptureOptions{Enabled: true, SampleRate: 100, BodyMaxBytes: 4096}
)

// SetCaptureOptions 更新采集参数（装配层在启动与配置变更后调用）。
//
// 边界收敛：BodyMaxBytes <= 0 时回落到 4096，避免「不截断」把大响应体写进库。
func SetCaptureOptions(o CaptureOptions) {
	if o.BodyMaxBytes <= 0 {
		o.BodyMaxBytes = 4096
	}
	if o.SampleRate < 0 {
		o.SampleRate = 0
	}
	if o.SampleRate > 100 {
		o.SampleRate = 100
	}
	optsMu.Lock()
	captureOpt = o
	optsMu.Unlock()
}

// CaptureOptionsSnapshot 当前采集参数（测试与诊断用）。
func CaptureOptionsSnapshot() CaptureOptions {
	optsMu.RLock()
	defer optsMu.RUnlock()
	return captureOpt
}

// captureEnabled 本次是否应当采集（开关 + 采样）。
func captureEnabled() bool {
	optsMu.RLock()
	o := captureOpt
	optsMu.RUnlock()
	if !o.Enabled {
		return false
	}
	if o.SampleRate <= 0 {
		observability.Inc("upstream_log_sampled_out_total", 1)
		return false
	}
	if o.SampleRate >= 100 {
		return true
	}
	if rand.Intn(100) >= o.SampleRate { //nolint:gosec // 采样不需要密码学随机
		observability.Inc("upstream_log_sampled_out_total", 1)
		return false
	}
	return true
}

// httpCall 一次 HTTP 往返的原始观测。
type httpCall struct {
	method     string
	rawURL     string
	reqBody    []byte
	statusCode int
	respBody   []byte
	err        error
	duration   time.Duration
	retryIndex int
}

// Trace 一次上游业务调用（可能含重试）的采集上下文。
//
// 非并发安全地跨 goroutine 使用是不允许的：它只在一次调用栈里被 httpDo 与 call 访问，
// 但适配器内部有 mutex 保护 token，为稳妥起见这里也用锁。
type Trace struct {
	mu       sync.Mutex
	op       string
	traceID  string
	attempts []httpCall // 已完成的 HTTP 往返（含失败重试）
	provider func() (id uint, name, typ string)
}

type ctxKey struct{}

// WithOp 把业务操作名与 trace_id 放进 ctx。
func WithOp(ctx context.Context, op, traceID string) context.Context {
	return context.WithValue(ctx, ctxKey{}, &traceHolder{op: op, traceID: traceID})
}

type traceHolder struct {
	op      string
	traceID string
	trace   *Trace
}

// opFromCtx 读取 ctx 里的 op 与 trace_id。
func opFromCtx(ctx context.Context) (string, string) {
	if ctx == nil {
		return "", ""
	}
	if h, ok := ctx.Value(ctxKey{}).(*traceHolder); ok && h != nil {
		return h.op, h.traceID
	}
	return "", ""
}

// NewTrace 在 ctx 上挂一个采集上下文并返回它。op/traceID 同时进 holder，
// 保证 httpDo 侧无需参数即可取到。
func NewTrace(ctx context.Context, op, traceID string) (context.Context, *Trace) {
	tr := &Trace{op: op, traceID: traceID}
	ctx = context.WithValue(ctx, ctxKey{}, &traceHolder{op: op, traceID: traceID, trace: tr})
	return ctx, tr
}

// TraceFromCtx 取 ctx 上的采集上下文；没有则返回 nil（登录等无业务判定场景）。
func TraceFromCtx(ctx context.Context) *Trace {
	if ctx == nil {
		return nil
	}
	if h, ok := ctx.Value(ctxKey{}).(*traceHolder); ok && h != nil {
		return h.trace
	}
	return nil
}

// SetProvider 让记录带上渠道信息（适配器在构造时注入，避免每次调用重复解析）。
func (t *Trace) SetProvider(fn func() (uint, string, string)) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.provider = fn
	t.mu.Unlock()
}

// capture 记录一次 HTTP 往返。
//
// 若此前已有待上报的往返（说明这是重试或同一次业务调用里的登录往返），先把前一次
// 结构性地上报 ——「第一次 401 → 重新登录 → 第二次成功」这段过程本身就是排障最需要
// 的信息，而登录请求体的脱敏（验收场景 3）也只能在这个位置被验证到。
func (t *Trace) capture(call httpCall) {
	if t == nil {
		return
	}
	t.mu.Lock()
	prev := append([]httpCall(nil), t.attempts...)
	// 重试序号：与本次同方法同 URL 的历史往返数（0 = 首次）。
	for i := range prev {
		if prev[i].method == call.method && prev[i].rawURL == call.rawURL {
			call.retryIndex++
		}
	}
	t.attempts = append(t.attempts, call)
	t.mu.Unlock()

	if len(prev) > 0 {
		last := prev[len(prev)-1]
		// 结构性记录没有业务判定，只能用 HTTP 层结果判定成功与否。
		t.emit(last, httpSuccess(last), "", "")
	}
}

// httpSuccess HTTP 层成功判定（无传输错误且状态码 2xx）。
func httpSuccess(call httpCall) bool {
	return call.err == nil && call.statusCode >= 200 && call.statusCode < 300
}

// Finish 结束采集：用 HTTP 层结果自动判定成功与否并上报。
//
// 适用于「返回 err 即业务失败」的适配器（如魔方云 call：body.error 会被包成 err）。
func (t *Trace) Finish(err error) {
	t.finish(err, false, "", "", false)
}

// FinishBusiness 结束采集：由调用方显式给出业务判定结果。
//
// 适用于 HTTP 2xx 仍可能是业务失败的平台（魔方财务响应壳 status/code 才是真相）。
// code/message 为空时会从 err 里的 *ProviderError 提取。
func (t *Trace) FinishBusiness(err error, success bool, code, message string) {
	t.finish(err, success, code, message, true)
}

// finish 生成最终记录并上报。
func (t *Trace) finish(err error, success bool, code, message string, explicit bool) {
	if t == nil {
		return
	}
	t.mu.Lock()
	if len(t.attempts) == 0 {
		t.mu.Unlock()
		return
	}
	last := t.attempts[len(t.attempts)-1]
	t.mu.Unlock()

	if !explicit {
		success = err == nil && httpSuccess(last)
	}
	if err != nil {
		if message == "" {
			message = err.Error()
		}
		var pe *ProviderError
		if errors.As(err, &pe) {
			if code == "" {
				// 业务码优先，没有业务码时退到 HTTP 状态码 —— 适配器经常只给
				// StatusCode（如「请求失败,HTTP状态码:404」），若不回填，
				// 失败的记录里 error_code 恒为空，按码聚合排障就无从下手。
				switch {
				case pe.Code != 0:
					code = itoa(pe.Code)
				case pe.StatusCode != 0:
					code = itoa(pe.StatusCode)
				}
			}
			if pe.Msg != "" {
				message = pe.Msg
			}
			if pe.StatusCode != 0 {
				last.statusCode = pe.StatusCode
			}
		}
	}
	t.emit(last, success, code, message)
}

// emit 生成并上报一条记录（含无业务判定的结构性往返）。
//
// 失败必留码（doc92 §10 场景 2）：业务码由调用方回填，没有业务码时退到 HTTP
// 状态码；纯传输层失败（DNS/连接被拒/超时）没有码可退，用固定 token "network"
// —— 宁可给一个可 grep 的类别，也不要让失败记录的错误码恒为空。
func (t *Trace) emit(call httpCall, success bool, code, message string) {
	if !captureEnabled() {
		return
	}
	if !success && code == "" {
		if call.statusCode != 0 {
			code = itoa(call.statusCode)
		} else {
			code = "network"
		}
	}
	if !success && call.err != nil && call.statusCode == 0 {
		message = firstNonEmpty(message, call.err.Error())
	}
	rec := buildRecord(t, call, success, code, message)
	recorder.Record(rec)
	observability.Inc("upstream_log_recorded_total", 1)
}

// Capture 适配器侧入口：httpDo/httpForm 拿到响应后调用。
//
// ctx 上没有 Trace（登录、健康检查等）时，直接生成一条独立记录并按结构判定成功，
// 保证「登录失败」这类最关键的排障线索不会因为没有业务包装而丢失。
func Capture(ctx context.Context, method, rawURL string, reqBody []byte, statusCode int, respBody []byte, callErr error, duration time.Duration) {
	if !captureEnabled() {
		return
	}
	call := httpCall{
		method: method, rawURL: rawURL, reqBody: reqBody,
		statusCode: statusCode, respBody: respBody, err: callErr, duration: duration,
	}
	if tr := TraceFromCtx(ctx); tr != nil {
		tr.capture(call)
		return
	}
	op, traceID := opFromCtx(ctx)
	tmp := &Trace{op: op, traceID: traceID}
	success := callErr == nil && statusCode >= 200 && statusCode < 300
	var message string
	if callErr != nil {
		message = callErr.Error()
	}
	tmp.emit(call, success, "", message)
}

// buildRecord 组装记录：摘要恒写，正文脱敏后按配置截断。
func buildRecord(t *Trace, call httpCall, success bool, code, message string) Record {
	optsMu.RLock()
	maxBytes := captureOpt.BodyMaxBytes
	optsMu.RUnlock()

	rawReq := call.reqBody
	if len(rawReq) == 0 {
		// GET 的参数在 query 上：把 query 当作「请求体」参与摘要与脱敏，
		// 否则 ?jwt=xxx 这类用法会漏脱敏。
		if u, err := url.Parse(call.rawURL); err == nil && u.RawQuery != "" {
			rawReq = []byte(u.RawQuery)
		}
	}
	reqDigest := digest(rawReq)
	reqBody := mask.MaskGenericBody(string(rawReq), maxBytes)

	respDigest := digest(call.respBody)
	respBody := mask.MaskGenericBody(string(call.respBody), maxBytes)

	var providerID uint
	var providerName, providerType string
	if t.provider != nil {
		providerID, providerName, providerType = t.provider()
	}

	return Record{
		ProviderID:        providerID,
		ProviderName:      providerName,
		ProviderType:      providerType,
		Op:                t.op,
		Method:            strings.ToUpper(call.method),
		URL:               mask.MaskURL(call.rawURL),
		RequestDigest:     reqDigest,
		RequestBody:       reqBody,
		RequestBytes:      len(rawReq),
		RequestTruncated:  len(rawReq) > maxBytes,
		StatusCode:        call.statusCode,
		ResponseDigest:    respDigest,
		ResponseBody:      respBody,
		ResponseBytes:     len(call.respBody),
		ResponseTruncated: len(call.respBody) > maxBytes,
		Success:           success,
		ErrorCode:         code,
		ErrorMessage:      mask.Truncate(message, 500),
		DurationMS:        int(call.duration.Milliseconds()),
		RetryIndex:        call.retryIndex,
		TraceID:           t.traceID,
		CreatedAt:         time.Now(),
	}
}

// digest 计算 sha256 摘要；空输入返回空串（而不是空内容的哈希，便于区分「没传」）。
func digest(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// RequestBodyOf 读取请求体的原始字节（供适配器在发起请求前调用）。
//
// 只读 req.GetBody：适配器构造请求时用 strings.NewReader 提供了 GetBody，
// 因此这里能拿到完整原文用于摘要，而不会与 client.Do 争抢同一个 body。
func RequestBodyOf(getBody func() (io.ReadCloser, error)) []byte {
	if getBody == nil {
		return nil
	}
	rc, err := getBody()
	if err != nil || rc == nil {
		return nil
	}
	defer rc.Close()
	// 摘要需要完整原文，但超过 1MB 的表单正文不属于本包要处理的形态，
	// 截住以避免把内存吃光。
	buf, _ := io.ReadAll(io.LimitReader(rc, 1<<20))
	return buf
}

// firstNonEmpty 返回第一个非空字符串（与适配器内部同名工具语义一致）。
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// itoa 整数转字符串（错误码回填用）。
func itoa(v int) string {
	if v == 0 {
		return ""
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
