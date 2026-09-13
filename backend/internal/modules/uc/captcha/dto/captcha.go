// Package dto 定义验证码与二次验证体系的请求/响应结构（doc91 §3.3/§4.1/§4.5）。
package dto

import "hostsent/backend/internal/modules/uc/captcha/model"

// SendCodeRequest 下发 OTP（公开接口，登录前也要用）。
//
// Target 允许为空：用户端已登录场景（/uc/security/verification/send）由服务层
// 从账号绑定关系解析目标；公开场景由服务层校验必填。
type SendCodeRequest struct {
	Scene       string `json:"scene" binding:"required"`
	Channel     string `json:"channel" binding:"required,oneof=email sms"`
	Target      string `json:"target"`
	CaptchaKey  string `json:"captcha_key"`
	CaptchaCode string `json:"captcha_code"`
	// SkipCaptcha 内部调用方已经校验过图形码时为 true。
	// 图形码是一次性的，服务层再验一次必然失败；注册/找回密码等
	// 「同一请求内先验图形码再发码」的链路必须置位（doc91 §5.4）。
	SkipCaptcha bool `json:"-"`
}

// SendCodeResponse 下发结果（target 只回打码值）。
type SendCodeResponse struct {
	Sent         bool   `json:"sent"`
	Channel      string `json:"channel"`
	TargetMasked string `json:"target_masked"`
	ExpireIn     int    `json:"expire_in"`
	Cooldown     int    `json:"cooldown"`
}

