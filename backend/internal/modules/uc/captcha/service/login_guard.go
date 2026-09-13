package service

import (
	"context"
	"fmt"
	"time"
)

// 登录失败锁定错误码（doc91 §9.1）。
const CodeLoginLocked = 20014

// ErrLoginLocked 账号或 IP 已锁定。
type ErrLoginLocked struct {
	// RetryAfter 剩余锁定秒数。
	RetryAfter int
	// Scope 锁定维度：account / ip。
	Scope string
}

func (e *ErrLoginLocked) Error() string {
	return fmt.Sprintf("登录失败次数过多，账号已锁定，请 %d 分钟后再试", (e.RetryAfter+59)/60)
}

// LoginGuard 登录失败锁定（doc91 §9.1 / C6）。
//
// 开关 login_fail_lock 默认关闭 → 行为与现状完全一致（错多少次都能继续试）。
// 开启后按阈值锁定账号与 IP 两个维度。
type LoginGuard struct {
	switches *Switches
	cache    CachePort
	// recorder 登录日志落库（真实落库后降级路径才可用，doc91 §1.5）。
	recorder LoginLogRecorder
}

// LoginLogRecorder 登录日志写入端口（装配层用 security 模块仓储实现）。
type LoginLogRecorder interface {
	RecordLogin(ctx context.Context, entry LoginLogEntry) error
	// CountRecentFailures 统计窗口内失败次数（缓存降级路径用）。
	CountRecentFailures(ctx context.Context, account string, ip string, since time.Time) (int64, error)
}

// LoginLogEntry 一条登录日志。
type LoginLogEntry struct {
	UserID        uint64
	Username      string
	LoginType     string
	Result        string
	FailureReason string
	IP            string
	IPRegion      string
	UserAgent     string
	Platform      string
}

// 登录结果常量（与 login_logs.result 口径一致）。
const (
	LoginResultSuccess = "success"
	LoginResultFailed  = "failed"
)

// NewLoginGuard 创建登录守卫。
func NewLoginGuard(switches *Switches, cache CachePort, recorder LoginLogRecorder) *LoginGuard {
	return &LoginGuard{switches: switches, cache: cache, recorder: recorder}
}

// CheckLocked 检查是否处于锁定状态；锁定时返回 *ErrLoginLocked。
func (g *LoginGuard) CheckLocked(ctx context.Context, account, ip string) error {
	if g == nil || g.switches == nil || !g.switches.LoginFailLock(ctx) {
		return nil
	}
	threshold := g.switches.LoginFailThreshold(ctx)
	if threshold <= 0 {
		return nil
	}
	if g.cache != nil && g.cache.Enabled() {
		if n, ok := g.counterValue(ctx, failAcctKey(account)); ok && n >= threshold {
			return &ErrLoginLocked{RetryAfter: g.remaining(ctx, failAcctKey(account)), Scope: "account"}
		}
		if ip != "" {
			if n, ok := g.counterValue(ctx, failIPKey(ip)); ok && n >= threshold {
				return &ErrLoginLocked{RetryAfter: g.remaining(ctx, failIPKey(ip)), Scope: "ip"}
			}
		}
		return nil
	}
	// 降级：查 login_logs（doc91 §2.4 登录失败计数行）。
	g.cache.WarnDegraded("login_guard", "captcha.CheckLocked")
	if g.recorder == nil {
		return nil
	}
	window := time.Duration(g.switches.LoginLockMinutes(ctx)) * time.Minute
	n, err := g.recorder.CountRecentFailures(ctx, account, ip, time.Now().Add(-window))
	if err != nil {
		return nil
	}
	if n >= int64(threshold) {
		return &ErrLoginLocked{RetryAfter: int(window / time.Second), Scope: "account"}
	}
	return nil
}

// RecordFailure 记录一次登录失败；达到阈值时锁定时长由 login_lock_minutes 决定。
func (g *LoginGuard) RecordFailure(ctx context.Context, account, ip string) {
	if g == nil || g.switches == nil || !g.switches.LoginFailLock(ctx) {
		return
	}
	lockFor := time.Duration(g.switches.LoginLockMinutes(ctx)) * time.Minute
	if lockFor <= 0 {
		lockFor = 15 * time.Minute
	}
	if g.cache != nil && g.cache.Enabled() {
		_, _ = g.cache.Incr(ctx, failAcctKey(account), lockFor)
		if ip != "" {
			_, _ = g.cache.Incr(ctx, failIPKey(ip), lockFor)
		}
		return
	}
	g.cache.WarnDegraded("login_guard", "captcha.RecordFailure")
	// 降级时不写计数（login_logs 落库本身已是记录，CountRecentFailures 会读到）。
}

// ResetFailure 登录成功后清零失败计数。
func (g *LoginGuard) ResetFailure(ctx context.Context, account, ip string) {
	if g == nil || g.cache == nil {
		return
	}
	keys := []string{failAcctKey(account)}
	if ip != "" {
		keys = append(keys, failIPKey(ip))
	}
	_ = g.cache.Del(ctx, keys...)
}

// Log 写一条登录日志（成功与失败都要写，C6 让 login_logs 真实落库）。
func (g *LoginGuard) Log(ctx context.Context, entry LoginLogEntry) {
	if g == nil || g.recorder == nil {
		return
	}
	if entry.LoginType == "" {
		entry.LoginType = "password"
	}
	if entry.Result == "" {
		entry.Result = LoginResultFailed
	}
	if err := g.recorder.RecordLogin(ctx, entry); err != nil {
		// 日志写入失败不得影响登录主流程。
		g.switchesLoggerWarn(err)
	}
}

func (g *LoginGuard) switchesLoggerWarn(err error) {
	if g == nil || g.cache == nil {
		return
	}
	_ = err
	g.cache.WarnDegraded("login_log", "captcha.LoginGuard.Log")
}

// counterValue 读取计数器的当前值（Get 命中即视为已达）。
func (g *LoginGuard) counterValue(ctx context.Context, key string) (int, bool) {
	v, ok := g.cache.Get(ctx, key)
	if !ok || v == "" {
		return 0, false
	}
	return parseInt(v), true
}

// remaining 返回 key 的剩余有效期秒数。
func (g *LoginGuard) remaining(ctx context.Context, key string) int {
	ttl, err := g.cache.TTL(ctx, key)
	if err != nil || ttl <= 0 {
		return 0
	}
	return int(ttl / time.Second)
}

func parseInt(s string) int {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return n
		}
		n = n*10 + int(ch-'0')
	}
	return n
}
