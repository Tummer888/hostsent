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

	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
)

// TestPendingInstanceLifecycle 针对真实库验证「待开通实例占位」的三步生命周期：
// 下单建占位（幂等）→ 开通成功就地转正（不留第二行）→ 订单作废删除占位。
//
// 用户实测问题：开通失败后实例模块里什么都看不到、无法重新开通。
// 占位行是"待开通"可见性的载体，转正/删除的 SQL 语义必须与运行期一致。
//
// 仅在 -tags live 时运行：go test -tags live ./internal/modules/admin/resource/sync/repository/
func TestPendingInstanceLifecycle(t *testing.T) {
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Skipf("无可用测试库：%v", err)
	}
	// 事务包裹并回滚，避免污染真实库。
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("开启事务失败: %v", tx.Error)
	}
	defer tx.Rollback()

	ctx := context.Background()
	repo := NewSyncRepository(tx)

	const orderID = uint64(9999201)
	const userID = uint64(2236) // 取真实账号：instances.user_id 有外键
	placeholder := func() *syncmodel.Instance {
		return &syncmodel.Instance{
			InstanceID:        "pending-LIVETEST-9999201",
			ProviderID:        7,
			UserID:            userID,
			Name:              "live-test-pending",
			CPU:               4,
			Memory:            4096,
			Disk:              40,
			Status:            "creating",
			BillingMode:       "monthly",
			OrderID:           orderID,
			SourceMode:        "upstream",
			SellProductID:     176,
			UpstreamProductID: 120,
		}
	}

	// 1) 建占位（幂等：重复下单/重试不会建出第二行）。
	if err := repo.CreatePendingInstance(ctx, placeholder()); err != nil {
		t.Fatalf("建占位失败: %v", err)
	}
	if err := repo.CreatePendingInstance(ctx, placeholder()); err != nil {
		t.Fatalf("重复建占位应幂等无错: %v", err)
	}
	var n int64
	if err := tx.Model(&syncmodel.Instance{}).Where("order_id = ?", orderID).Count(&n).Error; err != nil {
		t.Fatalf("计数失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("占位行数 = %d, want 1（幂等失效会留下幽灵实例）", n)
	}

	// 2) 开通成功：占位行就地转正（实例号换成上游号、状态/规格/IP/账期一次补齐）。
	real := placeholder()
	real.InstanceID = "1999992001"
	real.ProviderInstanceID = "1999992001"
	real.Status = "running"
	real.PublicIP = "203.0.113.9"
	real.OS = "CentOS"
	adopted, err := repo.AdoptPendingInstance(ctx, orderID, real)
	if err != nil {
		t.Fatalf("转正失败: %v", err)
	}
	if !adopted {
		t.Fatal("转正未命中占位行：会出现「待开通」幽灵行 + 真实行两条记录")
	}
	var row syncmodel.Instance
	if err := tx.Where("order_id = ?", orderID).Take(&row).Error; err != nil {
		t.Fatalf("转正后查询失败: %v", err)
	}
	if row.InstanceID != "1999992001" || row.ProviderInstanceID != "1999992001" || row.Status != "running" || row.PublicIP != "203.0.113.9" {
		t.Fatalf("转正字段未写入: %+v", row)
	}

	// 3) 已转正的行不再被"取消占位"误动（订单取消/关单只清未开通的占位）。
	cancelled, err := repo.CancelPendingInstanceByOrder(ctx, orderID)
	if err != nil {
		t.Fatalf("取消占位失败: %v", err)
	}
	if cancelled != 0 {
		t.Fatal("已开通（已转正）的实例不得被占位取消改动")
	}

	// 4) 未开通的占位：标为已取消（不是删行——未付款的流失要在列表里可见）。
	second := placeholder()
	second.InstanceID = "pending-LIVETEST-9999202"
	second.OrderID = 9999202
	if err := repo.CreatePendingInstance(ctx, second); err != nil {
		t.Fatalf("建第二个占位失败: %v", err)
	}
	cancelled, err = repo.CancelPendingInstanceByOrder(ctx, 9999202)
	if err != nil {
		t.Fatalf("取消第二个占位失败: %v", err)
	}
	if cancelled != 1 {
		t.Fatalf("取消行数 = %d, want 1", cancelled)
	}
	// 行必须还在，且状态/阶段落 cancelled —— 这是「被取消」能被界面看到的前提。
	var cancelledRow syncmodel.Instance
	if err := tx.Where("order_id = ?", uint64(9999202)).Take(&cancelledRow).Error; err != nil {
		t.Fatalf("取消后应仍能查到该行（删行会让未付款流失在列表里消失）: %v", err)
	}
	if cancelledRow.Status != "cancelled" || cancelledRow.LifecycleStage != "cancelled" {
		t.Fatalf("取消后应为 status=cancelled + stage=cancelled，实际 status=%q stage=%q",
			cancelledRow.Status, cancelledRow.LifecycleStage)
	}
}

