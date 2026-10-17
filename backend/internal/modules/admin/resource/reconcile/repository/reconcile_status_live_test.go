//go:build live

package repository

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// liveReconcileDB 打开真实库（与实例/lifecycle live 测试同一约定）。
func liveReconcileDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("LIVE_DB_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Skipf("无可用测试库：%v", err)
	}
	return db
}

// statusCase 一条构造用例：本地三列（status/stage/suspend_reason）+ 上游 domainstatus。
type statusCase struct {
	name          string
	status        string
	stage         string
	suspendReason string
	domainStatus  string // 写进 raw_data.domainstatus；空 = raw_data 无该字段
	// wantExcluded 该行应被 baseCTE 的 WHERE 排除（终态/占位）。
	wantExcluded bool
	wantAnomaly  string
	wantLocalSvc string
}

// TestLiveReconcileStatusDimensionIsolation 对真实库验证状态向判据只比**服务**维度。
//
// 用户实测误报：控制台关机后本地 status=stopped、上游 domainstatus 仍 Active，
// 对账把它判成 status_mismatch（台账文案"上游已暂停本地未同步"）。根因是拿
// 上游服务状态与本地机器电源状态机械相等比较 —— zjmf 没有电源字段，
// 关机只是机器没通电，服务和账单都还在。
//
// 这个用例把"关机""暂停""上游漂移"等组合逐一钉在真实 SQL 上：SQL 是本模块的
// 判据唯一来源（列表、汇总、台账三条路径共用 baseCTE），单元测试测不到它。
//
//	go test -tags live ./internal/modules/admin/resource/reconcile/repository/
func TestLiveReconcileStatusDimensionIsolation(t *testing.T) {
	db := liveReconcileDB(t)
	ctx := context.Background()
	mark := time.Now().UnixNano()

	// 用真实渠道/用户：instances.provider_id 与 user_id 有外键。
	var providerID, userID uint64
	if err := db.Raw("SELECT id FROM resource_providers WHERE deleted_at IS NULL ORDER BY id LIMIT 1").
		Scan(&providerID).Error; err != nil || providerID == 0 {
		t.Skipf("无可用渠道行：%v", err)
	}
	if err := db.Raw("SELECT id FROM users ORDER BY id LIMIT 1").Scan(&userID).Error; err != nil || userID == 0 {
		t.Skipf("无可用用户行：%v", err)
	}

	expire := time.Now().Add(20 * 24 * time.Hour)
	cases := []statusCase{
		{
			// 用户实测的那一条：关机后本地 stopped、上游仍 Active。
			// 服务还在（未暂停、未到期），所以两侧一致 —— 必须不报。
			name: "关机（本地已停止+上游Active）", status: "stopped",
			domainStatus: "Active", wantAnomaly: "ok", wantLocalSvc: "active",
		},
		{
			// 到期暂停：上游也 Suspended，两侧同向，不是差异。
			name: "到期暂停（本地suspended+上游Suspended）", status: "stopped",
			stage: "suspended", suspendReason: "expired",
			domainStatus: "Suspended", wantAnomaly: "ok", wantLocalSvc: "suspended",
		},
		{
			// 上游欠费被停而我方还在给客户算钱 —— 这才是真的该报的。
			name: "上游暂停本地仍在服务", status: "running",
			domainStatus: "Suspended", wantAnomaly: "status_mismatch", wantLocalSvc: "active",
		},
		{
			// 风控冻结：同上，上游不再服务而我方账期还在跑。
			name: "上游风控冻结本地仍在服务", status: "running",
			domainStatus: "Fraud", wantAnomaly: "status_mismatch", wantLocalSvc: "active",
		},
		{
			// 上游已删除：为不存在的机器计费，等级最高。
			name: "上游已删除本地仍在服务", status: "running",
			domainStatus: "Deleted", wantAnomaly: "status_stale", wantLocalSvc: "active",
		},
		{
			// 上游已取消（未付款超时）：同上。
			name: "上游已取消本地仍在服务", status: "stopped",
			domainStatus: "Cancelled", wantAnomaly: "status_stale", wantLocalSvc: "active",
		},
		{
			// 本地暂停没同步到上游（或上游被谁恢复了）：本地停服务、上游还在服务。
			name: "本地已暂停而上游Active", status: "stopped",
			stage: "suspended", suspendReason: "manual",
			domainStatus: "Active", wantAnomaly: "status_mismatch", wantLocalSvc: "suspended",
		},
		{
			// 宽限期机器仍可服务：与上游 Active 一致，不报。
			name: "宽限期内上游Active", status: "running",
			stage: "grace", domainStatus: "Active", wantAnomaly: "ok", wantLocalSvc: "grace",
		},
		{
			// 上游还在开通而本地已视为在用：开通流程没走完。
			name: "上游开通中本地已服务", status: "running",
			domainStatus: "Pending", wantAnomaly: "status_mismatch", wantLocalSvc: "active",
		},
		{
			// 非 zjmf 上游没有该字段：不做比对（≠ 比对不出）。
			name: "上游不提供状态字段", status: "stopped",
			domainStatus: "", wantAnomaly: "ok", wantLocalSvc: "active",
		},
		{
			// 已终止行整体不参与对账（避免永久预警）。
			name: "已终止行排除", status: "deleted",
			stage: "destroyed", domainStatus: "Deleted", wantExcluded: true,
		},
		{
			// 已取消（订单未付款超时）行同样排除：机器从未开通。
			name: "已取消行排除", status: "cancelled",
			stage: "cancelled", domainStatus: "", wantExcluded: true,
		},
	}

	ids := make([]uint64, 0, len(cases))
	for i, tc := range cases {
		instID := fmt.Sprintf("ZZ-RECON-%d-%d", mark, i)
		raw := ""
		if tc.domainStatus != "" {
			raw = fmt.Sprintf(`{"domainstatus":%q,"nextduedate":%d}`, tc.domainStatus, expire.Unix())
		}
		var id uint64
		err := db.Raw(`
			INSERT INTO instances (instance_id, provider_id, user_id, name, cpu, memory, disk,
				status, source_mode, provider_instance_id, billing_mode, lifecycle_stage,
				suspend_reason, expire_at, raw_data, created_at, updated_at)
			VALUES (?, ?, ?, ?, 2, 4096, 40, ?, 'upstream', ?, 'monthly', NULLIF(?, ''),
			        NULLIF(?, ''), ?, NULLIF(?, ''), now(), now())
			RETURNING id`,
			instID, providerID, userID, "对账维度验收 "+tc.name, tc.status,
			fmt.Sprintf("up-%d-%d", mark, i), tc.stage, tc.suspendReason, expire, raw,
		).Scan(&id).Error
		if err != nil {
			t.Fatalf("造行 %s 失败: %v", tc.name, err)
		}
		ids = append(ids, id)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM instances WHERE id IN ?", ids)
		db.Exec("DELETE FROM reconcile_findings WHERE instance_id IN ?", ids)
	})

	// 只查本次造的行：baseCTE 是判据唯一来源，直接跑它。
	sql := baseCTE + `
		SELECT id, COALESCE(raw_domainstatus,'') AS upstream_status,
		       local_service_state, status_anomaly
		  FROM marked WHERE id IN ?`
	var rows []struct {
		ID                uint64
		UpstreamStatus    string
		LocalServiceState string
		StatusAnomaly     string
	}
	if err := db.WithContext(ctx).Raw(sql, ids).Scan(&rows).Error; err != nil {
		t.Fatalf("对账 CTE 查询失败: %v", err)
	}
	got := map[uint64]struct {
		localSvc string
		anomaly  string
	}{}
	for _, r := range rows {
		got[r.ID] = struct {
			localSvc string
			anomaly  string
		}{r.LocalServiceState, r.StatusAnomaly}
	}

	for i, tc := range cases {
		id := ids[i]
		row, ok := got[id]
		if tc.wantExcluded {
			if ok {
				t.Errorf("%s：应被 WHERE 排除，实际参与了对账（local_svc=%s anomaly=%s）",
					tc.name, row.localSvc, row.anomaly)
			}
			continue
		}
		if !ok {
			t.Errorf("%s：未被查到（期望参与对账）", tc.name)
			continue
		}
		if row.anomaly != tc.wantAnomaly {
			t.Errorf("%s：status_anomaly = %q, want %q", tc.name, row.anomaly, tc.wantAnomaly)
		}
		if row.localSvc != tc.wantLocalSvc {
			t.Errorf("%s：local_service_state = %q, want %q", tc.name, row.localSvc, tc.wantLocalSvc)
		}
	}
}
