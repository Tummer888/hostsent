// Package service 实现验证码与二次验证的核心服务（doc91 §3–§6）。
package service

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ConfigSource 开关配置源：一次返回全部验证码相关配置键的当前值。
//
// 不直接依赖系统配置模块的仓储类型：装配层用「读一个键」的最小能力适配出本接口，
// 模块边界因此保持单向（captcha → 无业务模块依赖）。
type ConfigSource interface {
	All(ctx context.Context) (map[string]string, error)
}

// 配置键（doc91 §9.1 存量 + §9.2 新增）。
const (
	ConfigCaptchaEnabled        = "captcha_enabled"
	ConfigCaptchaProvider       = "captcha_provider"
	ConfigCaptchaImageTTL       = "captcha_image_ttl_seconds"
	ConfigVerifyCodeTTL         = "verify_code_ttl_seconds"
	ConfigSendInterval          = "verify_code_send_interval_seconds"
	ConfigDailyLimit            = "verify_code_daily_limit"
	ConfigMaxAttempts           = "verify_code_max_attempts"
	ConfigSendIPHourlyLimit     = "captcha_send_ip_hourly_limit"
	ConfigImageIPMinuteLimit    = "captcha_image_ip_minute_limit"
	ConfigLoginFailLock         = "login_fail_lock"
	ConfigLoginFailThreshold    = "login_fail_threshold"
	ConfigLoginLockMinutes      = "login_lock_minutes"
	ConfigMFARequired           = "mfa_required"
	ConfigMFARequiredLegacy     = "enable_mfa_required"
	ConfigRegisterEnabled       = "register_enabled"
	ConfigRegisterEnabledLegacy = "enable_user_register"
	ConfigAPIRateLimit          = "api_rate_limit"
)

// 默认值（与 doc91 §9.2 表一致）。
const (
	defaultCaptchaEnabled     = false
	defaultCaptchaProvider    = "native"
	defaultCaptchaImageTTL    = 120
	defaultVerifyCodeTTL      = 300
	defaultSendInterval       = 60
	defaultDailyLimit         = 10
	defaultMaxAttempts        = 5
	defaultSendIPHourlyLimit  = 20
	defaultImageIPMinuteLimit = 30
	defaultLoginFailThreshold = 5
	defaultLoginLockMinutes   = 15
)

// Switches 配置开关读取器：包一层 30 秒内存缓存，避免登录等热路径每次读库。
//
// 开关缺省即用默认值（默认安全侧），配置项不存在不报错。
type Switches struct {
	src  ConfigSource
	mu   sync.RWMutex
	vals map[string]string
	at   time.Time
}

const switchesCacheTTL = 30 * time.Second

// NewSwitches 创建开关读取器。
func NewSwitches(src ConfigSource) *Switches {
	return &Switches{src: src}
}

// raw 读取配置原文（带缓存）；未配置返回空串。
func (s *Switches) raw(ctx context.Context, key string) string {
	if s == nil || s.src == nil {
		return ""
	}
	s.mu.RLock()
	vals, at := s.vals, s.at
	s.mu.RUnlock()
	if vals == nil || time.Since(at) > switchesCacheTTL {
		vals = s.reload(ctx)
	}
	return vals[key]
}

// reload 全量载入配置（键数量有限，一次读全量比逐键查询更省）。
func (s *Switches) reload(ctx context.Context) map[string]string {
	vals, err := s.src.All(ctx)
	if err != nil {
		// 读库失败时不缓存空结果，下次调用重试；调用方按默认值走。
		s.mu.RLock()
		defer s.mu.RUnlock()
		if s.vals != nil {
			return s.vals
		}
		return map[string]string{}
	}
	s.mu.Lock()
	s.vals = vals
	s.at = time.Now()
	s.mu.Unlock()
	return vals
}

// Invalidate 失效缓存（后台改配置后立即生效）。
func (s *Switches) Invalidate() {
	s.mu.Lock()
	s.vals = nil
	s.at = time.Time{}
	s.mu.Unlock()
}

func (s *Switches) boolVal(ctx context.Context, key string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(s.raw(ctx, key))) {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		return def
	}
}

