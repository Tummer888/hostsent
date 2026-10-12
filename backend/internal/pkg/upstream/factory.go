package upstream

import (
	"fmt"
	"sync"
)

// ProviderFactory 按配置创建独立的 Provider 实例。
// 各上游适配器在 init() 中通过 RegisterFactory 注册构造函数，
// 使同一类型的多个提供商（不同 endpoint/密钥/区域）各自持有独立实例。
type ProviderFactory func(config *ProviderConfig) Provider

// 类型码别名：把历史类型码（如 mofangyun）解析到当前类型码（如 dcimcloud）。
//
// 为什么需要：类型码重命名后，存量 resource_providers 行在数据迁移跑完前仍是旧值，
// 若构建/描述符查询只认新码，这些渠道会立刻表现为「适配器未注册」而被熔断。
// 别名表让新旧码在整个过渡期都能用，与数据迁移互为冗余。
var (
	typeAliasMu sync.RWMutex
	typeAliases = map[string]string{}
)

// RegisterTypeAlias 登记一条类型码别名（由适配器包 init() 调用）。
func RegisterTypeAlias(legacy, current string) {
	if legacy == "" || current == "" || legacy == current {
		return
	}
	typeAliasMu.Lock()
	defer typeAliasMu.Unlock()
	typeAliases[legacy] = current
}

// ResolveType 把类型码解析为当前码（无别名时原样返回）。
func ResolveType(providerType string) string {
	typeAliasMu.RLock()
	defer typeAliasMu.RUnlock()
	if current, ok := typeAliases[providerType]; ok {
		return current
	}
	return providerType
}

// RegisterFactory 按类型注册工厂（由各上游包初始化时调用）。
func (m *ProviderManager) RegisterFactory(providerType string, factory ProviderFactory) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.factories[providerType] = factory
}

// Build 按类型 + 配置实例化真正的适配器。
func (m *ProviderManager) Build(providerType string, config *ProviderConfig) (Provider, error) {
	providerType = ResolveType(providerType)
	m.mu.RLock()
	defer m.mu.RUnlock()

	factory, ok := m.factories[providerType]
	if !ok {
		// 未注册类型给明确错误并列出已接入提供商，避免静默失败。
		return nil, fmt.Errorf("provider factory %q 未注册（已接入: %v，请确认上游 provider_type 正确或该上游是否已实现）", providerType, m.registeredTypes())
	}
	return factory(config), nil
}

// SupportsProductCatalog 判断某渠道类型能否列举商品目录（「导入上游商品」的数据源）。
// 以适配器是否实现 ProductCatalog 为准：自营算力平台（魔方云/kvm/hyperv/pve）只有池与实例能力，
// 没有商品目录，不能出现在导入弹窗的渠道列表里，否则运营会看到无法导入的自营选项。
// 配置为空时也够用——工厂构造函数不会因缺凭证报错，能力判定只关心接口实现。
func (m *ProviderManager) SupportsProductCatalog(providerType string) bool {
	if provider, err := m.Build(providerType, &ProviderConfig{Type: providerType}); err == nil {
		_, ok := provider.(ProductCatalog)
		return ok
	}
	// 兼容只用 Register 注册单例、未注册工厂的旧适配器。
	provider, err := m.GetProvider(providerType)
	if err != nil {
		return false
	}
	_, ok := provider.(ProductCatalog)
	return ok
}
