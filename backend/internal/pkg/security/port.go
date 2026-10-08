// Package security 定义「登录与关键操作安全」的跨模块端口（doc91 §5.1/§6.1）。
//
// 背景：验证码/OTP/登录锁定由 uc/captcha 实现，但消费方是管理端认证（admin/manager）
// 与用户端认证（uc/auth）。若让二者直接 import uc/captcha，会形成 admin → uc 的
// 反向依赖（本仓库既有约定是 uc → admin，反向仅此一处会破坏方向）。
//
// 解法：把端口与中性类型收在本包，实现方与消费方都只依赖本包。
//   - 实现：internal/modules/uc/captcha/service.Service
//   - 消费：admin/manager/service、uc/auth/service
package security

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Subject 策略解算主体：用户或管理员。
type Subject struct {
	// IsAdmin true 时设置读 admins 表扩展列，否则读 user_security_settings。
	IsAdmin bool
	ID      uint64
}

// 场景常量（与 uc/captcha/model 的 Scene* 一一对应，冻结于 doc91 §1.2.2）。
//
// 放在本包是为了让消费方（admin/manager、uc/auth）不必 import 验证码模块即可
// 引用场景名；两侧若改动场景名，编译器不会发现，因此这里逐字保持与 doc91 一致。
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

// 通道常量（与 uc/captcha/model 的 Channel* 一致）。
const (
	ChannelEmail = "email"
	ChannelSMS   = "sms"
)

// 业务错误码（doc91 §3.4/§4.1/§4.5）：与前端约定一致。
const (
	CodeCaptchaInvalid   = 20010 // 验证码错误或已过期（不区分两者）
	CodeSendTooFrequent  = 20011 // 发送过于频繁
	CodeSendDailyLimit   = 20012 // 超出每日/每小时上限
	CodeImageRateLimited = 20013 // 图形码下发频率超限（HTTP 429）
	CodeLoginLocked      = 20014 // 登录失败次数过多，账号锁定
	CodeTargetUnbound    = 20015 // 账号未绑定手机/邮箱
	CodeRegisterDisabled = 20016 // 注册已关闭
	CodeNeedVerification = 20017 // 关键操作需要二次验证
	// CodeSceneDisabled 场景策略不可用/参数错误（与既有模块口径一致）。
	CodeSceneDisabled = 20001
)

// 业务错误（用 errors.Is 判定，具体中文提示由 Error() 给出）。
var (
	// ErrTargetUnbound 目标未绑定。
	ErrTargetUnbound = errors.New("账号未绑定对应的手机号或邮箱")
	// ErrSceneDisabled 场景策略不可用。
	ErrSceneDisabled = errors.New("该场景验证策略不可用")
	// ErrRegisterDisabled 注册开关已关闭。
	ErrRegisterDisabled = errors.New("注册已关闭，请联系管理员")
	// ErrInvalidOTPToken 待验证令牌无效（aud 不符 / 已用 / 已过期）。
	ErrInvalidOTPToken = errors.New("登录验证已失效，请重新登录")
	// ErrNeedVerification 关键操作缺少有效验证票据。
	ErrNeedVerification = errors.New("该操作需要二次验证")
	// ErrInvalidCredential 账号或密码错误（不暴露账号是否存在）。
	ErrInvalidCredential = errors.New("用户名或密码错误")
	// ErrPasswordPolicy 新密码不满足平台密码策略（提示语由策略包给出）。
	ErrPasswordPolicy = errors.New("密码强度不符合要求")
	// ErrLoginDisabled 账号已禁用。
	ErrLoginDisabled = errors.New("账号已被禁用")
)

// ErrRateLimited 频控命中。
type ErrRateLimited struct {
	// RetryAfter 剩余秒数（间隔锁）。
	RetryAfter int
	// Daily 是否为日限/小时限（与间隔锁区分提示语）。
	Daily bool
}

func (e *ErrRateLimited) Error() string {
	if e.Daily {
		return "今日验证码发送次数已达上限，请稍后再试"
	}
	return fmt.Sprintf("发送过于频繁，请 %d 秒后重试", e.RetryAfter)
}

