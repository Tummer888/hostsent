//go:build live

package repository

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func liveClaimDSN() string {
	if dsn := os.Getenv("LIVE_DB_DSN"); dsn != "" {
		return dsn
	}
	return "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
}

// createClaimInstance 造一台仅用于抢占测试的实例行。
func createClaimInstance(t *testing.T, db *gorm.DB, mark string) uint64 {
	t.Helper()
	var id uint64
	err := db.Raw(`
		INSERT INTO instances (instance_id, provider_id, user_id, name, cpu, memory, disk, status,
			source_mode, provider_instance_id, billing_mode, created_at, updated_at)
		VALUES (?, 1, 2, ?, 2, 4096, 40, 'stopped', 'self', ?, 'monthly', now(), now())
		RETURNING id`, mark, "过渡态抢占用例 "+mark, mark).Scan(&id).Error
	if err != nil {
		t.Fatalf("创建实例 %s 失败: %v", mark, err)
	}
	t.Cleanup(func() { db.Exec("DELETE FROM instances WHERE id = ?", id) })
	return id
}

// TestClaimTransitionIsAtomic 直接验证抢占原语的互斥性。
//
// 这是"连点两次都下发"的根因所在：先查后写有检查-执行窗口，两个并发请求
// 双双通过。这里绕过服务层，直接并发打 ClaimTransition，断言恰好一个赢。
//
// 为什么必须在仓库层测：服务层的用例里，抢占之前还有一次 FindByID，
// 那次读会把窗口收窄到难以稳定复现，测不出实现是否为原子。
func TestClaimTransitionIsAtomic(t *testing.T) {
	db, err := gorm.Open(postgres.Open(liveClaimDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}
	repo := NewInstanceRepository(db)
	ctx := context.Background()
	mark := fmt.Sprintf("claim-%d", time.Now().UnixNano()%1e9)

	id := createClaimInstance(t, db, mark)

	const goroutines = 16
	now := time.Now()
	until := now.Add(3 * time.Minute)

	var wg sync.WaitGroup
	var winners int32
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // 同一瞬间齐发，最大化并发窗口
			claimed, cerr := repo.ClaimTransition(ctx, id, "starting", now, until)
			if cerr != nil {
				t.Errorf("抢占报错: %v", cerr)
				return
			}
			if claimed {
				atomic.AddInt32(&winners, 1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := atomic.LoadInt32(&winners); got != 1 {
		t.Fatalf("%d 个并发抢占应恰好 1 个成功，实际 %d 个（说明判定与写入不在同一条语句里）", goroutines, got)
	}
}

// TestClaimTransitionAllowsExpiredPreempt 验证超时后可抢占。
//
// 与 ensureNoActiveTransition 的超时放行同一口径：上一个动作超过典型耗时仍未
// 确认时，不该永久锁死实例（否则一次丢回执就等于永久禁操作）。
func TestClaimTransitionAllowsExpiredPreempt(t *testing.T) {
	db, err := gorm.Open(postgres.Open(liveClaimDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}
	repo := NewInstanceRepository(db)
	ctx := context.Background()
	mark := fmt.Sprintf("claim-exp-%d", time.Now().UnixNano()%1e9)

	id := createClaimInstance(t, db, mark)
	now := time.Now()

	// 第一次抢占：成功。
	ok, err := repo.ClaimTransition(ctx, id, "starting", now, now.Add(3*time.Minute))
	if err != nil || !ok {
		t.Fatalf("首次抢占应成功: ok=%v err=%v", ok, err)
	}
	// 窗口内再抢：必须失败。
	ok, err = repo.ClaimTransition(ctx, id, "rebooting", now, now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("窗口内抢占报错: %v", err)
	}
	if ok {
		t.Fatal("窗口内不得再次抢占（这正是重复下发指令的入口）")
	}

	// 把窗口改成已过（模拟上一次动作超时未确认）→ 应可抢占。
	if err := db.Exec("UPDATE instances SET transition_until = ? WHERE id = ?", now.Add(-1*time.Minute), id).Error; err != nil {
		t.Fatalf("改窗口失败: %v", err)
	}
	ok, err = repo.ClaimTransition(ctx, id, "rebooting", now, now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("超时后抢占报错: %v", err)
	}
	if !ok {
		t.Fatal("窗口已过（结果未知）时应允许新动作，否则一次丢回执会永久锁死实例")
	}
}

// TestClearTransitionIfOwner 验证回滚只清自己抢到的占位。
//
// 若不校验 owner：一个慢请求的失败回滚会清掉后来者已抢占的过渡态，
// 界面在动作执行中错误地恢复成空闲，用户又能再点一次。
func TestClearTransitionIfOwner(t *testing.T) {
	db, err := gorm.Open(postgres.Open(liveClaimDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}
	repo := NewInstanceRepository(db)
	ctx := context.Background()
	mark := fmt.Sprintf("claim-own-%d", time.Now().UnixNano()%1e9)

	id := createClaimInstance(t, db, mark)
	now := time.Now()

	if ok, err := repo.ClaimTransition(ctx, id, "rebooting", now, now.Add(3*time.Minute)); err != nil || !ok {
		t.Fatalf("抢占失败: ok=%v err=%v", ok, err)
	}
	// 用别的状态值回滚：不是自己抢的值，不得清除。
	if err := repo.ClearTransitionIfOwner(ctx, id, "starting"); err != nil {
		t.Fatalf("回滚报错: %v", err)
	}
	var state *string
	if err := db.Raw("SELECT transition_state FROM instances WHERE id = ?", id).Row().Scan(&state); err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if state == nil || *state != "rebooting" {
		t.Fatalf("非 owner 的清除不得生效，实际 %v", state)
	}

	// 用自己抢的值回滚：清除。
	if err := repo.ClearTransitionIfOwner(ctx, id, "rebooting"); err != nil {
		t.Fatalf("回滚报错: %v", err)
	}
	if err := db.Raw("SELECT transition_state FROM instances WHERE id = ?", id).Row().Scan(&state); err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if state != nil && *state != "" {
		t.Fatalf("owner 回滚应清空，实际 %q", *state)
	}
}
