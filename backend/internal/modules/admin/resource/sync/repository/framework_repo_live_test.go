//go:build live

package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestListDueSchedulesHealthGate 针对真实库验证：到期扫描必须把
// 熔断（sync_paused）/ 停用（status<>1）/ 已删除（deleted_at）的渠道排除在外。
//
// 这是本次故障的核心回归：等价口径的 FindDueProviders 有这四道闸门，
// 但调度器改走 ListDueSchedules 时漏了，导致 provider 1 熔断后仍每 60 秒被触发、
// 持续刷失败日志（实测累计 7000+ 条）。
//
// 仅在 -tags live 时运行：go test -tags live ./internal/modules/admin/resource/sync/repository/
func TestListDueSchedulesHealthGate(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Skipf("无可用测试库：%v", err)
	}

	// 用事务包裹并在最后强制回滚，避免污染真实库。
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("开启事务失败: %v", tx.Error)
	}
	defer tx.Rollback()

	ctx := context.Background()
	repo := NewFrameworkRepository(tx)

	insertProvider := func(name string, status int, paused bool, deleted bool) uint64 {
		var id uint64
		deletedAt := "NULL"
		if deleted {
			deletedAt = "now()"
		}
		sql := `INSERT INTO resource_providers
			(name, provider_type, api_endpoint, api_key, api_secret, status, sync_enabled, sync_paused, kind, consecutive_failures, deleted_at)
			VALUES (?, 'syncgateprobe', 'http://probe.invalid', 'k', 's', ?, true, ?, 'upstream', 0, ` + deletedAt + `) RETURNING id`
		if err := tx.Raw(sql, name, status, paused).Scan(&id).Error; err != nil {
			t.Fatalf("插入渠道 %s 失败: %v", name, err)
		}
		if err := tx.Exec(`INSERT INTO sync_schedules
			(provider_id, scope, interval_seconds, full_sync_interval_seconds, enabled, priority, next_run_at)
			VALUES (?, 'instance', 300, 86400, true, 0, now() - interval '1 hour')`, id).Error; err != nil {
			t.Fatalf("插入调度 %s 失败: %v", name, err)
		}
		return id
	}

	healthy := insertProvider("syncgateprobe-healthy", 1, false, false)
	paused := insertProvider("syncgateprobe-paused", 1, true, false)
	disabled := insertProvider("syncgateprobe-disabled", 0, false, false)
	deleted := insertProvider("syncgateprobe-deleted", 1, false, true)

	due, err := repo.ListDueSchedules(ctx, time.Now())
	if err != nil {
		t.Fatalf("ListDueSchedules 失败: %v", err)
	}
	got := map[uint64]bool{}
	for _, s := range due {
		got[s.ProviderID] = true
	}

	if !got[healthy] {
		t.Errorf("正常渠道 %d 应出现在到期集合", healthy)
	}
	for name, pid := range map[string]uint64{"熔断": paused, "停用": disabled, "已删除": deleted} {
		if got[pid] {
			t.Errorf("%s渠道 %d 不应出现在到期集合（会被每轮触发刷失败日志）", name, pid)
		}
	}
	t.Logf("到期集合命中正常渠道=%v，排除熔断=%v 停用=%v 已删除=%v",
		got[healthy], !got[paused], !got[disabled], !got[deleted])
}
