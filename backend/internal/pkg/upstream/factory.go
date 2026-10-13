package upstream

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
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
//
// 带实例缓存（见 ProviderManager.cached 的说明）：适配器的会话令牌是实例级字段，
// 每次新建都意味着丢掉令牌、下次动作要重新登录。同一份配置（端点+凭证+类型不变）
// 复用同一实例，登录只发生一次（令牌过期由适配器内部按 TTL 自行重登）。
//
// 返回的是**共享实例**，调用方不得修改其状态；这也是各适配器一直以来的用法
// （它们只读配置、内部自管令牌）。
func (m *ProviderManager) Build(providerType string, config *ProviderConfig) (Provider, error) {
	providerType = ResolveType(providerType)
	key := configFingerprint(providerType, config)

	m.mu.RLock()
	factory, ok := m.factories[providerType]
	// 读 map 前先判空：直接读 nil map 只得到零值（不算命中），是安全的；
	// 但零值构造的 ProviderManager（测试/历史代码）没有 cached，写入前必须初始化。
	if cached, hit := m.cached[key]; hit {
		m.mu.RUnlock()
		return cached, nil
	}
	m.mu.RUnlock()
	if !ok {
		// 未注册类型给明确错误并列出已接入提供商，避免静默失败。
		return nil, fmt.Errorf("provider factory %q 未注册（已接入: %v，请确认上游 provider_type 正确或该上游是否已实现）", providerType, m.registeredTypes())
	}
	provider := factory(config)
	// 工厂返回 nil 保持原有语义（原实现直接 return factory(config), nil）：
	// 不缓存也不报错，每次调用重新问工厂。缓存 nil 会把"暂时造不出来"
	// 固化成永久命中，之后渠道恢复也不会再尝试构建。
	if provider == nil {
		return nil, nil
	}
	// 双检：并发首次构建时可能已有别的 goroutine 写入，以先写入者为准 ——
	// 否则同配置会产生两个实例、各自登录一次，正是本次要消除的浪费。
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cached == nil {
		m.cached = make(map[string]Provider)
	}
	if existing, hit := m.cached[key]; hit {
		return existing, nil
	}
	m.cached[key] = provider
	return provider, nil
}

// configFingerprint 由配置算出缓存键：类型 + ID + 端点 + 凭证 + 账号语义字段。
//
// 凭证只进哈希，不作为键的字面部分 —— 键可能被日志/pprof 带出，
// 不该出现可读的密钥。任一字段变化都会得到新键，旧实例自然不再复用。
func configFingerprint(providerType string, config *ProviderConfig) string {
	if config == nil {
		return providerType + "|nil"
	}
	h := sha256.New()
	for _, part := range []string{
		providerType,
		strconv.FormatUint(uint64(config.ID), 10),
		config.APIEndpoint,
		config.APIKey,
		config.APISecret,
		config.Region,
		config.UpstreamType,
		config.AccountType,
		config.UserPrefix,
		strconv.FormatBool(config.Secure),
		config.Port,
		strconv.Itoa(config.Timeout),
	} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	sum := hex.EncodeToString(h.Sum(nil))
	return providerType + "|" + strconv.FormatUint(uint64(config.ID), 10) + "|" + sum[:16]
}

// InvalidateProvider 丢弃某渠道的缓存实例（渠道配置被改/删除/禁用后调用）。
//
// 配置指纹已能覆盖"改完再 Build"的情形；本方法用于渠道被删除/禁用的场景 ——
// 那时不会再 Build，但缓存实例仍持有已解密凭证，应主动释放。
func (m *ProviderManager) InvalidateProvider(providerID uint) {
	if providerID == 0 {
		return
	}
	suffix := "|" + strconv.FormatUint(uint64(providerID), 10) + "|"
	m.mu.Lock()
	defer m.mu.Unlock()
	for key := range m.cached {
		if strings.Contains(key, suffix) {
			delete(m.cached, key)
		}
	}
	delete(m.byID, providerID)
	delete(m.configs, providerID)
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
