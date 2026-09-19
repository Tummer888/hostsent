// Package dto 提供第三方登录的请求与响应结构（doc104 §6.6）。
package dto

import (
	"time"

	"hostsent/backend/internal/pkg/integration"
)

// ProviderTypeInfo 渠道类型元数据（描述符驱动前端动态表单）。
type ProviderTypeInfo struct {
	Type             string              `json:"type"`
	Name             string              `json:"name"`
	Mode             string              `json:"mode"`
	Icon             string              `json:"icon"`
	DocURL           string              `json:"doc_url"`
	AdapterVersion   string              `json:"adapter_version"`
	DefaultScopes    string              `json:"default_scopes"`
	Implemented      bool                `json:"implemented"`
	CredentialSchema []integration.Field `json:"credential_schema"`
}

// ProviderInfo 渠道配置（凭证只回脱敏值）。
type ProviderInfo struct {
	ID           uint64 `json:"id"`
	Provider     string `json:"provider"`
	Name         string `json:"name"`
	Enabled      bool   `json:"enabled"`
	Mode         string `json:"mode"`
	Scopes       string `json:"scopes"`
	Icon         string `json:"icon"`
	SortOrder    int64  `json:"sort_order"`
	HealthStatus string `json:"health_status"`
	LastError    string `json:"last_error"`
	LastCheckAt  string `json:"last_check_at"`
	Remark       string `json:"remark"`
	// Supported 该渠道是否已注册真实适配器（false = 只登记了描述符）。
	Supported bool `json:"supported"`
	// Implemented 描述符里声明的实现状态（与 Supported 同源，保留给前端旧字段）。
	Implemented bool `json:"implemented"`
	// Credentials 脱敏后的凭证（secret 字段形如 ab****yz）。
	Credentials map[string]string `json:"credentials"`
	// CredentialKeys 已配置的凭证字段名（前端判断「是否已填」而不看值）。
	CredentialKeys []string `json:"credential_keys"`
	// CallbackURL 回调地址，由后端按当前域名拼装，运营复制到三方开放平台。
	CallbackURL string    `json:"callback_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ProviderUpdateRequest 渠道配置更新。
//
// 凭证语义与支付/验证码渠道一致：回传脱敏值表示「未修改」，保留原密文。
type ProviderUpdateRequest struct {
	Enabled     *bool             `json:"enabled"`
	Scopes      string            `json:"scopes"`
	SortOrder   *int64            `json:"sort_order"`
	Remark      string            `json:"remark"`
	Credentials map[string]string `json:"credentials"`
}

// PublicProviderInfo 用户端可见的渠道元数据（**不含任何凭证字段**）。
type PublicProviderInfo struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`
	Icon     string `json:"icon"`
	Mode     string `json:"mode"`
}

// BindingInfo 一条绑定关系。
type BindingInfo struct {
	ID          uint64     `json:"id"`
	UserID      uint64     `json:"user_id"`
	Username    string     `json:"username"`
	Provider    string     `json:"provider"`
	OpenID      string     `json:"openid"`
	UnionID     string     `json:"unionid,omitempty"`
	Nickname    string     `json:"nickname"`
	Avatar      string     `json:"avatar"`
	Status      string     `json:"status"`
	BoundAt     time.Time  `json:"bound_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// MyBindingInfo 用户端「我的绑定」条目。
//
// 刻意不回显 openid：它是外部身份标识，用户看它没有意义，
// 泄露到前端还会让「换绑同一微信账号」这类操作变得可被社工利用。
type MyBindingInfo struct {
	Provider    string     `json:"provider"`
	Name        string     `json:"name"`
	Icon        string     `json:"icon"`
	Nickname    string     `json:"nickname"`
	Avatar      string     `json:"avatar"`
	Status      string     `json:"status"`
	BoundAt     time.Time  `json:"bound_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	// IsPrimary 是否主绑定（users.oauth_provider 快照指向的那一条）。
	IsPrimary bool `json:"is_primary"`
	// CanUnbind 解绑守卫的服务端预判，前端据此禁用按钮而不是点完才报错。
	CanUnbind bool `json:"can_unbind"`
}

// BindingListQuery 管理端绑定列表筛选。
type BindingListQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Provider string `form:"provider"`
	UserID   uint64 `form:"user_id"`
	Status   string `form:"status"`
	Keyword  string `form:"keyword"`
}

// BindingListResponse 分页响应。
type BindingListResponse struct {
	Items []BindingInfo `json:"items"`
	Meta  ListMeta      `json:"meta"`
}

// ListMeta 分页元信息。
type ListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// ExchangeRequest ticket 换正式令牌。
type ExchangeRequest struct {
	Ticket string `json:"ticket" binding:"required"`
}

// ExchangeResponse 换票结果。
//
// NeedBind 为 true 时 Token 为空：该第三方账号尚未注册且平台未开自动注册，
// 前端应提示用户「先用账号密码登录后绑定」。
type ExchangeResponse struct {
	Token    string `json:"token"`
	NeedBind bool   `json:"need_bind"`
	Provider string `json:"provider"`
	// User 登录成功时的用户摘要（与 /uc/auth/me 同构的最小集）。
	User *ExchangeUser `json:"user,omitempty"`
}

// ExchangeUser 换票成功返回的用户摘要。
type ExchangeUser struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Avatar   string `json:"avatar"`
	Status   string `json:"status"`
}
