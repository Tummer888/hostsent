package captcha

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

const providerTypeNative = "native"

func init() {
	RegisterDescriptor(providerTypeNative, CapabilityDescriptor{
		Type:           providerTypeNative,
		Name:           "原生图形验证码",
		Mode:           "native",
		Icon:           "shield",
		AdapterVersion: "v1",
		Builtin:        true,
	})
	RegisterFactory(providerTypeNative, func(cfg ProviderConfig) Provider {
		return &nativeProvider{cfg: cfg, store: cfg.Store}
	})
}

// nativeProvider 内置图形验证码：服务端生成 + 一次性校验。
//
// 答案只写进 Store（key 为 crypto/rand 32 字节 hex），响应体只含 key 与图片。
type nativeProvider struct {
	cfg   ProviderConfig
	store Store
}

func (p *nativeProvider) Type() string { return providerTypeNative }

// ChallengeKeyPrefix 图形码答案的 key 前缀（doc91 §2.3）。
const ChallengeKeyPrefix = "cap:img:"

// challengeTryKeyPrefix 图形码单 key 错误计数前缀。
const challengeTryKeyPrefix = "cap:img:try:"

// challengeKeyBytes 生成 key 的随机字节数（32 字节 hex = 64 字符，无法枚举）。
const challengeKeyBytes = 32

func (p *nativeProvider) Challenge(ctx context.Context, cfg ProviderConfig, scene string) (*Challenge, error) {
	store := p.store
	if store == nil {
		return nil, errors.New("captcha: native provider requires a store")
	}
	level := NormalizeLevel(cfg.Level)
	answer, pngBytes, err := NewNativeGenerator().Generate(level)
	if err != nil {
		return nil, fmt.Errorf("生成图形验证码失败: %w", err)
	}
	key, err := randomHex(challengeKeyBytes)
	if err != nil {
		return nil, err
	}
	ttl := cfg.ImageTTL
	if ttl <= 0 {
		ttl = 120 * time.Second
	}
	if err := store.Set(ctx, ChallengeKeyPrefix+key, strings.ToUpper(answer), ttl); err != nil {
		return nil, err
	}
	return &Challenge{
		CaptchaKey: key,
		ImagePNG:   pngBytes,
		Provider:   providerTypeNative,
	}, nil
}

func (p *nativeProvider) Verify(ctx context.Context, cfg ProviderConfig, scene string, payload map[string]string) error {
	store := p.store
	if store == nil {
		return errors.New("captcha: native provider requires a store")
	}
	key := strings.TrimSpace(payload["captcha_key"])
	input := strings.TrimSpace(payload["captcha_code"])
	if key == "" || input == "" {
		return ErrInvalid
	}
	maxAttempts := cfg.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	// 一次性取走答案：同一 key 第二次校验必然失败（防暴力重试）。
	stored, ok := store.GetDel(ctx, ChallengeKeyPrefix+key)
	if !ok {
		return ErrInvalid
	}
	if subtle.ConstantTimeCompare([]byte(strings.ToUpper(stored)), []byte(strings.ToUpper(input))) != 1 {
		if n, err := store.Incr(ctx, challengeTryKeyPrefix+key, 10*time.Minute); err == nil && n >= int64(maxAttempts) {
			// 已达上限：销毁兜底计数并强制换图。
			_ = n
			return ErrTooManyAttempts
		}
		return ErrInvalid
	}
	return nil
}

func (p *nativeProvider) Test(ctx context.Context, cfg ProviderConfig) error {
	// 内置实现无需外部依赖：能渲染出图即可用。
	_, _, err := NewNativeGenerator().Generate(LevelNormal)
	return err
}

// EncodeImageDataURI 把 PNG 字节编码为 data URI（响应体直接给前端 <img :src>）。
func EncodeImageDataURI(pngBytes []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)
}
