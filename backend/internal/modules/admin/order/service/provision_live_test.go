//go:build live

package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	orderrepo "hostsent/backend/internal/modules/admin/order/repository"
)

// liveDSN 读取真实库连接串（与其他 live 用例一致）。
func liveDSN() string {
	if dsn := os.Getenv("LIVE_DB_DSN"); dsn != "" {
		return dsn
	}
	return "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
}

// stubActivator 记录调用次数的开通执行器；可按序号返回错误。
type stubActivator struct {
	mu    sync.Mutex
	calls int
	failN int // 前 N 次调用返回错误
	err   error
}

func (s *stubActivator) Activate(ctx context.Context, id uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.calls <= s.failN {
		if s.err != nil {
			return s.err
		}
		return errors.New("上游开通超时（stub）")
	}
	return nil
}

func (s *stubActivator) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// execSQL 执行一条维护用 SQL（测试内构造/清理状态），失败即终止用例。
func execSQL(t *testing.T, db *gorm.DB, query string, args ...interface{}) {
	t.Helper()
	if err := db.Exec(query, args...).Error; err != nil {
		t.Fatalf("执行 SQL 失败: %v", err)
	}
}

// createProvisionOrder 造一张待开通的订单（履约任务只依赖 order_id/user_id，无需真实商品）。
func createProvisionOrder(t *testing.T, db *gorm.DB, userID uint64) *ordermodel.Order {
	t.Helper()
	order := &ordermodel.Order{
		OrderNo:     fmt.Sprintf("PV%s%06d", time.Now().Format("20060102150405"), time.Now().UnixNano()%1000000),
		UserID:      userID,
		ProductName: "P5 履约验收商品",
		Quantity:    1,
		TotalAmount: 10,
		PaidAmount:  10,
		Status:      ordermodel.OrderStatusPaid,
		// price_snapshot 为 jsonb 列，空串非法，需显式给空数组。
		PriceSnapshot: "[]",
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("创建验收订单失败: %v", err)
	}
	t.Cleanup(func() { db.Exec("DELETE FROM orders WHERE id = ?", order.ID) })
	return order
}

