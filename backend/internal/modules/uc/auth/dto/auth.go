package dto

// 登录方式（doc91 §5.3）。
const (
	// LoginTypePassword 账号密码登录（默认）。
	LoginTypePassword = "password"
	// LoginTypeSMS 手机验证码登录。
	LoginTypeSMS = "sms"
	// LoginTypeEmail 邮箱验证码登录。
	LoginTypeEmail = "email"
)

// LoginRequest 用户中心登录请求参数。
// 与前端 frontend-user/src/api/auth.ts 的 LoginParams 对齐。
//
// 兼容性：LoginType 为空时按 password 处理，Username/Password 仍走原有必填校验，
// 因此升级后旧前端不改也能登录（除非运营显式开启图形码/二次验证）。
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	// LoginType 登录方式：password（默认）/ sms / email（doc91 §5.3）。
	LoginType string `json:"login_type"`
	// Phone 短信登录时的手机号。
	Phone string `json:"phone"`
	// Email 邮箱登录时的邮箱。
	Email string `json:"email"`
	// Code 短信/邮箱登录时的一次性验证码。
	Code string `json:"code"`
	// CaptchaKey/CaptchaCode 图形验证码（场景策略要求时必填）。
	CaptchaKey  string `json:"captcha_key"`
	CaptchaCode string `json:"captcha_code"`
}

// RegisterRequest 用户中心注册请求参数。
// 与前端 frontend-user/src/api/auth.ts 的 RegisterParams 对齐。
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	Email    string `json:"email" binding:"required,email"`
	Phone    string `json:"phone"`
	// InviteCode 邀请码（可选）。携带时注册后绑定单级邀请关系，无效码不阻断注册。
	InviteCode string `json:"invite_code"`
	// CaptchaKey/CaptchaCode 图形验证码（user_register 场景要求时必填，doc91 §5.4）。
	CaptchaKey  string `json:"captcha_key"`
	CaptchaCode string `json:"captcha_code"`
	// EmailCode 注册邮箱验证码（user_register 场景 otp_required 时必填）。
	EmailCode string `json:"email_code"`
}

// UserInfo 用户中心用户信息，响应结构对齐前端
// frontend-user/src/api/auth.ts 中 LoginResponse.user 的字段定义。
type UserInfo struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"` // 显示名：优先取 real_name，为空时回退为 username
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Avatar   string `json:"avatar"`
	Role     string `json:"role"` // 用户中心角色，固定为 "user"
	Tier     string `json:"tier"` // 用户等级（标准用户 standard / 企业用户 business / 免费 free）
	Status   string `json:"status"`
	// —— 子账号信息（P4-03）——
	IsSubAccount bool   `json:"is_sub_account"`          // 是否子账号
	OwnerUserID  uint64 `json:"owner_user_id,omitempty"` // 子账号归属的主账号 ID
	OwnerName    string `json:"owner_name,omitempty"`    // 主账号用户名（子账号展示「XX 的子账号」）
	Remark       string `json:"remark,omitempty"`        // 子账号备注
	// Permissions 子账号已授予的客户侧权限码；主账号为空数组（前端据此隐藏入口，P4-09）。
	Permissions []string `json:"permissions"`
}

// LoginResponse 用户中心登录响应，结构对齐前端 LoginResponse：
// { token, user: { id, username, name, email, phone, avatar, role } }
//
// 二次验证（doc91 §4.6）：策略要求 OTP 时 NeedOTP=true 且不返回 token，
// 前端凭 OTPToken 调 /uc/auth/login/verify-otp 换正式令牌。字段可空，
// 旧前端按 token 是否存在判断即可。
type LoginResponse struct {
	Token string   `json:"token"`
	User  UserInfo `json:"user"`
	// NeedOTP 是否需要二次验证（为 true 时 Token 为空）。
	NeedOTP bool `json:"need_otp,omitempty"`
	// OTPToken 待验证令牌（aud=otp_pending，只能用一次）。
	OTPToken string `json:"otp_token,omitempty"`
	// OTPChannel 二次验证通道（email/sms）。
	OTPChannel string `json:"otp_channel,omitempty"`
	// OTPTargetMasked 二次验证目标打码值。
	OTPTargetMasked string `json:"otp_target_masked,omitempty"`
	// OTPExpireIn 待验证令牌有效期（秒）。
	OTPExpireIn int `json:"otp_expire_in,omitempty"`
}

// VerifyOTPRequest 登录二次验证请求（doc91 §4.6）。
type VerifyOTPRequest struct {
	OTPToken string `json:"otp_token" binding:"required"`
	Code     string `json:"code" binding:"required"`
}

// ForgotPasswordRequest 忘记密码：向账号绑定的邮箱/手机下发 OTP（doc91 §5.5）。
type ForgotPasswordRequest struct {
	Account string `json:"account" binding:"required"`
	// CaptchaKey/CaptchaCode 图形验证码（password_reset 场景要求时必填）。
	CaptchaKey  string `json:"captcha_key"`
	CaptchaCode string `json:"captcha_code"`
}

// ForgotPasswordResponse 固定返回 sent=true（防账号枚举，不回显目标）。
type ForgotPasswordResponse struct {
	Sent bool `json:"sent"`
}

// ResetPasswordRequest 重置密码：校验 OTP 后写新密码并撤销历史会话。
type ResetPasswordRequest struct {
	Account     string `json:"account" binding:"required"`
	Code        string `json:"code" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// UpdateProfileRequest 用户资料更新请求参数。
// 仅允许更新展示类字段（显示名、邮箱、手机、头像），用户名与密码走独立接口。
type UpdateProfileRequest struct {
	Name   string `json:"name"` // 显示名/真实姓名，写入 real_name 字段
	Email  string `json:"email" binding:"omitempty,email"`
	Phone  string `json:"phone"`
	Avatar string `json:"avatar"`
	// VerifyTicket 变更手机/邮箱时的二次验证票据（doc91 §6.3 动态判定）。
	//
	// 该路由同时改姓名/头像/邮箱/手机，无法用固定 scene 的中间件拦截；
	// 服务层比对旧值后按需消费 phone_bind / email_bind 的票据。
	VerifyTicket string `json:"verify_ticket"`
}

// ChangePasswordRequest 修改密码请求参数。
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"` // 旧密码，用于身份校验
	NewPassword string `json:"new_password" binding:"required"` // 新密码，强度由平台密码策略校验
}
