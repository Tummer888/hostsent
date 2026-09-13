package service

import (
	"context"
	"errors"
	"time"

	"hostsent/backend/internal/modules/uc/captcha/dto"
	appauth "hostsent/backend/internal/pkg/auth"
	"hostsent/backend/internal/pkg/captcha"
	"hostsent/backend/internal/pkg/passwordpolicy"
	"hostsent/backend/internal/pkg/security"
)

// SecurityPort 把验证码/OTP/登录锁定能力暴露给管理端与用户端认证服务（doc91 §5.1）。
//
// 实现内聚在本模块，但通过 internal/pkg/security 的中性端口对外，
// 使 admin/manager 与 uc/auth 都不必 import 本模块（避免 admin → uc 反向依赖）。
type SecurityPort struct {
	svc      *Service
	guard    *LoginGuard
	password *passwordpolicy.Policy
	jwtIssue *appauth.JWTIssuer
}

// NewSecurityPort 创建安全端口。
func NewSecurityPort(svc *Service, guard *LoginGuard, jwtIssue *appauth.JWTIssuer) *SecurityPort {
	p := &SecurityPort{svc: svc, guard: guard, jwtIssue: jwtIssue}
	if svc != nil {
		p.password = passwordpolicy.New(func(ctx context.Context, key string) string {
			return svc.switches.raw(ctx, key)
		})
	}
	return p
}

// 端口实现断言：编译期保证与 security.Port 一致。
var _ security.Port = (*SecurityPort)(nil)

// VerifyImage 校验图形码（一次性）。
//
// 返回 security.ErrCaptchaInvalid，让消费方用同一套错误码映射（doc91 §3.4）。
func (p *SecurityPort) VerifyImage(ctx context.Context, scene, captchaKey, captchaCode string) error {
	if p == nil || p.svc == nil {
		return nil
	}
	if err := p.svc.VerifyImage(ctx, scene, captchaKey, captchaCode); err != nil {
		if errors.Is(err, captcha.ErrInvalid) || errors.Is(err, captcha.ErrTooManyAttempts) {
			return security.ErrCaptchaInvalid
		}
		return err
	}
	return nil
}

// EffectiveImageRequired 场景是否要求图形码。
func (p *SecurityPort) EffectiveImageRequired(ctx context.Context, scene string, subject security.Subject) bool {
	if p == nil || p.svc == nil {
		return false
	}
	image, _, _ := p.svc.Policy().Required(ctx, scene, toSubject(subject))
	return image
}

// EffectiveOTP 场景是否要求二次验证及通道。
func (p *SecurityPort) EffectiveOTP(ctx context.Context, scene string, subject security.Subject) (bool, string) {
	if p == nil || p.svc == nil {
		return false, ""
	}
	_, otp, channel := p.svc.Policy().Required(ctx, scene, toSubject(subject))
	return otp, channel
}

// EffectiveScene 场景完整生效策略（公开接口 auth-config 用，不复用 Required，
// 因为 auth-config 需要在总闸关闭时也回显「预配置」状态）。
func (p *SecurityPort) EffectiveScene(ctx context.Context, scene string, subject security.Subject) security.EffectiveScene {
	out := security.EffectiveScene{Scene: scene}
	if p == nil || p.svc == nil {
		return out
	}
	eff, err := p.svc.Policy().EffectivePolicy(ctx, scene, toSubject(subject))
	if err != nil || eff == nil {
		return out
	}
	out.Name = eff.Name
	out.ImageRequired = eff.ImageRequired
	out.OTPRequired = eff.OTPRequired
	out.OTPChannel = eff.OTPChannel
	return out
}

// VerifyOTP 校验某目标的验证码（登录前场景）。
func (p *SecurityPort) VerifyOTP(ctx context.Context, scene, target, code string) error {
	if p == nil || p.svc == nil {
		return nil
	}
	if err := p.svc.VerifyCodeForTarget(ctx, scene, target, code); err != nil {
		if errors.Is(err, captcha.ErrInvalid) || errors.Is(err, captcha.ErrTooManyAttempts) {
			return security.ErrCaptchaInvalid
		}
		return err
	}
	return nil
}

