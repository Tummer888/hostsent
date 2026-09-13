// Package dingxiang 顶象验证码占位实现（doc91 §7.2）。
//
// 占位语义：能创建、能存凭证、能测试（返回「待接入」），不 500。
package dingxiang

import (
	"context"

	"hostsent/backend/internal/pkg/captcha"
	"hostsent/backend/internal/pkg/integration"
)

const providerType = "dingxiang"

func init() {
	captcha.RegisterDescriptor(providerType, captcha.CapabilityDescriptor{
		Type:           providerType,
		Name:           "顶象",
		Mode:           "api",
		Icon:           "dingxiang",
		DocURL:         "https://www.dingxiang-inc.com/docs",
		AdapterVersion: "v1",
		CredentialSchema: []integration.Field{
			{Key: "app_id", Label: "应用 ID", Type: integration.FieldTypeString, Required: true},
			{Key: "app_secret", Label: "应用密钥", Type: integration.FieldTypePassword, Required: true, Secret: true},
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
