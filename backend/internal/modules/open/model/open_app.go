// Package model 提供开放平台（P6/T6.1）的数据模型。
package model

import "time"

// OpenApp 状态。
const (
	OpenAppStatusEnabled  int = 1 // 启用
	OpenAppStatusDisabled int = 0 // 停用
)

// OpenApp 开放平台接入应用：下游以 app_id/secret 签名调用 /open/v1/*。
// app_secret / notify_secret 为 internal/pkg/crypto 加密密文（密钥 app.encrypt_key）。
// owner_user_id 是该下游在平台的归属账号：定价走其用户组折扣策略（D3），
// 代客下单扣其余额、实例归属也挂它。
type OpenApp struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	AppID        string    `gorm:"column:app_id;size:64;uniqueIndex;not null"`
	AppSecret    string    `gorm:"column:app_secret;type:text;not null"`
	Name         string    `gorm:"column:name;size:128;default:''"`
	Status       int       `gorm:"column:status;default:1"`
	RateLimit    int       `gorm:"column:rate_limit;default:0"` // 次/分钟，0=平台默认
	NotifyURL    string    `gorm:"column:notify_url;size:512;default:''"`
	NotifySecret string    `gorm:"column:notify_secret;type:text"`
	APIVersion   string    `gorm:"column:api_version;size:16;default:'v1'"`
	OwnerUserID  uint64    `gorm:"column:owner_user_id;not null;index"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名。
func (OpenApp) TableName() string { return "open_apps" }

// OpenAppScope 应用能力位（D2：能力位集合中不存在 instance:destroy）。
// scope ∈ catalog:read / order:create / instance:read / instance:renew /
// instance:power / instance:suspend / audit:read；object 预留范围限定（如分类 ID），
// 空串表示不限。
type OpenAppScope struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	AppID     uint64    `gorm:"column:app_id;not null;uniqueIndex:uk_open_app_scopes"`
	Scope     string    `gorm:"column:scope;size:64;not null;uniqueIndex:uk_open_app_scopes"`
	Object    string    `gorm:"column:object;size:64;not null;default:'';uniqueIndex:uk_open_app_scopes"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名。
func (OpenAppScope) TableName() string { return "open_app_scopes" }

// OpenAppIPRule 应用来源 IP 白名单（CIDR）。应用没有任何规则时放行全部来源。
type OpenAppIPRule struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	AppID     uint64    `gorm:"column:app_id;not null;index"`
	CIDR      string    `gorm:"column:cidr;size:64;not null"`
	Note      string    `gorm:"column:note;size:255;default:''"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名。
func (OpenAppIPRule) TableName() string { return "open_app_ip_rules" }
