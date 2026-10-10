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

// TestApplyLedgerToRows 双口径合并规则：
//   - 账本可作准的渠道（已同步过历史条目）一律用流水：本月 0 条目就是 0 成本，不退回快照推算
//     （否则漏记充值的月份会被推算成负数成本）；
//   - 账本不可作准的渠道用快照推算兜底；
//   - 差额只对「两种口径都有」的渠道累计（跨口径相加没有可比性）；
//   - source 反映渠道构成：全流水 / 全推算 / 混合 / 皆无。
func TestApplyLedgerToRows(t *testing.T) {
	rows := []dto.UpstreamLedgerRow{
		{ProviderID: 7, Consumption: floatPtr(100)}, // 账本可作准 → 流水 90
		{ProviderID: 9, Consumption: floatPtr(50)},  // 账本不可作准 → 快照 50
		{ProviderID: 11}, // 两者皆无 → none
		{ProviderID: 13, Consumption: floatPtr(20)}, // 账本可作准但本月 0 条目 → 流水口径 0
	}
	ledger := map[uint64]ledgerAgg{7: {Sum: 90, Count: 3}}
	authoritative := map[uint64]bool{7: true, 13: true}

	cost, ledgerTotal, diff, source := applyLedgerToRows(rows, ledger, authoritative)

	if rows[0].CostSource != "ledger" || rows[0].LedgerConsumption == nil || *rows[0].LedgerConsumption != 90 {
		t.Errorf("7 号渠道应取流水 90，实际 source=%s ledger=%v", rows[0].CostSource, rows[0].LedgerConsumption)
	}
	if rows[1].CostSource != "snapshot" {
		t.Errorf("9 号渠道应取快照，实际 %s", rows[1].CostSource)
	}
	if rows[2].CostSource != "none" {
		t.Errorf("无数据渠道 source 应为 none，实际 %s", rows[2].CostSource)
	}
	if rows[3].CostSource != "ledger" || rows[3].LedgerConsumption == nil || *rows[3].LedgerConsumption != 0 {
		t.Errorf("13 号渠道账本可作准，本月 0 条目应记为流水 0，实际 source=%s ledger=%v",
			rows[3].CostSource, rows[3].LedgerConsumption)
	}
	if cost != 140 { // 90（流水）+ 50（快照）+ 0（流水口径的零）
		t.Errorf("合并成本 = %v, want 140", cost)
	}
	if ledgerTotal != 90 {
		t.Errorf("流水合计 = %v, want 90", ledgerTotal)
	}
	if diff != -30 { // (90 − 100) + (0 − 20)
		t.Errorf("差额 = %v, want -30", diff)
	}
	if source != "mixed" {
		t.Errorf("source = %q, want mixed", source)
	}

	// 全流水 / 全推算 / 皆无 三种构成
	onlyLedger := []dto.UpstreamLedgerRow{{ProviderID: 7, Consumption: floatPtr(100)}}
	if _, _, _, src := applyLedgerToRows(onlyLedger, map[uint64]ledgerAgg{7: {Sum: 90, Count: 1}}, map[uint64]bool{7: true}); src != "ledger" {
		t.Errorf("全流水 source = %q, want ledger", src)
	}
	onlySnapshot := []dto.UpstreamLedgerRow{{ProviderID: 9, Consumption: floatPtr(50)}}
	if _, _, _, src := applyLedgerToRows(onlySnapshot, nil, nil); src != "snapshot" {
		t.Errorf("全推算 source = %q, want snapshot", src)
	}
	if _, _, _, src := applyLedgerToRows(nil, nil, nil); src != "none" {
		t.Errorf("无数据 source = %q, want none", src)
	}
}

// TestUpstreamLineCopy 口径来源决定页面文案（不同来源必须给不同标签，避免口径含糊）。
func TestUpstreamLineCopy(t *testing.T) {
	seen := map[string]string{}
	for _, source := range []string{"ledger", "mixed", "snapshot", "none"} {
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