// TestUpsertInstancesKeepsProviderInstanceID upsert 不得用空值抹掉已知上游实例号。
//
// 实测回归：实例 1003（instance_id=2049）的 provider_instance_id 在一次
// 同步/开通写入后变成空串，此后该实例被当成"待开通"——详情页能力位全关、
// 出现"重新开通"按钮、回源刷新直接跳过（订单 832 的推进日志长期是
// "refresh skipped status: ”"）。
//
// 该列是"是否已在上游开通"的唯一判据，只有知道真实实例号的写入方才有权改它。
//
//	go test -tags live ./internal/modules/admin/resource/sync/repository/
func TestUpsertInstancesKeepsProviderInstanceID(t *testing.T) {
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
	repo := NewSyncRepository(tx)

	const mark = "livetest-upsert-keep-9999203"
	// 1) 已开通实例：provider_instance_id 已写入。
	row := &syncmodel.Instance{
		InstanceID: mark, ProviderID: 7, UserID: 2236, Name: "keep-id",
		Status: "running", SourceMode: "upstream", SellProductID: 175,
		ProviderInstanceID: "9999203",
	}
	if err := repo.UpsertInstances(ctx, []syncmodel.Instance{*row}); err != nil {
		t.Fatalf("首次写入失败: %v", err)
	}

	// 2) 同一 instance_id 再 upsert，但本次没带上游实例号：不得清空既有值。
	blank := *row
	blank.ProviderInstanceID = ""
	blank.Status = "creating"
	if err := repo.UpsertInstances(ctx, []syncmodel.Instance{blank}); err != nil {
		t.Fatalf("空实例号 upsert 失败: %v", err)
	}
	var got syncmodel.Instance
	if err := tx.Where("instance_id = ?", mark).Take(&got).Error; err != nil {
		t.Fatalf("回查失败: %v", err)
	}
	if got.ProviderInstanceID != "9999203" {
		t.Fatalf("provider_instance_id 被空值覆盖为 %q，实例会退化成「待开通」", got.ProviderInstanceID)
	}
	if got.Status != "creating" {
		t.Fatalf("其余列应正常被 upsert 更新：status=%q, want creating", got.Status)
	}

	// 3) 非空值仍可正常改写（如上游换绑实例号）。
	rebind := blank
	rebind.ProviderInstanceID = "9999213"
	if err := repo.UpsertInstances(ctx, []syncmodel.Instance{rebind}); err != nil {
		t.Fatalf("换绑 upsert 失败: %v", err)
	}
	if err := tx.Where("instance_id = ?", mark).Take(&got).Error; err != nil {
		t.Fatalf("换绑后回查失败: %v", err)
	}
	if got.ProviderInstanceID != "9999213" {
		t.Fatalf("非空实例号应被写入：got %q, want 9999213", got.ProviderInstanceID)
	}
}

// TestAdoptPendingSkipsDeletedPlaceholder 被运营清除的占位行不得在履约迟到成功时"复活"。
//
// 回归背景（本轮）：占位行现在允许被运营直接删除（用户实测"实例没办法删除"）。
// 而 OpterPendingInstance 的命中条件是 provider_instance_id = ”，被删的占位行同样满足 ——
// 若不排除，履约迟到成功会把它就地转正成 status=running 但 lifecycle_stage 仍是
// destroyed 的行；而终态行按"终态不复活"规则不再接受回源刷新，那台真机器
// 此后永远刷不出真实状态（列表里显示已销毁、实际在跑）。
//
// 正确行为：跳过已删除的占位 → 走常规 upsert 落一条**新行**，旧行保持已删除。
func TestAdoptPendingSkipsDeletedPlaceholder(t *testing.T) {
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
	repo := NewSyncRepository(tx)
	const orderID = uint64(9999301)

	placeholder := &syncmodel.Instance{
		InstanceID:        "pending-LIVETEST-9999301",
		ProviderID:        7,
		UserID:            2236,
		Name:              "live-test-deleted-pending",
		Status:            "creating",
		BillingMode:       "monthly",
		OrderID:           orderID,
		SourceMode:        "self",
		SellProductID:     176,
		UpstreamProductID: 120,
	}
	if err := repo.CreatePendingInstance(ctx, placeholder); err != nil {
		t.Fatalf("建占位失败: %v", err)
	}
	// 运营清除该占位行（与 destroyPendingPlaceholder 同样的落库方式）。
	if err := tx.Model(&syncmodel.Instance{}).
		Where("order_id = ? AND provider_instance_id = ''", orderID).
		Updates(map[string]any{"status": "deleted", "lifecycle_stage": "destroyed"}).Error; err != nil {
		t.Fatalf("标记占位已删除失败: %v", err)
	}

	// 新行必须清零主键：这里模拟的是"另插一行"（履约侧对未命中占位的走 upsert），
	// 带着占位行的 ID 会撞 instances_pkey，那是用例构造错误而非产品行为。
	real := *placeholder
	real.ID = 0
	real.InstanceID = "19999301"
	real.ProviderInstanceID = "19999301"
	real.Status = "running"
	adopted, err := repo.AdoptPendingInstance(ctx, orderID, &real)
	if err != nil {
		t.Fatalf("转正失败: %v", err)
	}
	if adopted {
		t.Fatal("已删除的占位行不得被就地转正：会得到 status=running 但 stage=destroyed 的怪行，且终态行不再接受回源刷新")
	}
	// 旧行保持已删除（运营的删除意图被尊重）。
	var old syncmodel.Instance
	if err := tx.Where("instance_id = ?", "pending-LIVETEST-9999301").Take(&old).Error; err != nil {
		t.Fatalf("回查旧占位失败: %v", err)
	}
	if old.Status != "deleted" {
		t.Fatalf("旧占位应保持 deleted，实际 %q", old.Status)
	}
	// 履约侧随后走常规 upsert：机器必须能被记录下来，不能因为占位被删就丢机器。
	if err := repo.UpsertInstances(ctx, []syncmodel.Instance{real}); err != nil {
		t.Fatalf("常规 upsert 失败: %v", err)
	}
	var fresh syncmodel.Instance
	if err := tx.Where("instance_id = ?", "19999301").Take(&fresh).Error; err != nil {
		t.Fatalf("新行未落库：占位删除后履约成功必须仍能记录机器: %v", err)
	}
	if fresh.Status != "running" {
		t.Fatalf("新行状态应为 running，实际 %q", fresh.Status)
	}
	if fresh.LifecycleStage == "destroyed" {
		t.Fatal("新行不得继承已删除占位的 destroyed 阶段（终态不复活规则会让它永远刷不出状态）")
	}
}

