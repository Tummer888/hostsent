// Package model 定义第三方登录的绑定关系与渠道配置（doc104 §6.1）。
//
// 两张表分工明确：
//   - user_oauth_bindings：用户与外部身份的绑定关系（权威数据）；
//   - oauth_providers：渠道凭证配置（形态对齐 captcha_providers / realname_providers）。
//
// users.oauth_provider / users.oauth_openid 保留为「主绑定」快照（管理端列表与详情
// 已在消费），首次绑定时同步写入，解绑主绑定时按剩余绑定择一或清空。
package model

import "time"

// 绑定状态。
const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

// 服务商健康状态（对齐 captcha_providers 口径）。
const (
	HealthHealthy = "healthy"
	HealthDown    = "down"
	HealthPending = "pending"
)

// UserOAuthBinding 用户第三方账号绑定（user_oauth_bindings）。
type UserOAuthBinding struct {
	ID       uint64 `gorm:"primaryKey;autoIncrement"`
	UserID   uint64 `gorm:"column:user_id;not null;index:idx_user_oauth_bindings_user"`
	Provider string `gorm:"column:provider;size:32;not null;uniqueIndex:uk_user_oauth_bindings_provider_openid,priority:1"`
	OpenID   string `gorm:"column:openid;size:128;not null;uniqueIndex:uk_user_oauth_bindings_provider_openid,priority:2"`
	// UnionID 微信开放平台跨应用唯一标识；其他渠道为空。
	UnionID     string     `gorm:"column:unionid;size:128;not null;default:''"`
	Nickname    string     `gorm:"column:nickname;size:64;not null;default:''"`
	Avatar      string     `gorm:"column:avatar;size:255;not null;default:''"`
	Status      string     `gorm:"size:16;not null;default:active"`
	BoundAt     time.Time  `gorm:"column:bound_at;not null"`
	LastLoginAt *time.Time `gorm:"column:last_login_at"`
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`
}

// TableName 返回绑定关系表名。
func (UserOAuthBinding) TableName() string {
	return "user_oauth_bindings"
}

// OAuthProvider 第三方登录渠道配置（oauth_providers）。
type OAuthProvider struct {
	ID       uint64 `gorm:"primaryKey;autoIncrement"`
	Provider string `gorm:"size:32;not null;uniqueIndex:uk_oauth_providers_provider,WHERE:deleted_at IS NULL"`
	Name     string `gorm:"size:100;not null"`
	// Enabled 是否在登录页展示并允许发起授权。
	Enabled bool `gorm:"not null;default:false"`
	// Mode api / qr，由描述符决定。
	Mode       string `gorm:"size:20;not null;default:'api'"`
	Descriptor string `gorm:"type:jsonb"`
	// Credentials 字段级加密后的凭证 JSON（secret 字段落库前加密，回显脱敏）。
	Credentials  string     `gorm:"type:jsonb"`
	Scopes       string     `gorm:"size:255;not null;default:''"`
	Icon         string     `gorm:"size:64;not null;default:''"`
	SortOrder    int64      `gorm:"column:sort_order;not null;default:0"`
	HealthStatus string     `gorm:"column:health_status;size:20;not null;default:''"`
	LastError    string     `gorm:"column:last_error;type:text"`
	LastCheckAt  *time.Time `gorm:"column:last_check_at"`
	Remark       string     `gorm:"size:255;not null;default:''"`
	CreatedAt    time.Time  `gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime"`
	DeletedAt    *time.Time `gorm:"index"`
}

// TableName 返回渠道配置表名。
func (OAuthProvider) TableName() string {
	return "oauth_providers"
}
