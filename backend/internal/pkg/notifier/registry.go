package notifier

import (
	"fmt"
	"sort"
	"sync"
)

// ============================================================================
// 工厂 + 描述符注册表：照抄 internal/pkg/payment 的注册表模式。
// 各 provider 包在 init() 里登记，新增一家 = 复制一个文件 + 改 Send 里的 HTTP 调用。
// ============================================================================

// Factory 按渠道配置构造发送器实例。
type Factory func(cfg ChannelConfig) Sender

var (
	factoryMu sync.RWMutex
	factories = map[string]Factory{}

	descriptorMu sync.RWMutex
	descriptors  = map[string]CapabilityDescriptor{}
)

// RegisterFactory 由 provider 包 init() 调用，按类型登记工厂。
func RegisterFactory(providerType string, f Factory) {
	factoryMu.Lock()
	defer factoryMu.Unlock()
	factories[providerType] = f
}

// New 按类型构造发送器；未注册返回 ErrAdapterNotImplemented。
func New(providerType string, cfg ChannelConfig) (Sender, error) {
	factoryMu.RLock()
	defer factoryMu.RUnlock()
	f, ok := factories[providerType]
	if !ok {
		return nil, ErrAdapterNotImplemented
	}
	return f(cfg), nil
}

// RegisteredFactoryTypes 返回已注册工厂的类型（升序）。
func RegisteredFactoryTypes() []string {
	factoryMu.RLock()
	defer factoryMu.RUnlock()
	types := make([]string, 0, len(factories))
	for t := range factories {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}

// IsRegistered 判断类型是否已注册工厂。
func IsRegistered(providerType string) bool {
	factoryMu.RLock()
	defer factoryMu.RUnlock()
	_, ok := factories[providerType]
	return ok
}

// RegisterDescriptor 由 provider 包 init() 调用，登记能力描述符。
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

// AllDescriptors 返回全部描述符（按 type 升序）；Implemented 由工厂注册情况运行时计算。
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

// DescriptorsByCategory 按类别过滤描述符；category 为空返回全部。
func DescriptorsByCategory(category string) []CapabilityDescriptor {
	all := AllDescriptors()
	out := make([]CapabilityDescriptor, 0, len(all))
	for _, d := range all {
		if category == "" || d.Category == category {
			out = append(out, d)
		}
	}
	return out
}

// ValidateCredentials 按描述符校验必填凭证字段。
func ValidateCredentials(d CapabilityDescriptor, creds map[string]string) error {
	for _, f := range d.CredentialSchema {
		if !f.Required {
			continue
		}
		if v, ok := creds[f.Key]; !ok || v == "" {
			return fmt.Errorf("notifier: 凭证字段 %s 必填", f.Key)
		}
	}
	return nil
}