// VerifyCodeRequest 校验 OTP 并换取关键操作票据。
type VerifyCodeRequest struct {
	Scene string `json:"scene" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

// VerifyCodeResponse 校验成功后的票据。
type VerifyCodeResponse struct {
	VerifyTicket string `json:"verify_ticket"`
	ExpireIn     int    `json:"expire_in"`
}

// ImageChallengeResponse 图形码挑战响应。
//
// 关键安全点：响应体只有 key 与图片（或第三方前端参数），**不含答案**、
// 不含 SVG 文本、不含按字符拆分的图元坐标（doc91 §3.2）。
// Params 仅供第三方 SDK 初始化（captcha_id 这类可公开字段），native 时为空。
type ImageChallengeResponse struct {
	CaptchaKey  string            `json:"captcha_key"`
	ImageBase64 string            `json:"image_base64"`
	ExpireIn    int               `json:"expires_in"`
	Provider    string            `json:"provider"`
	Params      map[string]string `json:"params,omitempty"`
}

// ScenePolicyInfo 公开接口下发的场景策略（前端据此决定是否渲染验证码）。
type ScenePolicyInfo struct {
	ImageRequired bool   `json:"image_required"`
	OTPRequired   bool   `json:"otp_required"`
	OTPChannel    string `json:"otp_channel,omitempty"`
}

// AuthConfigResponse `/public/auth-config`：避免「后端要、前端没显示」的死锁。
type AuthConfigResponse struct {
	CaptchaEnabled bool                       `json:"captcha_enabled"`
	Scenes         map[string]ScenePolicyInfo `json:"scenes"`
	ImageProvider  string                     `json:"image_provider"`
	ThirdParty     ThirdPartyConfig           `json:"third_party"`
}

// ThirdPartyConfig 第三方图形码前端 SDK 初始化参数（native 时为空）。
type ThirdPartyConfig struct {
	Provider  string `json:"provider"`
	CaptchaID string `json:"captcha_id"`
}

// ProviderInfo 管理端服务商列表项（凭证只回脱敏值）。
type ProviderInfo struct {
	ID             uint64            `json:"id"`
	ProviderType   string            `json:"provider_type"`
	Name           string            `json:"name"`
	Mode           string            `json:"mode"`
	Endpoint       string            `json:"endpoint"`
	Priority       int               `json:"priority"`
	HealthStatus   string            `json:"health_status"`
	LastError      string            `json:"last_error"`
	LastCheckAt    string            `json:"last_check_at"`
	Status         int               `json:"status"`
	IsDefault      bool              `json:"is_default"`
	Remark         string            `json:"remark"`
	Credentials    map[string]string `json:"credentials"`
	CredentialKeys []string          `json:"credential_keys"`
	Implemented    bool              `json:"implemented"`
	Builtin        bool              `json:"builtin"`
	Supported      bool              `json:"supported"` // 适配器是否已注册工厂
}

// ProviderTypeInfo 可选服务商类型（描述符驱动前端动态表单）。
type ProviderTypeInfo struct {
	Type             string `json:"type"`
	Name             string `json:"name"`
	Mode             string `json:"mode"`
	Icon             string `json:"icon"`
	DocURL           string `json:"doc_url"`
	AdapterVersion   string `json:"adapter_version"`
	Implemented      bool   `json:"implemented"`
	Builtin          bool   `json:"builtin"`
	CredentialSchema any    `json:"credential_schema"`
}

// ProviderUpsertRequest 新建/更新服务商。
type ProviderUpsertRequest struct {
	ProviderType string            `json:"provider_type" binding:"required"`
	Name         string            `json:"name"`
	Endpoint     string            `json:"endpoint"`
	Credentials  map[string]string `json:"credentials"`
	Scenes       []string          `json:"scenes"`
	Priority     int               `json:"priority"`
	Status       *int              `json:"status"`
	IsDefault    *bool             `json:"is_default"`
	Remark       string            `json:"remark"`
}

// ProviderTestRequest 测试发送/连通性。
type ProviderTestRequest struct {
	Target string `json:"target"`
}

// PolicyInfo 场景策略列表项。
type PolicyInfo struct {
	Scene                string `json:"scene"`
	Name                 string `json:"name"`
	ImageRequired        bool   `json:"image_required"`
	ImageProviderID      uint64 `json:"image_provider_id"`
	ImageLevel           string `json:"image_level"`
	OTPRequired          bool   `json:"otp_required"`
	OTPChannel           string `json:"otp_channel"`
	MinChannelLevel      int    `json:"min_channel_level"`
	UserCanTighten       bool   `json:"user_can_tighten"`
	UserCanChooseChannel bool   `json:"user_can_choose_channel"`
	MaxAttempts          int    `json:"max_attempts"`
	TTLSeconds           int    `json:"ttl_seconds"`
	SendIntervalSeconds  int    `json:"send_interval_seconds"`
	DailyLimitPerTarget  int    `json:"daily_limit_per_target"`
	Status               string `json:"status"`
	Remark               string `json:"remark"`
	// PlatformForced 图形码或 OTP 开启时置位：开启后用户无法关闭（前端显示「平台强制」徽标）。
	PlatformForced bool `json:"platform_forced"`
}

// PolicyUpdateRequest 更新场景策略。
type PolicyUpdateRequest struct {
	ImageRequired        *bool   `json:"image_required"`
	ImageProviderID      *uint64 `json:"image_provider_id"`
	ImageLevel           *string `json:"image_level"`
	OTPRequired          *bool   `json:"otp_required"`
	OTPChannel           *string `json:"otp_channel"`
	MinChannelLevel      *int    `json:"min_channel_level"`
	UserCanTighten       *bool   `json:"user_can_tighten"`
	UserCanChooseChannel *bool   `json:"user_can_choose_channel"`
	MaxAttempts          *int    `json:"max_attempts"`
	TTLSeconds           *int    `json:"ttl_seconds"`
	SendIntervalSeconds  *int    `json:"send_interval_seconds"`
	DailyLimitPerTarget  *int    `json:"daily_limit_per_target"`
	Status               *string `json:"status"`
	Remark               *string `json:"remark"`
}

// SceneStatInfo 按场景统计项。
type SceneStatInfo struct {
	Scene    string  `json:"scene"`
	Total    int64   `json:"total"`
	Used     int64   `json:"used"`
	Failed   int64   `json:"failed"`
	Expired  int64   `json:"expired"`
	PassRate float64 `json:"pass_rate"`
	CostFen  int64   `json:"cost_fen"`
	CostYuan float64 `json:"cost_yuan"`
}

// StatsResponse 统计响应。
type StatsResponse struct {
	From  string          `json:"from"`
	To    string          `json:"to"`
	Items []SceneStatInfo `json:"items"`
}

// SecuritySettingsResponse 用户安全设置（只回 effective，不回原始请求值）。
type SecuritySettingsResponse struct {
	MFAEnabled         bool             `json:"mfa_enabled"`
	MFAChannel         string           `json:"mfa_channel"`
	TrustWindowMinutes int              `json:"trust_window_minutes"`
	Phone              string           `json:"phone"`
	PhoneMasked        string           `json:"phone_masked"`
	PhoneBound         bool             `json:"phone_bound"`
	Email              string           `json:"email"`
	EmailMasked        string           `json:"email_masked"`
	EmailBound         bool             `json:"email_bound"`
	Scenes             []EffectiveScene `json:"scenes"`
	Notice             string           `json:"notice"`
}

// EffectiveScene 单场景生效策略（前端只渲染它）。
type EffectiveScene struct {
	Scene           string `json:"scene"`
	Name            string `json:"name"`
	ImageRequired   bool   `json:"image_required"`
	OTPRequired     bool   `json:"otp_required"`
	OTPChannel      string `json:"otp_channel"`
	MinChannelLevel int    `json:"min_channel_level"`
	PlatformForced  bool   `json:"platform_forced"`
	UserCanTighten  bool   `json:"user_can_tighten"`
	Notice          string `json:"notice"`
	Source          string `json:"source"` // platform / user_tightened
}

// SecuritySettingsUpdateRequest 用户设置更新：只表达「加严」，没有「关闭」语义。
type SecuritySettingsUpdateRequest struct {
	MFAEnabled         *bool                          `json:"mfa_enabled"`
	MFAChannel         string                         `json:"mfa_channel"`
	SceneOverrides     map[string]model.SceneOverride `json:"scene_overrides"`
	TrustWindowMinutes *int                           `json:"trust_window_minutes"`
}

// ChannelOption 可选通道（用户端通道选择列表）。
type ChannelOption struct {
	Channel   string `json:"channel"`
	Label     string `json:"label"`
	Level     int    `json:"level"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// VerificationRequirementResponse 某场景的验证要求（关键操作前端据此弹框）。
//
// NeedVerification=false 有两种含义：策略本就不需要验证，或已有票据可直接执行；
// 前端用 TicketExists 区分（ticket 存在时不再弹框，直接重放原请求）。
type VerificationRequirementResponse struct {
	Scene            string          `json:"scene"`
	ImageRequired    bool            `json:"image_required"`
	Channel          string          `json:"channel"`
	TargetMasked     string          `json:"target_masked"`
	NeedVerification bool            `json:"need_verification"`
	TicketExists     bool            `json:"ticket_exists"`
	VerifyTicketTTL  int             `json:"verify_ticket_ttl"`
	Channels         []ChannelOption `json:"channels"`
}
