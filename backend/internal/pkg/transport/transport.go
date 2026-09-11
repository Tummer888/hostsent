// Package transport 实现契约③「传输契约」（docs/实施计划/16 §5、17 §P2/T2.4）。
//
// 现状与目标：
//   - 各适配器自己拼签名、自己判定错误码、无统一限流与退避；
//   - 本期抽出 Signer / 错误码归一 / 渠道级令牌桶 / 可配超时重试的骨架，
//     供后续接入异构上游（阿里云 RPC、腾讯云 TC3）时复用。
//
// 边界（17 §8）：不改写 mofangyun / mofangfinance 现有传输实现，
// 本包只提供新适配器可用的构件，并让既有 ProviderError 能被统一分类，
// 从而让同步熔断/告警能按错误性质决策（重试 / 熔断 / 人工）。
package transport

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"hostsent/backend/internal/pkg/upstream"
)

// ============================================================================
// 错误码归一
// ============================================================================

// ErrKind 归一后的上游错误类别，决定后续处置方式。
type ErrKind string

const (
	ErrAuthFailed       ErrKind = "AuthFailed"       // 凭证失效/权限不足 → 熔断 + 告警
	ErrRateLimited      ErrKind = "RateLimited"      // 限流 → 长退避后重试
	ErrQuotaExceeded    ErrKind = "QuotaExceeded"    // 配额用尽 → 人工
	ErrNotFound         ErrKind = "NotFound"         // 对象不存在 → 幂等视为成功
	ErrInvalidParam     ErrKind = "InvalidParam"     // 参数错误 → 不重试，人工
	ErrUpstreamInternal ErrKind = "UpstreamInternal" // 上游 5xx → 正常退避重试
	ErrNotSupported     ErrKind = "NotSupported"     // 能力缺失 → 不重试
	ErrNetwork          ErrKind = "Network"          // 网络层错误 → 退避重试
	ErrUnknown          ErrKind = "Unknown"          // 未识别 → 保守重试
)

// KindOf 将错误归一为 ErrKind。识别顺序：能力缺失 → ProviderError → 网络 → 兜底。
func KindOf(err error) ErrKind {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return ErrNetwork
	}
	var notImpl upstream.ErrNotImplemented
	if errors.As(err, &notImpl) {
		return ErrNotSupported
	}
	var pe *upstream.ProviderError
	if errors.As(err, &pe) {
		return kindOfProviderError(pe)
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return ErrNetwork
	}
	return ErrUnknown
}

// kindOfProviderError 依据 HTTP 状态码与上游业务码判定类别。
func kindOfProviderError(pe *upstream.ProviderError) ErrKind {
	switch pe.StatusCode {
	case 401, 403:
		return ErrAuthFailed
	case 404:
		return ErrNotFound
	case 429:
		return ErrRateLimited
	case 400, 422:
		return ErrInvalidParam
	}
	if pe.StatusCode >= 500 {
		return ErrUpstreamInternal
	}
	// 魔方系适配器在 HTTP 2xx 时把业务码放在 Code 字段（如 405=token 失效），
	// 部分实现也把 HTTP 状态塞进 Code；按数值区间兜底判定。
	switch pe.Code {
	case 401, 403:
		return ErrAuthFailed
	case 404:
		return ErrNotFound
	case 429:
		return ErrRateLimited
	}
	if pe.Code >= 500 {
		return ErrUpstreamInternal
	}
	msg := strings.ToLower(pe.Msg)
	switch {
	case strings.Contains(msg, "token") || strings.Contains(msg, "jwt") ||
		strings.Contains(msg, "密码错误") || strings.Contains(msg, "unauthor") ||
		strings.Contains(msg, "登录失败") || strings.Contains(msg, "凭证"):
		return ErrAuthFailed
	case strings.Contains(msg, "限流") || strings.Contains(msg, "too many request") ||
		strings.Contains(msg, "rate limit") || strings.Contains(msg, "频繁"):
		return ErrRateLimited
	case strings.Contains(msg, "not found") || strings.Contains(msg, "不存在"):
		return ErrNotFound
	case strings.Contains(msg, "不支持") || strings.Contains(msg, "not supported") ||
		strings.Contains(msg, "未实现"):
		return ErrNotSupported
	case strings.Contains(msg, "已存在") || strings.Contains(msg, "exists"):
		// 幂等重复创建，视为可接受结果。
		return ErrNotFound
	}
	if pe.Err != nil {
		var netErr net.Error
		if errors.As(pe.Err, &netErr) {
			return ErrNetwork
		}
	}
	return ErrUnknown
}

