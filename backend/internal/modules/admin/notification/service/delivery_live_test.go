//go:build live

package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
	notifyrepo "hostsent/backend/internal/modules/admin/notification/repository"
	"hostsent/backend/internal/pkg/notifier"
	// 空导入触发 smtp provider 的 init()：否则 notifier.New("smtp") 返回
	// ErrAdapterNotImplemented，投递会被判为「待接入」而 skipped 而非 failed。
	_ "hostsent/backend/internal/pkg/notifier/provider/smtp"
)

// stubResolver 按预设返回渠道或错误，避免依赖真实渠道表。
type stubResolver struct {
	cfg *notifier.ChannelConfig
	err error
}

func (s *stubResolver) Resolve(context.Context, string, string) (*notifier.ChannelConfig, error) {
	return s.cfg, s.err
}
func (s *stubResolver) ResolveByID(context.Context, uint64) (*notifier.ChannelConfig, error) {
	return s.cfg, s.err
}

// switchAlways 让通道总开关恒为 true（否则 SMS 直接 skipped）。
func switchAlways(context.Context, string) (string, bool, error) { return "true", true, nil }

// newLiveWorker 建一个把投递记录写入真实库、渠道解析由桩控制的 worker。
func newLiveWorker(t *testing.T, resolver ChannelResolver, opts DeliveryWorkerOptions) (*DeliveryWorker, *gorm.DB) {
	t.Helper()
	db := liveDB(t)
	if opts.Interval <= 0 {
		opts.Interval = time.Minute // 测试直接调 RunOnce，不依赖 ticker
	}
	w := NewDeliveryWorker(DeliveryWorkerDeps{
		Repo:         notifyrepo.NewDeliveryRepository(db),
		Resolver:     resolver,
		SwitchReader: switchAlways,
		Options:      opts,
		Logger:       zap.NewNop(),
	})
	return w, db
}

// insertDelivery 造一条待投递行，测试后删除。
func insertDelivery(t *testing.T, db *gorm.DB, channel, recipient string, maxAttempts int) uint64 {
	t.Helper()
	repo := notifyrepo.NewDeliveryRepository(db)
	row := &notifymodel.NotificationDelivery{
		Event: "accept_worker", Channel: channel, TargetType: "user",
		Recipient: recipient, Title: "验收", Content: "验收正文",
		ContentFormat: notifymodel.FormatText, SendStatus: notifymodel.DeliveryStatusPending,
		MaxAttempts: maxAttempts, SourceModule: "acceptance", SourceID: t.Name(),
		Vars: "{}", // jsonb 列拒绝空串（22P02），必须给合法 JSON。
	}
	if err := repo.Enqueue(context.Background(), row); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	t.Cleanup(func() {
		db.Exec("delete from notification_deliveries where id = ?", row.ID)
	})
	return row.ID
}

func readDelivery(t *testing.T, db *gorm.DB, id uint64) notifymodel.NotificationDelivery {
	t.Helper()
	var d notifymodel.NotificationDelivery
	if err := db.Where("id = ?", id).First(&d).Error; err != nil {
		t.Fatalf("read delivery %d: %v", id, err)
	}
	return d
}

// 场景 11：收件地址无效 → 按退避重试，到 max_attempts 后转 dead，
// attempts 与 next_retry_at 必须如实记录。
func TestAcceptanceDeliveryRetriesThenDead(t *testing.T) {
	// smtp 渠道但主机不可达：真实发送失败（非配置类），应走 failed→dead。
	resolver := &stubResolver{cfg: &notifier.ChannelConfig{
		Type: "smtp", Category: notifier.CategoryMail,
		Credentials: map[string]string{
			"host": "127.0.0.1", "port": "1", // 必然拒连
			"from": "a@b.c",
		},
	}}
	w, db := newLiveWorker(t, resolver, DeliveryWorkerOptions{MaxAttempts: 2})
	id := insertDelivery(t, db, notifymodel.ChannelMail, "nobody@example.com", 2)

	// 第一轮：失败 → failed + attempts=1 + next_retry_at 非空。
	w.RunOnce(context.Background())
	got := readDelivery(t, db, id)
	if got.SendStatus != notifymodel.DeliveryStatusFailed {
		t.Fatalf("first attempt should be failed, got %s (%s)", got.SendStatus, got.FailReason)
	}
	if got.Attempts != 1 || got.NextRetryAt == nil {
		t.Fatalf("first attempt must record attempts=1 and a retry time, got attempts=%d next=%v", got.Attempts, got.NextRetryAt)
	}

	// 把退避时间提到过去，触发第二轮 → 达到 max_attempts 转 dead。
	db.Exec("update notification_deliveries set next_retry_at = now() - interval '1 minute' where id = ?", id)
	w.RunOnce(context.Background())
	got = readDelivery(t, db, id)
	if got.SendStatus != notifymodel.DeliveryStatusDead {
		t.Fatalf("second failed attempt should be dead, got %s", got.SendStatus)
	}
	if got.FailReason == "" {
		t.Fatal("dead row must keep the failure reason")
	}
}

