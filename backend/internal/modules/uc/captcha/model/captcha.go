// Package model 定义用户端验证码与二次验证体系的数据模型（doc91 §1）。
//
// 与迁移 042_captcha_and_mfa.sql 的 DDL 逐列对应；新增列必须同步两处，
// 否则启动期 AutoMigrate 与迁移脚本会漂移。
package model

import "time"

// 验证码通道。
const (
	ChannelImage = "image"
	ChannelEmail = "email"
	ChannelSMS   = "sms"
	ChannelTOTP  = "totp"
)

// 通道强度等级（doc91 §4.2）：min_channel_level 用它比较，数字越大越强。
const (
	ChannelLevelImage = 1
	ChannelLevelEmail = 2
	ChannelLevelSMS   = 3
	ChannelLevelTOTP  = 4
)

// ChannelLevel 返回通道强度等级；未知通道返回 0（低于任何基线，会被提回基线）。
func ChannelLevel(channel string) int {
	switch channel {
	case ChannelImage:
		return ChannelLevelImage
	case ChannelEmail:
		return ChannelLevelEmail
	case ChannelSMS:
		return ChannelLevelSMS
	case ChannelTOTP:
		return ChannelLevelTOTP
	default:
		return 0
	}
}

// ChannelLabel 通道中文名（用于「已按平台安全要求使用 X 验证」提示）。
func ChannelLabel(channel string) string {
	switch channel {
	case ChannelImage:
		return "图形验证码"
	case ChannelEmail:
		return "邮箱验证码"
	case ChannelSMS:
		return "短信验证码"
	case ChannelTOTP:
		return "动态口令"
	default:
		return channel
	}
}

// 验证码状态。
const (
	CodeStatusPending = "pending"
	CodeStatusUsed    = "used"
	CodeStatusExpired = "expired"
	CodeStatusFailed  = "failed"
)

// 策略 / 服务商通用状态。
const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

// 服务商健康状态。
const (
	HealthHealthy = "healthy"
	HealthDown    = "down"
	HealthPending = "pending"
)

// 验证场景编码（doc91 §1.2.2，共 17 项）。
const (
	SceneAdminLogin      = "admin_login"
	SceneUserLogin       = "user_login"
	SceneUserLoginSMS    = "user_login_sms"
	SceneUserLoginEmail  = "user_login_email"
	SceneUserRegister    = "user_register"
	ScenePasswordReset   = "password_reset"
	ScenePasswordChange  = "password_change"
	ScenePhoneBind       = "phone_bind"
	SceneEmailBind       = "email_bind"
	SceneWithdrawApply   = "withdraw_apply"
	ScenePayoutApply     = "payout_apply"
	SceneAPIKeyCreate    = "apikey_create"
	SceneAPIKeyView      = "apikey_view"
	SceneInstanceDestroy = "instance_destroy"
	SceneInstanceResize  = "instance_resize"
	SceneAdminGrant      = "admin_grant_change"
	SceneRealnameSubmit  = "realname_submit"
)

// CaptchaProvider 验证码服务商（captcha_providers）。
type CaptchaProvider struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement"`
	ProviderType string     `gorm:"column:provider_type;size:50;not null;index"`
	Name         string     `gorm:"size:100;not null"`
	Mode         string     `gorm:"size:20;not null;default:'api'"`
	Descriptor   string     `gorm:"type:jsonb"`
	Credentials  string     `gorm:"type:jsonb"`
	Endpoint     string     `gorm:"size:255;not null;default:''"`
	Scenes       string     `gorm:"type:jsonb"`
	Priority     int        `gorm:"not null;default:0"`
	HealthStatus string     `gorm:"column:health_status;size:20;not null;default:''"`
	LastError    string     `gorm:"column:last_error;type:text"`
	LastCheckAt  *time.Time `gorm:"column:last_check_at"`
	Status       int        `gorm:"not null;default:1"`
	IsDefault    bool       `gorm:"column:is_default;not null;default:false"`
	Remark       string     `gorm:"size:255;not null;default:''"`
	CreatedAt    time.Time  `gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime"`
	DeletedAt    *time.Time `gorm:"index"`
}

func (CaptchaProvider) TableName() string { return "captcha_providers" }