// Retryable 判断该错误类别是否值得重试。
func Retryable(kind ErrKind) bool {
	switch kind {
	case ErrRateLimited, ErrUpstreamInternal, ErrNetwork, ErrUnknown:
		return true
	default:
		return false
	}
}

// ShouldPauseProvider 判断该错误是否应触发渠道熔断（凭证/能力类问题重试无意义）。
func ShouldPauseProvider(kind ErrKind) bool {
	return kind == ErrAuthFailed || kind == ErrNotSupported
}

// Describe 给出面向运维的中文说明，供 last_sync_error 使用。
func Describe(kind ErrKind) string {
	switch kind {
	case ErrAuthFailed:
		return "上游认证失败（凭证/权限问题）"
	case ErrRateLimited:
		return "上游限流"
	case ErrQuotaExceeded:
		return "上游配额用尽"
	case ErrNotFound:
		return "上游对象不存在"
	case ErrInvalidParam:
		return "请求参数错误"
	case ErrUpstreamInternal:
		return "上游内部错误"
	case ErrNotSupported:
		return "上游不支持该能力"
	case ErrNetwork:
		return "网络不可达"
	default:
		return "上游调用失败"
	}
}

// ============================================================================
// 限流退避
// ============================================================================

// RateLimitSpec 渠道限流参数（与 upstream.RateLimitSpec 同形，便于直接透传）。
type RateLimitSpec = upstream.RateLimitSpec

// Limiter 令牌桶限流器：按渠道维度控制 QPS，突发不超 burst。
type Limiter struct {
	mu       sync.Mutex
	tokens   float64
	last     time.Time
	qps      float64
	capacity float64

	now func() time.Time // 便于测试注入
}

// NewLimiter 创建令牌桶；qps<=0 表示不限流，返回 nil 由调用方跳过等待。
func NewLimiter(spec RateLimitSpec) *Limiter {
	if spec.QPS <= 0 {
		return nil
	}
	capacity := spec.Burst
	if capacity <= 0 {
		capacity = spec.QPS
	}
	return &Limiter{
		tokens:   float64(capacity),
		last:     time.Now(),
		qps:      float64(spec.QPS),
		capacity: float64(capacity),
		now:      time.Now,
	}
}

// Wait 阻塞直到取得一个令牌或 ctx 结束，返回实际等待时长。
// limiter 为 nil（未配置限流）时立即返回。
func (l *Limiter) Wait(ctx context.Context) (time.Duration, error) {
	if l == nil {
		return 0, nil
	}
	start := l.now()
	for {
		wait := l.reserve()
		if wait <= 0 {
			return l.now().Sub(start), nil
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return l.now().Sub(start), ctx.Err()
		case <-timer.C:
		}
	}
}

// reserve 补充令牌并返回需要等待的时长；取到令牌返回 0。
// 调用方需串行调用（Wait 内部循环调用），故用互斥保护。
func (l *Limiter) reserve() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	elapsed := now.Sub(l.last).Seconds()
	if elapsed > 0 {
		l.tokens = math.Min(l.capacity, l.tokens+elapsed*l.qps)
		l.last = now
	}
	if l.tokens >= 1 {
		l.tokens--
		return 0
	}
	need := (1 - l.tokens) / l.qps
	return time.Duration(need * float64(time.Second))
}

