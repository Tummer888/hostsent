package service

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
	notifyrepo "hostsent/backend/internal/modules/admin/notification/repository"
	sysconfigrepo "hostsent/backend/internal/modules/admin/system/repository"
	"hostsent/backend/internal/pkg/notifier"
)

// MailChannel 邮件投递通道（兼容门面）。
//
// Deprecated: doc90 起外发邮件统一走投递队列（notification_deliveries + DeliveryWorker）。
// 本接口只为保留既有调用方（/notifications/mail-test 与历史装配）而存在；
// 新代码请直接用 notifier.Sender 或 DeliveryService。
//
// 内部实现已换成 notifier 的 SMTP provider：中文主题 Q 编码、HTML/长正文 base64 传输
// 编码等三个细节由 provider 统一保证（旧实现三处都错，见 doc90 §2.4）。
type MailChannel interface {
	// SendAsync 保留签名：直发一封邮件（不入队），失败只记日志。
	SendAsync(n notifymodel.Notification)
	// SendTestMail 发送测试邮件。
	SendTestMail(to string) error
}

// SMTPConfig SMTP 配置（从 system_configs 读取，兼容历史键）。
type SMTPConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
	Enabled  bool
}

type mailChannel struct {
	db      *gorm.DB
	cfgRepo sysconfigrepo.ConfigRepository
	repo    notifyrepo.NotificationRepository
	logger  *zap.Logger
}

func NewMailChannel(db *gorm.DB, cfgRepo sysconfigrepo.ConfigRepository, repo notifyrepo.NotificationRepository, logger *zap.Logger) MailChannel {
	return &mailChannel{db: db, cfgRepo: cfgRepo, repo: repo, logger: logger}
}

// loadSMTPConfig 从 system_configs 读取 SMTP 配置。
func (m *mailChannel) loadSMTPConfig(ctx context.Context) (*SMTPConfig, error) {
	cfg := &SMTPConfig{}
	keys := map[string]*string{
		"smtp_host": &cfg.Host,
		"smtp_port": &cfg.Port,
		"smtp_user": &cfg.User,
		"smtp_pass": &cfg.Password,
		"smtp_from": &cfg.From,
	}
	for key, ptr := range keys {
		c, err := m.cfgRepo.FindByKey(ctx, key)
		if err == nil {
			*ptr = c.ConfigValue
		}
	}
	if c, err := m.cfgRepo.FindByKey(ctx, "mail_channel_enabled"); err == nil {
		cfg.Enabled = c.ConfigValue == "true"
	}
	if cfg.Host == "" || cfg.Port == "" || cfg.From == "" {
		cfg.Enabled = false
	}
	return cfg, nil
}

// channelConfig 把历史 system_configs 参数映射为 notifier 渠道配置。
func (m *mailChannel) channelConfig(ctx context.Context) (*notifier.ChannelConfig, error) {
	cfg, err := m.loadSMTPConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, fmt.Errorf("%w: 邮件通道未启用，请在系统设置中配置 SMTP", notifier.ErrChannelDisabled)
	}
	return &notifier.ChannelConfig{
		ChannelCode: "legacy_smtp",
		Type:        "smtp",
		Category:    notifier.CategoryMail,
		Sender:      cfg.From,
		Credentials: map[string]string{
			"host": cfg.Host, "port": cfg.Port,
			"username": cfg.User, "password": cfg.Password,
			"from": cfg.From,
		},
	}, nil
}

// SendAsync 直发一封邮件；失败只记日志，不影响已有站内信。
//
// Deprecated: 新代码请写投递队列，由 DeliveryWorker 统一投递与重试。
func (m *mailChannel) SendAsync(n notifymodel.Notification) {
	go func() {
		ctx := context.Background()
		cfg, err := m.channelConfig(ctx)
		if err != nil {
			m.logger.Warn("mail: channel unavailable", zap.Error(err))
			return
		}
		email, err := m.resolveAddress(ctx, n.UserID)
		if err != nil {
			m.logger.Warn("mail: resolve address failed", zap.Uint64("user_id", n.UserID), zap.Error(err))
			return
		}
		if err := m.sendWith(ctx, cfg, email, n.Title, n.Content, notifier.FormatText); err != nil {
			m.logger.Warn("mail: send failed", zap.Uint64("user_id", n.UserID), zap.Error(err))
			return
		}
		m.logger.Info("mail: sent", zap.Uint64("user_id", n.UserID), zap.String("email", email))
	}()
}

// SendTestMail 发送测试邮件。
func (m *mailChannel) SendTestMail(to string) error {
	ctx := context.Background()
	cfg, err := m.channelConfig(ctx)
	if err != nil {
		return err
	}
	return m.sendWith(ctx, cfg, to, "HostSent 测试邮件",
		"这是一封来自 HostSent 系统的测试邮件，收到即表示 SMTP 配置正常。", notifier.FormatText)
}

func (m *mailChannel) sendWith(ctx context.Context, cfg *notifier.ChannelConfig, to, subject, body, format string) error {
	sender, err := notifier.New(cfg.Type, *cfg)
	if err != nil {
		return err
	}
	_, err = sender.Send(ctx, *cfg, notifier.Message{
		Category:  notifier.CategoryMail,
		Recipient: to,
		Subject:   subject,
		Body:      body,
		Format:    format,
	})
	return err
}

// resolveAddress 从 users 表获取收件人邮箱。
func (m *mailChannel) resolveAddress(ctx context.Context, userID uint64) (string, error) {
	if userID == 0 {
		return "", fmt.Errorf("broadcast notification has no single email address")
	}
	var email string
	err := m.db.WithContext(ctx).Table("users").
		Where("id = ?", userID).
		Select("email").
		Row().Scan(&email)
	if err != nil {
		return "", err
	}
	if email == "" {
		return "", fmt.Errorf("user %d has no email", userID)
	}
	return email, nil
}
