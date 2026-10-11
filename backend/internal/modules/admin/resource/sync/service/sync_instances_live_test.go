//go:build live

package service

import (
	"context"
	"os"
	"testing"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	providerrepo "hostsent/backend/internal/modules/admin/resource/provider/repository"
	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
	syncrepo "hostsent/backend/internal/modules/admin/resource/sync/repository"
	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// stubInstanceAdmin 返回固定实例列表，且不填 UserID——与 mofangyun 适配器现状一致
// （面板 user_id 与本地 users 主键不同空间，适配器无从映射）。
type stubInstanceAdmin struct {
	items []*pkgmodel.StandardInstance
}

func (s *stubInstanceAdmin) ListInstances(context.Context, map[string]string) ([]*pkgmodel.StandardInstance, error) {
	return s.items, nil
}
func (s *stubInstanceAdmin) DeleteInstance(context.Context, string) error { return nil }
func (s *stubInstanceAdmin) ResizeInstance(context.Context, string, *pkgmodel.StandardProductSpec) error {
	return nil
}

// stubProvider 只需满足 upstream.Provider（Descriptor 驱动 scope 判定）。
type stubProvider struct{ admin *stubInstanceAdmin }

func (s *stubProvider) GetType() string { return "stub" }
func (s *stubProvider) GetName() string { return "stub" }
func (s *stubProvider) Capabilities() upstream.CapabilityDescriptor {
	return upstream.CapabilityDescriptor{SyncScopes: []string{syncmodel.ScopeInstance}}
}
func (s *stubProvider) HealthCheck(context.Context) error { return nil }
func (s *stubProvider) ListInstances(ctx context.Context, f map[string]string) ([]*pkgmodel.StandardInstance, error) {
	return s.admin.ListInstances(ctx, f)
}
func (s *stubProvider) DeleteInstance(ctx context.Context, id string) error {
	return s.admin.DeleteInstance(ctx, id)
}
func (s *stubProvider) ResizeInstance(ctx context.Context, id string, sp *pkgmodel.StandardProductSpec) error {
	return s.admin.ResizeInstance(ctx, id, sp)
}

// TestSyncInstancesOwnership 针对真实库验证实例同步的归属落库：
//  1. 适配器不填 UserID 的新实例不再撞 fk_instances_user（23503），任务转为成功；
//  2. 既有自营实例的 user_id / source_mode 不被同步改写（否则实例会从用户控制台消失）；
//  3. 同步写入的实例归属为合法账号（>0 且存在于 users）。
//
// 仅在 -tags live 时运行：go test -tags live ./internal/modules/admin/resource/sync/service/
func TestSyncInstancesOwnership(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Skipf("无可用测试库：%v", err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("开启事务失败: %v", tx.Error)
	}
	defer tx.Rollback()

	ctx := context.Background()
	var providerID uint64
	if err := tx.Raw(`INSERT INTO resource_providers
		(name, provider_type, api_endpoint, api_key, api_secret, status, sync_enabled, sync_paused, kind, consecutive_failures)
		VALUES ('syncownprobe','syncownprobe','http://probe.invalid','k','s',1,true,false,'compute',0) RETURNING id`).
		Scan(&providerID).Error; err != nil {
		t.Fatalf("建渠道失败: %v", err)
	}

	// 造两条上游实例，一条全新、一条本地已有自营归属（模拟我方订单开通的机器）。
	admin := &stubInstanceAdmin{items: []*pkgmodel.StandardInstance{
		{UpstreamID: "syncown-new-1", Name: "probe-new", Status: pkgmodel.InstanceStatusRunning,
			RawData: map[string]interface{}{"id": "syncown-new-1"}},
		{UpstreamID: "syncown-self-1", Name: "probe-self", Status: pkgmodel.InstanceStatusRunning,
			RawData: map[string]interface{}{"id": "syncown-self-1"}},
	}}
	// 既有自营行：user_id 取一个真实用户，source_mode=self。
	var realUser uint64
	if err := tx.Raw(`SELECT id FROM users WHERE status='active' AND deleted_at IS NULL ORDER BY id LIMIT 1`).Scan(&realUser).Error; err != nil || realUser == 0 {
		t.Skipf("库中没有可用用户，跳过（realUser=%d err=%v）", realUser, err)
	}
	if err := tx.Exec(`INSERT INTO instances
		(instance_id, provider_id, user_id, name, cpu, memory, disk, status, source_mode, sell_product_id, upstream_product_id, billing_mode, created_at, updated_at)
		VALUES ('syncown-self-1', ?, ?, 'probe-self', 2, 2048, 40, 'running', 'self', 777, 0, 'monthly', now(), now())`,
		providerID, realUser).Error; err != nil {
		t.Fatalf("造既有自营实例失败: %v", err)
	}

	engine := &SyncEngine{
		syncRepo:      syncrepo.NewSyncRepository(tx),
		frameworkRepo: syncrepo.NewFrameworkRepository(tx),
		providerRepo:  providerrepo.NewProviderRepository(tx),
		logger:        zap.NewNop(),
	}
	task := &syncmodel.SyncTask{ProviderID: providerID, TaskType: syncmodel.ScopeInstance}
	if err := engine.syncInstances(ctx, task, &stubProvider{admin: admin}); err != nil {
		t.Fatalf("syncInstances 失败（归属未映射会撞 23503）: %v", err)
	}

	type row struct {
		InstanceID string
		UserID     uint64
		SourceMode string
	}
	var rows []row
	if err := tx.Raw(`SELECT instance_id, user_id, source_mode FROM instances
		WHERE provider_id = ? ORDER BY instance_id`, providerID).Scan(&rows).Error; err != nil {
		t.Fatalf("回查实例失败: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("应写入 2 条实例，实得 %d：%+v", len(rows), rows)
	}
	byID := map[string]row{}
	for _, r := range rows {
		byID[r.InstanceID] = r
	}

	if got := byID["syncown-new-1"]; got.UserID == 0 {
		t.Errorf("新实例归属仍为 0（会撞 fk_instances_user）：%+v", got)
	}
	var ownerExists int64
	if err := tx.Raw(`SELECT count(*) FROM users WHERE id = ?`, byID["syncown-new-1"].UserID).Scan(&ownerExists).Error; err != nil {
		t.Fatalf("校验归属账号失败: %v", err)
	}
	if ownerExists != 1 {
		t.Errorf("新实例归属账号 %d 不存在于 users", byID["syncown-new-1"].UserID)
	}

	self := byID["syncown-self-1"]
	if self.UserID != realUser {
		t.Errorf("既有自营实例归属被改写：%d → %d", realUser, self.UserID)
	}
	if self.SourceMode != "self" {
		t.Errorf("既有自营实例链路语义被改写：self → %q", self.SourceMode)
	}
	t.Logf("新实例归属=%d（合法账号），自营实例归属=%d 链路=%s 均未被覆盖",
		byID["syncown-new-1"].UserID, self.UserID, self.SourceMode)
}