// 场景 12：无可用渠道 → skipped 且不重试（配置问题区别于发送失败）。
func TestAcceptanceDeliverySkippedWhenNoChannel(t *testing.T) {
	w, db := newLiveWorker(t, &stubResolver{err: ErrChannelNotConfigured}, DeliveryWorkerOptions{})
	id := insertDelivery(t, db, notifymodel.ChannelMail, "x@example.com", 5)

	w.RunOnce(context.Background())
	got := readDelivery(t, db, id)
	if got.SendStatus != notifymodel.DeliveryStatusSkipped {
		t.Fatalf("no-channel delivery should be skipped, got %s", got.SendStatus)
	}
	if got.Attempts != 0 || got.NextRetryAt != nil {
		t.Fatalf("skipped is terminal for the queue: attempts=%d next=%v", got.Attempts, got.NextRetryAt)
	}
}

// 场景 13（bug ② 回归）：对 failed 投递点重投 → attempts 归零、状态回 pending，
// 且下一轮真的会再发一次。
func TestAcceptanceDeliveryRetryResetsAndResends(t *testing.T) {
	resolver := &stubResolver{cfg: &notifier.ChannelConfig{
		Type: "smtp", Category: notifier.CategoryMail,
		Credentials: map[string]string{"host": "127.0.0.1", "port": "1", "from": "a@b.c"},
	}}
	w, db := newLiveWorker(t, resolver, DeliveryWorkerOptions{MaxAttempts: 5})
	id := insertDelivery(t, db, notifymodel.ChannelMail, "retry@example.com", 5)

	w.RunOnce(context.Background())
	if got := readDelivery(t, db, id); got.SendStatus != notifymodel.DeliveryStatusFailed {
		t.Fatalf("precondition failed, got %s", got.SendStatus)
	}

	svc := NewDeliveryService(notifyrepo.NewDeliveryRepository(db), notifyrepo.NewNotificationRepository(db))
	if err := svc.Retry(context.Background(), id); err != nil {
		t.Fatalf("retry: %v", err)
	}
	got := readDelivery(t, db, id)
	if got.SendStatus != notifymodel.DeliveryStatusPending || got.Attempts != 0 || got.NextRetryAt != nil {
		t.Fatalf("retry must reset to pending/attempts=0, got %s attempts=%d next=%v", got.SendStatus, got.Attempts, got.NextRetryAt)
	}

	// 再跑一轮证明它真的被重新领取（失败变回 failed，attempts 从 0 → 1）。
	w.RunOnce(context.Background())
	if got := readDelivery(t, db, id); got.Attempts != 1 {
		t.Fatalf("re-queued delivery must be re-attempted, attempts=%d", got.Attempts)
	}
}

// 场景 10 的库内证据：站内信通道由 worker 直接标 sent 且 sent_at 非空。
func TestAcceptanceInboxDeliveryMarkedSent(t *testing.T) {
	w, db := newLiveWorker(t, &stubResolver{}, DeliveryWorkerOptions{})
	id := insertDelivery(t, db, notifymodel.ChannelInbox, "", 5)

	w.RunOnce(context.Background())
	got := readDelivery(t, db, id)
	if got.SendStatus != notifymodel.DeliveryStatusSent || got.SentAt == nil {
		t.Fatalf("inbox delivery must be sent with sent_at set, got %s sent_at=%v", got.SendStatus, got.SentAt)
	}
}

// ---------------------------------------------------------------------------
// 场景 6/7：HTML 邮件正文按 HTML 传输（不是转义后的源码），中文主题 Q 编码。
// ---------------------------------------------------------------------------

// captureSMTP 一个极小的 SMTP 收集器，只实现 EHLO/MAIL/RCPT/DATA/QUIT。
type captureSMTP struct {
	addr string
	mu   sync.Mutex
	raw  []string
	ln   net.Listener
}

