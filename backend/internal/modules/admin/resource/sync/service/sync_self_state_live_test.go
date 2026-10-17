//go:build live

package service

import (
	"context"
	"os"
	"strconv"
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

// flakyInstanceAdmin 可控的分页实例清单：pages 是**逐页**返回的结果（按请求的 page
// 取对应下标），用来验证真分页与清单可信度判据。
//
// ignorePage=true 模拟"适配器不认 page 参数"：无论请求哪一页都返回同一页，
// 此前会让引擎在第二页看到重复数据却以为拉全了。
type flakyInstanceAdmin struct {
	pages      [][]*pkgmodel.StandardInstance
	ignorePage bool
	err        error
	calls      []map[string]string
}

func (f *flakyInstanceAdmin) ListInstances(_ context.Context, filters map[string]string) ([]*pkgmodel.StandardInstance, error) {
	f.calls = append(f.calls, filters)
	if f.err != nil {
		return nil, f.err
	}
	if f.ignorePage {
		if len(f.pages) == 0 {
			return nil, nil
		}
		return f.pages[0], nil
	}
	page := 1
	if v := filters["page"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	if page > len(f.pages) {
		return nil, nil
	}
	return f.pages[page-1], nil
}

func (f *flakyInstanceAdmin) DeleteInstance(context.Context, string) error { return nil }
func (f *flakyInstanceAdmin) ResizeInstance(context.Context, string, *pkgmodel.StandardProductSpec) error {
	return nil
}

type flakyProvider struct{ admin *flakyInstanceAdmin }

func (p *flakyProvider) GetType() string { return "flaky" }
func (p *flakyProvider) GetName() string { return "flaky" }
func (p *flakyProvider) Capabilities() upstream.CapabilityDescriptor {
	return upstream.CapabilityDescriptor{SyncScopes: []string{syncmodel.ScopeInstance}}
}
func (p *flakyProvider) HealthCheck(context.Context) error { return nil }
func (p *flakyProvider) ListInstances(ctx context.Context, f map[string]string) ([]*pkgmodel.StandardInstance, error) {
	return p.admin.ListInstances(ctx, f)
}
func (p *flakyProvider) DeleteInstance(ctx context.Context, id string) error {
	return p.admin.DeleteInstance(ctx, id)
}
func (p *flakyProvider) ResizeInstance(ctx context.Context, id string, sp *pkgmodel.StandardProductSpec) error {
	return p.admin.ResizeInstance(ctx, id, sp)
}

// liveSyncFixture 建一个事务内的渠道 + 一台可用用户，返回清理后的引擎。
func liveSyncFixture(t *testing.T) (*gorm.DB, uint64, uint64, *SyncEngine) {
	t.Helper()
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
	t.Cleanup(func() { tx.Rollback() })

	var providerID uint64
	if err := tx.Raw(`INSERT INTO resource_providers
		(name, provider_type, api_endpoint, api_key, api_secret, status, sync_enabled, sync_paused, kind, consecutive_failures)
		VALUES ('selfstateprobe','selfstateprobe','http://probe.invalid','k','s',1,true,false,'compute',0)
		RETURNING id`).Scan(&providerID).Error; err != nil {
		t.Fatalf("建渠道失败: %v", err)
	}
	var userID uint64
	if err := tx.Raw(`SELECT id FROM users WHERE status='active' AND deleted_at IS NULL ORDER BY id LIMIT 1`).
		Scan(&userID).Error; err != nil || userID == 0 {
		t.Skipf("库中没有可用用户（userID=%d err=%v）", userID, err)
	}
	engine := &SyncEngine{
		syncRepo:      syncrepo.NewSyncRepository(tx),
		frameworkRepo: syncrepo.NewFrameworkRepository(tx),
		providerRepo:  providerrepo.NewProviderRepository(tx),
		logger:        zap.NewNop(),
	}
	return tx, providerID, userID, engine
}

// seedInstance 插入一条本地实例行（合成数据，绝不碰真实实例）。
func seedInstance(t *testing.T, tx *gorm.DB, providerID, userID uint64, instanceID, providerInstanceID, status, stage string) {
	t.Helper()
	if err := tx.Exec(`INSERT INTO instances
		(instance_id, provider_id, user_id, name, cpu, memory, disk, status, source_mode,
		 provider_instance_id, lifecycle_stage, billing_mode, created_at, updated_at)
		VALUES (?, ?, ?, ?, 2, 2048, 40, ?, 'self', ?, ?, 'monthly', now(), now())`,
		instanceID, providerID, userID, "probe-"+instanceID, status, providerInstanceID, stage).Error; err != nil {
		t.Fatalf("插入实例 %s 失败: %v", instanceID, err)
	}
}

func readStatus(t *testing.T, tx *gorm.DB, instanceID string) (status, stage, transition string) {
	t.Helper()
	var row struct {
		Status          string
		LifecycleStage  string
		TransitionState string
	}
	if err := tx.Raw(`SELECT COALESCE(status,'') AS status,
		COALESCE(lifecycle_stage,'') AS lifecycle_stage,
		COALESCE(transition_state,'') AS transition_state
		FROM instances WHERE instance_id = ?`, instanceID).Scan(&row).Error; err != nil {
		t.Fatalf("回查实例 %s 失败: %v", instanceID, err)
	}
	return row.Status, row.LifecycleStage, row.TransitionState
}

// TestLiveSyncMarksPanelDeletedAsDeleted 面板侧删除必须同步到本地（用户第二十三轮口径）。
//
// 场景：自营实例在魔方云面板被删了，上游清单里不再出现。
// 修复前：MarkInstancesOffline 硬编码 source_mode='upstream'，自营行永远不被标记，
// 本地继续显示"运行中"（幽灵机）；且即便标记了，落的是只写不读的 offline。
func TestLiveSyncMarksPanelDeletedAsDeleted(t *testing.T) {
	tx, providerID, userID, engine := liveSyncFixture(t)

	seedInstance(t, tx, providerID, userID, "selfstate-keep", "selfstate-keep", "running", "")
	seedInstance(t, tx, providerID, userID, "selfstate-gone", "selfstate-gone", "running", "")

	admin := &flakyInstanceAdmin{pages: [][]*pkgmodel.StandardInstance{{
		{UpstreamID: "selfstate-keep", Name: "keep", Status: pkgmodel.InstanceStatusRunning},
	}}}
	task := &syncmodel.SyncTask{ProviderID: providerID, TaskType: syncmodel.ScopeInstance}
	if err := engine.syncInstances(context.Background(), task, &flakyProvider{admin: admin}); err != nil {
		t.Fatalf("syncInstances 失败: %v", err)
	}

	if st, _, _ := readStatus(t, tx, "selfstate-keep"); st != "running" {
		t.Errorf("仍在上游的实例状态不该被改，实得 %q", st)
	}
	st, stage, transition := readStatus(t, tx, "selfstate-gone")
	if st != string(pkgmodel.InstanceStatusDeleted) {
		t.Errorf("面板已删除的实例应同步为 deleted，实得 %q（自营未纳入缺席判定 / 仍落 offline）", st)
	}
	if stage != "" {
		t.Errorf("面板删除不得改写生命周期阶段（阶段是到期处置进度），实得 %q", stage)
	}
	if transition != "" {
		t.Errorf("面板删除应清过渡态（否则展示层会把「已删除」盖成「重启中」），实得 %q", transition)
	}
	// 差异明细里要能看到这次标记（运营在同步中心核对）。
	var diffCount int64
	if err := tx.Raw(`SELECT count(*) FROM sync_diffs
		WHERE provider_id = ? AND scope = ? AND action = ? AND external_id = ?`,
		providerID, syncmodel.ScopeInstance, syncmodel.DiffOffline, "selfstate-gone").Scan(&diffCount).Error; err != nil {
		t.Fatalf("回查差异失败: %v", err)
	}
	if diffCount != 1 {
		t.Errorf("应写一条 offline 差异（供同步中心展示），实得 %d 条", diffCount)
	}
}

// TestLiveSyncKeepsTransitionalStatus 过渡态保护：本地刚下发关机、上游还没转过来时，
// 列表同步不得把前瞻状态冲回旧值。
//
// 回归背景：运营点完关机，下一轮同步（默认 300 秒）把 status 变回 running，
// 界面看起来像关机没生效；详情轮询同样会被打回。
func TestLiveSyncKeepsTransitionalStatus(t *testing.T) {
	tx, providerID, userID, engine := liveSyncFixture(t)

	seedInstance(t, tx, providerID, userID, "selfstate-stop", "selfstate-stop", "stopped", "")
	if err := tx.Exec(`UPDATE instances SET transition_state = ?, transition_started_at = now(),
		transition_until = now() + interval '3 minutes' WHERE instance_id = ?`,
		pkgmodel.InstanceTransitionStopping, "selfstate-stop").Error; err != nil {
		t.Fatalf("写过渡态失败: %v", err)
	}

	// 上游仍报 running（动作尚未生效）。
	admin := &flakyInstanceAdmin{pages: [][]*pkgmodel.StandardInstance{{
		{UpstreamID: "selfstate-stop", Name: "stop", Status: pkgmodel.InstanceStatusRunning},
	}}}
	task := &syncmodel.SyncTask{ProviderID: providerID, TaskType: syncmodel.ScopeInstance}
	if err := engine.syncInstances(context.Background(), task, &flakyProvider{admin: admin}); err != nil {
		t.Fatalf("syncInstances 失败: %v", err)
	}
	if st, _, _ := readStatus(t, tx, "selfstate-stop"); st != "stopped" {
		t.Errorf("过渡未确认时应保留本地前瞻状态 stopped，实得 %q（上游旧值把它冲掉了）", st)
	}

	// 上游到达目标状态 stopped：这次允许采纳（并保留过渡态到窗口结束）。
	admin.pages = [][]*pkgmodel.StandardInstance{{
		{UpstreamID: "selfstate-stop", Name: "stop", Status: pkgmodel.InstanceStatusStopped},
	}}
	if err := engine.syncInstances(context.Background(), task, &flakyProvider{admin: admin}); err != nil {
		t.Fatalf("syncInstances 失败: %v", err)
	}
	if st, _, _ := readStatus(t, tx, "selfstate-stop"); st != "stopped" {
		t.Errorf("上游已到 stopped，本地应保持 stopped，实得 %q", st)
	}
}

// TestLiveSyncAbsenceGuard 空清单 / 分页不全时绝不判缺席。
//
// 回归背景：面板账号权限不足或接口降级会返回空 data（HTTP 200），照单全收会把该渠道
// 所有实例一次性标成「已删除」—— 满屏幽灵删除，而机器还好端端在跑。
func TestLiveSyncAbsenceGuard(t *testing.T) {
	ctx := context.Background()

	t.Run("空清单不标记", func(t *testing.T) {
		tx, providerID, userID, engine := liveSyncFixture(t)
		seedInstance(t, tx, providerID, userID, "selfstate-guard1", "selfstate-guard1", "running", "")
		admin := &flakyInstanceAdmin{pages: nil}
		task := &syncmodel.SyncTask{ProviderID: providerID, TaskType: syncmodel.ScopeInstance}
		if err := engine.syncInstances(ctx, task, &flakyProvider{admin: admin}); err != nil {
			t.Fatalf("syncInstances 失败: %v", err)
		}
		if st, _, _ := readStatus(t, tx, "selfstate-guard1"); st != "running" {
			t.Errorf("空清单不得把实例标成已删除，实得 %q", st)
		}
	})

	t.Run("分页触顶不标记", func(t *testing.T) {
		tx, providerID, userID, engine := liveSyncFixture(t)
		seedInstance(t, tx, providerID, userID, "selfstate-guard2", "selfstate-guard2", "running", "")
		// 构造 1000 页满页（触 instanceListMaxPages），清单不可信。
		fullPage := make([]*pkgmodel.StandardInstance, 0, instanceListPageSize)
		for i := 0; i < instanceListPageSize; i++ {
			fullPage = append(fullPage, &pkgmodel.StandardInstance{
				UpstreamID: "bulk-" + itoa(i), Name: "bulk", Status: pkgmodel.InstanceStatusRunning,
			})
		}
		admin := &flakyInstanceAdmin{ignorePage: true, pages: [][]*pkgmodel.StandardInstance{fullPage}}
		task := &syncmodel.SyncTask{ProviderID: providerID, TaskType: syncmodel.ScopeInstance}
		if err := engine.syncInstances(ctx, task, &flakyProvider{admin: admin}); err != nil {
			t.Fatalf("syncInstances 失败: %v", err)
		}
		if st, _, _ := readStatus(t, tx, "selfstate-guard2"); st != "running" {
			t.Errorf("清单不可信时不得判缺席，实得 %q", st)
		}
		// 适配器忽略 page 时必须在第二页就收手：否则每轮全量对账要打 1000 次
		// 上游（上限页数），把面板拖垮。
		if len(admin.calls) > 2 {
			t.Errorf("适配器不认 page 时应尽快收手，实际请求 %d 次", len(admin.calls))
		}
	})
}

// TestLiveSyncFollowsSecondPage 超过一页的清单必须翻页拉全，否则第 101 台起的机器
// 永远同步不到、也永远判不了删除。
func TestLiveSyncFollowsSecondPage(t *testing.T) {
	tx, providerID, userID, engine := liveSyncFixture(t)

	// 本地第二页上的实例（上游清单里在 page=2）。
	seedInstance(t, tx, providerID, userID, "selfstate-p2", "selfstate-p2", "running", "")

	page1 := make([]*pkgmodel.StandardInstance, 0, instanceListPageSize)
	for i := 0; i < instanceListPageSize; i++ {
		page1 = append(page1, &pkgmodel.StandardInstance{
			UpstreamID: "page1-" + itoa(i), Name: "p1", Status: pkgmodel.InstanceStatusRunning,
		})
	}
	page2 := []*pkgmodel.StandardInstance{
		{UpstreamID: "selfstate-p2", Name: "p2", Status: pkgmodel.InstanceStatusStopped},
	}
	admin := &flakyInstanceAdmin{pages: [][]*pkgmodel.StandardInstance{page1, page2}}
	task := &syncmodel.SyncTask{ProviderID: providerID, TaskType: syncmodel.ScopeInstance}
	if err := engine.syncInstances(context.Background(), task, &flakyProvider{admin: admin}); err != nil {
		t.Fatalf("syncInstances 失败: %v", err)
	}

	if len(admin.calls) < 2 {
		t.Fatalf("应至少请求两页，实际 %d 次：%+v", len(admin.calls), admin.calls)
	}
	// 第二页的实例被同步（状态取自 page2），且**没有**被判成已删除。
	st, _, _ := readStatus(t, tx, "selfstate-p2")
	if st != "stopped" {
		t.Errorf("第二页实例应被同步为 stopped，实得 %q（说明只拉了第一页）", st)
	}
}

// TestLiveSyncKeepsStatusWhenUpstreamSilent 上游没给电源状态时保留本地值。
//
// 回归背景：dcimcloud 的 buildInstance 此前把缺失的 status 默认填成 running。
// 列表接口一旦裁剪/漏返该字段（旧版面板、网关），每 5 分钟一轮的实例同步会把
// 一批已关机的机器刷回「运行中」，运维看到的状态与实际相反。
func TestLiveSyncKeepsStatusWhenUpstreamSilent(t *testing.T) {
	tx, providerID, userID, engine := liveSyncFixture(t)
	seedInstance(t, tx, providerID, userID, "selfstate-silent", "selfstate-silent", "stopped", "")

	// 上游返回空状态（字段缺失）。
	admin := &flakyInstanceAdmin{pages: [][]*pkgmodel.StandardInstance{{
		{UpstreamID: "selfstate-silent", Name: "silent", Status: ""},
	}}}
	task := &syncmodel.SyncTask{ProviderID: providerID, TaskType: syncmodel.ScopeInstance}
	if err := engine.syncInstances(context.Background(), task, &flakyProvider{admin: admin}); err != nil {
		t.Fatalf("syncInstances 失败: %v", err)
	}
	if st, _, _ := readStatus(t, tx, "selfstate-silent"); st != "stopped" {
		t.Errorf("上游未给状态时应保留本地 stopped，实得 %q（被默认值刷成运行中）", st)
	}
}

// TestLiveSyncUsesProviderInstanceID 缺席判定按上游实例号匹配，不是本地展示号。
//
// 「按上游号重新关联且未迁移展示号」会把两者分开（instance_id=本地号，
// provider_instance_id=上游号）。用展示号比对会把一台仍在上游的机器判成已删除。
func TestLiveSyncUsesProviderInstanceID(t *testing.T) {
	tx, providerID, userID, engine := liveSyncFixture(t)

	// 本地展示号 selfstate-local，上游号 up-777。
	seedInstance(t, tx, providerID, userID, "selfstate-local", "up-777", "running", "")

	admin := &flakyInstanceAdmin{pages: [][]*pkgmodel.StandardInstance{{
		{UpstreamID: "up-777", Name: "relinked", Status: pkgmodel.InstanceStatusStopped},
	}}}
	task := &syncmodel.SyncTask{ProviderID: providerID, TaskType: syncmodel.ScopeInstance}
	if err := engine.syncInstances(context.Background(), task, &flakyProvider{admin: admin}); err != nil {
		t.Fatalf("syncInstances 失败: %v", err)
	}

	st, _, _ := readStatus(t, tx, "selfstate-local")
	if st == string(pkgmodel.InstanceStatusDeleted) {
		t.Fatal("按展示号比对会把这台仍在上游的实例判成已删除（匹配键必须用 provider_instance_id）")
	}
	if st != "stopped" {
		t.Errorf("上游报 stopped，本地应同步为 stopped，实得 %q（说明同步没认到既有行）", st)
	}
	// 且不得因为展示号不同而另插一行。
	var rowCount int64
	if err := tx.Raw(`SELECT count(*) FROM instances WHERE provider_id = ? AND provider_instance_id = ?`,
		providerID, "up-777").Scan(&rowCount).Error; err != nil {
		t.Fatalf("回查行数失败: %v", err)
	}
	if rowCount != 1 {
		t.Errorf("同一台上游机器应只有一行，实得 %d 行（同步按上游号另插了行）", rowCount)
	}
}

func itoa(i int) string { return strconv.Itoa(i) }
