package realname

import (
	"context"
	"strings"
	"testing"

	"hostsent/backend/internal/pkg/integration"
)

// 描述符与工厂：Implemented 由工厂注册情况推导（与 pkg/oauth 同款约定）。
func TestRegistry_DescriptorAndFactory(t *testing.T) {
	const probe = "registry_probe_realname"

	RegisterDescriptor(probe, CapabilityDescriptor{
		Type: probe,
		Name: "探针核验",
		Mode: "api",
		CredentialSchema: []integration.Field{
			{Key: "app_id", Label: "AppID", Required: true},
		},
	})
	t.Cleanup(func() {
		descriptorMu.Lock()
		delete(descriptors, probe)
		descriptorMu.Unlock()
	})

	if _, ok := Descriptor(probe); !ok {
		t.Fatal("注册描述符后应能查到")
	}
	for _, d := range AllDescriptors() {
		if d.Type == probe && d.Implemented {
			t.Fatal("无工厂时 Implemented 必须为 false")
		}
	}

	RegisterFactory(probe, func(cfg ProviderConfig) Provider { return &probeProvider{} })
	t.Cleanup(func() {
		factoryMu.Lock()
		delete(factories, probe)
		factoryMu.Unlock()
	})

	if !IsRegistered(probe) {
		t.Fatal("注册工厂后 IsRegistered 应为 true")
	}
	for _, d := range AllDescriptors() {
		if d.Type == probe && !d.Implemented {
			t.Fatal("有工厂时 Implemented 必须为 true")
		}
	}
	if _, err := New("no_such_realname_type", ProviderConfig{}); err == nil {
		t.Fatal("未注册类型必须报错")
	}
}

// IsInitializer：跳转式核验的能力用类型断言发现，不能靠描述符自述。
func TestIsInitializer(t *testing.T) {
	if IsInitializer(&probeProvider{}) {
		t.Fatal("只实现 Provider 的适配器不应被判定为 Initializer")
	}
	if !IsInitializer(&probeInitializer{}) {
		t.Fatal("实现 Initialize/Query 的适配器应被判定为 Initializer")
	}
}

// 必填校验：错误信息带中文字段标签。
func TestValidateCredentials(t *testing.T) {
	d := CapabilityDescriptor{
		CredentialSchema: []integration.Field{
			{Key: "app_id", Label: "应用 AppID", Required: true},
			{Key: "private_key", Label: "应用私钥", Required: true, Secret: true},
			{Key: "gateway_url", Label: "网关地址", Required: false},
		},
	}
	if err := ValidateCredentials(d, map[string]string{"app_id": "a", "private_key": "k"}); err != nil {
		t.Fatalf("必填齐全时不应报错: %v", err)
	}
	err := ValidateCredentials(d, map[string]string{"app_id": "a"})
	if err == nil {
		t.Fatal("缺 private_key 必须报错")
	}
	if !strings.Contains(err.Error(), "应用私钥") {
		t.Fatalf("错误信息应包含中文字段标签，实际: %v", err)
	}
}

// DirectVerify 对跳转式核验必须明确返回 ErrAdapterNotImplemented，
// 而不是返回一个「核验未通过」——后者会被上层当成业务失败并误驳回用户。
func TestProviderErrorSemantics(t *testing.T) {
	if !strings.Contains(ErrAdapterNotImplemented.Error(), "尚未接入") {
		t.Fatal("未实现错误应给出可读提示")
	}
	if ErrVerifyFailed == ErrAdapterNotImplemented {
		t.Fatal("「未接入」与「核验未通过」必须是两个不同的错误")
	}
}

type probeProvider struct{}

func (p *probeProvider) Type() string { return "registry_probe_realname" }
func (p *probeProvider) DirectVerify(ctx context.Context, cfg ProviderConfig, req VerifyRequest) (*VerifyResult, error) {
	return nil, nil
}
func (p *probeProvider) Test(ctx context.Context, cfg ProviderConfig) error { return nil }

type probeInitializer struct{ probeProvider }

func (p *probeInitializer) Initialize(ctx context.Context, cfg ProviderConfig, req VerifyRequest) (*AuthChallenge, error) {
	return nil, nil
}
func (p *probeInitializer) Query(ctx context.Context, cfg ProviderConfig, txnNo string) (*VerifyResult, error) {
	return nil, nil
}