// CaptchaPolicy 场景策略（平台基线，D4「强制」侧）。
type CaptchaPolicy struct {
	ID                   uint64    `gorm:"primaryKey;autoIncrement"`
	Scene                string    `gorm:"size:64;not null;uniqueIndex:uk_captcha_policies_scene"`
	Name                 string    `gorm:"size:100;not null"`
	ImageRequired        bool      `gorm:"column:image_required;not null;default:false"`
	ImageProviderID      uint64    `gorm:"column:image_provider_id;not null;default:0"`
	ImageLevel           string    `gorm:"column:image_level;size:20;not null;default:'normal'"`
	OTPRequired          bool      `gorm:"column:otp_required;not null;default:false"`
	OTPChannel           string    `gorm:"column:otp_channel;size:20;not null;default:'email'"`
	MinChannelLevel      int       `gorm:"column:min_channel_level;not null;default:0"`
	UserCanTighten       bool      `gorm:"column:user_can_tighten;not null;default:true"`
	UserCanChooseChannel bool      `gorm:"column:user_can_choose_channel;not null;default:true"`
	MaxAttempts          int       `gorm:"column:max_attempts;not null;default:5"`
	TTLSeconds           int       `gorm:"column:ttl_seconds;not null;default:0"`
	SendIntervalSeconds  int       `gorm:"column:send_interval_seconds;not null;default:0"`
	DailyLimitPerTarget  int       `gorm:"column:daily_limit_per_target;not null;default:0"`
	BindConfigKey        string    `gorm:"column:bind_config_key;size:64;not null;default:''"`
	Status               string    `gorm:"size:20;not null;default:'active'"`
	Remark               string    `gorm:"size:255;not null;default:''"`
	CreatedAt            time.Time `gorm:"autoCreateTime"`
	UpdatedAt            time.Time `gorm:"autoUpdateTime"`
}

func (CaptchaPolicy) TableName() string { return "captcha_policies" }

// VerificationCode 验证码记录（审计 + Redis 降级读）。
//
// 绝不存明文 code：降级校验用 sha256(input + code_salt) 与 code_hash 比对。
type VerificationCode struct {
	ID            uint64     `gorm:"primaryKey;autoIncrement"`
	Scene         string     `gorm:"size:64;not null;index"`
	Channel       string     `gorm:"size:20;not null"`
	Target        string     `gorm:"size:191;not null;default:''"`
	TargetHash    string     `gorm:"column:target_hash;size:64;not null;default:'';index"`
	CodeHash      string     `gorm:"column:code_hash;size:128;not null"`
	CodeSalt      string     `gorm:"column:code_salt;size:32;not null"`
	CaptchaKey    string     `gorm:"column:captcha_key;size:64;not null;default:''"`
	Status        string     `gorm:"size:20;not null;default:'pending';index"`
	Attempts      int        `gorm:"not null;default:0"`
	MaxAttempts   int        `gorm:"column:max_attempts;not null;default:5"`
	ProviderID    uint64     `gorm:"column:provider_id;not null;default:0"`
	ProviderMsgID string     `gorm:"column:provider_msg_id;size:128;not null;default:''"`
	CostFen       int        `gorm:"column:cost_fen;not null;default:0"`
	RequestIP     string     `gorm:"column:request_ip;size:64;not null;default:''"`
	UserAgent     string     `gorm:"column:user_agent;size:255;not null;default:''"`
	OperatorID    uint64     `gorm:"column:operator_id;not null;default:0"`
	ExpireAt      time.Time  `gorm:"column:expire_at;not null"`
	UsedAt        *time.Time `gorm:"column:used_at"`
	CreatedAt     time.Time  `gorm:"autoCreateTime"`
}

func (VerificationCode) TableName() string { return "verification_codes" }

// UserSecuritySettings 用户加严设置（D4「加严」侧）。
type UserSecuritySettings struct {
	ID                 uint64    `gorm:"primaryKey;autoIncrement"`
	UserID             uint64    `gorm:"column:user_id;not null;uniqueIndex:uk_user_security_settings_user"`
	MFAEnabled         bool      `gorm:"column:mfa_enabled;not null;default:false"`
	MFAChannel         string    `gorm:"column:mfa_channel;size:20;not null;default:''"`
	SceneOverrides     string    `gorm:"column:scene_overrides;type:jsonb"`
	TrustWindowMinutes int       `gorm:"column:trust_window_minutes;not null;default:0"`
	UpdatedAt          time.Time `gorm:"autoUpdateTime"`
	CreatedAt          time.Time `gorm:"autoCreateTime"`
}

func (UserSecuritySettings) TableName() string { return "user_security_settings" }

// SceneOverride 场景级加严覆盖项。
type SceneOverride struct {
	OTPRequired *bool  `json:"otp_required,omitempty"`
	OTPChannel  string `json:"otp_channel,omitempty"`
	// ImageRequired 仅允许置 true（加严）。
	ImageRequired bool `json:"image_required,omitempty"`
}
