//go:build live

package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/instance/dto"
	instancedto "hostsent/backend/internal/modules/admin/instance/dto"
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
type fakeControlProvider struct {
	started    []string
	stopped    []string
	suspended  []string
	terminated []string
	failStart  bool
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
	if f.failStart {
		return fmt.Errorf("upstream unreachable")
	}
	f.started = append(f.started, id)
	return nil
}
func (f *fakeControlProvider) StopInstance(_ context.Context, id string, _ bool) error {
	f.stopped = append(f.stopped, id)
	return nil
}
func (f *fakeControlProvider) RestartInstance(_ context.Context, id string) error { return nil }
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
	in := &instancedto.RefundInstanceRequest{
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
