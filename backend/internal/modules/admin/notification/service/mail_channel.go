package service

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
	notifyrepo "hostsent/backend/internal/modules/admin/notification/repository"
	sysconfigrepo "hostsent/backend/internal/modules/admin/system/repository"
)

// MailChannel 邮件投递通道，SMTP 参数从 system_configs 读取。
type MailChannel interface {
	SendAsync(n notifymodel.Notification)
	SendTestMail(to string) error
}

// SMTPConfig SMTP 配置。
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

// SendAsync 异步发送并回写发送结果；失败不影响站内信。
func (m *mailChannel) SendAsync(n notifymodel.Notification) {
	go func() {
		ctx := context.Background()
		n.Channel = notifymodel.ChannelMail
		n.SendStatus = notifymodel.SendStatusPending
		if err := m.repo.CreateInbox(ctx, &n); err != nil {
			m.logger.Debug("mail: notification record may already exist", zap.Error(err))
			return
		}
		email, err := m.resolveAddress(ctx, n.UserID)
		if err != nil {
			_ = m.repo.UpdateSendStatus(ctx, n.ID, notifymodel.SendStatusFailed, err.Error())
			m.logger.Warn("mail: resolve address failed", zap.Uint64("user_id", n.UserID), zap.Error(err))
			return
		}
		if err := m.send(ctx, email, n.Title, n.Content); err != nil {
			_ = m.repo.UpdateSendStatus(ctx, n.ID, notifymodel.SendStatusFailed, err.Error())
			m.logger.Warn("mail: send failed", zap.Uint64("user_id", n.UserID), zap.Error(err))
			return
		}
		_ = m.repo.UpdateSendStatus(ctx, n.ID, notifymodel.SendStatusSent, "")
		m.logger.Info("mail: sent", zap.Uint64("user_id", n.UserID), zap.String("email", email))
	}()
}

// SendTestMail 发送测试邮件。
func (m *mailChannel) SendTestMail(to string) error {
	ctx := context.Background()
	return m.send(ctx, to, "HostSent 测试邮件", "这是一封来自 HostSent 系统的测试邮件，收到即表示 SMTP 配置正常。")
}

// send 通过 SMTP 发送邮件。
func (m *mailChannel) send(ctx context.Context, to, subject, body string) error {
	cfg, err := m.loadSMTPConfig(ctx)
	if err != nil {
		return fmt.Errorf("load smtp config: %w", err)
	}
	if !cfg.Enabled {
		return fmt.Errorf("mail channel is not enabled, please configure SMTP in system settings")
	}
	msg := strings.Join([]string{
		"From: " + cfg.From,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	addr := cfg.Host + ":" + cfg.Port
	var auth smtp.Auth
	if cfg.User != "" && cfg.Password != "" {
		auth = smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)
	}
	return smtp.SendMail(addr, auth, cfg.From, []string{to}, []byte(msg))
}
