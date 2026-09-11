package upstream

import (
	"context"
	"testing"
)

// mockProvider 用于测试管理器注册/查询逻辑的占位 Provider（仅实现最小公共能力）。
type mockProvider struct{}

func (m *mockProvider) GetType() string                   { return "mock" }
func (m *mockProvider) GetName() string                   { return "mock" }
func (m *mockProvider) HealthCheck(context.Context) error { return nil }
func (m *mockProvider) Capabilities() CapabilityDescriptor {
	return CapabilityDescriptor{Kind: KindUpstream, SignerType: SignerNone}
}

func TestProviderManagerRegisterAndGet(t *testing.T) {
	mgr := &ProviderManager{
		providers: make(map[string]Provider),
		byID:      make(map[uint]Provider),
		configs:   make(map[uint]*ProviderConfig),
	}

	mgr.Register("mock", &mockProvider{})

	p, err := mgr.GetProvider("mock")
	if err != nil {
		t.Fatalf("GetProvider failed: %v", err)
	}
	if p.GetType() != "mock" {
		t.Errorf("unexpected provider type: %s", p.GetType())
	}

	if _, err := mgr.GetProvider("unknown"); err == nil {
		t.Errorf("expected error for unknown provider, got nil")
	}
}

func TestProviderManagerLoadConfig(t *testing.T) {
	mgr := &ProviderManager{
		providers: make(map[string]Provider),
		byID:      make(map[uint]Provider),
		configs:   make(map[uint]*ProviderConfig),
	}

	mgr.Register("mock", &mockProvider{})

	cfg := &ProviderConfig{ID: 1, Type: "mock", Name: "Mock"}
	if err := mgr.LoadConfig(context.Background(), cfg); err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	p, err := mgr.GetProviderByID(1)
	if err != nil {
		t.Fatalf("GetProviderByID failed: %v", err)
	}
	if p.GetType() != "mock" {
		t.Errorf("unexpected provider type by id: %s", p.GetType())
	}
}

func TestProviderManagerListProviders(t *testing.T) {
	mgr := &ProviderManager{
		providers: make(map[string]Provider),
		byID:      make(map[uint]Provider),
		configs:   make(map[uint]*ProviderConfig),
	}

	mgr.Register("a", &mockProvider{})
	mgr.Register("b", &mockProvider{})

	types := mgr.ListProviders()
	if len(types) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(types))
	}
}
