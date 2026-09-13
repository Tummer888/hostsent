// Package tencent 腾讯云验证码占位实现（doc91 §7.2）。
//
// 占位语义：能创建、能存凭证、能测试（返回「待接入」），不 500。
package tencent

import (
	"context"

	"hostsent/backend/internal/pkg/captcha"
	"hostsent/backend/internal/pkg/integration"
)

const providerType = "tencent"

func init() {
	captcha.RegisterDescriptor(providerType, captcha.CapabilityDescriptor{
		Type:           providerType,
		Name:           "腾讯云验证码",
		Mode:           "api",
		Icon:           "tencent",
		DocURL:         "https://cloud.tencent.com/document/product/1110",
		AdapterVersion: "v1",
		CredentialSchema: []integration.Field{
			{Key: "secret_id", Label: "SecretId", Type: integration.FieldTypeString, Required: true},
			{Key: "secret_key", Label: "SecretKey", Type: integration.FieldTypePassword, Required: true, Secret: true},
			{Key: "captcha_app_id", Label: "验证码应用 ID", Type: integration.FieldTypeString, Required: true},
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
