// Package tencent 腾讯云短信通道占位实现（doc90 §2.3）。
package tencent

import (
	"context"

	"hostsent/backend/internal/pkg/integration"
	"hostsent/backend/internal/pkg/notifier"
)

const providerType = "tencent_sms"

func init() {
	notifier.RegisterDescriptor(providerType, notifier.CapabilityDescriptor{
		Type:     providerType,
		Name:     "腾讯云短信",
		Category: notifier.CategorySMS,
		Mode:     "api",
		Icon:     "tencent",
		DocURL:   "https://cloud.tencent.com/document/product/382",
		CredentialSchema: []integration.Field{
			{Key: "secret_id", Label: "SecretId", Type: integration.FieldTypeString, Required: true},
			{Key: "secret_key", Label: "SecretKey", Type: integration.FieldTypePassword, Required: true, Secret: true},
			{Key: "sdk_app_id", Label: "短信应用 SDKAppID", Type: integration.FieldTypeString, Required: true},
			{Key: "region", Label: "地域", Type: integration.FieldTypeString, Placeholder: "ap-guangzhou"},
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