// Backoff 计算第 attempt 次（从 1 起）重试前的等待时长：指数退避 + 抖动。
// RateLimited 单独走更长基线（上游明确要求退避，避免加剧封禁）。
func Backoff(kind ErrKind, attempt int, base time.Duration) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if base <= 0 {
		base = 500 * time.Millisecond
	}
	if kind == ErrRateLimited {
		base *= 4
	}
	shift := attempt - 1
	if shift > 6 { // 封顶 64 倍，避免溢出
		shift = 6
	}
	d := base * time.Duration(1<<uint(shift))
	const maxDelay = 2 * time.Minute
	if d > maxDelay {
		d = maxDelay
	}
	// ±20% 抖动，避免多渠道同时重试形成尖峰。
	jitter := 1 + (rand.Float64()*0.4 - 0.2)
	return time.Duration(float64(d) * jitter)
}

// ============================================================================
// 可配超时 + 重试
// ============================================================================

// RetryPolicy 可配的调用策略（对应 resource_providers.timeout_seconds/retry_max）。
type RetryPolicy struct {
	Timeout    time.Duration // 单次调用超时，<=0 取 DefaultTimeout
	MaxRetries int           // 额外重试次数，<=0 表示不重试
	BaseDelay  time.Duration // 退避基线，<=0 取 DefaultBaseDelay
}

// DefaultTimeout 与 schema 默认值一致（原 provider_service 硬编码 15s 的显式化）。
const (
	DefaultTimeout   = 15 * time.Second
	DefaultBaseDelay = 500 * time.Millisecond
)

// Normalize 补齐零值。
func (p RetryPolicy) Normalize() RetryPolicy {
	if p.Timeout <= 0 {
		p.Timeout = DefaultTimeout
	}
	if p.MaxRetries < 0 {
		p.MaxRetries = 0
	}
	if p.BaseDelay <= 0 {
		p.BaseDelay = DefaultBaseDelay
	}
	return p
}

// Do 按策略执行 fn：每次调用套独立超时，失败按错误类别决定是否退避重试。
// 不可重试的错误（凭证/参数/能力缺失）立即返回，不浪费上游配额。
func (p RetryPolicy) Do(ctx context.Context, fn func(context.Context) error) error {
	p = p.Normalize()
	var lastErr error
	for attempt := 0; attempt <= p.MaxRetries; attempt++ {
		if attempt > 0 {
			kind := KindOf(lastErr)
			if !Retryable(kind) {
				return lastErr
			}
			delay := Backoff(kind, attempt, p.BaseDelay)
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		callCtx, cancel := context.WithTimeout(ctx, p.Timeout)
		err := fn(callCtx)
		cancel()
		if err == nil {
			return nil
		}
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return ctx.Err()
		}
		lastErr = err
	}
	return lastErr
}

// ============================================================================
// 错误包装
// ============================================================================

// NormalizedError 携带归一类别的最小包装，便于上层按类别分支而无需重复分类。
type NormalizedError struct {
	Kind ErrKind
	Err  error
}

func (e *NormalizedError) Error() string { return e.Err.Error() }
func (e *NormalizedError) Unwrap() error { return e.Err }

// Normalize 包装错误并标注类别；err 为 nil 时返回 nil。
func Normalize(err error) error {
	if err == nil {
		return nil
	}
	return &NormalizedError{Kind: KindOf(err), Err: err}
}

// FormatAttempt 供日志/错误信息展示重试进度。
func FormatAttempt(attempt, max int) string {
	return fmt.Sprintf("attempt %d/%d", attempt, max)
}

// ParseRateLimit 从列值解析限流参数（0 视为不限）。
func ParseRateLimit(qps, burst int) RateLimitSpec {
	return RateLimitSpec{QPS: qps, Burst: burst}
}

// ParsePositiveInt 解析正整数配置，失败返回 fallback。
func ParsePositiveInt(raw string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
