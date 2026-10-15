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
	"gorm.io/gorm/logger"
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

// TestPurgeRefusesNonTerminalAtRepositoryLevel Purge 的终态守卫必须在仓库层也成立。
//
// 为什么两层都要（而不是只留服务层）：Purge 是**物理删除**，仓库层是最后一道闸门。
// 若只有服务层守卫，任何绕过服务层的新调用点（后台任务、脚本、将来新增的批处理）
// 都能直接把一台在跑的机器从库里抹掉 —— 而"通知渠道终止机器"那一步就永远不会发生，
// 本地干净、渠道侧继续计费。判据写在 DELETE 语句里（不是先查后删），
// 并发下"查完到删之间状态被改回"的窗口也随之关闭。
func TestPurgeRefusesNonTerminalAtRepositoryLevel(t *testing.T) {
	db, err := gorm.Open(postgres.Open(liveClaimDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Skipf("无可用测试库：%v", err)
	}
	ctx := context.Background()
	repo := NewInstanceRepository(db)
	mark := fmt.Sprintf("p18-purge-%d", time.Now().UnixNano()%1e6)

	var id uint64
	if err := db.Raw(`
		INSERT INTO instances (instance_id, provider_id, user_id, name, cpu, memory, disk, status,
			source_mode, provider_instance_id, billing_mode, order_id, created_at, updated_at)
		VALUES (?, 1, 2, ?, 1, 1024, 20, 'running', 'self', ?, 'monthly', 0, now(), now())
		RETURNING id`, mark, "p18 仓库层守卫 "+mark, mark).Scan(&id).Error; err != nil {
		t.Fatalf("造实例失败: %v", err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM instance_operations WHERE instance_id = ?", id)
		db.Exec("DELETE FROM instances WHERE id = ?", id)
	})

	// 在跑（非终态）→ 仓库层必须拒绝：影响行数为 0，且行还在。
	affected, err := repo.Purge(ctx, id)
	if err != nil {
		t.Fatalf("Purge 不应报错（拒绝通过 0 行表达）: %v", err)
	}
	if affected != 0 {
		t.Fatalf("非终态行被删除了（affected=%d）：这会绕过「销毁」那一步", affected)
	}
	var n int64
	db.Raw("SELECT count(*) FROM instances WHERE id = ?", id).Scan(&n)
	if n != 1 {
		t.Fatal("被拒绝的 Purge 不得真的删掉行")
	}

	// 落终态后 → 允许删除。
	if err := db.Exec("UPDATE instances SET status = 'deleted', lifecycle_stage = 'destroyed' WHERE id = ?", id).Error; err != nil {
		t.Fatalf("置终态失败: %v", err)
	}
	affected, err = repo.Purge(ctx, id)
	if err != nil {
		t.Fatalf("终态行 Purge 失败: %v", err)
	}
	if affected != 1 {
		t.Fatalf("终态行应被删除，affected=%d", affected)
	}
}
