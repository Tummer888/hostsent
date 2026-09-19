// Package oauth 提供第三方登录服务商的中立契约与注册表（doc104 §6.3）。
//
// 与 pkg/payment / pkg/captcha / pkg/notifier / pkg/realname 同构：
// 新增一家登录渠道 = 一个适配器包 + 能力描述符 + 一行 oauth_providers，核心零改动。
//
// 与实名核验包（pkg/realname）分开而不是合并：两者的凭证体系、交互形态
// （跳转授权 vs 核验）、生命周期都不同，合并会让「加一家登录渠道」被迫改动
// 实名域的类型，违反中立构件的最小依赖原则（对齐 pkg/integration 的拆分理由）。
package oauth

import (
	"context"

	"hostsent/backend/internal/pkg/integration"
)

// CapabilityDescriptor 登录渠道能力描述符（驱动后台动态凭证表单）。
type CapabilityDescriptor struct {
	// Type 类型标识：wechat / qq / alipay。
	Type string `json:"type"`
	// Name 中文名。
	Name string `json:"name"`
	// Mode api（标准 OAuth2 授权码）/ qr（扫码）。
	Mode string `json:"mode"`
	// CredentialSchema 凭证字段（secret 字段字段级加密，回显脱敏）。
	CredentialSchema []integration.Field `json:"credential_schema"`
	// DefaultScopes 默认申请的授权范围。
	DefaultScopes string `json:"default_scopes"`
	// Icon 图标标识。
	Icon string `json:"icon"`
	// DocURL 官方文档地址。
	DocURL string `json:"doc_url"`
	// AdapterVersion 适配器版本。
	AdapterVersion string `json:"adapter_version"`
	// Implemented 是否已注册真实实现（运行时计算，非落库）。
	Implemented bool `json:"implemented"`
}

// ProviderConfig 一次调用使用的、已解密的渠道配置。
type ProviderConfig struct {
	ProviderID  uint64
	Type        string
	Endpoint    string
	Credentials map[string]string
	Scopes      string
}

// Token 授权码换取的访问令牌。
type Token struct {
	AccessToken  string
	RefreshToken string
	OpenID       string
	UnionID      string
	ExpiresIn    int
	// Raw 上游原始字段，只允许落日志，绝不回显。
	Raw map[string]string
}

// ExternalUser 外部身份资料。
type ExternalUser struct {
	OpenID   string
	UnionID  string
	Nickname string
	Avatar   string
}

// Provider 第三方登录适配器。
type Provider interface {
	// Type 渠道标识。
	Type() string
	// AuthorizeURL 拼装授权地址。state 由调用方生成并校验，适配器只负责拼接。
	AuthorizeURL(cfg ProviderConfig, state, redirectURI string) string
	// Exchange 用授权码换令牌。
	Exchange(ctx context.Context, cfg ProviderConfig, code, redirectURI string) (*Token, error)
	// UserInfo 拉取外部身份资料。
	UserInfo(ctx context.Context, cfg ProviderConfig, token *Token) (*ExternalUser, error)
	// Test 凭证与连通性测试。
	Test(ctx context.Context, cfg ProviderConfig) error
}

// ValidateCredentials 按描述符校验必填凭证字段。
func ValidateCredentials(d CapabilityDescriptor, creds map[string]string) error {
	return validateCredentials(d, creds)
}