func newCaptureSMTP(t *testing.T) *captureSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := &captureSMTP{addr: ln.Addr().String(), ln: ln}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(conn)
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return s
}

func (s *captureSMTP) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	_, _ = conn.Write([]byte("220 capture ESMTP\r\n"))
	r := bufio.NewReader(conn)
	var body []byte
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		up := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(up, "EHLO"), strings.HasPrefix(up, "HELO"):
			_, _ = conn.Write([]byte("250-capture\r\n250 8BITMIME\r\n"))
		case strings.HasPrefix(up, "MAIL"), strings.HasPrefix(up, "RCPT"):
			_, _ = conn.Write([]byte("250 OK\r\n"))
		case up == "DATA":
			_, _ = conn.Write([]byte("354 go\r\n"))
			var buf bytes.Buffer
			for {
				dl, derr := r.ReadString('\n')
				if derr != nil {
					break
				}
				if strings.TrimRight(dl, "\r\n") == "." {
					break
				}
				buf.WriteString(dl)
			}
			body = buf.Bytes()
			s.mu.Lock()
			s.raw = append(s.raw, string(body))
			s.mu.Unlock()
			_, _ = conn.Write([]byte("250 queued\r\n"))
		case up == "QUIT":
			_, _ = conn.Write([]byte("221 bye\r\n"))
			return
		default:
			_, _ = conn.Write([]byte("250 OK\r\n"))
		}
	}
}

func (s *captureSMTP) last(t *testing.T) string {
	t.Helper()
	// SMTP 投递在后台 goroutine 完成，给一点时间。
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		if len(s.raw) > 0 {
			got := s.raw[len(s.raw)-1]
			s.mu.Unlock()
			return got
		}
		s.mu.Unlock()
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("no SMTP message captured")
	return ""
}

func TestAcceptanceHTMLEmailAndChineseSubject(t *testing.T) {
	sink := newCaptureSMTP(t)
	host, port, _ := net.SplitHostPort(sink.addr)
	resolver := &stubResolver{cfg: &notifier.ChannelConfig{
		Type: "smtp", Category: notifier.CategoryMail,
		Credentials: map[string]string{
			"host": host, "port": port, "from": "HostSent <no-reply@hostsent.local>", "use_tls": "false",
		},
	}}
	w, db := newLiveWorker(t, resolver, DeliveryWorkerOptions{})
	repo := notifyrepo.NewDeliveryRepository(db)
	row := &notifymodel.NotificationDelivery{
		Event: "accept_html", Channel: notifymodel.ChannelMail, TargetType: "user",
		Recipient: "html@hostsent.local", Title: "中文主题：实例即将到期",
		Content:       "<h1>中文标题</h1><p>正文 <b>加粗</b> 结束</p>",
		ContentFormat: notifymodel.FormatHTML, SendStatus: notifymodel.DeliveryStatusPending,
		MaxAttempts: 3, SourceModule: "acceptance", SourceID: t.Name(), Vars: "{}",
	}
	if err := repo.Enqueue(context.Background(), row); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	t.Cleanup(func() { db.Exec("delete from notification_deliveries where id = ?", row.ID) })

	w.RunOnce(context.Background())
	if got := readDelivery(t, db, row.ID); got.SendStatus != notifymodel.DeliveryStatusSent {
		t.Fatalf("html mail should send, got %s (%s)", got.SendStatus, got.FailReason)
	}

	raw := sink.last(t)
	if !strings.Contains(raw, "Content-Type: text/html; charset=UTF-8") {
		t.Errorf("HTML format must be declared as text/html, raw:\n%s", raw)
	}
	// 主题含中文必须 Q 编码（否则客户端乱码）。
	if !strings.Contains(raw, "Subject: =?UTF-8?") {
		t.Errorf("chinese subject must be RFC2047-encoded, raw:\n%s", raw)
	}
	// 正文是 base64 传输编码，解码后应还原 HTML 标签而非转义实体。
	parts := strings.SplitN(raw, "\r\n\r\n", 2)
	if len(parts) != 2 {
		t.Fatalf("unexpected raw message shape:\n%s", raw)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(parts[1], "\r\n", ""))
	if err != nil {
		t.Fatalf("body must be base64: %v\n%s", err, raw)
	}
	body := string(decoded)
	if !strings.Contains(body, "<h1>中文标题</h1>") || !strings.Contains(body, "<b>加粗</b>") {
		t.Fatalf("HTML body must survive unescaped, got %q", body)
	}
}
