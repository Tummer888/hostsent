// Package smtp 提供 notifier 的 SMTP 邮件发送实现（doc90 §2.4）。
//
// 三个必须做对的细节（现状 mail_channel.go 全错，见 doc90 §2.4）：
//  1. 中文主题必须用 mime.QEncoding.Encode 编码，否则乱码；
//  2. HTML 正文用 base64 传输编码，否则长行超 998 字节被 SMTP 拒收；
//  3. base64 顺带解决「正文含单独 . 行会被截断」的问题。
package smtp

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"hostsent/backend/internal/pkg/integration"
	"hostsent/backend/internal/pkg/notifier"
)

const providerType = "smtp"

func init() {
	notifier.RegisterDescriptor(providerType, notifier.CapabilityDescriptor{
		Type:     providerType,
		Name:     "SMTP 邮件",
		Category: notifier.CategoryMail,
		Mode:     "api",
		Icon:     "mail",
		CredentialSchema: []integration.Field{
			{Key: "host", Label: "SMTP 服务器", Type: integration.FieldTypeString, Required: true, Placeholder: "smtp.example.com"},
			{Key: "port", Label: "端口", Type: integration.FieldTypeNumber, Required: true, Default: "587", Help: "587=STARTTLS，465=SSL，25=明文"},
			{Key: "username", Label: "用户名", Type: integration.FieldTypeString, Placeholder: "no-reply@example.com"},
			{Key: "password", Label: "密码 / 授权码", Type: integration.FieldTypePassword, Secret: true},
			{Key: "from", Label: "发件人", Type: integration.FieldTypeString, Required: true, Placeholder: "HostSent <no-reply@example.com>"},
			{Key: "use_tls", Label: "启用 TLS", Type: integration.FieldTypeBool, Default: "false"},
		},
		AdapterVersion: "v1",
	})
	notifier.RegisterFactory(providerType, func(cfg notifier.ChannelConfig) notifier.Sender {
		return &smtpSender{cfg: cfg}
	})
}

type smtpSender struct {
	cfg notifier.ChannelConfig
}

func (s *smtpSender) Category() string { return notifier.CategoryMail }
func (s *smtpSender) Type() string     { return providerType }

// Send 发送一封邮件。
func (s *smtpSender) Send(ctx context.Context, cfg notifier.ChannelConfig, msg notifier.Message) (*notifier.Result, error) {
	if err := s.deliver(ctx, cfg, msg.Recipient, msg.Subject, msg.Body, msg.Format); err != nil {
		return nil, err
	}
	return &notifier.Result{ProviderCode: "smtp_ok"}, nil
}

// Test 发送一封测试邮件。
func (s *smtpSender) Test(ctx context.Context, cfg notifier.ChannelConfig, target string) error {
	return s.deliver(ctx, cfg, target, "HostSent 测试邮件",
		"这是一封来自 HostSent 系统的测试邮件，收到即表示 SMTP 配置正常。", notifier.FormatText)
}

// deliver 组装 MIME 报文并投递。
func (s *smtpSender) deliver(ctx context.Context, cfg notifier.ChannelConfig, to, subject, body, format string) error {
	creds := cfg.Credentials
	host := firstNonEmpty(creds["host"], cfg.Endpoint)
	portStr := creds["port"]
	if host == "" || portStr == "" {
		return fmt.Errorf("%w: SMTP 服务器与端口未配置", notifier.ErrChannelDisabled)
	}
	port, err := strconv.Atoi(strings.TrimSpace(portStr))
	if err != nil || port <= 0 {
		return fmt.Errorf("%w: SMTP 端口无效", notifier.ErrChannelDisabled)
	}
	from := firstNonEmpty(creds["from"], cfg.Sender)
	if from == "" {
		return fmt.Errorf("%w: 发件人未配置", notifier.ErrChannelDisabled)
	}
	if strings.TrimSpace(to) == "" {
		return fmt.Errorf("notifier: 收件人为空")
	}

	contentType := "text/plain; charset=UTF-8"
	if format == notifier.FormatHTML {
		contentType = "text/html; charset=UTF-8"
	}
	// 中文主题必须编码（Q 编码），否则客户端乱码。
	encodedSubject := mime.QEncoding.Encode("UTF-8", subject)
	// base64 传输编码：规避长行与单独 "." 行截断（doc90 §2.4 细节 2/3）。
	encodedBody := base64.StdEncoding.EncodeToString([]byte(body))
	// base64 每 76 字符折行，符合 RFC 2045。
	var b strings.Builder
	for i := 0; i < len(encodedBody); i += 76 {
		end := i + 76
		if end > len(encodedBody) {
			end = len(encodedBody)
		}
		b.WriteString(encodedBody[i:end])
		b.WriteString("\r\n")
	}
	headers := strings.Join([]string{
		"From: " + from,
		"To: " + to,
		"Subject: " + encodedSubject,
		"MIME-Version: 1.0",
		"Content-Type: " + contentType,
		"Content-Transfer-Encoding: base64",
		"",
		"",
	}, "\r\n")
	raw := []byte(headers + b.String())

	addr := net.JoinHostPort(host, portStr)
	var auth smtp.Auth
	user := strings.TrimSpace(creds["username"])
	pass := creds["password"]
	if user != "" && pass != "" {
		auth = smtp.PlainAuth("", user, pass, host)
	}
	useTLS := strings.EqualFold(strings.TrimSpace(creds["use_tls"]), "true") || port == 465

	if port == 465 {
		return sendImplicitTLS(addr, host, auth, from, to, raw)
	}
	if useTLS {
		return sendStartTLS(addr, host, auth, from, to, raw)
	}
	return smtp.SendMail(addr, auth, from, []string{to}, raw)
}

// sendImplicitTLS 处理 465 端口（连接即 TLS，smtp.SendMail 不支持）。
func sendImplicitTLS(addr, host string, auth smtp.Auth, from, to string, raw []byte) error {
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, &tls.Config{ServerName: host})
	if err != nil {
		return fmt.Errorf("连接 SMTP 服务器失败: %w", err)
	}
	return submit(conn, host, auth, from, to, raw)
}

// sendStartTLS 显式 STARTTLS（587 端口；服务端不支持时回落明文，与标准库行为一致）。
func sendStartTLS(addr, host string, auth smtp.Auth, from, to string, raw []byte) error {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("连接 SMTP 服务器失败: %w", err)
	}
	return submit(conn, host, auth, from, to, raw)
}

func submit(conn net.Conn, host string, auth smtp.Auth, from, to string, raw []byte) error {
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer func() { _ = c.Close() }()
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: host}); err != nil {
			return fmt.Errorf("STARTTLS 失败: %w", err)
		}
	}
	if auth != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(auth); err != nil {
				return fmt.Errorf("SMTP 认证失败: %w", err)
			}
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(raw); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
