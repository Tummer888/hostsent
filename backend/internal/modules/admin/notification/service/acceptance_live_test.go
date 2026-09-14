//go:build live

package service

import (
	"context"
	"os"
	"strings"
	"testing"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	notifydto "hostsent/backend/internal/modules/admin/notification/dto"
	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
	notifyrepo "hostsent/backend/internal/modules/admin/notification/repository"
	"hostsent/backend/internal/pkg/notifier"
)

// liveDSN 连接本地开发库（与 lifecycle_live_test.go 同一约定）。
func liveDSN() string {
	if dsn := os.Getenv("LIVE_DB_DSN"); dsn != "" {
		return dsn
	}
	return "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
}

func liveDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open(liveDSN()), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Skipf("live db unavailable: %v", err)
	}
	return db
}

// mailCountingSender 记录被投递到"邮箱"的消息，避免依赖真实 SMTP。
type mailCountingSender struct {
	sent int
}

func (m *mailCountingSender) SendAsync(notifymodel.Notification) {}
func (m *mailCountingSender) SendTestMail(string) error          { m.sent++; return nil }

// liveFixture 真实仓储 + 计数 mailChannel 的通知服务。
type liveFixture struct {
	svc  NotificationService
	db   *gorm.DB
	mail *mailCountingSender
}

func newLiveFixture(t *testing.T) *liveFixture {
	t.Helper()
	db := liveDB(t)
	mail := &mailCountingSender{}
	svc := NewNotificationService(
		db,
		notifyrepo.NewNotificationRepository(db),
		notifyrepo.NewTemplateRepository(db),
		notifyrepo.NewPreferenceRepository(db),
		notifyrepo.NewAnnouncementRepository(db),
		mail,
		zap.NewNop(),
	)
	svc.SetDeliveryDeps(
		notifyrepo.NewDeliveryRepository(db),
		notifyrepo.NewSmsTemplateRepository(db),
		notifyrepo.NewRecipientResolver(db),
	)
	return &liveFixture{svc: svc, db: db, mail: mail}
}

// withTemplateChannels 临时改事件的通道开关，测试结束恢复原值。
func (f *liveFixture) withTemplateChannels(t *testing.T, event string, inboxOn, mailOn bool) {
	t.Helper()
	var tpl notifymodel.NotificationTemplate
	if err := f.db.Where("event = ?", event).First(&tpl).Error; err != nil {
		t.Skipf("template %s missing: %v", event, err)
	}
	origInbox, origMail := tpl.InboxOn, tpl.MailOn
	if err := f.db.Model(&notifymodel.NotificationTemplate{}).Where("id = ?", tpl.ID).
		Updates(map[string]any{"inbox_on": inboxOn, "mail_on": mailOn}).Error; err != nil {
		t.Fatalf("update template %s: %v", event, err)
	}
	t.Cleanup(func() {
		_ = f.db.Model(&notifymodel.NotificationTemplate{}).Where("id = ?", tpl.ID).
			Updates(map[string]any{"inbox_on": origInbox, "mail_on": origMail}).Error
	})
}

// publish 发布一条验收事件，并把产生的行登记为测试后清理对象。
func (f *liveFixture) publish(t *testing.T, event string, userID uint64, sourceID string) error {
	t.Helper()
	t.Cleanup(func() {
		f.db.Exec("delete from notifications where source_module = 'acceptance' and source_id = ?", sourceID)
		f.db.Exec("delete from notification_deliveries where source_module = 'acceptance' and source_id = ?", sourceID)
	})
	return f.svc.Publish(context.Background(), notifydto.PublishInput{
		Event: event, UserID: userID, Target: notifymodel.TargetUser,
		Vars:         map[string]string{"order_no": sourceID, "amount": "1.00", "instance_mark": "m", "days_left": "1", "expire_at": "x", "balance": "0", "threshold": "1", "provider_name": "p", "reason": "r"},
		SourceModule: "acceptance", SourceID: sourceID,
	})
}