// ErrLoginLocked 账号或 IP 已锁定。
type ErrLoginLocked struct {
	// RetryAfter 剩余锁定秒数。
	RetryAfter int
	// Scope 锁定维度：account / ip。
	Scope string
}

func (e *ErrLoginLocked) Error() string {
	return fmt.Sprintf("登录失败次数过多，账号已锁定，请 %d 分钟后再试", (e.RetryAfter+59)/60)
}

// CodeOf 把安全域错误翻译为业务错误码；非本域错误返回 0。
//
// 消费方（handler）据此返回统一响应，避免每个 handler 各写一份类型断言。
func CodeOf(err error) int {
	if err == nil {
		return 0
	}
	var limited *ErrRateLimited
	var locked *ErrLoginLocked
	var blocked *ErrBlacklisted
	switch {
	case errors.As(err, &blocked):
		return CodeBlacklisted
	case errors.As(err, &limited):
		if limited.Daily {
			return CodeSendDailyLimit
		}
		return CodeSendTooFrequent
	case errors.As(err, &locked):
		return CodeLoginLocked
	case errors.Is(err, ErrTargetUnbound):
		return CodeTargetUnbound
	case errors.Is(err, ErrSceneDisabled):
		return CodeSceneDisabled
	case errors.Is(err, ErrRegisterDisabled):
		return CodeRegisterDisabled
	case errors.Is(err, ErrNeedVerification), errors.Is(err, ErrInvalidOTPToken):
		return CodeNeedVerification
	case errors.Is(err, ErrCaptchaInvalid), errors.Is(err, ErrTooManyAttempts):
		return CodeCaptchaInvalid
	default:
		return 0
	}
}

// 图形码错误（由内部 pkg/captcha 返回，这里重新声明避免本包依赖实现）。
var (
	// ErrCaptchaInvalid 图形码错误或已过期。
	ErrCaptchaInvalid = errors.New("验证码错误或已过期")
	// ErrTooManyAttempts 校验次数超限。
	ErrTooManyAttempts = errors.New("验证码错误次数过多，请重新获取")
)

// 登录结果（与 login_logs.result 口径一致）。
const (
	LoginResultSuccess = "success"
	LoginResultFailed  = "failed"
)

// 登录主体域（与 login_logs.subject_type / user_sessions.subject_type 口径一致）。
const (
	SubjectTypeUser  = "user"
	SubjectTypeAdmin = "admin"
)

// LoginLogEntry 一条登录日志。
type LoginLogEntry struct {
	UserID        uint64
	Username      string
	LoginType     string
	Result        string
	FailureReason string
	IP            string
	UserAgent     string
	Platform      string
	// DeviceFingerprint 客户端上报的设备指纹（可空）。
	//
	// 落 login_logs 有两个用途，缺一即两条能力同时失效：
	//   - 风险规则的「设备变更」判定要拿历史成功登录的设备做基线；
	//   - 黑名单按设备类型拉黑时，「命中记录」正是按这一列关联的。
	DeviceFingerprint string
	// SubjectType 登录主体域（login_logs.subject_type）：空值按客户域落库。
	// 管理端登录必须显式传 admin —— login_logs.user_id 承载两个 ID 空间的值
	// （users.id 与 admins.id），不区分域的话员工后台登录会被算到撞号的客户头上。
	SubjectType string
}

// —— 风控（异常行为监控 + 黑名单）中性类型 ——

// 黑名单类型（与 blacklists.type 口径一致，doc06 §4.4）。
//
// phone / email 是 doc06 要求而此前前端枚举缺失的两类：短信/邮箱验证码登录
// 的账号目标就是手机号与邮箱，撞库时封 IP 会连带正常用户，封手机号/邮箱才精准。
const (
	BlacklistTypeIP     = "ip"
	BlacklistTypeUser   = "user"
	BlacklistTypeDevice = "device"
	BlacklistTypePhone  = "phone"
	BlacklistTypeEmail  = "email"
)

// 黑名单命中时的业务错误码（沿用安全域码段 200xx，前端按提示语渲染）。
const (
	// CodeBlacklisted 命中黑名单，拒绝本次登录。
	CodeBlacklisted = 20018
)

// ErrBlacklisted 命中黑名单。
type ErrBlacklisted struct {
	// Scope 命中的维度：ip / user / device / phone / email。
	Scope string
	// Reason 拉黑原因（可空，运营填写的原文）。
	Reason string
}