// TestLiveProvisionTaskLifecycle 验证 T5.1 开通任务的幂等投递、原子领取、退避重试与转人工。
func TestLiveProvisionTaskLifecycle(t *testing.T) {
	db, err := gorm.Open(postgres.Open(liveDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}
	ctx := context.Background()
	repo := orderrepo.NewProvisionTaskRepository(db)
	const userID = 2

	// 1) 幂等投递：同一订单重复 Enqueue 只保留一行。
	order := createProvisionOrder(t, db, userID)
	task := &ordermodel.ProvisionTask{
		OrderID: order.ID, UserID: userID, ProductID: 0, SourceMode: "self",
		Status: ordermodel.ProvisionTaskPending, MaxAttempts: 3,
	}
	for i := 0; i < 3; i++ {
		if err := repo.Enqueue(ctx, task); err != nil {
			t.Fatalf("第 %d 次投递失败: %v", i+1, err)
		}
	}
	defer db.Exec("DELETE FROM provision_tasks WHERE order_id = ?", order.ID)
	var rows int64
	if err := db.Model(&ordermodel.ProvisionTask{}).Where("order_id = ?", order.ID).Count(&rows).Error; err != nil {
		t.Fatalf("统计任务行失败: %v", err)
	}
	if rows != 1 {
		t.Fatalf("幂等投递期望 1 行，实际 %d 行", rows)
	}

	// 2) 原子领取：attempts 累加、租约生效，租约内不能再次领取。
	claimed, err := repo.Claim(ctx, time.Minute)
	if err != nil || claimed == nil {
		t.Fatalf("领取任务失败: task=%v err=%v", claimed, err)
	}
	if claimed.OrderID != order.ID || claimed.Attempts != 1 || claimed.Status != ordermodel.ProvisionTaskRunning {
		t.Fatalf("领取结果异常: %+v", claimed)
	}
	if again, _ := repo.Claim(ctx, time.Minute); again != nil {
		t.Fatalf("租约内不应再领到任务，实际领到 %+v", again)
	}

	// 3) 失败退避：MarkRetry 后 locked_until 作为"下次可领取时间"，立即领取应落空。
	next := time.Now().Add(time.Hour)
	if err := repo.MarkRetry(ctx, claimed.ID, "stub 失败", next); err != nil {
		t.Fatalf("标记重试失败: %v", err)
	}
	if got, _ := repo.Claim(ctx, time.Minute); got != nil {
		t.Fatalf("退避期内不应领到任务，实际 %+v", got)
	}

	// 4) 重复投递失败任务应重置为 pending 并清空退避，可立即再次领取。
	if err := repo.Enqueue(ctx, task); err != nil {
		t.Fatalf("重新投递失败: %v", err)
	}
	reclaimed, err := repo.Claim(ctx, time.Minute)
	if err != nil || reclaimed == nil {
		t.Fatalf("重新投递后应可领取: task=%v err=%v", reclaimed, err)
	}
	if reclaimed.Attempts != 2 {
		t.Fatalf("attempts 期望 2，实际 %d", reclaimed.Attempts)
	}

	// 5) 达到上限转人工队列（终态，不再被领取）。
	if err := repo.MarkManual(ctx, reclaimed.ID, "连续失败"); err != nil {
		t.Fatalf("标记人工队列失败: %v", err)
	}
	if got, _ := repo.Claim(ctx, time.Minute); got != nil {
		t.Fatalf("人工队列任务不应被领取，实际 %+v", got)
	}
}

// TestLiveProvisionWorkerProcess 验证工作池单任务处理：失败退避、达上限转人工并告警、成功收尾。
func TestLiveProvisionWorkerProcess(t *testing.T) {
	db, err := gorm.Open(postgres.Open(liveDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}
	ctx := context.Background()
	repo := orderrepo.NewProvisionTaskRepository(db)
	const userID = 2

	// 场景一：连续失败到上限 → manual + 告警。
	order := createProvisionOrder(t, db, userID)
	defer db.Exec("DELETE FROM provision_tasks WHERE order_id = ?", order.ID)
	if err := repo.Enqueue(ctx, &ordermodel.ProvisionTask{
		OrderID: order.ID, UserID: userID, Status: ordermodel.ProvisionTaskPending, MaxAttempts: 2,
	}); err != nil {
		t.Fatalf("投递失败: %v", err)
	}

	var notified int
	activator := &stubActivator{failN: 99}
	worker := NewProvisionWorker(repo, activator, func(context.Context, *ordermodel.ProvisionTask) { notified++ },
		zap.NewNop(), ProvisionWorkerOptions{BackoffMin: time.Millisecond})

	for round := 1; round <= 2; round++ {
		task, err := repo.Claim(ctx, time.Minute)
		if err != nil || task == nil {
			t.Fatalf("第 %d 轮领取失败: task=%v err=%v", round, task, err)
		}
		if task.Attempts != round {
			t.Fatalf("第 %d 轮 attempts 期望 %d，实际 %d", round, round, task.Attempts)
		}
		// 退避时间置为过去，使下一轮可立即领取（等价于等待退避结束）。
		worker.process(ctx, task)
		execSQL(t, db, "UPDATE provision_tasks SET locked_until = now() - interval '1 minute' WHERE id = ?", task.ID)
	}
	final, err := repo.FindByOrderID(ctx, order.ID)
	if err != nil {
		t.Fatalf("读取任务失败: %v", err)
	}
	if final.Status != ordermodel.ProvisionTaskManual {
		t.Fatalf("连续失败后期望 manual，实际 %s（last_error=%s）", final.Status, final.LastError)
	}
	if notified != 1 {
		t.Fatalf("转人工应告警 1 次，实际 %d 次", notified)
	}

	// 场景二：一次失败后成功 → success。
	order2 := createProvisionOrder(t, db, userID)
	defer db.Exec("DELETE FROM provision_tasks WHERE order_id = ?", order2.ID)
	if err := repo.Enqueue(ctx, &ordermodel.ProvisionTask{
		OrderID: order2.ID, UserID: userID, Status: ordermodel.ProvisionTaskPending, MaxAttempts: 5,
	}); err != nil {
		t.Fatalf("投递失败: %v", err)
	}
	activator2 := &stubActivator{failN: 1}
	worker2 := NewProvisionWorker(repo, activator2, nil, zap.NewNop(), ProvisionWorkerOptions{BackoffMin: time.Millisecond})
	for round := 1; round <= 2; round++ {
		task, err := repo.Claim(ctx, time.Minute)
		if err != nil || task == nil {
			t.Fatalf("第 %d 轮领取失败: task=%v err=%v", round, task, err)
		}
		worker2.process(ctx, task)
		execSQL(t, db, "UPDATE provision_tasks SET locked_until = now() - interval '1 minute' WHERE id = ?", task.ID)
	}
	final2, err := repo.FindByOrderID(ctx, order2.ID)
	if err != nil {
		t.Fatalf("读取任务失败: %v", err)
	}
	if final2.Status != ordermodel.ProvisionTaskSuccess {
		t.Fatalf("二次成功后期望 success，实际 %s", final2.Status)
	}
	if activator2.callCount() != 2 {
		t.Fatalf("激活器调用期望 2 次，实际 %d 次", activator2.callCount())
	}
}

// TestLiveProvisionStaleReap 验证租约过期的 running 任务被回收重试（进程崩溃兜底）。
func TestLiveProvisionStaleReap(t *testing.T) {
	db, err := gorm.Open(postgres.Open(liveDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}
	ctx := context.Background()
	repo := orderrepo.NewProvisionTaskRepository(db)

	order := createProvisionOrder(t, db, 2)
	defer db.Exec("DELETE FROM provision_tasks WHERE order_id = ?", order.ID)
	if err := repo.Enqueue(ctx, &ordermodel.ProvisionTask{
		OrderID: order.ID, UserID: 2, Status: ordermodel.ProvisionTaskPending, MaxAttempts: 3,
	}); err != nil {
		t.Fatalf("投递失败: %v", err)
	}
	if _, err := repo.Claim(ctx, time.Minute); err != nil {
		t.Fatalf("领取失败: %v", err)
	}
	// 模拟进程崩溃：租约已过期但状态仍 running。
	execSQL(t, db, "UPDATE provision_tasks SET locked_until = now() - interval '1 hour' WHERE order_id = ?", order.ID)
	// 领取查询本身即回收过期 running 任务（WHERE 分支内联了 running+过期），故可直接领到。
	reclaimed, err := repo.Claim(ctx, time.Minute)
	if err != nil || reclaimed == nil {
		t.Fatalf("过期租约任务应可被回收领取: task=%v err=%v", reclaimed, err)
	}
	if reclaimed.ID == 0 || reclaimed.Status != ordermodel.ProvisionTaskRunning {
		t.Fatalf("回收结果异常: %+v", reclaimed)
	}
	if n, err := repo.ReleaseStale(ctx, time.Now()); err != nil {
		t.Fatalf("回收残留失败: %v", err)
	} else if n != 0 {
		t.Fatalf("刚领取的任务不应被 ReleaseStale 回收，实际回收 %d 行", n)
	}
}
