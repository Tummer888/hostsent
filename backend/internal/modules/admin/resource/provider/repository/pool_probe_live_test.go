//go:build live

package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	providermodel "hostsent/backend/internal/modules/admin/resource/provider/model"
	providerrepo "hostsent/backend/internal/modules/admin/resource/provider/repository"
)

// newPoolProbeTx 连测试库并开一个最后回滚的事务，返回 tx 与 ctx。
func newPoolProbeTx(t *testing.T) (*gorm.DB, context.Context) {
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
	return tx, context.Background()
}

// insertProbeProvider 插入一个临时渠道并返回 id。
func insertProbeProvider(t *testing.T, tx *gorm.DB, name string) uint64 {
	t.Helper()
	var id uint64
	if err := tx.Raw(`INSERT INTO resource_providers
		(name, provider_type, api_endpoint, api_key, api_secret, status, sync_enabled, kind, consecutive_failures)
		VALUES (?, 'probe', 'http://probe.invalid', 'k', 's', 1, true, 'upstream', 0) RETURNING id`, name).
		Scan(&id).Error; err != nil {
		t.Fatalf("插入渠道失败: %v", err)
	}
	return id
}

// TestUpdatePoolProbesLive 校验探针列回写：一条 UPDATE ... FROM (VALUES ...) 同时更新
// 多个池，且 probe_at 的类型推断正确。
//
// 曾实测失败：裸占位符在 VALUES 里被 PG 统一推断成 text，与 probe_at(timestamptz) 冲突
// → "column probe_at is of type timestamp with time zone but expression is of type text"，
// 整个 pool 同步任务连败。这条用例把 VALUES 的显式类型转换钉住。
func TestUpdatePoolProbesLive(t *testing.T) {
	tx, ctx := newPoolProbeTx(t)
	providerID := insertProbeProvider(t, tx, "probeprobe")

	for _, id := range []string{"node:1", "node:2"} {
		if err := tx.Exec(`INSERT INTO resource_pools (provider_id, upstream_id, name, pool_type, status)
			VALUES (?, ?, 'node', 'node', 1)`, providerID, id).Error; err != nil {
			t.Fatalf("插入池 %s 失败: %v", id, err)
		}
	}

	sampled := time.Now().UTC().Truncate(time.Second)
	repo := providerrepo.NewPoolRepository(tx)
	if err := repo.UpdatePoolProbes(ctx, providerID, []providerrepo.PoolProbe{
		{UpstreamID: "node:1", Status: "healthy", Message: "连接成功", At: sampled},
		{UpstreamID: "node:2", Status: "warning", Message: "状态采样失败", At: sampled},
	}); err != nil {
		t.Fatalf("UpdatePoolProbes 失败: %v", err)
	}

	var rows []struct {
		UpstreamID   string
		ProbeStatus  string
		ProbeMessage string
		ProbeAt      *time.Time
	}
	if err := tx.Raw(`SELECT upstream_id, probe_status, probe_message, probe_at
		FROM resource_pools WHERE provider_id = ? ORDER BY upstream_id`, providerID).Scan(&rows).Error; err != nil {
		t.Fatalf("回查失败: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("得到 %d 行, want 2", len(rows))
	}
	if rows[0].ProbeStatus != "healthy" || rows[0].ProbeMessage != "连接成功" || rows[0].ProbeAt == nil {
		t.Errorf("node:1 探针列不对: %+v", rows[0])
	}
	if rows[1].ProbeStatus != "warning" || rows[1].ProbeAt == nil {
		t.Errorf("node:2 探针列不对: %+v", rows[1])
	}
	t.Logf("两行探针结论均写入，probe_at=%v（timestamptz 类型推断正确）", rows[0].ProbeAt.Format(time.RFC3339))
}

// TestUpsertPoolsKeepsProbesLive 校验 UpsertPools 不覆盖探针列：
// 没有实时通道的池（区域）每轮都走 UpsertPools，若它连 probe_* 一起写，
// 就会把节点池由 UpdatePoolProbes 写入的结论清零。
func TestUpsertPoolsKeepsProbesLive(t *testing.T) {
	tx, ctx := newPoolProbeTx(t)
	providerID := insertProbeProvider(t, tx, "probeupsert")

	if err := tx.Exec(`INSERT INTO resource_pools
		(provider_id, upstream_id, name, pool_type, status, probe_status, probe_message, probe_at)
		VALUES (?, 'node:9', 'old', 'node', 1, 'healthy', '既有结论', now())`, providerID).Error; err != nil {
		t.Fatalf("插入池失败: %v", err)
	}

	repo := providerrepo.NewPoolRepository(tx)
	// 只带普通列的池行（探针列为零值）——正是无实时通道的池的样子。
	rows := []providermodel.ResourcePool{{
		ProviderID: providerID, UpstreamID: "node:9", Name: "new", PoolType: "node", Status: 1,
	}}
	if err := repo.UpsertPools(ctx, providerID, rows); err != nil {
		t.Fatalf("UpsertPools 失败: %v", err)
	}

	var got struct {
		Name         string
		ProbeStatus  string
		ProbeMessage string
	}
	if err := tx.Raw(`SELECT name, probe_status, probe_message FROM resource_pools
		WHERE provider_id = ? AND upstream_id = 'node:9'`, providerID).Scan(&got).Error; err != nil {
		t.Fatalf("回查失败: %v", err)
	}
	if got.Name != "new" {
		t.Errorf("普通列应被更新为 new，实得 %q", got.Name)
	}
	if got.ProbeStatus != "healthy" || got.ProbeMessage != "既有结论" {
		t.Errorf("探针列不应被 UpsertPools 覆盖，实得 status=%q message=%q", got.ProbeStatus, got.ProbeMessage)
	}
	t.Logf("UpsertPools 更新了普通列(%q)且保留了探针结论(%q)", got.Name, got.ProbeMessage)
}
