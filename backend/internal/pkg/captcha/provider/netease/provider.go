// Package netease 网易易盾验证码适配（doc91 §7.3，本轮唯一的第三方真实现）。
//
// 关键安全点：前端 SDK 通过后只拿到 validate 票据，**票据必须回服务端换真结果**。
// 只在客户端判断 result === true 等于没验证——攻击者直接伪造该字段即可绕过。
//
// 校验两步走：
//  1. 拿 validate 调易盾 /api/v2/verify，用 secret_key 做 MD5 签名；
//  2. 同一 validate 票据 5 分钟内不可重复使用（本地 SETNX 语义防重放，
//     易盾侧也做，但本地再做一次才能挡住「票据泄漏后被重放」）。
package netease

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"hostsent/backend/internal/pkg/captcha"
	"hostsent/backend/internal/pkg/integration"
)

const providerType = "netease"

// defaultAPIURL 易盾校验接口默认地址（凭证 api_url 可覆盖）。
const defaultAPIURL = "http://c.dun.163.com/api/v2/verify"

// replayTTL 校验票据防重放窗口（与易盾侧一致）。
const replayTTL = 5 * time.Minute

// httpTimeout 单次校验超时：登录链路上的同步调用，不能拖长。
const httpTimeout = 5 * time.Second

func init() {
	captcha.RegisterDescriptor(providerType, captcha.CapabilityDescriptor{
		Type:           providerType,
		Name:           "网易易盾",
		Mode:           "api",
		Icon:           "shield",
		DocURL:         "https://support.dun.163.com/documents/1553921518330177",
		AdapterVersion: "v1",
		CredentialSchema: []integration.Field{
			{Key: "captcha_id", Label: "验证码 ID（captchaId）", Type: integration.FieldTypeString, Required: true},
			{Key: "secret_id", Label: "密钥 ID（secretId）", Type: integration.FieldTypeString, Required: true},
			{Key: "secret_key", Label: "密钥 Key（secretKey）", Type: integration.FieldTypePassword, Required: true, Secret: true},
			{Key: "api_url", Label: "校验接口地址", Type: integration.FieldTypeString, Default: defaultAPIURL},
		},
	})
	captcha.RegisterFactory(providerType, func(cfg captcha.ProviderConfig) captcha.Provider {
		return &provider{cfg: cfg}
	})
}

type provider struct {
	cfg captcha.ProviderConfig
}

func (p *provider) Type() string { return providerType }

// Challenge 返回前端 SDK 初始化参数。
//
// CaptchaKey 既作为本次挑战的标识，也作为易盾 verify 的 user 参数：
// 前端回传票据时必须带上同一个 key，签名因此与本会话绑定。
func (p *provider) Challenge(ctx context.Context, cfg captcha.ProviderConfig, scene string) (*captcha.Challenge, error) {
	id := strings.TrimSpace(p.credentials(cfg)["captcha_id"])
	if id == "" {
		return nil, captcha.ErrProviderDisabled
	}
	key, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	return &captcha.Challenge{
		CaptchaKey: key,
		Provider:   providerType,
		Params: map[string]string{
			"provider":      providerType,
			"captcha_id":    id,
			"challenge_key": key,
			"scene":         scene,
		},
	}, nil
}

// Verify 服务端二次校验 validate 票据。
func (p *provider) Verify(ctx context.Context, cfg captcha.ProviderConfig, scene string, payload map[string]string) error {
	creds := p.credentials(cfg)
	captchaID := strings.TrimSpace(creds["captcha_id"])
	secretID := strings.TrimSpace(creds["secret_id"])
	secretKey := strings.TrimSpace(creds["secret_key"])
	if captchaID == "" || secretID == "" || secretKey == "" {
		return captcha.ErrProviderDisabled
	}

	// validate 是易盾回传的一次性票据；也兼容前端把票据放在 captcha_code 里。
	validate := strings.TrimSpace(payload["validate"])
	if validate == "" {
		validate = strings.TrimSpace(payload["captcha_code"])
	}
	if validate == "" {
		return captcha.ErrInvalid
	}
	user := strings.TrimSpace(payload["captcha_key"])
	if user == "" {
		user = strings.TrimSpace(payload["user"])
	}

	// 本地防重放：同一票据 5 分钟内只允许成功一次。
	if cfg.Store != nil && cfg.Store.Enabled() {
		n, err := cfg.Store.Incr(ctx, "cap:vfy:"+providerType+":"+validate, replayTTL)
		if err == nil && n > 1 {
			return captcha.ErrInvalid
		}
	}

	// sign = md5(secretId + validate + user + secretKey)
	sum := md5.Sum([]byte(secretID + validate + user + secretKey))
	sign := hex.EncodeToString(sum[:])

	form := url.Values{}
	form.Set("captchaId", captchaID)
	form.Set("validate", validate)
	form.Set("user", user)
	form.Set("secretId", secretID)
	form.Set("sign", sign)

	endpoint := strings.TrimSpace(creds["api_url"])
	if endpoint == "" {
		endpoint = strings.TrimSpace(cfg.Endpoint)
	}
	if endpoint == "" {
		endpoint = defaultAPIURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("易盾校验请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("易盾校验返回异常状态: %d", resp.StatusCode)
	}

	var out struct {
		Result *bool  `json:"result"`
		Error  any    `json:"error"`
		Msg    string `json:"msg"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return fmt.Errorf("易盾校验响应解析失败: %w", err)
	}
	if out.Result == nil || !*out.Result {
		// 校验失败统一返回 ErrInvalid：不把易盾的错误细节透给攻击者。
		return captcha.ErrInvalid
	}
	return nil
}

// Test 凭证连通性测试：调一次校验接口，票据必然无效，但能区分
// 「凭证/网络问题」与「实现未接入」——前者报错细节，后者返回 ErrAdapterNotImplemented。
func (p *provider) Test(ctx context.Context, cfg captcha.ProviderConfig) error {
	creds := p.credentials(cfg)
	if strings.TrimSpace(creds["captcha_id"]) == "" ||
		strings.TrimSpace(creds["secret_id"]) == "" ||
		strings.TrimSpace(creds["secret_key"]) == "" {
		return captcha.ErrProviderDisabled
	}
	// 用明显无效的 validate 探测：能拿到响应即说明地址与网络可达。
	err := p.Verify(ctx, cfg, "test", map[string]string{"validate": "test", "captcha_key": "test"})
	if errors.Is(err, captcha.ErrInvalid) {
		return nil
	}
	return err
}

// credentials 取本次调用凭证：优先用注入的 cfg，其次实例 cfg（装配层两处都会填）。
func (p *provider) credentials(cfg captcha.ProviderConfig) map[string]string {
	if len(cfg.Credentials) > 0 {
		return cfg.Credentials
	}
	return p.cfg.Credentials
}

// randomHex 生成 n 字节随机 hex（crypto/rand，不可预测）。
func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
