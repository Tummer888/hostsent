package payment

import (
	"fmt"
	"sort"
	"sync"
)

// ============================================================================
// 工厂注册表：按渠道类型构造独立 Gateway 实例。
// 同一类型可有多个渠道实例（不同商户号/费率/结算账户），各自持有独立配置。
// ============================================================================

// Factory 按渠道配置构造网关实例。
type Factory func(cfg *ChannelConfig) Gateway

var (
	factoryMu sync.RWMutex
	factories = map[string]Factory{}
)

// RegisterFactory 由适配器包 init() 调用，按类型登记工厂。
func RegisterFactory(providerType string, f Factory) {
	factoryMu.Lock()
	defer factoryMu.Unlock()
	factories[providerType] = f
}

// Build 按类型构造网关实例。
func Build(providerType string, cfg *ChannelConfig) (Gateway, error) {
	factoryMu.RLock()
	defer factoryMu.RUnlock()
	f, ok := factories[providerType]
	if !ok {
		return nil, fmt.Errorf("支付渠道类型 %q 未注册（已接入: %v，请确认渠道类型正确或该渠道适配器是否已实现）", providerType, RegisteredTypes())
	}
	return f(cfg), nil
}

// RegisteredTypes 返回已注册工厂的类型（升序）。
func RegisteredTypes() []string {
	factoryMu.RLock()
	defer factoryMu.RUnlock()
	types := make([]string, 0, len(factories))
	for t := range factories {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}

// IsRegistered 判断类型是否已注册适配器。
func IsRegistered(providerType string) bool {
	factoryMu.RLock()
	defer factoryMu.RUnlock()
	_, ok := factories[providerType]
	return ok
}