func (e *ErrBlacklisted) Error() string {
	if e == nil {
		return "该账号或来源已被限制登录"
	}
	// 不把 reason 直接拼进面向用户的提示：那是运营写的内部备注（可能含
	// 「撞库」「盗号」这类判断），回给攻击者等于免费的情报。
	return "该账号或来源已被限制登录，如有疑问请联系客服"
}

// 登录风险信号：一次登录尝试的时点特征。
//
// 判定放在实现侧（captcha 模块的 LoginSignals），消费方（uc/auth、admin/manager）
// 只负责采集并透传。刻意不传密码、验证码等敏感字段 —— 信号是给规则引擎做统计的，
// 不是登录凭据的副本。
type LoginSignal struct {
	// Username 登录账号名。
	Username string
	// UserID 解析到的用户/管理员 ID；账号不存在时为 0。
	//
	// 为 0 时仍然要上报 —— 撞库正是「大批不存在的账号」这种形态，
	// 拿不到 user_id 只是没法归因到某个客户，IP 维度的规则照样要生效。
	UserID uint64
	// IP 客户端地址（已由 netutil.ClientIP 解析过可信代理想象）。
	IP string
	// DeviceFingerprint 设备指纹；前端未上报时为空，此时设备类规则整体跳过
	// （拿空串当指纹会让所有无指纹的登录互相「设备变更」，是纯粹的误报源）。
	DeviceFingerprint string
	// SubjectType user / admin（见 SubjectType* 常量）。
	SubjectType string
	// LoginType password / sms / email / oauth / impersonate…
	LoginType string
	// Success 本次是否登录成功。
	Success bool
	// FailureReason 失败原因（bad_password / user_not_found / captcha / locked…）。
	FailureReason string
	// UserAgent 原始 UA（规则引擎按 UA 变化辅助判断异常设备）。
	UserAgent string
	// At 本次尝试的起始时刻。
	//
	// 规则引擎的「历史常用设备 / 常用 IP」必须只统计**这次尝试之前**的记录：
	// 登录日志是在判定之前落库的（这样失败计数才能把本次算进去，与「失败 5 次
	// 就报警」的直觉一致），因此历史查询必须带上时间上界，否则本次登录自己写的
	// 那行会立刻进入「常用」，设备变更规则永远不会触发。
	At time.Time
}

// BlacklistHit 一次黑名单命中的上报（供实现侧累加 hit_count 并留痕）。
type BlacklistHit struct {
	// Type 命中的黑名单类型。
	Type string
	// TargetValue 命中的命中值。
	TargetValue string
	// Username 触发本次命中的账号（留痕用）。
	Username string
	// IP 触发时的客户端地址。
	IP string
	// DeviceFingerprint 触发时的设备指纹。
	DeviceFingerprint string
}

// BlacklistChecker 黑名单校验能力（由 admin/user/security 的黑名单守卫实现）。
//
// 单独抽一个接口而不是塞进 Port：实现方不同（Port 由 uc/captcha 实现，
// 本接口由 admin 侧实现），装配层用组合适配器把两者拼成一个 Port，
// 这样 uc/captcha 不需要 import admin（该模块的自我约束是「不依赖任何业务模块」）。
type BlacklistChecker interface {
	Check(ctx context.Context, account, ip, deviceFingerprint string) error
}

// LoginObserver 登录信号观察者（由 admin/user/security 的风险规则引擎实现）。
//
// Observe 必须**永不返回错误**也不需要返回值：它是旁路的，规则引擎的任何
// 失败都不该让登录失败。这一点在接口签名上直接表达出来。
type LoginObserver interface {
	Observe(ctx context.Context, sig LoginSignal)
}

// Target 账号的验证码接收目标。
type Target struct {
	// UserID 解析到的用户 ID（找不到账号时为 0），供重置密码等场景直接使用。
	UserID        uint64
	Phone         string
	PhoneVerified bool
	Email         string
	EmailVerified bool
	Username      string
	Exists        bool
}

