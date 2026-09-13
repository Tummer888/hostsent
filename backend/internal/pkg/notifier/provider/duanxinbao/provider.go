// Package duanxinbao 短信宝通道占位实现（doc90 §2.3）。
package duanxinbao

import (
	"context"

	"hostsent/backend/internal/pkg/integration"
	"hostsent/backend/internal/pkg/notifier"
)

const providerType = "duanxinbao_sms"

func init() {
	notifier.RegisterDescriptor(providerType, notifier.CapabilityDescriptor{
		Type:     providerType,
		Name:     "短信宝",
		Category: notifier.CategorySMS,
		Mode:     "api",
		Icon:     "sms",
		CredentialSchema: []integration.Field{
			{Key: "api_url", Label: "接口地址", Type: integration.FieldTypeString, Required: true, Placeholder: "https://api.smsbao.com/sms"},
			{Key: "username", Label: "账号", Type: integration.FieldTypeString, Required: true},
			{Key: "api_key", Label: "密码 / API Key", Type: integration.FieldTypePassword, Required: true, Secret: true},
		},
		AdapterVersion: "v1",
	})
	notifier.RegisterFactory(providerType, func(cfg notifier.ChannelConfig) notifier.Sender {
		return &sender{}
	})
}

type sender struct{}

func (s *sender) Category() string { return notifier.CategorySMS }
func (s *sender) Type() string     { return providerType }

func (s *sender) Send(ctx context.Context, cfg notifier.ChannelConfig, msg notifier.Message) (*notifier.Result, error) {
	return nil, notifier.ErrAdapterNotImplemented
}

func (s *sender) Test(ctx context.Context, cfg notifier.ChannelConfig, target string) error {
	return notifier.ErrAdapterNotImplemented
}
