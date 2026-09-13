package captcha

import (
	"fmt"
	"sort"
	"sync"

	"hostsent/backend/internal/pkg/integration"
)

// CapabilityDescriptor 验证码服务商能力描述符（驱动后台「类型选择 → 动态凭证表单」）。
type CapabilityDescriptor struct {
	// Type 类型标识：native / netease / aliyun / tencent / geetest / dingxiang。
	Type string `json:"type"`
	// Name 中文名。
	Name string `json:"name"`
	// Mode native / api / manual。
	Mode string `json:"mode"`
	// CredentialSchema 凭证字段（secret 字段字段级加密，回显脱敏）。
	CredentialSchema []integration.Field `json:"credential_schema"`
	// Icon 图标标识。
	Icon string `json:"icon"`
	// DocURL 官方文档地址。
	DocURL string `json:"doc_url"`
	// AdapterVersion 适配器版本。
	AdapterVersion string `json:"adapter_version"`
	// Implemented 是否已注册真实实现（运行时计算，非落库）。
	Implemented bool `json:"implemented"`
	// Builtin 内置兜底 provider（不可删除、不可停用）。
	Builtin bool `json:"builtin"`
}

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

// ValidateCredentials 按描述符校验必填凭证字段（与 notifier 口径一致）。
func ValidateCredentials(d CapabilityDescriptor, creds map[string]string) error {
	for _, f := range d.CredentialSchema {
		if !f.Required {
			continue
		}
		if v, ok := creds[f.Key]; !ok || v == "" {
			label := f.Label
			if label == "" {
				label = f.Key
			}
			return fmt.Errorf("验证码服务商凭证字段 %s 必填", label)
		}
	}
	return nil
}
