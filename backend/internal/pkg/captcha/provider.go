package captcha

import (
	"context"
	"time"
)

// Store 验证码存储端口（由 internal/pkg/cache.Client 实现）。
//
// 未启用 Redis 时缓存客户端内部降级为进程内 LRU，因此本接口的语义不变；
// 多实例降级下图形码校验不可信，故 Enabled()==false 时服务层统一放行（doc91 §2.4 D11）。
type Store interface {
	Enabled() bool
	Set(ctx context.Context, key string, val any, ttl time.Duration) error
	GetDel(ctx context.Context, key string) (string, bool)
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

// Challenge 一次验证码挑战：图形码返回图片，第三方返回前端初始化参数。
type Challenge struct {
	// CaptchaKey 本次挑战的 key（32 字节随机 hex），用于回传校验。
	CaptchaKey string
	// ImagePNG 图形码 PNG 字节；第三方为空。
	ImagePNG []byte
	// Provider 实际承担本次挑战的服务商类型（回落时与策略不一致）。
	Provider string
	// Params 第三方前端 SDK 初始化参数（native 为空）。
	Params map[string]string
}

// ProviderConfig 一次调用使用的、已解密的验证码服务商配置。
type ProviderConfig struct {
	ProviderID  uint64
	Type        string
	Endpoint    string
	Credentials map[string]string

	// Store 缓存端口（native 存放答案与错误计数用）；由装配层注入。
	Store Store
	// Level 图形码干扰强度（native 使用）。
	Level string
	// ImageTTL 图形码答案有效期（native 使用）。
	ImageTTL time.Duration
	// MaxAttempts 单 key 最大校验次数（native 使用）。
	MaxAttempts int
}

// Provider 验证码服务商适配器。
//
// 占位 provider（阿里云/腾讯云/极验/顶象）注册描述符与工厂，方法返回
// ErrAdapterNotImplemented，使渠道在 UI 上完整可见且不 500（doc91 §7.2）。
type Provider interface {
	// Type 服务商类型：native / netease / ...
	Type() string
	// Challenge 生成一次挑战。
	Challenge(ctx context.Context, cfg ProviderConfig, scene string) (*Challenge, error)
	// Verify 用前端回传的载荷校验（第三方为票据，native 为 key+code）。
	Verify(ctx context.Context, cfg ProviderConfig, scene string, payload map[string]string) error
	// Test 连通性/凭证测试。
	Test(ctx context.Context, cfg ProviderConfig) error
}
