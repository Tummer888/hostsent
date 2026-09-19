// Package realname 提供实名认证核验服务商的中立契约与注册表（doc104 §5.5）。
//
// 与 pkg/payment / pkg/notifier / pkg/captcha / pkg/upstream 同构：
// 新增一家核验服务商 = 一个适配器包 + 能力描述符 + 一行 realname_providers，
// 核心零改动。装配层用空导入触发 init() 注册。
//
// 能力用类型断言发现（对齐 pkg/payment 的 Collector/Refunder 手法）：
//   - 所有 provider 都必须实现 Provider；
//   - 需要用户跳转完成核验的（支付宝人脸认证）额外实现 Initializer。
package realname

import (
	"context"
	"time"

	"hostsent/backend/internal/pkg/integration"
)

// CapabilityDescriptor 实名核验服务商能力描述符（驱动后台「类型选择 → 动态凭证表单」）。
type CapabilityDescriptor struct {
	// Type 类型标识：manual / alipay。
	Type string `json:"type"`
	// Name 中文名。
	Name string `json:"name"`
	// Mode 交互形态：manual（纯人工）/ api（无跳转核验）/ redirect（需用户跳转）。
	Mode string `json:"mode"`
	// CredentialSchema 凭证字段（secret 字段字段级加密，回显脱敏）。
	CredentialSchema []integration.Field `json:"credential_schema"`
	// CertifyModes 支持的认证方式（人脸 / 证件照 …），供后台下拉。
	CertifyModes []integration.FieldOption `json:"certify_modes"`
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

// VerifyRequest 一次核验请求。
//
// 注意：IDNumber / Mobile 必须是**明文**——三方核验接口不收脱敏值。
// 上层从 verification_applications 的密文列解密后传入，本包不负责加解密。
type VerifyRequest struct {
	ApplicationID uint64
	UserID        uint64
	// RealName 真实姓名 / 企业名称。
	RealName string
	// IDNumber 证件号（个人为身份证号，企业为统一社会信用代码）。
	IDNumber string
	// Mobile 手机号（三要素核验需要；二要素为空）。
	Mobile string
	// CertifyMode 认证方式，取值由描述符 CertifyModes 声明（如 FACE / CERT_PHOTO）。
	CertifyMode string
}

// VerifyResult 核验结果。
//
// Passed 是唯一权威判据；BizCode/Message 供审计与前端提示。
// Raw 是上游原始响应片段，**只允许落日志，绝不回显给前端**。
type VerifyResult struct {
	Passed  bool
	BizCode string
	Message string
	TxnNo   string
	Raw     map[string]string
}

// AuthChallenge 跳转式核验的初始化结果。
type AuthChallenge struct {
	// AuthURL 用户需访问的认证地址。
	AuthURL string
	// TxnNo 本次核验的流水号，用于后续 Query。
	TxnNo string
	// ExpireAt 认证入口失效时间。
	ExpireAt time.Time
}

// ProviderConfig 一次调用使用的、已解密的服务商配置。
type ProviderConfig struct {
	ProviderID  uint64
	Type        string
	Endpoint    string
	Credentials map[string]string
}

// Provider 实名核验服务商适配器。
type Provider interface {
	// Type 服务商类型：manual / alipay / ...
	Type() string
	// DirectVerify 无跳转核验。不支持该形态的 provider 返回 ErrAdapterNotImplemented。
	DirectVerify(ctx context.Context, cfg ProviderConfig, req VerifyRequest) (*VerifyResult, error)
	// Test 凭证与连通性测试。
	Test(ctx context.Context, cfg ProviderConfig) error
}

// Initializer 需要用户跳转完成核验的服务商（支付宝人脸认证属此类）。
type Initializer interface {
	// Initialize 生成认证入口。
	Initialize(ctx context.Context, cfg ProviderConfig, req VerifyRequest) (*AuthChallenge, error)
	// Query 查询一次核验的结果。
	Query(ctx context.Context, cfg ProviderConfig, txnNo string) (*VerifyResult, error)
}