// TestUpsertInstancesKeepsLocalExpireWhenUpstreamSilent 上游不返回到期时，同步不得抹掉本地账期。
//
// 用户口径（自营链路权威模型）：**到期日由我方设定** —— 自营实例的账期来自订单周期
// 与我方的续费动作，魔方云不提供到期字段（dcimcloud 适配器从不填
// StandardInstance.ExpireAt）。而实例同步（scope=instance，默认 300 秒一轮）
// 的 upsert 对 expire_at 是无条件覆盖 `excluded.expire_at`：
// 适配器没给值 → 指针为 nil → 写进去就是 NULL。
//
// 后果：一台正常在用的自营机器，每 5 分钟被同步清一次账期；
// 接着生命周期推进器看到 expire_at 为空（它靠 expire_at 判定到期），
// 「即将到期」列表与到期提醒全部失效，被清成 NULL 的行再也回不到账期视图里。
//
// 修法与本文件既有的 provider_instance_id / credential_password 同一套路：
// **空值不得覆盖已有值**（只在本次确实带到期时间时才写）。
//
//	go test -tags live ./internal/modules/admin/resource/sync/repository/
func TestUpsertInstancesKeepsLocalExpireWhenUpstreamSilent(t *testing.T) {
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
	repo := NewSyncRepository(tx)

	const mark = "livetest-upsert-keep-expire-9999204"
	expire := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	// 1) 自营实例：账期由我方设定（订单/续费写入），上游实例号已知。
	row := &syncmodel.Instance{
		InstanceID: mark, ProviderID: 61, UserID: 2236, Name: "keep-expire",
		Status: "running", SourceMode: "self", SellProductID: 175,
		ProviderInstanceID: "9999204", ExpireAt: &expire,
	}
	if err := repo.UpsertInstances(ctx, []syncmodel.Instance{*row}); err != nil {
		t.Fatalf("首次写入失败: %v", err)
	}

	// 2) 模拟实例同步：魔方云只回状态/规格，不返回到期时间（ExpireAt 为零值）。
	synced := *row
	synced.Name = "keep-expire-renamed"
	synced.ExpireAt = nil
	if err := repo.UpsertInstances(ctx, []syncmodel.Instance{synced}); err != nil {
		t.Fatalf("同步 upsert 失败: %v", err)
	}
	var got syncmodel.Instance
	if err := tx.Where("instance_id = ?", mark).Take(&got).Error; err != nil {
		t.Fatalf("回查失败: %v", err)
	}
	if got.ExpireAt == nil {
		t.Fatal("本地账期被同步抹成 NULL：自营到期日由我方设定，上游不返回到期时不得覆盖")
	}
	if !got.ExpireAt.Equal(expire) {
		t.Fatalf("本地账期被改动：got %v, want %v", got.ExpireAt, expire)
	}
	if got.Name != "keep-expire-renamed" {
		t.Fatalf("其余列应正常同步：name=%q", got.Name)
	}

	// 3) 上游确实返回到期时仍可正常改写（对接链路的上游账期是权威）。
	later := expire.AddDate(0, 1, 0)
	withExpire := synced
	withExpire.ExpireAt = &later
	if err := repo.UpsertInstances(ctx, []syncmodel.Instance{withExpire}); err != nil {
		t.Fatalf("带到期 upsert 失败: %v", err)
	}
	if err := tx.Where("instance_id = ?", mark).Take(&got).Error; err != nil {
		t.Fatalf("带到期后回查失败: %v", err)
	}
	if got.ExpireAt == nil || !got.ExpireAt.Equal(later) {
		t.Fatalf("上游返回到期时应可改写：got %v, want %v", got.ExpireAt, later)
	}
}