func (s *Switches) intVal(ctx context.Context, key string, def int) int {
	v := strings.TrimSpace(s.raw(ctx, key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def
	}
	return n
}

func (s *Switches) strVal(ctx context.Context, key, def string) string {
	v := strings.TrimSpace(s.raw(ctx, key))
	if v == "" {
		return def
	}
	return v
}

// boolWithFallback 优先读主键，未配置时回落兼容键。
func (s *Switches) boolWithFallback(ctx context.Context, key, legacy string, def bool) bool {
	if v := strings.ToLower(strings.TrimSpace(s.raw(ctx, key))); v != "" {
		switch v {
		case "true", "1", "yes":
			return true
		case "false", "0", "no":
			return false
		}
	}
	return s.boolVal(ctx, legacy, def)
}

// CaptchaEnabled 全局总闸（默认 false，不阻断任何现有登录路径）。
func (s *Switches) CaptchaEnabled(ctx context.Context) bool {
	return s.boolVal(ctx, ConfigCaptchaEnabled, defaultCaptchaEnabled)
}

// CaptchaProvider 兜底 provider 类型。
func (s *Switches) CaptchaProvider(ctx context.Context) string {
	return s.strVal(ctx, ConfigCaptchaProvider, defaultCaptchaProvider)
}

// ImageTTL 图形码有效期。
func (s *Switches) ImageTTL(ctx context.Context) time.Duration {
	return time.Duration(s.intVal(ctx, ConfigCaptchaImageTTL, defaultCaptchaImageTTL)) * time.Second
}

// VerifyCodeTTL OTP 有效期。
func (s *Switches) VerifyCodeTTL(ctx context.Context) time.Duration {
	return time.Duration(s.intVal(ctx, ConfigVerifyCodeTTL, defaultVerifyCodeTTL)) * time.Second
}

// SendInterval OTP 发送间隔。
func (s *Switches) SendInterval(ctx context.Context) time.Duration {
	return time.Duration(s.intVal(ctx, ConfigSendInterval, defaultSendInterval)) * time.Second
}

// DailyLimit 单目标每日发送上限。
func (s *Switches) DailyLimit(ctx context.Context) int {
	return s.intVal(ctx, ConfigDailyLimit, defaultDailyLimit)
}

// MaxAttempts 单 key 校验次数上限。
func (s *Switches) MaxAttempts(ctx context.Context) int {
	return s.intVal(ctx, ConfigMaxAttempts, defaultMaxAttempts)
}

// SendIPHourlyLimit 单 IP 小时发送上限。
func (s *Switches) SendIPHourlyLimit(ctx context.Context) int {
	return s.intVal(ctx, ConfigSendIPHourlyLimit, defaultSendIPHourlyLimit)
}

// ImageIPMinuteLimit 单 IP 图形码下发上限。
func (s *Switches) ImageIPMinuteLimit(ctx context.Context) int {
	return s.intVal(ctx, ConfigImageIPMinuteLimit, defaultImageIPMinuteLimit)
}

// LoginFailLock 登录失败锁定开关。
func (s *Switches) LoginFailLock(ctx context.Context) bool {
	return s.boolVal(ctx, ConfigLoginFailLock, false)
}

// LoginFailThreshold 登录失败阈值。
func (s *Switches) LoginFailThreshold(ctx context.Context) int {
	return s.intVal(ctx, ConfigLoginFailThreshold, defaultLoginFailThreshold)
}

// LoginLockMinutes 锁定时长（分钟）。
func (s *Switches) LoginLockMinutes(ctx context.Context) int {
	return s.intVal(ctx, ConfigLoginLockMinutes, defaultLoginLockMinutes)
}

// MFARequired 全局强制二次验证（default 侧兜底：为 true 时所有场景提升为需要 OTP）。
func (s *Switches) MFARequired(ctx context.Context) bool {
	return s.boolWithFallback(ctx, ConfigMFARequired, ConfigMFARequiredLegacy, false)
}

// RegisterEnabled 注册开关（兼容历史 enable_user_register）。
func (s *Switches) RegisterEnabled(ctx context.Context) bool {
	return s.boolWithFallback(ctx, ConfigRegisterEnabled, ConfigRegisterEnabledLegacy, true)
}

// APIRateLimit 每分钟请求上限（0=不限）。
func (s *Switches) APIRateLimit(ctx context.Context) int {
	return s.intVal(ctx, ConfigAPIRateLimit, 0)
}

// PasswordPolicy 读取密码策略键（doc91 §9.1，internal/pkg/passwordpolicy 消费）。
const (
	ConfigPasswordMinLength     = "password_min_length"
	ConfigPasswordMaxLength     = "password_max_length"
	ConfigPasswordRequireUpper  = "password_require_upper"
	ConfigPasswordRequireLower  = "password_require_lower"
	ConfigPasswordRequireDigit  = "password_require_digit"
	ConfigPasswordRequireSymbol = "password_require_symbol"
)

// PasswordPolicyValues 返回密码策略 6 项原始值（供 passwordpolicy.New 组装）。
func (s *Switches) PasswordPolicyValues(ctx context.Context) map[string]string {
	return map[string]string{
		ConfigPasswordMinLength:     s.raw(ctx, ConfigPasswordMinLength),
		ConfigPasswordMaxLength:     s.raw(ctx, ConfigPasswordMaxLength),
		ConfigPasswordRequireUpper:  s.raw(ctx, ConfigPasswordRequireUpper),
		ConfigPasswordRequireLower:  s.raw(ctx, ConfigPasswordRequireLower),
		ConfigPasswordRequireDigit:  s.raw(ctx, ConfigPasswordRequireDigit),
		ConfigPasswordRequireSymbol: s.raw(ctx, ConfigPasswordRequireSymbol),
	}
}

// switchKeys 开关表里用到的全部配置键（装配层一次性载入）。
var switchKeys = []string{ConfigCaptchaEnabled,
	ConfigCaptchaProvider,
	ConfigCaptchaImageTTL,
	ConfigVerifyCodeTTL,
	ConfigSendInterval,
	ConfigDailyLimit,
	ConfigMaxAttempts,
	ConfigSendIPHourlyLimit,
	ConfigImageIPMinuteLimit,
	ConfigLoginFailLock,
	ConfigLoginFailThreshold,
	ConfigLoginLockMinutes,
	ConfigMFARequired,
	ConfigMFARequiredLegacy,
	ConfigRegisterEnabled,
	ConfigRegisterEnabledLegacy,
	ConfigAPIRateLimit,
	ConfigPasswordMinLength,
	ConfigPasswordMaxLength,
	ConfigPasswordRequireUpper,
	ConfigPasswordRequireLower,
	ConfigPasswordRequireDigit,
	ConfigPasswordRequireSymbol,
}

// SwitchKeys 返回开关表用到的全部配置键（装配层据此构造配置源）。
func SwitchKeys() []string { return switchKeys }
