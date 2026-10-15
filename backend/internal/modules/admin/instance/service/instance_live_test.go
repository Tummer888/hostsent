//go:build live

package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/instance/dto"
	"hostsent/backend/internal/modules/admin/instance/repository"
	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// liveInstanceDSN 读取真实库连接串（与 lifecycle live 测试同一约定）。
func liveInstanceDSN() string {
	if dsn := os.Getenv("LIVE_DB_DSN"); dsn != "" {
		return dsn
	}
	return "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
}

// fakeControlProvider 只实现 InstanceControl + InstanceSuspension + InstanceTermination，
// 用于观察批量/退款联动是否真的调到了上游。
//
// 计数用原子量而不是 append 到切片：并发用例里多个 goroutine 会同时进
// StartInstance，裸 append 在竞态下会丢写 —— 那会让"上游被调用几次"的断言
// 得到假阳性（实测：非原子的抢占实现也能通过用例）。
type fakeControlProvider struct {
	started    []string
	stopped    []string
	suspended  []string
	terminated []string
	failStart  bool

	mu          sync.Mutex
	startCalls  int32 // 每次调用都计数（含失败）
	startOK     int32 // 只计成功下发（"上游真的收到几条指令"用它）
	stopCalls   int32
	restartCall int32
}

func (f *fakeControlProvider) GetType() string                   { return "fake" }
func (f *fakeControlProvider) GetName() string                   { return "fake" }
func (f *fakeControlProvider) HealthCheck(context.Context) error { return nil }
func (f *fakeControlProvider) Capabilities() upstream.CapabilityDescriptor {
	return upstream.CapabilityDescriptor{Kind: "compute"}
}
func (f *fakeControlProvider) GetInstance(context.Context, string) (*pkgmodel.StandardInstance, error) {
	return &pkgmodel.StandardInstance{Status: pkgmodel.InstanceStatusRunning}, nil
}
func (f *fakeControlProvider) StartInstance(_ context.Context, id string) error {
	atomic.AddInt32(&f.startCalls, 1)
	if f.failStart {
		return fmt.Errorf("upstream unreachable")
	}
	atomic.AddInt32(&f.startOK, 1)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = append(f.started, id)
	return nil
}
func (f *fakeControlProvider) StopInstance(_ context.Context, id string, _ bool) error {
	atomic.AddInt32(&f.stopCalls, 1)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = append(f.stopped, id)
	return nil
}
func (f *fakeControlProvider) RestartInstance(_ context.Context, _ string) error {
	atomic.AddInt32(&f.restartCall, 1)
	return nil
}
func (f *fakeControlProvider) VNC(context.Context, string) (upstream.VNCResult, error) {
	return upstream.VNCResult{}, nil
}
func (f *fakeControlProvider) SuspendInstance(_ context.Context, id, _ string) error {
	f.suspended = append(f.suspended, id)
	return nil
}
func (f *fakeControlProvider) UnsuspendInstance(_ context.Context, _ string) error { return nil }
func (f *fakeControlProvider) TerminateInstance(_ context.Context, id, _ string) error {
	f.terminated = append(f.terminated, id)
	return nil
}

// createOpsInstance 造一台实例用于批量/退款用例。
func createOpsInstance(t *testing.T, db *gorm.DB, mark string, status string, orderID uint64) uint64 {
	t.Helper()
	var id uint64
	err := db.Raw(`
		INSERT INTO instances (instance_id, provider_id, user_id, name, cpu, memory, disk, status,
			source_mode, provider_instance_id, billing_mode, order_id, created_at, updated_at)
		VALUES (?, 1, 2, ?, 2, 4096, 40, ?, 'self', ?, 'monthly', ?, now(), now())
		RETURNING id`, mark, "批量/退款验收 "+mark, status, mark, orderID).Scan(&id).Error
	if err != nil {
		t.Fatalf("创建实例 %s 失败: %v", mark, err)
	}
	t.Cleanup(func() {
		// 先清流水再删实例：instance_operations 记的是操作语义，用例造的行不应留在真实库里。
		db.Exec("DELETE FROM instance_operations WHERE instance_id = ?", id)
		db.Exec("DELETE FROM instances WHERE id = ?", id)
	})
	return id
}

func newOpsService(t *testing.T, db *gorm.DB, fake *fakeControlProvider) InstanceService {
	t.Helper()
	svc := NewInstanceService(
		repository.NewInstanceRepository(db),
		repository.NewOperationRepository(db),
		repository.NewRelatedRepository(db),
		func(context.Context, uint64) (upstream.Provider, error) { return fake, nil },
		zap.NewNop(),
	)
	return svc
}