// SendToTarget 向指定目标下发验证码（注册/找回密码等登录前场景）。
func (p *SecurityPort) SendToTarget(ctx context.Context, in security.SendInput) (*security.SendResult, error) {
	if p == nil || p.svc == nil {
		return &security.SendResult{Sent: false}, nil
	}
	resp, err := p.svc.SendCode(ctx, dto.SendCodeRequest{
		Scene:       in.Scene,
		Channel:     in.Channel,
		Target:      in.Target,
		CaptchaKey:  in.CaptchaKey,
		CaptchaCode: in.CaptchaCode,
	}, toSubject(in.Subject), 0, in.IP, in.UserAgent)
	if err != nil {
		return nil, translateError(err)
	}
	return &security.SendResult{
		Sent:         resp.Sent,
		Channel:      resp.Channel,
		TargetMasked: resp.TargetMasked,
		ExpireIn:     resp.ExpireIn,
		Cooldown:     resp.Cooldown,
	}, nil
}

// IssueOTPPending 下发二次验证码并发放待验证令牌。
func (p *SecurityPort) IssueOTPPending(ctx context.Context, scene string, subject security.Subject, ip, userAgent string) (*security.PendingResult, error) {
	if p == nil || p.svc == nil {
		return nil, errors.New("安全模块未启用")
	}
	target, channel, err := p.svc.ResolveTarget(ctx, scene, toSubject(subject))
	if err != nil {
		return nil, translateError(err)
	}
	// 复用 SendCode 的频控与落库链路：日志脱敏、验证码不落明文都在那一处保证。
	resp, err := p.svc.SendCode(ctx, dto.SendCodeRequest{
		Scene:   scene,
		Channel: channel,
		Target:  target,
	}, toSubject(subject), subject.ID, ip, userAgent)
	if err != nil {
		return nil, translateError(err)
	}
	token, err := p.svc.IssuePendingToken(ctx, toSubject(subject), scene)
	if err != nil {
		return nil, err
	}
	return &security.PendingResult{
		Token:        token,
		Channel:      resp.Channel,
		TargetMasked: resp.TargetMasked,
		ExpireIn:     int(OTPPendingTTL / time.Second),
	}, nil
}

// ConsumeOTPPending 校验待验证令牌 + 验证码。
func (p *SecurityPort) ConsumeOTPPending(ctx context.Context, token, code string) (*security.PendingUser, error) {
	if p == nil || p.svc == nil {
		return nil, security.ErrInvalidOTPToken
	}
	user, err := p.svc.ConsumePendingToken(ctx, token, code)
	if err != nil {
		if errors.Is(err, security.ErrInvalidOTPToken) {
			return nil, err
		}
		return nil, translateError(err)
	}
	return &security.PendingUser{UserID: user.UserID, Scene: user.Scene, IsAdmin: user.IsAdmin, Channel: user.Channel}, nil
}

// CheckLocked 登录失败锁定检查。
func (p *SecurityPort) CheckLocked(ctx context.Context, account, ip string) error {
	if p == nil || p.guard == nil {
		return nil
	}
	if err := p.guard.CheckLocked(ctx, account, ip); err != nil {
		var locked *ErrLoginLocked
		if errors.As(err, &locked) {
			return &security.ErrLoginLocked{RetryAfter: locked.RetryAfter, Scope: locked.Scope}
		}
		return err
	}
	return nil
}

// RecordFailure 记录一次登录失败。
func (p *SecurityPort) RecordFailure(ctx context.Context, account, ip string) {
	if p == nil || p.guard == nil {
		return
	}
	p.guard.RecordFailure(ctx, account, ip)
}

