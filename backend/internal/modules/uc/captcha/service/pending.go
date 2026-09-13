package service

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"hostsent/backend/internal/modules/uc/captcha/model"
	appauth "hostsent/backend/internal/pkg/auth"
	captchapkg "hostsent/backend/internal/pkg/captcha"
	"hostsent/backend/internal/pkg/security"
)

// OTPPendingTTL 登录二次验证待验证令牌有效期（doc91 §4.6）。
const OTPPendingTTL = appauth.OTPPendingTTL

// PendingUserInfo 待验证令牌校验通过后的身份。
type PendingUserInfo struct {
	UserID  uint64
	Scene   string
	IsAdmin bool
	// Channel 本次二次验证使用的通道（登录成功后写 login_logs 用）。
	Channel string
}

// ResolveTarget 解析主体的 OTP 目标与通道（按生效策略选通道）。
//
// 目标不存在时返回 ErrTargetUnbound（调用方映射为 20015）。
func (s *Service) ResolveTarget(ctx context.Context, scene string, subject Subject) (string, string, error) {
	target, channel, _, err := s.resolveTargetWithChannel(ctx, scene, subject)
	return target, channel, err
}

// resolveTargetWithChannel 与 ResolveTarget 同逻辑，额外返回解析到的用户 ID
// （改密/重置等场景需要把会话与账号对应起来）。
func (s *Service) resolveTargetWithChannel(ctx context.Context, scene string, subject Subject) (string, string, uint64, error) {
	if s.resolver == nil {
		return "", "", 0, ErrTargetUnbound
	}
	var info *TargetInfo
	var err error
	if subject.IsAdmin {
		info, err = s.resolver.ResolveAdmin(ctx, subject.ID, "")
	} else {
		info, err = s.resolver.ResolveUser(ctx, subject.ID, "")
	}
	if err != nil || info == nil || !info.Exists {
		return "", "", 0, ErrTargetUnbound
	}
	channel := model.ChannelEmail
	if eff, perr := s.policy.EffectivePolicy(ctx, scene, subject); perr == nil && eff != nil && eff.OTPChannel != "" {
		channel = eff.OTPChannel
	}
	if channel == model.ChannelSMS {
		if strings.TrimSpace(info.Phone) == "" {
			return "", "", 0, ErrTargetUnbound
		}
		return info.Phone, channel, info.UserID, nil
	}
	if strings.TrimSpace(info.Email) == "" {
		return "", "", 0, ErrTargetUnbound
	}
	return info.Email, channel, info.UserID, nil
}

// IssuePendingToken 生成登录二次验证待验证令牌，并在缓存中登记 jti。
//
// 令牌只承载身份与场景，不含任何权限；jti 登记使令牌只能用一次（doc91 §4.6）。
func (s *Service) IssuePendingToken(ctx context.Context, subject Subject, scene string) (string, error) {
	if s.jwtIssuer == nil {
		return "", errors.New("待验证令牌签发器未配置")
	}
	// jti 用与 verify ticket 同源的随机实现（32 字节 hex，不可枚举）。
	jti := randomTicket()
	if jti == "" {
		return "", errors.New("生成待验证令牌标识失败")
	}
	if s.cache != nil {
		if err := s.cache.Set(ctx, OTPPendingKey(jti), encodePendingClaims(subject, scene), OTPPendingTTL); err != nil {
			return "", err
		}
	}
	return s.jwtIssuer.GenerateOTPPending(subject.ID, scene, subject.IsAdmin, jti)
}

// ConsumePendingToken 校验待验证令牌 + 验证码，成功即销毁 jti。
func (s *Service) ConsumePendingToken(ctx context.Context, token, code string) (*PendingUserInfo, error) {
	if s.jwtIssuer == nil {
		return nil, security.ErrInvalidOTPToken
	}
	// aud 校验是防越权的关键：普通访问令牌在此被拒（doc91 §4.6）。
	claims, err := s.jwtIssuer.ParseOTPPending(strings.TrimSpace(token))
	if err != nil {
		return nil, security.ErrInvalidOTPToken
	}
	if s.cache != nil && s.cache.Enabled() {
		raw, ok := s.cache.Get(ctx, OTPPendingKey(claims.ID))
		if !ok {
			// jti 不存在或已被消费：令牌已用过。
			return nil, security.ErrInvalidOTPToken
		}
		if uid, _, _, ok := decodePendingClaims(raw); !ok || uid != claims.UserID {
			return nil, security.ErrInvalidOTPToken
		}
	}
	subject := Subject{IsAdmin: claims.IsAdmin, ID: claims.UserID}
	target, channel, _, terr := s.resolveTargetWithChannel(ctx, claims.Scene, subject)
	if terr != nil {
		return nil, terr
	}
	if verr := s.consumeCode(ctx, claims.Scene, target, code); verr != nil {
		return nil, translateCaptchaError(verr)
	}
	// 验证码正确才销毁 jti：验证码输错不应导致整个登录流程重来。
	if s.cache != nil && s.cache.Enabled() {
		_ = s.cache.Del(ctx, OTPPendingKey(claims.ID))
	}
	return &PendingUserInfo{UserID: claims.UserID, Scene: claims.Scene, IsAdmin: claims.IsAdmin, Channel: channel}, nil
}

// translateCaptchaError 把底层 pkg/captcha 错误翻译为中性安全域错误。
func translateCaptchaError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, captchapkg.ErrInvalid) || errors.Is(err, captchapkg.ErrTooManyAttempts) {
		return security.ErrCaptchaInvalid
	}
	return err
}

// encodePendingClaims 序列化 jti 载荷（uid|is_admin|scene）。
func encodePendingClaims(subject Subject, scene string) string {
	isAdmin := "0"
	if subject.IsAdmin {
		isAdmin = "1"
	}
	return strconv.FormatUint(subject.ID, 10) + "|" + isAdmin + "|" + scene
}

// decodePendingClaims 解析 jti 载荷。
func decodePendingClaims(raw string) (uint64, bool, string, bool) {
	parts := strings.SplitN(raw, "|", 3)
	if len(parts) != 3 {
		return 0, false, "", false
	}
	id, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, false, "", false
	}
	return id, parts[1] == "1", parts[2], true
}