func (f *liveFixture) countDeliveries(t *testing.T, sourceID, channel string) int64 {
	t.Helper()
	var n int64
	f.db.Model(&notifymodel.NotificationDelivery{}).
		Where("source_module = 'acceptance' and source_id = ? and channel = ?", sourceID, channel).Count(&n)
	return n
}

// acceptUserID 找一个有邮箱的活跃用户（场景 16/17/18 的发布对象）。
func acceptUserID(t *testing.T, db *gorm.DB) uint64 {
	t.Helper()
	var id uint64
	if err := db.Raw(`select id from users where email <> '' and status = 'active' order by id limit 1`).Scan(&id).Error; err != nil || id == 0 {
		t.Skipf("no active user with email: %v", err)
	}
	return id
}

// 场景 18（bug ⑤ 回归）：触发邮件通知时 notifications 不多出 mail 行，
// 邮件只出现在 notification_deliveries。
func TestAcceptanceMailGoesToDeliveriesOnly(t *testing.T) {
	f := newLiveFixture(t)
	uid := acceptUserID(t, f.db)
	const source = "accept-mail-only"
	f.withTemplateChannels(t, notifymodel.EventOrderPaid, true, true)
	t.Cleanup(func() {
		_ = f.db.Exec("delete from notification_preferences where user_id = ? and event = 'order_paid'", uid).Error
	})

	if err := f.publish(t, notifymodel.EventOrderPaid, uid, source); err != nil {
		t.Fatalf("publish: %v", err)
	}

	var mailInboxRows int64
	f.db.Model(&notifymodel.Notification{}).
		Where("source_module = 'acceptance' and source_id = ? and channel = 'mail'", source).Count(&mailInboxRows)
	if mailInboxRows != 0 {
		t.Fatalf("notifications must not hold mail rows (bug ⑤), got %d", mailInboxRows)
	}
	if n := f.countDeliveries(t, source, notifymodel.ChannelMail); n != 1 {
		t.Fatalf("expected 1 mail delivery, got %d", n)
	}
	if n := f.countDeliveries(t, source, notifymodel.ChannelInbox); n != 0 {
		t.Fatalf("inbox does not go through delivery queue, got %d", n)
	}
}

// 场景 16（bug ④ 回归）：用户把某事件的站内信关掉后，该事件不再落 notifications。
func TestAcceptanceInboxPreferenceOffSuppressesInbox(t *testing.T) {
	f := newLiveFixture(t)
	uid := acceptUserID(t, f.db)
	const source = "accept-inbox-off"
	f.withTemplateChannels(t, notifymodel.EventOrderPaid, true, false)

	// 关掉站内信偏好（走真实仓储，等同用户端保存偏好）。
	prefRepo := notifyrepo.NewPreferenceRepository(f.db)
	pref := &notifymodel.NotificationPreference{UserID: uid, Event: notifymodel.EventOrderPaid, InboxOn: false, MailOn: false, SmsOn: false}
	if err := prefRepo.Upsert(context.Background(), pref); err != nil {
		t.Fatalf("upsert preference: %v", err)
	}
	t.Cleanup(func() {
		_ = f.db.Exec("delete from notification_preferences where user_id = ? and event = 'order_paid'", uid).Error
	})

	if err := f.publish(t, notifymodel.EventOrderPaid, uid, source); err != nil {
		t.Fatalf("publish: %v", err)
	}
	var n int64
	f.db.Model(&notifymodel.Notification{}).
		Where("source_module = 'acceptance' and source_id = ?", source).Count(&n)
	if n != 0 {
		t.Fatalf("inbox row must not be written when preference is off, got %d", n)
	}
}

