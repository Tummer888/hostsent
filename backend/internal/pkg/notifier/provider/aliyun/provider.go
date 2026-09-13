// Package aliyun 阿里云短信通道占位实现（doc90 §2.3）。
//
// 占位语义：注册描述符 + 工厂，使渠道能在 UI 上完整创建、填写凭证并加密保存；
// Send/Test 返回 notifier.ErrAdapterNotImplemented（明确的「待接入」业务错误，不是 500）。
// 接入时只需把 Send 换成阿里云 Dysmsapi 的 SDK 调用。
package aliyun

import (
	"context"

	"hostsent/backend/internal/pkg/integration"
	"hostsent/backend/internal/pkg/notifier"
)

const providerType = "aliyun_sms"

func init() {
	notifier.RegisterDescriptor(providerType, notifier.CapabilityDescriptor{
		Type:     providerType,
		Name:     "阿里云短信",
		Category: notifier.CategorySMS,
		Mode:     "api",
		Icon:     "aliyun",
		DocURL:   "https://help.aliyun.com/zh/sms/",
		CredentialSchema: []integration.Field{
			{Key: "access_key_id", Label: "AccessKey ID", Type: integration.FieldTypeString, Required: true},
			{Key: "access_key_secret", Label: "AccessKey Secret", Type: integration.FieldTypePassword, Required: true, Secret: true},
			{Key: "region_id", Label: "地域", Type: integration.FieldTypeSelect, Required: true, Default: "cn-hangzhou",
				Options: []integration.FieldOption{
					{Label: "华东 1（杭州）", Value: "cn-hangzhou"},
					{Label: "华北 2（北京）", Value: "cn-beijing"},
				}},
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

// Send 占位：返回「待接入」，由上层映射为明确业务错误码。
func (s *sender) Send(ctx context.Context, cfg notifier.ChannelConfig, msg notifier.Message) (*notifier.Result, error) {
	return nil, notifier.ErrAdapterNotImplemented
}

// Test 占位：同样返回「待接入」，UI 据此显示待接入而非失败。
func (s *sender) Test(ctx context.Context, cfg notifier.ChannelConfig, target string) error {
	return notifier.ErrAdapterNotImplemented
}
