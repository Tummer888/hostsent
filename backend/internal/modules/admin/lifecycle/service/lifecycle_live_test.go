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

	lifecycledto "hostsent/backend/internal/modules/admin/lifecycle/dto"
	lifecyclemodel "hostsent/backend/internal/modules/admin/lifecycle/model"
	lifecyclerepo "hostsent/backend/internal/modules/admin/lifecycle/repository"
	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
	"hostsent/backend/internal/pkg/upstream"
)

// liveLifecycleDSN 读取真实库连接串。
func liveLifecycleDSN() string {
	if dsn := os.Getenv("LIVE_DB_DSN"); dsn != "" {
		return dsn
	}
	return "host=127.0.0.1 port=5432 user=hostsent password=hostsent sslmode=disable"
}

// fakeLifecycleProvider 只实现暂停/恢复/终止三种能力，用于观察推进器是否真的调了上游。
type fakeLifecycleProvider struct {
	suspended   []string
	unsuspended []string
	terminated  []string
	resolveErr  error
}

func (f *fakeLifecycleProvider) GetType() string                   { return "fake" }
func (f *fakeLifecycleProvider) GetName() string                   { return "fake" }
func (f *fakeLifecycleProvider) HealthCheck(context.Context) error { return nil }
func (f *fakeLifecycleProvider) Capabilities() upstream.CapabilityDescriptor {
	return upstream.CapabilityDescriptor{Kind: "compute"}
}
func (f *fakeLifecycleProvider) SuspendInstance(_ context.Context, id, reason string) error {
	f.suspended = append(f.suspended, id)
	return nil
}
func (f *fakeLifecycleProvider) UnsuspendInstance(_ context.Context, id string) error {
	f.unsuspended = append(f.unsuspended, id)
	return nil
}
func (f *fakeLifecycleProvider) TerminateInstance(_ context.Context, id, reason string) error {
	f.terminated = append(f.terminated, id)
	return nil
}

// stageRecordCollector 收集推进器写入的阶段审计。
type stageRecordCollector struct {
	records []StageActionRecord
}

func (c *stageRecordCollector) RecordStageAction(_ context.Context, in StageActionRecord) error {
	c.records = append(c.records, in)
	return nil
}

// execSQL 执行一条维护用 SQL（测试内构造/恢复状态），失败即终止用例。
func execSQL(t *testing.T, db *gorm.DB, query string, args ...interface{}) {
	t.Helper()
	if err := db.Exec(query, args...).Error; err != nil {
		t.Fatalf("执行 SQL 失败: %v", err)
	}
}

// isolateInstances 把既有实例的 expire_at 临时置空，保证推进器只扫描本用例造的行。
// 返回恢复函数：无论用例成败都复原，避免污染开发库数据。
func isolateInstances(t *testing.T, db *gorm.DB) func() {
	t.Helper()
	type row struct {
		ID       uint64
		ExpireAt *time.Time
	}
	var snapshot []row
	if err := db.Raw("SELECT id, expire_at FROM instances WHERE expire_at IS NOT NULL").Scan(&snapshot).Error; err != nil {
		t.Fatalf("快照实例失败: %v", err)
	}
	if len(snapshot) > 0 {
		execSQL(t, db, "UPDATE instances SET expire_at = NULL WHERE expire_at IS NOT NULL")
	}
	restored := false
	restore := func() {
		if restored {
			return
		}
		restored = true
		for _, r := range snapshot {
			execSQL(t, db, "UPDATE instances SET expire_at = ? WHERE id = ?", r.ExpireAt, r.ID)
		}
	}
	t.Cleanup(restore)
	return restore
}

