package service

import (
	"testing"
	"time"

	reconmodel "hostsent/backend/internal/modules/admin/resource/reconcile/model"
	"hostsent/backend/internal/modules/admin/resource/reconcile/repository"
)

func f(v float64) *float64 { return &v }

// TestFindingsFromRow_Expansion 一行对账结果按命中项展开成多条独立差异。
//
// 为什么必须一分为多：三类问题的处置动作完全不同（改价 / 顺账期 / 追上游状态），
// 合成一条会让"定价处理完了、账期还没处理"这种状态无法表达。
func TestFindingsFromRow_Expansion(t *testing.T) {
	exp := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	up := exp.Add(48 * time.Hour)
	row := repository.ReconcileRow{
		ID:             42,
		ProviderID:     19,
		LocalPrice:     f(38),
		UpstreamCost:   f(42.5),
		MarginAmount:   f(-4.5),
		PriceAnomaly:   "below_cost",
		ExpireAnomaly:  "expire_early",
		ExpireDiffDays: f(2),
		LocalExpire:    &exp,
		UpstreamExpire: &up,
		Status:         "running",
		StatusAnomaly:  "status_mismatch",
		UpstreamStatus: "Suspended",
	}
	got := findingsFromRow(row)
	if len(got) != 3 {
		t.Fatalf("应展开 3 条差异（价格+到期+状态），实际 %d 条: %+v", len(got), got)
	}
	byKind := map[string]reconmodel.ReconcileFinding{}
	for _, f := range got {
		byKind[f.Kind] = f
	}
	price, ok := byKind[reconmodel.KindPriceBelowCost]
	if !ok {
		t.Fatal("缺少 price_below_cost")
	}
	if price.Severity != reconmodel.SeverityDanger {
		t.Errorf("售价低于成本应为 danger，实际 %s", price.Severity)
	}
	if price.LocalValue != "38.00" || price.UpstreamValue != "42.50" {
		t.Errorf("价格两侧快照不正确: %q / %q", price.LocalValue, price.UpstreamValue)
	}
	if price.InstanceID != 42 || price.ProviderID != 19 {
		t.Errorf("关联字段不正确: %+v", price)
	}

	expire, ok := byKind[reconmodel.KindExpireEarly]
	if !ok {
		t.Fatal("缺少 expire_early")
	}
	if expire.LocalValue != "2026-01-01" || expire.UpstreamValue != "2026-01-03" {
		t.Errorf("到期两侧快照不正确: %q / %q", expire.LocalValue, expire.UpstreamValue)
	}

	status, ok := byKind[reconmodel.KindStatusMismatch]
	if !ok {
		t.Fatal("缺少 status_mismatch")
	}
	if status.LocalValue != "running" || status.UpstreamValue != "Suspended" {
		t.Errorf("状态两侧快照不正确: %q / %q", status.LocalValue, status.UpstreamValue)
	}
	if status.Severity != reconmodel.SeverityWarning {
		t.Errorf("上游暂停本地运行应为 warning，实际 %s", status.Severity)
	}
}

// TestFindingsFromRow_NoAnomaly 无异常不产生任何差异行。
func TestFindingsFromRow_NoAnomaly(t *testing.T) {
	row := repository.ReconcileRow{ID: 1, PriceAnomaly: "ok", ExpireAnomaly: "ok", StatusAnomaly: "ok"}
	if got := findingsFromRow(row); len(got) != 0 {
		t.Fatalf("无异常应无差异行，实际 %+v", got)
	}
}

// TestFindingsFromRow_MissingIgnored 数据缺失（缺售价/缺成本/缺到期）不入台账。
//
// 理由：那是"配不上"，不是"对不上"。在完成商品绑定与成本录入之前，这些实例
// 会一直缺失，落成台账只会产生永远无法 resolve 的噪声，把真正的资金风险淹没。
func TestFindingsFromRow_MissingIgnored(t *testing.T) {
	row := repository.ReconcileRow{
		ID:            7,
		PriceAnomaly:  "missing_cost",
		ExpireAnomaly: "missing_upstream_expire",
		StatusAnomaly: "ok",
	}
	if got := findingsFromRow(row); len(got) != 0 {
		t.Fatalf("数据缺失不应入台账，实际 %+v", got)
	}
}

// TestKindSeverityMapping 每个差异类型都登记了等级（漏登记会退化成 warning）。
func TestKindSeverityMapping(t *testing.T) {
	kinds := []string{
		reconmodel.KindPriceBelowCost, reconmodel.KindPriceThinMargin,
		reconmodel.KindExpireEarly, reconmodel.KindExpireLate,
		reconmodel.KindStatusMismatch, reconmodel.KindStatusStale,
	}
	for _, k := range kinds {
		if _, ok := reconmodel.KindSeverity[k]; !ok {
			t.Errorf("差异类型 %s 未登记等级", k)
		}
		if _, ok := reconmodel.KindNames[k]; !ok {
			t.Errorf("差异类型 %s 未登记展示名", k)
		}
	}
	// 资金/账期/上游已删直接受损 → danger。
	for _, k := range []string{reconmodel.KindPriceBelowCost, reconmodel.KindExpireEarly, reconmodel.KindExpireLate, reconmodel.KindStatusStale} {
		if reconmodel.SeverityOf(k) != reconmodel.SeverityDanger {
			t.Errorf("%s 应为 danger", k)
		}
	}
	// 未登记类型按 warning 兜底（宁可多提醒也不漏资金风险）。
	if reconmodel.SeverityOf("unknown_kind") != reconmodel.SeverityWarning {
		t.Error("未登记类型应兜底为 warning")
	}
}
