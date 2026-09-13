package adapter

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"hostsent/backend/internal/modules/uc/captcha/service"
	"hostsent/backend/internal/pkg/notifier"
)

// ChannelResolver 发送侧渠道路由（doc90 §3.2），按类别 + 场景返回已解密配置。
//
// 本包只声明接口：实现由消息中心 ChannelService 提供，装配层注入，
// 避免 captcha 模块反向 import admin/notification。
type ChannelResolver interface {
	Resolve(ctx context.Context, category, scene string) (*notifier.ChannelConfig, error)
}

// ChannelSender 把 OTP 下发接到消息中心渠道表。
//
// 这是 doc91 §5.1 登记的唯一「切换点」的实现：Service.Sender 接口不变，
// 由「直读 system_configs」换成「渠道表优先、历史 SMTP 兜底」，
// 调用方（captcha_service.go）一行不改。
//
// 规则：
//   - 邮件：优先 notification_channels（category=mail，场景 otp 优先、默认渠道兜底）；
//     渠道未配置或类型未接入时回退历史 SMTP 键，保证既有部署升级后邮件验证码照常下发。
//     渠道已解析但发送失败（服务商/网络问题）不回退——回退会掩盖真实故障并可能重复投递。
//   - 短信：只走渠道表；`sms_channel_enabled` 未显式开启时拒绝下发（默认安全侧，
//     升级后不会突然开始发短信）。
type ChannelSender struct {
	resolver ChannelResolver
	// legacy 历史 SMTP 直发实现，仅作为邮件路径的配置级兜底。
	legacy service.Sender
	// cfg 读 system_configs 原文，用于判断短信总开关。
	cfg service.ConfigValueReader
}

// NewChannelSender 创建渠道路由发送器。
func NewChannelSender(resolver ChannelResolver, legacy service.Sender, cfg service.ConfigValueReader) *ChannelSender {
	return &ChannelSender{resolver: resolver, legacy: legacy, cfg: cfg}
}

// SendMail 下发验证码邮件（HTML 正文）。
func (s *ChannelSender) SendMail(ctx context.Context, to, subject, body string) error {
	if s.resolver != nil {
		if cfg, err := s.resolver.Resolve(ctx, notifier.CategoryMail, sceneOTP); err == nil {
			if sender, nerr := notifier.New(cfg.Type, *cfg); nerr == nil {
				_, serr := sender.Send(ctx, *cfg, notifier.Message{
					Category:  notifier.CategoryMail,
					Recipient: to,
					Subject:   subject,
					Body:      service.RenderMailBody(subject, body),
					Format:    notifier.FormatHTML,
				})
				return serr
			}
			// 走到这里说明渠道类型没有接入实现：回退历史 SMTP 而不是直接失败。
		}
		return s.legacySendMail(ctx, to, subject, body)
	}
	return s.legacySendMail(ctx, to, subject, body)
}

// SendSMS 下发验证码短信（渠道表）。
func (s *ChannelSender) SendSMS(ctx context.Context, target, content string) error {
	if !s.smsSwitchOn(ctx) {
		return fmt.Errorf("%w: 短信通道总开关未开启（系统设置 → 短信）", notifier.ErrChannelDisabled)
	}
	if s.resolver == nil {
		return notifier.ErrSMSNotConfigured
	}
	cfg, err := s.resolver.Resolve(ctx, notifier.CategorySMS, sceneOTP)
	if err != nil {
		return fmt.Errorf("%w: 请先在消息中心配置可用的短信渠道", notifier.ErrSMSNotConfigured)
	}
	sender, err := notifier.New(cfg.Type, *cfg)
	if err != nil {
		if errors.Is(err, notifier.ErrAdapterNotImplemented) {
			return fmt.Errorf("%w: 短信服务商[%s]适配器待接入", notifier.ErrChannelDisabled, cfg.Type)
		}
		return err
	}
	_, err = sender.Send(ctx, *cfg, notifier.Message{
		Category:   notifier.CategorySMS,
		Recipient:  target,
		Body:       content,
		Format:     notifier.FormatText,
		SignName:   cfg.SignName,
		TemplateID: cfg.TemplateCode,
	})
	return err
}

// legacySendMail 回退历史 SMTP 直发（未接渠道表的既有部署）。
func (s *ChannelSender) legacySendMail(ctx context.Context, to, subject, body string) error {
	if s.legacy == nil {
		return fmt.Errorf("%w: 邮件通道未启用，请在消息中心配置邮件渠道", notifier.ErrChannelDisabled)
	}
	return s.legacy.SendMail(ctx, to, subject, body)
}

// smsSwitchOn 短信总开关：缺失或非 true 一律视为未开启（默认不发送）。
func (s *ChannelSender) smsSwitchOn(ctx context.Context) bool {
	if s.cfg == nil {
		return false
	}
	v, ok, err := s.cfg(ctx, "sms_channel_enabled")
	if err != nil || !ok {
		return false
	}
	v = strings.TrimSpace(v)
	return v == "true" || v == "1"
}

// sceneOTP 渠道场景：验证码（强制送达，不受用户偏好影响）。
const sceneOTP = "otp"
