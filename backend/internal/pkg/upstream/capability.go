package upstream

import (
	"sort"
	"sync"
)

// ============================================================================
// 契约②：能力契约（字段级能力描述）
//
// 背景（docs/实施计划/16 §4、17 §P2/T2.1）：
//   现有能力靠类型断言（ProductCatalog/InstanceControl/...）发现，只有"有/没有"，
//   后台无法提前展示"这个渠道支持什么、缺什么"，也无法在开通前按能力拦截。
//   CapabilityDescriptor 把能力升级为可查询、可展示、可校验的字段级描述，
//   同时由 provider_types 注册表驱动后台动态表单与能力矩阵。
//
// 兼容：Provider 接口保留既有能力子接口与类型断言调用点（sync_engine 的三类同步、
//   instance_service.capabilities）继续可用；新代码应优先查描述符。
// ============================================================================

// 链路类型（与 resource/provider/model 的 kind 常量同值）。
const (
	KindUpstream = "upstream" // 上游转售
	KindCompute  = "compute"  // 算力平台（自营执行器）
)

// 同步 scope（同步框架 P3 按此生成"渠道 × scope"任务；本期只落契约）。
const (
	ScopeCatalog  = "catalog"  // 商品目录
	ScopePrice    = "price"    // 价格
	ScopePool     = "pool"     // 资源池/节点
	ScopeRegion   = "region"   // 区域
	ScopeImage    = "image"    // 镜像
	ScopeInstance = "instance" // 实例
	ScopeStock    = "stock"    // 库存
)

// 能力操作（开通前校验用；与类型断言一一对应，另含后续补齐的续费/重装）。
const (
	OpProvision = "provision" // 开通
	OpRenew     = "renew"     // 续费
	OpStart     = "start"
	OpStop      = "stop"
	OpRestart   = "restart"
	OpVNC       = "vnc"
	OpResize    = "resize"
	OpReinstall = "reinstall"
	OpDestroy   = "destroy"
	OpSnapshot  = "snapshot"
)

// 续费模式：order=上游下单续费，direct=直接延期，none=不支持。
const (
	RenewModeOrder  = "order"
	RenewModeDirect = "direct"
	RenewModeNone   = "none"
)

// 销毁模式：immediate=立即销毁，delayed=延迟/回收站，unsupported=不支持。
const (
	DestroyModeImmediate   = "immediate"
	DestroyModeDelayed     = "delayed"
	DestroyModeUnsupported = "unsupported"
)

// 签名策略类型（传输契约 §5，由 transport 包实现）。
const (
	SignerNone   = "none"
	SignerBearer = "bearer"
	SignerForm   = "form"
	SignerToken  = "token" // 自定义请求头令牌（如魔方云 access-token）
	SignerAKSK   = "ak_sk" // 阿里云 RPC
	SignerTC3    = "tc3"   // 腾讯云 TC3-HMAC-SHA256
)

// FieldType 凭证/端点字段的控件类型，用于后台动态表单渲染。
const (
	FieldTypeString   = "string"
	FieldTypePassword = "password"
	FieldTypeNumber   = "number"
	FieldTypeSelect   = "select"
	FieldTypeBool     = "bool"
	FieldTypeTextarea = "textarea"
)

// FieldOption 下拉选项。
type FieldOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Field 凭证/端点字段描述：驱动后台"添加渠道"动态表单 + 字段级加密。
type Field struct {
	Key         string        `json:"key"`
	Label       string        `json:"label"`
	Type        string        `json:"type"` // FieldType*
	Required    bool          `json:"required"`
	Secret      bool          `json:"secret"` // true=落库前字段级加密，回显脱敏
	Placeholder string        `json:"placeholder,omitempty"`
	Help        string        `json:"help,omitempty"`
	Default     string        `json:"default,omitempty"`
	Options     []FieldOption `json:"options,omitempty"`
}

// RateLimitSpec 渠道级限流参数（令牌桶）。
type RateLimitSpec struct {
	QPS   int `json:"qps"`   // 每秒令牌数，<=0 表示不限
	Burst int `json:"burst"` // 桶容量，<=0 时取 QPS
}

