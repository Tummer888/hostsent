package payment

import (
	"sort"
	"sync"

	"hostsent/backend/internal/pkg/integration"
)

// ============================================================================
// 能力契约：CapabilityDescriptor 把渠道能力升级为可查询、可展示、可校验的
// 字段级描述，驱动后台"渠道类型选择 → 动态凭证表单 → 能力矩阵"。
// 注册表模式与 pkg/upstream 完全一致（适配器 init() 时登记）。
// ============================================================================

// CapabilityDescriptor 支付渠道能力描述符。
type CapabilityDescriptor struct {
	// Mode 支付模式：api（接口）/ manual（线下人工）。
	Mode string `json:"mode"`
	// Scenes 支持的支付场景。
	Scenes []string `json:"scenes"`
	// Operations 支持的能力操作（collect/query/close/refund/payout）。
	Operations []string `json:"operations"`
	// CredentialSchema 凭证字段（驱动动态表单 + 字段级加密）。
	CredentialSchema []integration.Field `json:"credential_schema"`
	// EndpointSchema 端点/连接参数字段。
	EndpointSchema []integration.Field `json:"endpoint_schema"`
	// CertMode 证书模式：key（仅密钥）/ cert（需上传证书）。
	CertMode string `json:"cert_mode"`
	// SignerType 签名类型（none/md5/rsa2/v3），供能力矩阵展示。
	SignerType string `json:"signer_type"`
	// SupportsPayout 是否支持出款（打款/代付）。
	SupportsPayout bool `json:"supports_payout"`
	// SupportsPartialRefund 是否支持部分退款。
	SupportsPartialRefund bool `json:"supports_partial_refund"`
	// SupportsQuery 是否支持主动查单（无回调渠道必需）。
	SupportsQuery bool `json:"supports_query"`
	// Currency 币种（默认 CNY）。
	Currency string `json:"currency"`
	// FeeRate 默认费率（比例，0.006=0.6%）。
	FeeRate float64 `json:"fee_rate"`
	// SettleMode 结算模式（T+0/T+1）。
	SettleMode string `json:"settle_mode"`
	// Implemented 适配器是否已注册（运行时计算，非落库）。
	Implemented bool `json:"implemented"`
	// DocURL 官方文档地址（后台帮助入口）。
	DocURL string `json:"doc_url"`
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

// HasScene 判断是否声明了某场景。
func (d CapabilityDescriptor) HasScene(scene string) bool {
	for _, s := range d.Scenes {
		if s == scene {
			return true
		}
	}
	return false
}

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

// Descriptor 查询某类型的能力描述符。
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
