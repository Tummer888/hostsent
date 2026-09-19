package realname

import (
	"sort"
	"sync"
)

// Factory 按配置构造 provider 实例。
type Factory func(cfg ProviderConfig) Provider

var (
	factoryMu sync.RWMutex
	factories = map[string]Factory{}

	descriptorMu sync.RWMutex
	descriptors  = map[string]CapabilityDescriptor{}
)

// RegisterFactory 由 provider 包 init() 调用。
func RegisterFactory(providerType string, f Factory) {
	factoryMu.Lock()
	defer factoryMu.Unlock()
	factories[providerType] = f
}

// New 按类型构造 provider；未注册返回 ErrAdapterNotImplemented。
func New(providerType string, cfg ProviderConfig) (Provider, error) {
	factoryMu.RLock()
	defer factoryMu.RUnlock()
	f, ok := factories[providerType]
	if !ok {
		return nil, ErrAdapterNotImplemented
	}
	return f(cfg), nil
}

// IsRegistered 判断类型是否已注册工厂。
func IsRegistered(providerType string) bool {
	factoryMu.RLock()
	defer factoryMu.RUnlock()
	_, ok := factories[providerType]
	return ok
}

// RegisterDescriptor 由 provider 包 init() 调用。
func RegisterDescriptor(providerType string, d CapabilityDescriptor) {
	descriptorMu.Lock()
	defer descriptorMu.Unlock()
	descriptors[providerType] = d
}

// Descriptor 查询某类型的描述符。
func Descriptor(providerType string) (CapabilityDescriptor, bool) {
	descriptorMu.RLock()
	defer descriptorMu.RUnlock()
	d, ok := descriptors[providerType]
	return d, ok
}

// RegisteredDescriptorTypes 返回已登记描述符的类型（升序）。
func RegisteredDescriptorTypes() []string {
	descriptorMu.RLock()
	defer descriptorMu.RUnlock()
	types := make([]string, 0, len(descriptors))
	for t := range descriptors {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}

// AllDescriptors 返回全部描述符（按 type 升序）；Implemented 按工厂注册情况计算。
func AllDescriptors() []CapabilityDescriptor {
	descriptorMu.RLock()
	defer descriptorMu.RUnlock()
	factoryMu.RLock()
	defer factoryMu.RUnlock()
	out := make([]CapabilityDescriptor, 0, len(descriptors))
	for t, d := range descriptors {
		_, ok := factories[t]
		d.Implemented = ok
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}

// IsInitializer 判断某类型是否支持跳转式核验（供服务层决定要不要给用户返回认证 URL）。
func IsInitializer(p Provider) bool {
	_, ok := p.(Initializer)
	return ok
}
