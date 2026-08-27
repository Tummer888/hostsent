package upstream

import (
	"context"
	"testing"

	"hostsent/backend/internal/pkg/model"
)

// mockProvider 用于测试管理器注册/查询逻辑的占位 Provider。
type mockProvider struct{}

func (m *mockProvider) GetType() string { return "mock" }
func (m *mockProvider) GetName() string { return "mock" }
func (m *mockProvider) HealthCheck(context.Context) error { return nil }
func (m *mockProvider) ListProducts(context.Context) ([]*model.StandardProduct, error) { return nil, nil }
func (m *mockProvider) GetProduct(context.Context, string) (*model.StandardProduct, error) { return nil, nil }
func (m *mockProvider) CreateInstance(context.Context, *model.CreateInstanceRequest) (*model.StandardInstance, error) { return nil, nil }
func (m *mockProvider) ListInstances(context.Context, map[string]string) ([]*model.StandardInstance, error) { return nil, nil }
func (m *mockProvider) GetInstance(context.Context, string) (*model.StandardInstance, error) { return nil, nil }
func (m *mockProvider) StartInstance(context.Context, string) error { return nil }
func (m *mockProvider) StopInstance(context.Context, string, bool) error { return nil }
func (m *mockProvider) RestartInstance(context.Context, string) error { return nil }
func (m *mockProvider) DeleteInstance(context.Context, string) error { return nil }
func (m *mockProvider) ResizeInstance(context.Context, string, *model.StandardProductSpec) error { return nil }
func (m *mockProvider) ListPools(context.Context) ([]*StandardPool, error) { return nil, nil }
func (m *mockProvider) GetAccountInfo(context.Context) (*AccountInfo, error) { return nil, nil }

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