// CapabilityDescriptor 渠道能力描述符（契约②核心结构）。
type CapabilityDescriptor struct {
	Kind             string         `json:"kind"`              // upstream | compute
	SyncScopes       []string       `json:"sync_scopes"`       // Scope* 列表
	SpecAtoms        []string       `json:"spec_atoms"`        // 本渠道能提供/接受的原子 key
	BillingCycles    []string       `json:"billing_cycles"`    // month/year/hour/day
	Operations       []string       `json:"operations"`        // Op* 列表
	RenewMode        string         `json:"renew_mode"`        // RenewMode*
	DestroyMode      string         `json:"destroy_mode"`      // DestroyMode*
	CredentialSchema []Field        `json:"credential_schema"` // 凭证字段（动态表单 + 加密）
	EndpointSchema   []Field        `json:"endpoint_schema"`   // 接入地址/区域等连接参数
	SignerType       string         `json:"signer_type"`       // Signer*
	RateLimit        RateLimitSpec  `json:"rate_limit"`
	SupportsPaging   bool           `json:"supports_paging"`
	FieldDictionary  map[string]any `json:"field_dictionary,omitempty"` // 平台字段字典（D5 预埋，值可含"待核"标记）
	Implemented      bool           `json:"implemented"`                // 适配器是否已注册（运行时计算，非落库）
}

// HasOperation 判断是否声明了某能力操作。
func (d CapabilityDescriptor) HasOperation(op string) bool {
	for _, o := range d.Operations {
		if o == op {
			return true
		}
	}
	return false
}

// HasScope 判断是否声明了某同步 scope。
func (d CapabilityDescriptor) HasScope(scope string) bool {
	for _, s := range d.SyncScopes {
		if s == scope {
			return true
		}
	}
	return false
}

// ============================================================================
// 描述符注册表：适配器 init() 时登记，供注册表/后台/校验统一查询。
// 与 ProviderFactory 同一模式（见 factory.go），避免 provider_service 里再写死地图。
// ============================================================================

var (
	descriptorMu sync.RWMutex
	descriptors  = map[string]CapabilityDescriptor{}
)

// RegisterDescriptor 由各适配器包 init() 调用，登记能力描述符。
func RegisterDescriptor(providerType string, d CapabilityDescriptor) {
	descriptorMu.Lock()
	defer descriptorMu.Unlock()
	descriptors[providerType] = d
}

// Descriptor 查询某类型的描述符；第二个返回值为是否存在注册记录。
func Descriptor(providerType string) (CapabilityDescriptor, bool) {
	descriptorMu.RLock()
	defer descriptorMu.RUnlock()
	d, ok := descriptors[providerType]
	return d, ok
}

// RegisteredDescriptorTypes 返回已登记描述符的类型（升序，便于稳定输出）。
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

// CapabilitiesOf 读取适配器能力：优先描述符注册表，退回类型断言推导，
// 最后返回保守的"仅基础能力"描述符——保证任何 Provider 都能给出可用结果。
// 新代码应优先调用本函数，而不是直接做类型断言。
func CapabilitiesOf(p Provider) CapabilityDescriptor {
	if p == nil {
		return CapabilityDescriptor{Kind: KindUpstream, RenewMode: RenewModeNone, DestroyMode: DestroyModeUnsupported, SignerType: SignerNone}
	}
	if d, ok := Descriptor(p.GetType()); ok {
		return d
	}
	d := CapabilityDescriptor{
		Kind:        KindUpstream,
		RenewMode:   RenewModeNone,
		DestroyMode: DestroyModeUnsupported,
		SignerType:  SignerNone,
	}
	if _, ok := p.(ProductCatalog); ok {
		d.SyncScopes = append(d.SyncScopes, ScopeCatalog, ScopePrice)
		d.Operations = append(d.Operations, OpProvision)
	}
	if _, ok := p.(PoolReader); ok {
		d.SyncScopes = append(d.SyncScopes, ScopePool, ScopeRegion)
	}
	if _, ok := p.(InstanceProvisioning); ok {
		d.Operations = append(d.Operations, OpProvision)
	}
	if _, ok := p.(InstanceControl); ok {
		d.SyncScopes = append(d.SyncScopes, ScopeInstance)
		d.Operations = append(d.Operations, OpStart, OpStop, OpRestart, OpVNC)
	}
	if _, ok := p.(InstanceAdministration); ok {
		d.Operations = append(d.Operations, OpResize, OpDestroy)
		d.DestroyMode = DestroyModeImmediate
	}
	return d
}
