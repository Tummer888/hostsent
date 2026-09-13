// Package geetest 极验验证码占位实现（doc91 §7.2）。
//
// 占位语义：能创建、能存凭证、能测试（返回「待接入」），不 500。
package geetest

import (
	"context"

	"hostsent/backend/internal/pkg/captcha"
	"hostsent/backend/internal/pkg/integration"
)

const providerType = "geetest"

func init() {
	captcha.RegisterDescriptor(providerType, captcha.CapabilityDescriptor{
		Type:           providerType,
		Name:           "极验",
		Mode:           "api",
		Icon:           "geetest",
		DocURL:         "https://docs.geetest.com/",
		AdapterVersion: "v1",
		CredentialSchema: []integration.Field{
			{Key: "captcha_id", Label: "验证 ID（captchaId）", Type: integration.FieldTypeString, Required: true},
			{Key: "captcha_key", Label: "验证 Key（captchaKey）", Type: integration.FieldTypePassword, Required: true, Secret: true},
		},
	})
	captcha.RegisterFactory(providerType, func(cfg captcha.ProviderConfig) captcha.Provider {
		return &provider{}
	})
}

type provider struct{}

func (p *provider) Type() string { return providerType }

func (p *provider) Challenge(ctx context.Context, cfg captcha.ProviderConfig, scene string) (*captcha.Challenge, error) {
	return nil, captcha.ErrAdapterNotImplemented
}

func (p *provider) Verify(ctx context.Context, cfg captcha.ProviderConfig, scene string, payload map[string]string) error {
	return captcha.ErrAdapterNotImplemented
}

func (p *provider) Test(ctx context.Context, cfg captcha.ProviderConfig) error {
	return captcha.ErrAdapterNotImplemented
}
