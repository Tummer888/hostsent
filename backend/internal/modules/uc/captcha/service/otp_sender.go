package service

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	notifierpkg "hostsent/backend/internal/pkg/notifier"
	// 空导入触发 smtp provider 的 init() 注册（描述符 + 工厂）。
	_ "hostsent/backend/internal/pkg/notifier/provider/smtp"
)

// ConfigValueReader 读取系统配置原文的最小端口（避免本包依赖 system 仓储类型）。
// 约定：键不存在返回 ("", false, nil)，不报错。
type ConfigValueReader func(ctx context.Context, key string) (string, bool, error)

// directOTPSender 最小发送实现（邮件兜底路径）：
//   - 邮件：直接读 system_configs 的既有 SMTP 6 键，复用 notifier 的 smtp provider
//     （中文主题 Q 编码 + HTML base64 传输编码，doc90 §2.4）；
//   - 短信：返回 notifier.ErrSMSNotConfigured。
//
// **doc91 与 doc90 之间唯一登记的切换点**：captchaBundle 注入的 Sender 来源。
// doc90 落地渠道表后，装配层用 adapter.ChannelSender 包住本实现——邮件优先走
// notification_channels、渠道缺失时回退到这里的 SMTP 6 键，短信只走渠道表。
// Service 侧接口与调用代码不变（doc91 §5.1）。
type directOTPSender struct {
	db  *gorm.DB
	cfg ConfigValueReader
}

// NewDirectOTPSender 创建直发实现。
func NewDirectOTPSender(db *gorm.DB, cfg ConfigValueReader) Sender {
	return &directOTPSender{db: db, cfg: cfg}
}

// SMTPConfig 从 system_configs 读出的 SMTP 配置。
type SMTPConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
	Enabled  bool
}

func (s *directOTPSender) loadSMTPConfig(ctx context.Context) (*SMTPConfig, error) {
	cfg := &SMTPConfig{}
	if s.cfg == nil {
		return cfg, nil
	}
	keys := map[string]*string{
		"smtp_host": &cfg.Host,
		"smtp_port": &cfg.Port,
		"smtp_user": &cfg.User,
		"smtp_pass": &cfg.Password,
		"smtp_from": &cfg.From,
	}
	for key, ptr := range keys {
		if v, ok, err := s.cfg(ctx, key); err == nil && ok {
			*ptr = v
		}
	}
	if v, ok, err := s.cfg(ctx, "mail_channel_enabled"); err == nil && ok {
		cfg.Enabled = strings.TrimSpace(v) == "true"
	}
	if cfg.Host == "" || cfg.Port == "" || cfg.From == "" {
		cfg.Enabled = false
	}
	return cfg, nil
}

// SendMail 通过 SMTP 下发验证码邮件（HTML 正文）。
func (s *directOTPSender) SendMail(ctx context.Context, to, subject, body string) error {
	cfg, err := s.loadSMTPConfig(ctx)
	if err != nil {
		return fmt.Errorf("读取 SMTP 配置失败: %w", err)
	}
	if !cfg.Enabled {
		return fmt.Errorf("%w: 邮件通道未启用，请先在系统设置中配置 SMTP", notifierpkg.ErrChannelDisabled)
	}
	// 复用 smtp provider：中文主题编码与 base64 传输编码都在其中实现。
	sender, err := notifierpkg.New("smtp", notifierpkg.ChannelConfig{Type: "smtp"})
	if err != nil {
		return err
	}
	_, err = sender.Send(ctx, notifierpkg.ChannelConfig{
		Type:   "smtp",
		Sender: cfg.From,
		Credentials: map[string]string{
			"host":     cfg.Host,
			"port":     cfg.Port,
			"username": cfg.User,
			"password": cfg.Password,
			"from":     cfg.From,
		},
	}, notifierpkg.Message{
		Category:  notifierpkg.CategoryMail,
		Recipient: to,
		Subject:   subject,
		Body:      RenderMailBody(subject, body),
		Format:    notifierpkg.FormatHTML,
	})
	return err
}

// SendSMS doc91 阶段短信通道未落地：返回明确错误（doc90 完成后由 ChannelResolver 接管）。
func (s *directOTPSender) SendSMS(ctx context.Context, target, content string) error {
	return notifierpkg.ErrSMSNotConfigured
}

// RenderMailBody 邮件正文 HTML 外壳（直发与渠道路由发送器共用同一份观感）。
func RenderMailBody(subject, body string) string {
	return `<!DOCTYPE html><html><body style="font-family:-apple-system,'Segoe UI',Roboto,sans-serif;color:#1f2937;">
<h3 style="margin:0 0 12px;font-size:16px;">` + escapeHTML(subject) + `</h3>
<p style="margin:0 0 8px;font-size:14px;line-height:1.6;">` + escapeHTML(body) + `</p>
<p style="margin:16px 0 0;font-size:12px;color:#9ca3af;">本邮件由系统自动发送，请勿回复。</p>
</body></html>`
}

func escapeHTML(s string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return replacer.Replace(s)
}