// SendInput 向指定目标下发验证码的入参。
type SendInput struct {
	Scene       string
	Channel     string // email / sms
	Target      string
	Subject     Subject
	CaptchaKey  string
	CaptchaCode string
	// SkipCaptcha true 表示调用方已在业务层完成图形码校验（注册/找回密码内部调用）。
	SkipCaptcha bool
	IP          string
	UserAgent   string
}

// SendResult 下发结果（target 只回打码值）。
type SendResult struct {
	Sent         bool
	Channel      string
	TargetMasked string
	ExpireIn     int
	Cooldown     int
}

// PendingResult 登录二次验证待验证令牌。
type PendingResult struct {
	Token        string
	Channel      string
	TargetMasked string
	ExpireIn     int
}

// PendingUser 待验证令牌校验通过后的身份。
type PendingUser struct {
	UserID  uint64
	Scene   string
	IsAdmin bool
	// Channel 本次二次验证使用的通道（email/sms）。
	Channel string
}

// EffectiveScene 某场景的生效策略（前端渲染 + 登录链路判定）。
type EffectiveScene struct {
	Scene         string
	Name          string
	ImageRequired bool
	OTPRequired   bool
	OTPChannel    string
}

// Port 登录与关键操作安全端口（由 uc/captcha service 实现）。
//
// 所有方法在「安全功能未启用」时必须退化为放行/正常返回，绝不阻断登录：
// 默认 captcha_enabled=false，升级当天行为与升级前一致（doc91 硬约束）。
type Port interface {
	// VerifyImage 校验图形码；一次性，失败累计。
	VerifyImage(ctx context.Context, scene, captchaKey, captchaCode string) error
	// EffectiveImageRequired 场景是否要求图形码。
	EffectiveImageRequired(ctx context.Context, scene string, subject Subject) bool
	// EffectiveOTP 场景是否要求二次验证，以及使用的通道。
	EffectiveOTP(ctx context.Context, scene string, subject Subject) (required bool, channel string)
	// EffectiveScene 场景完整生效策略（公开接口 auth-config 与登录链路共用）。
	EffectiveScene(ctx context.Context, scene string, subject Subject) EffectiveScene
	// VerifyOTP 校验某目标的验证码（登录短信/邮箱、注册、找回密码等登录前场景）。
	VerifyOTP(ctx context.Context, scene, target, code string) error
	// SendToTarget 向指定目标下发验证码（注册/找回密码等登录前场景）。
	SendToTarget(ctx context.Context, in SendInput) (*SendResult, error)
	// IssueOTPPending 下发二次验证码并发放待验证令牌。
	IssueOTPPending(ctx context.Context, scene string, subject Subject, ip, userAgent string) (*PendingResult, error)
	// ConsumeOTPPending 校验待验证令牌 + 验证码，成功返回身份。
	ConsumeOTPPending(ctx context.Context, token, code string) (*PendingUser, error)
	// ConsumeVerifyTicket 校验并消费关键操作票据（一次）。
	//
	// 供无法挂中间件的动态判定场景使用：`PUT /uc/profile` 的改手机/改邮箱
	// 需要按「哪个字段变了」选场景，只能在服务层判定（doc91 §6.3）。
	ConsumeVerifyTicket(ctx context.Context, scene string, subject Subject, ticket string) bool
	// CheckLocked 登录失败锁定检查；锁定时返回 *ErrLoginLocked。
	CheckLocked(ctx context.Context, account, ip string) error
	// RecordFailure 记录一次登录失败。
	RecordFailure(ctx context.Context, account, ip string)
	// ResetFailure 登录成功后清零失败计数。
	ResetFailure(ctx context.Context, account, ip string)
	// Log 写一条登录日志（成功与失败都要写）。
	Log(ctx context.Context, entry LoginLogEntry)
	// RegisterEnabled 注册开关。
	RegisterEnabled(ctx context.Context) bool
	// ValidatePassword 校验新密码是否满足平台密码策略。
	ValidatePassword(ctx context.Context, password string) error
	// MarkVerified 写回绑定验证时间（注册/绑定成功后调用）。
	MarkVerified(ctx context.Context, userID uint64, channel string) error
	// LookupTarget 按账号（用户名/邮箱/手机号）解析验证码目标。
	LookupTarget(ctx context.Context, account string) (*Target, error)
}
