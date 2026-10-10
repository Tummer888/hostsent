package service

import (
	"testing"
	"time"

	"hostsent/backend/internal/modules/admin/finance/cost/dto"
	"hostsent/backend/internal/modules/admin/finance/cost/model"
	"hostsent/backend/internal/pkg/upstream"
)

// TestLedgerKindConstantsAlign 账本方向常量必须与协议层同值：装配层按字符串透传，
// 两边任一侧改名而另一侧没跟上，就会静默同步不到数据（这里锁死）。
func TestLedgerKindConstantsAlign(t *testing.T) {
	if model.LedgerKindConsume != upstream.LedgerKindConsume {
		t.Errorf("consume 常量不一致：model=%q upstream=%q", model.LedgerKindConsume, upstream.LedgerKindConsume)
	}
	if model.LedgerKindTopup != upstream.LedgerKindTopup {
		t.Errorf("topup 常量不一致：model=%q upstream=%q", model.LedgerKindTopup, upstream.LedgerKindTopup)
	}
}

func floatPtr(v float64) *float64 { return &v }

// TestApplyLedgerToRows 流水口径落行规则：
//   - 账本可作准的渠道（已同步过历史条目）消耗/充值一律取流水：本期 0 条目就是 0
//     （「本期确实没消费」），不拿别的口径顶替；
//   - 未接入账本或尚未同步的渠道两者为 null、来源记 none（页面提示，成本可用成本项配置登记）；
//   - source 反映是否有渠道出数：ledger / none。
func TestApplyLedgerToRows(t *testing.T) {
	rows := []dto.UpstreamLedgerRow{
		{ProviderID: 7},  // 账本可作准 → 流水 90 / 充值 800
		{ProviderID: 9},  // 未同步账本 → none
		{ProviderID: 13}, // 账本可作准但本月 0 条目 → 流水口径 0
	}
	consume := map[uint64]ledgerAgg{7: {Sum: 90, Count: 3}}
	topup := map[uint64]ledgerAgg{7: {Sum: 800, Count: 2}}
	authoritative := map[uint64]bool{7: true, 13: true}

	cost, topupTotal, source := applyLedgerToRows(rows, consume, topup, authoritative)

	if rows[0].CostSource != "ledger" || rows[0].Consumption == nil || *rows[0].Consumption != 90 || rows[0].ConsumptionEntries != 3 {
		t.Errorf("7 号渠道应取流水 90（3 笔），实际 source=%s consumption=%v entries=%d",
			rows[0].CostSource, rows[0].Consumption, rows[0].ConsumptionEntries)
	}
	if rows[0].TopupTotal == nil || *rows[0].TopupTotal != 800 {
		t.Errorf("7 号渠道期间充值应为 800，实际 %v", rows[0].TopupTotal)
	}
	if rows[1].CostSource != "none" || rows[1].Consumption != nil || rows[1].TopupTotal != nil {
		t.Errorf("9 号渠道无账本数据应为 none 且消耗/充值为 null，实际 source=%s consumption=%v topup=%v",
			rows[1].CostSource, rows[1].Consumption, rows[1].TopupTotal)
	}
	if rows[2].CostSource != "ledger" || rows[2].Consumption == nil || *rows[2].Consumption != 0 {
		t.Errorf("13 号渠道账本可作准，本月 0 条目应记为流水 0，实际 source=%s consumption=%v",
			rows[2].CostSource, rows[2].Consumption)
	}
	if cost != 90 { // 90（7 号流水）+ 0（13 号流水口径的零）
		t.Errorf("上游成本合计 = %v, want 90", cost)
	}
	if topupTotal != 800 {
		t.Errorf("充值合计 = %v, want 800", topupTotal)
	}
	if source != "ledger" {
		t.Errorf("source = %q, want ledger", source)
	}

	// 无渠道出数 → none
	if _, _, src := applyLedgerToRows([]dto.UpstreamLedgerRow{{ProviderID: 9}}, nil, nil, nil); src != "none" {
		t.Errorf("无账本渠道 source = %q, want none", src)
	}
	if _, _, src := applyLedgerToRows(nil, nil, nil, nil); src != "none" {
		t.Errorf("空渠道 source = %q, want none", src)
	}
}

// TestUpstreamLineCopy 口径来源决定页面文案（不同来源必须给不同标签，避免口径含糊）。
func TestUpstreamLineCopy(t *testing.T) {
	seen := map[string]string{}
	for _, source := range []string{"ledger", "none"} {
		label, usage := upstreamLineCopy(source)
		if label == "" || usage == "" {
			t.Errorf("source=%s 的文案不完整：%q / %q", source, label, usage)
		}
		if prev, dup := seen[label]; dup {
			t.Errorf("source=%s 与 %s 标签重复：%q", source, prev, label)
		}
		seen[label] = source
	}
}

// TestDuesWithin 到期窗口聚合：窗口内到期（含已到期未付）的主机都要算进去，金额与台数一起给。
func TestDuesWithin(t *testing.T) {
	now := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	dues := []upstream.DueHost{
		{UpstreamID: "1", Amount: 16, NextDueAt: now.AddDate(0, 0, 3)},
		{UpstreamID: "2", Amount: 111.3, NextDueAt: now.AddDate(0, 0, 30)}, // 边界：恰好 30 天，不计
		{UpstreamID: "3", Amount: 200, NextDueAt: now.AddDate(0, 0, 200)},
		{UpstreamID: "4", Amount: 0, NextDueAt: now.AddDate(0, 0, -1)}, // 已过期：仍需付费才能恢复 → 计入
	}
	total, count := duesWithin(dues, now, 30)
	if total != 16 {
		t.Errorf("duesWithin 金额 = %v, want 16（只算窗口内，且排除恰好第 30 天）", total)
	}
	if count != 2 {
		t.Errorf("duesWithin 台数 = %d, want 2（含已到期未付的那台）", count)
	}
}
