package upstream

import "fmt"

// ProviderFactory 按配置创建独立的 Provider 实例。
// 各上游适配器在 init() 中通过 RegisterFactory 注册构造函数，
// 使同一类型的多个提供商（不同 endpoint/密钥/区域）各自持有独立实例。
type ProviderFactory func(config *ProviderConfig) Provider

// RegisterFactory 按类型注册工厂（由各上游包初始化时调用）。
func (m *ProviderManager) RegisterFactory(providerType string, factory ProviderFactory) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.factories[providerType] = factory
}

// Build 按类型 + 配置实例化真正的适配器。
func (m *ProviderManager) Build(providerType string, config *ProviderConfig) (Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	factory, ok := m.factories[providerType]
	if !ok {
		// 未注册类型给明确错误并列出已接入提供商，避免静默失败。
		return nil, fmt.Errorf("provider factory %q 未注册（已接入: %v，请确认上游 provider_type 正确或该上游是否已实现）", providerType, m.registeredTypes())
	}
	return factory(config), nil
}
