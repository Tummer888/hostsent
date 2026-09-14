//go:build live

package repository

import (
	"context"
	"fmt"
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
)

func liveDSN() string {
	if dsn := os.Getenv("LIVE_DB_DSN"); dsn != "" {
		return dsn
	}
	return "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
}

func liveGorm(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open(liveDSN()), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Skipf("live db unavailable: %v", err)
	}
	return db
}

func readPref(t *testing.T, db *gorm.DB, userID uint64, event string) notifymodel.NotificationPreference {
	t.Helper()
	var got notifymodel.NotificationPreference
	if err := db.Where("user_id = ? and event = ?", userID, event).First(&got).Error; err != nil {
		t.Fatalf("read %s: %v", event, err)
	}
	return got
}

func TestLivePrefFalsePersists(t *testing.T) {
	db := liveGorm(t)
	repo := NewPreferenceRepository(db)
	const userID = uint64(7)

	db.Exec("delete from notification_preferences where user_id = ? and event like 'zzz_pref_%'", userID)

	// 插入路径：全 false 必须原样落库（默认 inbox_on=true 不能覆盖显式 false）。
	if err := repo.BatchUpsert(context.Background(), []notifymodel.NotificationPreference{
		{UserID: userID, Event: "zzz_pref_insert", InboxOn: false, MailOn: false, SmsOn: false},
	}); err != nil {
		t.Fatalf("batch upsert: %v", err)
	}
	got := readPref(t, db, userID, "zzz_pref_insert")
	fmt.Printf("insert path: inbox=%v mail=%v sms=%v\n", got.InboxOn, got.MailOn, got.SmsOn)
	if got.InboxOn || got.MailOn || got.SmsOn {
		t.Errorf("insert path dropped explicit false: %+v", got)
	}

	// 更新路径：先 true 再 false，第二次必须真的写回 false。
	if err := repo.BatchUpsert(context.Background(), []notifymodel.NotificationPreference{
		{UserID: userID, Event: "zzz_pref_update", InboxOn: true, MailOn: true, SmsOn: true},
	}); err != nil {
		t.Fatalf("batch upsert true: %v", err)
	}
	if err := repo.BatchUpsert(context.Background(), []notifymodel.NotificationPreference{
		{UserID: userID, Event: "zzz_pref_update", InboxOn: false, MailOn: false, SmsOn: false},
	}); err != nil {
		t.Fatalf("batch upsert false: %v", err)
	}
	got = readPref(t, db, userID, "zzz_pref_update")
	fmt.Printf("update path: inbox=%v mail=%v sms=%v\n", got.InboxOn, got.MailOn, got.SmsOn)
	if got.InboxOn || got.MailOn || got.SmsOn {
		t.Errorf("update path dropped explicit false: %+v", got)
	}

	db.Exec("delete from notification_preferences where user_id = ? and event like 'zzz_pref_%'", userID)
}
