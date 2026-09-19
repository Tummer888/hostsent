// Package manual 实名核验的内置兜底 provider：不做任何三方核验。
//
// 与 pkg/payment/manual 同构——它是「平台尚未接入任何三方核验」时的合法状态，
// 此时提交的申请直接进入人工审核队列（pending）。这不是占位实现，
// 而是真实可用的业务路径：很多平台的实名认证本来就是纯人工审核。
package manual

import (
	"context"

	"hostsent/backend/internal/pkg/integration"
	"hostsent/backend/internal/pkg/realname"
)

// Type 服务商类型标识。
const Type = "manual"

func init() {
	realname.RegisterDescriptor(Type, realname.CapabilityDescriptor{
		Type: Type,
		Name: "人工审核",
		// manual：提交即入队，由运营在管理端整单通过/驳回（doc104 §5.1）。
		Mode:           "manual",
		AdapterVersion: "1.0.0",
		Icon:           "user-check",
		Builtin:        true,
		CredentialSchema: []integration.Field{
			{
				Key:         "review_hint",
				Label:       "审核提示",
				Type:        integration.FieldTypeTextarea,
				Required:    false,
				Placeholder: "展示给审核人员的操作提示，例如「企业主体需核对营业执照有效期」",
			},
		},
	})
	realname.RegisterFactory(Type, func(cfg realname.ProviderConfig) realname.Provider {
		return &Provider{cfg: cfg}
	})
}

// Provider 人工审核 provider。
type Provider struct{ cfg realname.ProviderConfig }

// Type 返回服务商类型。
func (p *Provider) Type() string { return Type }

// DirectVerify 人工审核没有自动核验能力：明确返回未实现，
// 由服务层按「入队人工审核」处理，而不是当成失败。
func (p *Provider) DirectVerify(_ context.Context, _ realname.ProviderConfig, _ realname.VerifyRequest) (*realname.VerifyResult, error) {
	return nil, realname.ErrAdapterNotImplemented
}

// Test 人工审核无需连通性测试，凭证存在即为可用。
func (p *Provider) Test(_ context.Context, cfg realname.ProviderConfig) error {
	return realname.ValidateCredentials(mustDescriptor(), cfg.Credentials)
}

func mustDescriptor() realname.CapabilityDescriptor {
	d, _ := realname.Descriptor(Type)
	return d
}