// 场景 17：用户关掉站内信，但 OTP 类强制事件仍然落库（IsMandatoryEvent 豁免）。
func TestAcceptanceMandatoryEventBypassesPreference(t *testing.T) {
	f := newLiveFixture(t)
	uid := acceptUserID(t, f.db)
	// 强制事件的模板可能不存在，直接用 PublishInput 走 system 模板验证豁免逻辑：
	// 这里通过 prefAllowed 的公开行为断言 —— 关掉偏好后强制事件仍被允许。
	prefRepo := notifyrepo.NewPreferenceRepository(f.db)
	if err := prefRepo.Upsert(context.Background(), &notifymodel.NotificationPreference{
		UserID: uid, Event: "login_otp", InboxOn: false, MailOn: false, SmsOn: false,
	}); err != nil {
		t.Fatalf("upsert preference: %v", err)
	}
	t.Cleanup(func() {
		_ = f.db.Exec("delete from notification_preferences where user_id = ? and event = 'login_otp'", uid).Error
	})

	svc := f.svc.(*notificationService)
	if !svc.prefAllowed(context.Background(), uid, "login_otp", notifymodel.ChannelInbox) {
		t.Fatal("mandatory event must bypass the inbox preference (doc90 §11 场景 17)")
	}
	// 对照组：非强制事件必须先有偏好行、再关掉，才应被拦。没有偏好行时按模板默认放行，
	// 所以这里显式写一行 false 再断言。
	if err := prefRepo.Upsert(context.Background(), &notifymodel.NotificationPreference{
		UserID: uid, Event: notifymodel.EventOrderPaid, InboxOn: false,
	}); err != nil {
		t.Fatalf("upsert control preference: %v", err)
	}
	t.Cleanup(func() {
		_ = f.db.Exec("delete from notification_preferences where user_id = ? and event = 'order_paid'", uid).Error
	})
	if svc.prefAllowed(context.Background(), uid, notifymodel.EventOrderPaid, notifymodel.ChannelInbox) {
		t.Fatal("control: non-mandatory event must respect the preference")
	}
}

// 场景 14（bug ③ 回归）：同一 (source_module, source_id) 连续发布两次只落一行，
// 且第二次返回成功（幂等唯一索引冲突被视为已发布，不报错）。
func TestAcceptanceDuplicatePublishIsIdempotent(t *testing.T) {
	f := newLiveFixture(t)
	uid := acceptUserID(t, f.db)
	const source = "accept-idem"
	f.withTemplateChannels(t, notifymodel.EventOrderPaid, true, false)
	t.Cleanup(func() {
		_ = f.db.Exec("delete from notification_preferences where user_id = ? and event = 'order_paid'", uid).Error
	})

	if err := f.publish(t, notifymodel.EventOrderPaid, uid, source); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	if err := f.publish(t, notifymodel.EventOrderPaid, uid, source); err != nil {
		t.Fatalf("second publish must succeed (bug ③): %v", err)
	}
	var n int64
	f.db.Model(&notifymodel.Notification{}).
		Where("source_module = 'acceptance' and source_id = ?", source).Count(&n)
	if n != 1 {
		t.Fatalf("duplicate publish must yield exactly 1 inbox row, got %d", n)
	}
}

// 场景 5：短信模板用了未注册变量时必须拒存，且错误信息带变量名。
func TestAcceptanceSmsTemplateRejectsUnregisteredVar(t *testing.T) {
	db := liveDB(t)
	svc := NewSmsTemplateService(notifyrepo.NewSmsTemplateRepository(db), notifyrepo.NewTemplateVarRepository(db))
	_, err := svc.Create(context.Background(), notifydto.SmsTemplateSaveRequest{
		Code: "accept_bad_var", Name: "验收", Scene: "login",
		Content: "验证码 {code}，参考 {zzz_unregistered}",
	})
	if err == nil {
		t.Fatal("expected rejection for unregistered var")
	}
	if !strings.Contains(err.Error(), "zzz_unregistered") {
		t.Fatalf("error must name the offending variable, got %q", err.Error())
	}
}

// 场景 12 的语义前置：MandatoryEvents 白名单非空且包含登录验证码。
func TestAcceptanceMandatoryEventWhitelist(t *testing.T) {
	events := notifier.MandatoryEvents()
	want := map[string]bool{"login_otp": false, "password_reset": false, "register_verify": false}
	for _, e := range events {
		if _, ok := want[e]; ok {
			want[e] = true
		}
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("mandatory whitelist missing %q", k)
		}
	}
}