// createLifecycleInstance 造一个落在指定到期窗口的实例，返回实例记录 ID。
func createLifecycleInstance(t *testing.T, db *gorm.DB, mark string, expireAt time.Time, sourceMode string) uint64 {
	t.Helper()
	var id uint64
	err := db.Raw(`
		INSERT INTO instances (instance_id, provider_id, user_id, name, cpu, memory, disk, status,
			source_mode, provider_instance_id, billing_mode, expire_at, created_at, updated_at)
		VALUES (?, 1, 2, ?, 2, 4096, 40, 'running', ?, ?, 'monthly', ?, now(), now())
		RETURNING id`,
		mark, "P5 生命周期验收 "+mark, sourceMode, mark, expireAt).Scan(&id).Error
	if err != nil {
		t.Fatalf("创建实例 %s 失败: %v", mark, err)
	}
	t.Cleanup(func() { db.Exec("DELETE FROM instances WHERE id = ?", id) })
	return id
}

// TestLiveLifecycleAdvancer 验证 T5.4 阶段推进：
// 宽限只落阶段不调上游；暂停/销毁各调一次上游并落阶段；重复扫描不再调上游；续费后回退 active。
func TestLiveLifecycleAdvancer(t *testing.T) {
	db, err := gorm.Open(postgres.Open(liveLifecycleDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}
	ctx := context.Background()
	restore := isolateInstances(t, db)
	defer restore()

	policyRepo := lifecyclerepo.NewPolicyRepository(db)
	instanceRepo := lifecyclerepo.NewInstanceReader(db)
	policy, err := policyRepo.Get(ctx)
	if err != nil {
		t.Fatalf("读取策略失败: %v", err)
	}

	now := time.Now()
	graceID := createLifecycleInstance(t, db, fmt.Sprintf("p5-grace-%d", now.UnixNano()%1e6), now.AddDate(0, 0, -1), "self")
	suspID := createLifecycleInstance(t, db, fmt.Sprintf("p5-susp-%d", now.UnixNano()%1e6), now.AddDate(0, 0, -(policy.GraceDays+3)), "self")
	destID := createLifecycleInstance(t, db, fmt.Sprintf("p5-dest-%d", now.UnixNano()%1e6), now.AddDate(0, 0, -(policy.GraceDays+policy.DestroyKeepDays+5)), "self")

	fp := &fakeLifecycleProvider{}
	rec := &stageRecordCollector{}
	adv := NewLifecycleAdvancer(instanceRepo, policyRepo, func(context.Context, uint64) (upstream.Provider, error) {
		return fp, nil
	}, rec, zap.NewNop())

	stageOf := func(id uint64) string {
		var st string
		if err := db.Raw("SELECT COALESCE(lifecycle_stage,'') FROM instances WHERE id = ?", id).Scan(&st).Error; err != nil {
			t.Fatalf("读取阶段失败: %v", err)
		}
		return st
	}

	// 第一轮：grace 只落阶段；suspended/destroyed 各调一次上游。
	if _, err := adv.AdvanceOnce(ctx); err != nil {
		t.Fatalf("首轮推进失败: %v", err)
	}
	if got := stageOf(graceID); got != lifecyclemodel.StageGrace {
		t.Fatalf("grace 阶段期望 grace，实际 %q", got)
	}
	if got := stageOf(suspID); got != lifecyclemodel.StageSuspended {
		t.Fatalf("suspended 阶段期望 suspended，实际 %q", got)
	}
	if got := stageOf(destID); got != lifecyclemodel.StageDestroyed {
		t.Fatalf("destroyed 阶段期望 destroyed，实际 %q", got)
	}
	if len(fp.suspended) != 1 || len(fp.terminated) != 1 {
		t.Fatalf("首轮上游调用期望 suspend=1 terminate=1，实际 suspend=%d terminate=%d", len(fp.suspended), len(fp.terminated))
	}
	if len(rec.records) < 3 {
		t.Fatalf("阶段审计期望至少 3 条，实际 %d 条", len(rec.records))
	}

	// 第二轮：阶段已落库，必须零上游调用（幂等，不重复动作）。
	beforeSuspend, beforeTerminate := len(fp.suspended), len(fp.terminated)
	if _, err := adv.AdvanceOnce(ctx); err != nil {
		t.Fatalf("次轮推进失败: %v", err)
	}
	if len(fp.suspended) != beforeSuspend || len(fp.terminated) != beforeTerminate {
		t.Fatalf("重复扫描不应再调上游: suspend %d→%d, terminate %d→%d",
			beforeSuspend, len(fp.suspended), beforeTerminate, len(fp.terminated))
	}

	// 续费后回退：把已暂停实例的到期时间推后，推进器应解除暂停并复位 active。
	execSQL(t, db, "UPDATE instances SET expire_at = ? WHERE id = ?", now.AddDate(0, 1, 0), suspID)
	if _, err := adv.AdvanceOnce(ctx); err != nil {
		t.Fatalf("回退推进失败: %v", err)
	}
	if got := stageOf(suspID); got != lifecyclemodel.StageActive {
		t.Fatalf("续费后期望回退 active，实际 %q", got)
	}
	if len(fp.unsuspended) != 1 {
		t.Fatalf("回退期望调用恢复 1 次，实际 %d 次", len(fp.unsuspended))
	}
}

// TestLiveLifecycleCapabilityMissing 平台不具备暂停/销毁能力时：
// 必须落阶段并显式告警（不静默、不无限重试），且不阻断下一轮扫描。
func TestLiveLifecycleCapabilityMissing(t *testing.T) {
	db, err := gorm.Open(postgres.Open(liveLifecycleDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}
	ctx := context.Background()
	restore := isolateInstances(t, db)
	defer restore()

	policyRepo := lifecyclerepo.NewPolicyRepository(db)
	instanceRepo := lifecyclerepo.NewInstanceReader(db)
	policy, err := policyRepo.Get(ctx)
	if err != nil {
		t.Fatalf("读取策略失败: %v", err)
	}
	// 只实现电源能力的"伪平台"：暂停/恢复/终止全部缺失。
	bare := &bareProvider{}
	var warnings int
	adv := NewLifecycleAdvancer(instanceRepo, policyRepo, func(context.Context, uint64) (upstream.Provider, error) {
		return bare, nil
	}, &stageRecordCollector{}, zap.NewNop())
	adv.SetCapabilityNotifier(func(context.Context, *syncmodel.Instance, string, error) { warnings++ })

	now := time.Now()
	suspID := createLifecycleInstance(t, db, fmt.Sprintf("p5-nocap-%d", now.UnixNano()%1e6), now.AddDate(0, 0, -(policy.GraceDays+3)), "self")

	if _, err := adv.AdvanceOnce(ctx); err != nil {
		t.Fatalf("缺能力推进不应报错: %v", err)
	}
	var st string
	if err := db.Raw("SELECT COALESCE(lifecycle_stage,'') FROM instances WHERE id = ?", suspID).Scan(&st).Error; err != nil {
		t.Fatalf("读取阶段失败: %v", err)
	}
	if st != lifecyclemodel.StageSuspended {
		t.Fatalf("缺能力仍应落阶段 suspended（仅标记 + 人工跟进），实际 %q", st)
	}
	if warnings != 1 {
		t.Fatalf("缺能力应显式告警 1 次，实际 %d 次", warnings)
	}
	// 再次扫描不应重复告警（阶段已落库，不再进入候选）。
	if _, err := adv.AdvanceOnce(ctx); err != nil {
		t.Fatalf("次轮推进失败: %v", err)
	}
	if warnings != 1 {
		t.Fatalf("重复扫描不应重复告警，实际 %d 次", warnings)
	}
}

// bareProvider 仅提供 Provider 最小接口（无任何实例操作能力）。
type bareProvider struct{}

func (b *bareProvider) GetType() string                   { return "bare" }
func (b *bareProvider) GetName() string                   { return "bare" }
func (b *bareProvider) HealthCheck(context.Context) error { return nil }
func (b *bareProvider) Capabilities() upstream.CapabilityDescriptor {
	return upstream.CapabilityDescriptor{Kind: "compute"}
}

// TestLiveRenewalDualChain 验证 T5.2 双链路续费语义：
//   - 链路 A（upstream）：上游成功以上游账期为权威（sync_state=upstream_ok）；
//     上游缺能力/失败时必须报错且**不得只改本地账期**（sync_state=failed）；
//   - 链路 B（self）：平台无续费接口属预期，回落本地账期顺延（sync_state=local_only）。
func TestLiveRenewalDualChain(t *testing.T) {
	db, err := gorm.Open(postgres.Open(liveLifecycleDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 DB 失败: %v", err)
	}
	ctx := context.Background()

	// 用真实仓库构造续费服务；pricingSvc=nil 时按「单价 × 期数」算价（管理员代续费路径不涉及钱包）。
	svc := &renewalService{
		db:           db,
		renewalRepo:  lifecyclerepo.NewRenewalRepository(db),
		policyRepo:   lifecyclerepo.NewPolicyRepository(db),
		autoRepo:     lifecyclerepo.NewAutoRenewRepository(db),
		instanceRepo: lifecyclerepo.NewInstanceReader(db),
		orderWriter:  lifecyclerepo.NewOrderWriter(db),
		logger:       zap.NewNop(),
	}

	var productID uint64
	if err := db.Raw("INSERT INTO products (code, name, price, status, created_at, updated_at) VALUES (?, ?, 99, 1, now(), now()) RETURNING id",
		fmt.Sprintf("p5-renew-%d", time.Now().UnixNano()%1e6), "P5 续费验收商品").Scan(&productID).Error; err != nil {
		t.Fatalf("创建验收商品失败: %v", err)
	}
	t.Cleanup(func() { db.Exec("DELETE FROM products WHERE id = ?", productID) })

	createChainInstance := func(mark, sourceMode string, providerID uint64) uint64 {
		var id uint64
		base := time.Now().AddDate(0, 0, 5)
		err := db.Raw(`
			INSERT INTO instances (instance_id, provider_id, user_id, name, cpu, memory, disk, status,
				source_mode, sell_product_id, provider_instance_id, billing_mode, expire_at, created_at, updated_at)
			VALUES (?, ?, 2, ?, 2, 4096, 40, 'running', ?, ?, ?, 'monthly', ?, now(), now())
			RETURNING id`,
			mark, providerID, "P5 续费验收 "+mark, sourceMode, productID, mark, base).Scan(&id).Error
		if err != nil {
			t.Fatalf("创建实例 %s 失败: %v", mark, err)
		}
		t.Cleanup(func() {
			var orderID uint64
			db.Raw("SELECT COALESCE(order_id,0) FROM instance_renewals WHERE instance_id = ? ORDER BY id DESC LIMIT 1", id).Scan(&orderID)
			db.Exec("DELETE FROM instance_renewals WHERE instance_id = ?", id)
			db.Exec("DELETE FROM instances WHERE id = ?", id)
			if orderID > 0 {
				db.Exec("DELETE FROM orders WHERE id = ?", orderID)
			}
		})
		return id
	}

	expireOf := func(id uint64) *time.Time {
		var t0 *time.Time
		if err := db.Raw("SELECT expire_at FROM instances WHERE id = ?", id).Scan(&t0).Error; err != nil {
			t.Fatalf("读取到期时间失败: %v", err)
		}
		return t0
	}
	latestRenewal := func(id uint64) *lifecyclemodel.InstanceRenewal {
		var r lifecyclemodel.InstanceRenewal
		if err := db.Where("instance_id = ?", id).Order("id DESC").First(&r).Error; err != nil {
			t.Fatalf("读取续费记录失败: %v", err)
		}
		return &r
	}

	// 场景一：链路 A 上游成功，以上游返回账期为权威。
	chainA := createChainInstance(fmt.Sprintf("p5-a-ok-%d", time.Now().UnixNano()%1e6), syncmodel.SourceModeUpstream, 7)
	upstreamExpire := time.Now().AddDate(0, 2, 0).Truncate(time.Second)
	svc.SetUpstreamRenewer(func(context.Context, *syncmodel.Instance, int, string) (*upstream.RenewResult, error) {
		return &upstream.RenewResult{NewExpireAt: upstreamExpire, UpstreamOrderRef: "INV-9001"}, nil
	})
	if _, err := svc.AdminRenew(ctx, 1, chainA, &lifecycledto.AdminRenewRequest{PeriodCount: 1}); err != nil {
		t.Fatalf("链路 A 续费应成功: %v", err)
	}
	ra := latestRenewal(chainA)
	if ra.Status != lifecyclemodel.RenewalStatusSuccess || ra.SyncState != lifecyclemodel.RenewalSyncUpstreamOK {
		t.Fatalf("链路 A 期望 success/upstream_ok，实际 %s/%s", ra.Status, ra.SyncState)
	}
	if ra.UpstreamOrderID != "INV-9001" {
		t.Fatalf("链路 A 应记录上游回执，实际 %q", ra.UpstreamOrderID)
	}
	if got := expireOf(chainA); got == nil || !got.Truncate(time.Second).Equal(upstreamExpire) {
		t.Fatalf("链路 A 应以本地账期为准以上游账期为权威 %s，实际 %v", upstreamExpire, got)
	}

	// 场景二：链路 A 上游缺续费能力 → 必须失败，且本地账期不得变动。
	chainAFail := createChainInstance(fmt.Sprintf("p5-a-fail-%d", time.Now().UnixNano()%1e6), syncmodel.SourceModeUpstream, 7)
	beforeFail := expireOf(chainAFail)
	svc.SetUpstreamRenewer(func(context.Context, *syncmodel.Instance, int, string) (*upstream.RenewResult, error) {
		return nil, upstream.MissingCapability("mofangfinance", upstream.OpRenew)
	})
	if _, err := svc.AdminRenew(ctx, 1, chainAFail, &lifecycledto.AdminRenewRequest{PeriodCount: 1}); err == nil {
		t.Fatal("链路 A 上游缺能力时必须报错，实际成功")
	}
	rf := latestRenewal(chainAFail)
	if rf.Status != lifecyclemodel.RenewalStatusPending {
		t.Fatalf("失败后续费单应保持 pending 可重试，实际 %s", rf.Status)
	}
	if rf.SyncState != lifecyclemodel.RenewalSyncFailed {
		t.Fatalf("失败后 sync_state 期望 failed，实际 %q", rf.SyncState)
	}
	if after := expireOf(chainAFail); after == nil || !after.Equal(*beforeFail) {
		t.Fatalf("上游失败时本地账期不得变动：before=%v after=%v", beforeFail, after)
	}

	// 场景三：链路 B 平台无续费接口 → 本地账期顺延，sync_state=local_only。
	chainB := createChainInstance(fmt.Sprintf("p5-b-%d", time.Now().UnixNano()%1e6), syncmodel.SourceModeSelf, 1)
	beforeB := *expireOf(chainB)
	svc.SetUpstreamRenewer(func(context.Context, *syncmodel.Instance, int, string) (*upstream.RenewResult, error) {
		return nil, upstream.MissingCapability("mofangyun", upstream.OpRenew)
	})
	if _, err := svc.AdminRenew(ctx, 1, chainB, &lifecycledto.AdminRenewRequest{PeriodCount: 1}); err != nil {
		t.Fatalf("链路 B 应回落本地顺延并成功: %v", err)
	}
	rb := latestRenewal(chainB)
	if rb.Status != lifecyclemodel.RenewalStatusSuccess || rb.SyncState != lifecyclemodel.RenewalSyncLocalOnly {
		t.Fatalf("链路 B 期望 success/local_only，实际 %s/%s", rb.Status, rb.SyncState)
	}
	if after := *expireOf(chainB); !after.After(beforeB) {
		t.Fatalf("链路 B 本地账期应顺延: before=%v after=%v", beforeB, after)
	}
}