// TestLiveBatchAction 验证批量运维的三态归类：
// 已在目标状态 → skipped（不算事故）；真实上游失败 → failed；成功 → success。
func TestLiveBatchAction(t *testing.T) {
	db, err := gorm.Open(postgres.Open(liveInstanceDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}
	ctx := context.Background()
	mark := time.Now().UnixNano() % 1e6

	runningID := createOpsInstance(t, db, fmt.Sprintf("p1-batch-run-%d", mark), "running", 0)
	stoppedID := createOpsInstance(t, db, fmt.Sprintf("p1-batch-stop-%d", mark), "stopped", 0)
	missingID := uint64(999999999)

	fake := &fakeControlProvider{}
	svc := newOpsService(t, db, fake)

	// 场景一：对「运行中 + 已关机 + 不存在的 ID」批量开机。
	// 期望：stopped 成功、running 跳过（已在运行中）、不存在 失败；上游只被调用一次。
	resp, err := svc.BatchAction(ctx, Operator{Type: "admin", Name: "batch-test"}, &dto.BatchActionRequest{
		Action: "on",
		IDs:    []uint64{runningID, stoppedID, missingID},
	})
	if err != nil {
		t.Fatalf("批量开机失败: %v", err)
	}
	if resp.Total != 3 || resp.Succeeded != 1 || resp.Skipped != 1 || resp.Failed != 1 {
		t.Fatalf("批量结果期望 3/1/1/1，实际 total=%d success=%d skip=%d fail=%d",
			resp.Total, resp.Succeeded, resp.Skipped, resp.Failed)
	}
	if len(fake.started) != 1 || fake.started[0] != fmt.Sprintf("p1-batch-stop-%d", mark) {
		t.Fatalf("上游开机期望只调用已关机那台，实际 %v", fake.started)
	}

	// 场景二：上游真实失败 → failed（与 skipped 区分开）。
	// 场景二：上游真实失败 → failed（与 skipped 区分开）。
	// 用新的已关机实例：上一场景已把 stoppedID 开机，再点开机会被正确判为 skipped。
	failID := createOpsInstance(t, db, fmt.Sprintf("p1-batch-fail-%d", mark), "stopped", 0)
	fake.failStart = true
	resp2, err := svc.BatchAction(ctx, Operator{Type: "admin"}, &dto.BatchActionRequest{
		Action: "on",
		IDs:    []uint64{failID},
	})
	if err != nil {
		t.Fatalf("批量开机（失败场景）不应整体报错: %v", err)
	}
	if resp2.Failed != 1 || resp2.Items[0].Status != "failed" {
		t.Fatalf("上游失败应归 failed，实际 %+v", resp2.Items)
	}

	// 场景三：非法动作整体拒绝。
	if _, err := svc.BatchAction(ctx, Operator{Type: "admin"}, &dto.BatchActionRequest{
		Action: "nuke", IDs: []uint64{stoppedID},
	}); err == nil {
		t.Fatal("非法批量动作应被拒绝")
	}
}

// TestLiveDisposeOnRefund 验证退款联动策略：
// none（默认）不动作；suspend 仅对全额退款生效；部分退款不动机器。
func TestLiveDisposeOnRefund(t *testing.T) {
	db, err := gorm.Open(postgres.Open(liveInstanceDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}
	ctx := context.Background()
	mark := time.Now().UnixNano() % 1e6
	const orderID = uint64(880000000000)

	id := createOpsInstance(t, db, fmt.Sprintf("p1-refund-%d", mark), "running", orderID)

	fake := &fakeControlProvider{}
	svc := newOpsService(t, db, fake)

	statusOf := func() string {
		var st string
		if err := db.Raw("SELECT status FROM instances WHERE id = ?", id).Scan(&st).Error; err != nil {
			t.Fatalf("读取状态失败: %v", err)
		}
		return st
	}
	in := &dto.RefundInstanceRequest{
		OrderID: orderID, OrderNo: "UC-TEST", UserID: 2,
		RefundNo: "RF-TEST", RefundAmount: 100, PaidAmount: 100, FullRefund: true,
	}

	// 策略未注入（nil）→ 视为 none：不动作。
	if err := svc.DisposeOnRefund(ctx, in); err != nil {
		t.Fatalf("策略 none 时不应报错: %v", err)
	}
	if len(fake.suspended) != 0 || statusOf() != "running" {
		t.Fatalf("策略 none 时不得动实例：suspended=%v status=%s", fake.suspended, statusOf())
	}

	// 策略 destroy + 部分退款 → 仍不动（部分退款是补偿，机器还在用）。
	svc.SetRefundPolicyReader(func(context.Context) string { return "destroy" })
	partial := *in
	partial.FullRefund = false
	if err := svc.DisposeOnRefund(ctx, &partial); err != nil {
		t.Fatalf("部分退款不应报错: %v", err)
	}
	if len(fake.terminated) != 0 || statusOf() != "running" {
		t.Fatalf("部分退款不得处置实例：terminated=%v status=%s", fake.terminated, statusOf())
	}

	// 策略 suspend + 全额退款 → 真暂停。
	svc.SetRefundPolicyReader(func(context.Context) string { return "suspend" })
	if err := svc.DisposeOnRefund(ctx, in); err != nil {
		t.Fatalf("suspend 策略应成功: %v", err)
	}
	if len(fake.suspended) != 1 || statusOf() != "stopped" {
		t.Fatalf("suspend 策略应暂停实例：suspended=%v status=%s", fake.suspended, statusOf())
	}
	// 审计：暂停流水里必须带退款单号，事后能反查。
	var cnt int64
	db.Raw("SELECT count(*) FROM instance_operations WHERE instance_id = ? AND action = 'suspend' AND params LIKE '%RF-TEST%'", id).Scan(&cnt)
	if cnt == 0 {
		t.Fatal("退款处置必须写带退款单号的 suspend 流水")
	}
}

// TestLiveConcurrentPowerDoubleSubmit 验证"连点两次电源操作"只下发一次（端到端）。
//
// 用户实测：在实例详情页连续点两次「开机」，操作记录里两条开机都成功了 ——
// 上游因此堆了两条同样指令。
//
// 注意：本用例走完整服务层，覆盖"用户看到的行为"（一次成功 + 其余 20003），
// 但**不是**抢占原语的证明 —— 抢占之前还有一次 FindByID，那次读会把窗口收窄。
// 原语本身的互斥性由 repository 包的 TestClaimTransitionIsAtomic 直接钉住
// （那条用例在非原子实现下会真实失败，已实测）。
func TestLiveConcurrentPowerDoubleSubmit(t *testing.T) {
	db, err := gorm.Open(postgres.Open(liveInstanceDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}
	ctx := context.Background()
	mark := time.Now().UnixNano() % 1e6

	id := createOpsInstance(t, db, fmt.Sprintf("p2-race-%d", mark), "stopped", 0)

	fake := &fakeControlProvider{}
	svc := newOpsService(t, db, fake)

	// 8 个并发请求同时开机：模拟用户连点 + 多标签页重复提交。
	const goroutines = 8
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start // 同一瞬间开跑，最大化并发窗口
			errs[idx] = svc.Power(ctx, Operator{Type: "admin", Name: "race-test"}, id, "on")
		}(i)
	}
	close(start)
	wg.Wait()

	var okCount, conflictCount int
	for _, e := range errs {
		switch {
		case e == nil:
			okCount++
		case errors.Is(e, ErrStatusConflict):
			conflictCount++
		default:
			t.Errorf("非预期错误类型: %v", e)
		}
	}

	if okCount != 1 {
		t.Fatalf("并发开机应只有 1 次成功，实际成功 %d 次（冲突 %d）", okCount, conflictCount)
	}
	if conflictCount != goroutines-1 {
		t.Fatalf("其余 %d 个请求都应被拒绝（20003），实际 %d", goroutines-1, conflictCount)
	}
	// 核心断言：上游只收到一条指令 —— 这正是用户看到的"任务累积"。
	if got := atomic.LoadInt32(&fake.startCalls); got != 1 {
		t.Fatalf("上游应只收到 1 条开机指令，实际 %d 条", got)
	}

	// 过渡态恰好一个，且是 starting（抢占写入的）。
	var state string
	var startedAt, until *time.Time
	if err := db.Raw("SELECT transition_state, transition_started_at, transition_until FROM instances WHERE id = ?", id).
		Row().Scan(&state, &startedAt, &until); err != nil {
		t.Fatalf("读取过渡态失败: %v", err)
	}
	if state != pkgmodel.InstanceTransitionStarting {
		t.Fatalf("过渡态应为 %s，实际 %q", pkgmodel.InstanceTransitionStarting, state)
	}
	if startedAt == nil || until == nil {
		t.Fatal("过渡态的时间列必须一并写入")
	}
}

// TestLiveFailedDispatchReleasesClaim 验证"抢到了但下发失败"会把占位还回去。
//
// 若不还：实例会被一个从未执行成功的动作锁到窗口超时，运营在 3 分钟内
// 什么都做不了 —— 比不锁更糟。
func TestLiveFailedDispatchReleasesClaim(t *testing.T) {
	db, err := gorm.Open(postgres.Open(liveInstanceDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}
	ctx := context.Background()
	mark := time.Now().UnixNano() % 1e6

	id := createOpsInstance(t, db, fmt.Sprintf("p2-release-%d", mark), "stopped", 0)
	fake := &fakeControlProvider{failStart: true}
	svc := newOpsService(t, db, fake)

	if err := svc.Power(ctx, Operator{Type: "admin"}, id, "on"); err == nil {
		t.Fatal("上游失败时应返回错误")
	}
	var state *string
	if err := db.Raw("SELECT transition_state FROM instances WHERE id = ?", id).Row().Scan(&state); err != nil {
		t.Fatalf("读取过渡态失败: %v", err)
	}
	if state != nil && *state != "" {
		t.Fatalf("下发失败的占位必须回滚，实际残留 %q", *state)
	}

	// 回滚后立即可重试成功（不该被上一轮的占位拦住）。
	fake.failStart = false
	if err := svc.Power(ctx, Operator{Type: "admin"}, id, "on"); err != nil {
		t.Fatalf("占位回滚后应能重试成功: %v", err)
	}
	if got := atomic.LoadInt32(&fake.startOK); got != 1 {
		t.Fatalf("重试后应恰好 1 条指令成功下发，实际 %d", got)
	}
	if got := atomic.LoadInt32(&fake.startCalls); got != 2 {
		t.Fatalf("应共调用上游 2 次（1 次失败 + 1 次重试成功），实际 %d", got)
	}
}