// ResetFailure 登录成功后清零失败计数。
func (p *SecurityPort) ResetFailure(ctx context.Context, account, ip string) {
	if p == nil || p.guard == nil {
		return
	}
	p.guard.ResetFailure(ctx, account, ip)
}

// Log 写一条登录日志。
func (p *SecurityPort) Log(ctx context.Context, entry security.LoginLogEntry) {
	if p == nil || p.guard == nil {
		return
	}
	p.guard.Log(ctx, LoginLogEntry{
		UserID:        entry.UserID,
		Username:      entry.Username,
		LoginType:     entry.LoginType,
		Result:        entry.Result,
		FailureReason: entry.FailureReason,
		IP:            entry.IP,
		IPRegion:      entry.IPRegion,
		UserAgent:     entry.UserAgent,
		Platform:      entry.Platform,
	})
}

// RegisterEnabled 注册开关。
func (p *SecurityPort) RegisterEnabled(ctx context.Context) bool {
	if p == nil || p.svc == nil {
		return true
	}
	return p.svc.Switches().RegisterEnabled(ctx)
}

// ValidatePassword 校验平台密码策略（6 个既有开关的接线点，doc91 §9.1）。
func (p *SecurityPort) ValidatePassword(ctx context.Context, password string) error {
	if p == nil || p.password == nil {
		return passwordpolicy.Default().Validate(ctx, password)
	}
	return p.password.Validate(ctx, password)
}

// ConsumeVerifyTicket 校验并消费关键操作票据（供 profile 动态判定等场景调用）。
func (p *SecurityPort) ConsumeVerifyTicket(ctx context.Context, scene string, subject security.Subject, ticket string) bool {
	if p == nil || p.svc == nil {
		return true
	}
	return p.svc.ConsumeVerifyTicket(ctx, scene, toSubject(subject), ticket)
}

// MarkVerified 写回绑定验证时间。
func (p *SecurityPort) MarkVerified(ctx context.Context, userID uint64, channel string) error {
	if p == nil || p.svc == nil || p.svc.resolver == nil {
		return nil
	}
	return p.svc.resolver.SetVerifiedAt(ctx, userID, channel, time.Now())
}

// LookupTarget 按账号解析验证码目标（找不到返回 Exists=false，不报错）。
func (p *SecurityPort) LookupTarget(ctx context.Context, account string) (*security.Target, error) {
	if p == nil || p.svc == nil || p.svc.resolver == nil {
		return &security.Target{}, nil
	}
	info, err := p.svc.resolver.ResolveUser(ctx, 0, account)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return &security.Target{}, nil
	}
	return &security.Target{
		UserID:        info.UserID,
		Phone:         info.Phone,
		PhoneVerified: info.PhoneVerified,
		Email:         info.Email,
		EmailVerified: info.EmailVerified,
		Username:      info.Username,
		Exists:        info.Exists,
	}, nil
}

// toSubject 中性主体 → 本模块主体。
func toSubject(s security.Subject) Subject {
	return Subject{IsAdmin: s.IsAdmin, ID: s.ID}
}

// TranslateError 把本模块错误翻译为中性安全域错误（装配层在注入 Port 实现后，
// 供 admin/manager 与 uc/auth 的 handler 统一取错误码；避免它们 import 本模块）。
func TranslateError(err error) error { return translateError(err) }

// translateError 把本模块错误翻译为中性安全域错误（保持 errors.Is 可判定）。
func translateError(err error) error {
	if err == nil {
		return nil
	}
	var limited *ErrRateLimited
	if errors.As(err, &limited) {
		return &security.ErrRateLimited{RetryAfter: limited.RetryAfter, Daily: limited.Daily}
	}
	switch {
	case errors.Is(err, ErrTargetUnbound):
		return security.ErrTargetUnbound
	case errors.Is(err, ErrSceneDisabled):
		return security.ErrSceneDisabled
	case errors.Is(err, captcha.ErrInvalid), errors.Is(err, captcha.ErrTooManyAttempts):
		return security.ErrCaptchaInvalid
	default:
		return err
	}
}
