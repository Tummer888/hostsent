// Package aliyun 阿里云验证码占位实现（doc91 §7.2）。
//
// 占位语义与 doc90 的短信通道一致：注册描述符 + 工厂，使类型在管理端完整可见、
// 凭证可加密保存；Challenge/Verify/Test 返回 captcha.ErrAdapterNotImplemented
// （明确的「待接入」业务错误，不是 500）。
//
// 接入时把 Verify 换成阿里云验证码服务端的校验调用即可，调用方代码不改。
package aliyun

import (
	"context"

	"hostsent/backend/internal/pkg/captcha"
	"hostsent/backend/internal/pkg/integration"
)

const providerType = "aliyun"

func init() {
	captcha.RegisterDescriptor(providerType, captcha.CapabilityDescriptor{
		Type:           providerType,
		Name:           "阿里云验证码",
		Mode:           "api",
		Icon:           "aliyun",
		DocURL:         "https://help.aliyun.com/zh/captcha/",
		AdapterVersion: "v1",
		CredentialSchema: []integration.Field{
			{Key: "access_key_id", Label: "AccessKey ID", Type: integration.FieldTypeString, Required: true},
			{Key: "access_key_secret", Label: "AccessKey Secret", Type: integration.FieldTypePassword, Required: true, Secret: true},
			{Key: "scene_id", Label: "验证场景 ID", Type: integration.FieldTypeString, Required: true},
		},
	})
	captcha.RegisterFactory(providerType, func(cfg captcha.ProviderConfig) captcha.Provider {
		return &provider{}
	})
}

type provider struct{}

func (p *provider) Type() string { return providerType }

// Challenge 占位：返回「待接入」，由上层回落 native。
func (p *provider) Challenge(ctx context.Context, cfg captcha.ProviderConfig, scene string) (*captcha.Challenge, error) {
	return nil, captcha.ErrAdapterNotImplemented
}

// Verify 占位：同上。
func (p *provider) Verify(ctx context.Context, cfg captcha.ProviderConfig, scene string, payload map[string]string) error {
	return captcha.ErrAdapterNotImplemented
}

// Test 占位：返回「待接入」，UI 据此显示待接入而非失败。
func (p *provider) Test(ctx context.Context, cfg captcha.ProviderConfig) error {
	return captcha.ErrAdapterNotImplemented
}
