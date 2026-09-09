package upstream

import (
	"context"
	"fmt"
	"sync"
)

// ProviderManager 统一管理所有已注册的 Provider 及其配置。
type ProviderManager struct {
	mu        sync.RWMutex
	providers map[string]Provider        // key: provider_type（注册的单例，兼容旧用法）
	factories map[string]ProviderFactory // key: provider_type -> 构造函数（优先使用）
	byID      map[uint]Provider          // key: provider_id
	configs   map[uint]*ProviderConfig   // key: provider_id
}

var (
	managerInstance *ProviderManager
	once            sync.Once
)

// GetProviderManager 获取 Provider 管理器单例
func GetProviderManager() *ProviderManager {
	once.Do(func() {
		managerInstance = &ProviderManager{
			providers: make(map[string]Provider),
			factories: make(map[string]ProviderFactory),
			byID:      make(map[uint]Provider),
			configs:   make(map[uint]*ProviderConfig),
		}
	})
	return managerInstance
}

// Register 按类型注册 Provider（初始化时调用）
func (m *ProviderManager) Register(providerType string, provider Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[providerType] = provider
}

// GetProvider 根据类型获取 Provider
func (m *ProviderManager) GetProvider(providerType string) (Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	provider, ok := m.providers[providerType]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", providerType)
	}
	return provider, nil
}

// GetProviderByID 根据配置 ID 获取 Provider 实例
func (m *ProviderManager) GetProviderByID(id uint) (Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	provider, ok := m.byID[id]
	if !ok {
		return nil, fmt.Errorf("provider with id %d not found", id)
	}
	return provider, nil
}

// LoadConfig 加载提供商配置到管理器。
// 优先通过工厂按配置构建独立实例（支持同类型多提供商）；若未注册工厂，
// 则回退到按类型注册的单例（兼容仅用 Register 注册的旧用法）。
func (m *ProviderManager) LoadConfig(_ context.Context, config *ProviderConfig) error {
	provider, err := m.Build(config.Type, config)
	if err != nil {
		provider, err = m.GetProvider(config.Type)
		if err != nil {
			return err
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.byID[config.ID] = provider
	m.configs[config.ID] = config
	return nil
}

// ListProviders 获取所有已注册的提供商类型。
// 合并单例注册与工厂注册的类型（工厂是各适配器当前的实际注册方式）。
func (m *ProviderManager) ListProviders() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	seen := make(map[string]struct{}, len(m.providers)+len(m.factories))
	for t := range m.providers {
		seen[t] = struct{}{}
	}
	for t := range m.factories {
		seen[t] = struct{}{}
	}
	types := make([]string, 0, len(seen))
	for t := range seen {
		types = append(types, t)
	}
	return types
}

// registeredTypes 返回已注册工厂的类型列表（供 Build 报错时提示可用上游）。
// 调用方需已持有 m.mu（读锁或写锁），本方法不加锁。
func (m *ProviderManager) registeredTypes() []string {
	types := make([]string, 0, len(m.factories))
	for t := range m.factories {
		types = append(types, t)
	}
	return types
}
