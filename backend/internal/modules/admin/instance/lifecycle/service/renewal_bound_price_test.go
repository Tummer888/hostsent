package service

import (
	"testing"

	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
	"hostsent/backend/internal/pkg/pricing"
)

// TestBoundRenewalQuoteSinglePeriod 绑定价按期为单价：开 30、续 18 的约定，
// 续一个月就收 18（不被商品价 30 覆盖）。
func TestBoundRenewalQuoteSinglePeriod(t *testing.T) {
	q := boundRenewalQuoteFor(18, 30, 1, "monthly")
	if q.Final != 18 {
		t.Errorf("实付应为绑定价 18，实际 %v", q.Final)
	}
	// 商品标准价 30 → 原价 30，优惠 12（展示"本来要收多少、按承诺收了多少"）。
	if q.Original != 30 {
		t.Errorf("原价应取商品标准价 30，实际 %v", q.Original)
	}
	if q.Discount != 12 {
		t.Errorf("差额应为 12，实际 %v", q.Discount)
	}
	if q.Source != pricing.SourceBound {
		t.Errorf("来源应标记 bound，实际 %q", q.Source)
	}
	if q.Snapshot != "[]" {
		t.Errorf("无折扣明细时快照必须是合法空数组，实际 %q", q.Snapshot)
	}
}

// TestBoundRenewalQuoteMultiplePeriods 续多期按单价乘期数（续费期数不是周期倍率）。
func TestBoundRenewalQuoteMultiplePeriods(t *testing.T) {
	q := boundRenewalQuoteFor(18, 30, 3, "monthly")
	if q.Final != 54 {
		t.Errorf("续 3 期应收 18×3=54，实际 %v", q.Final)
	}
}

// TestBoundRenewalQuoteAnnualUsesCycleMultiplier 年付的标准价按周期倍率放大后再比较，
// 否则拿"月单价"当"年标准价"会让差额虚高成优惠。
func TestBoundRenewalQuoteAnnualUsesCycleMultiplier(t *testing.T) {
	// 绑定价 18/月、商品年付标准价 30/月 → 标准年价 360，实付 18（这里是单期年付）。
	q := boundRenewalQuoteFor(18, 30, 1, "annually")
	if q.Final != 18 {
		t.Errorf("实付应为绑定年价 18，实际 %v", q.Final)
	}
	if q.Original != 360 {
		t.Errorf("年付原价应为 30×12=360，实际 %v", q.Original)
	}
}

// TestBoundRenewalQuoteNoHigherListPrice 商品现价低于绑定价（或查不到）时不虚报原价，
// 免得把"客户其实买贵了"显示成"享受了优惠"。
func TestBoundRenewalQuoteNoHigherListPrice(t *testing.T) {
	q := boundRenewalQuoteFor(18, 10, 1, "monthly")
	if q.Original != 18 || q.Discount != 0 {
		t.Errorf("现价更低时原价应等于实付且无优惠，实际 orig=%v disc=%v", q.Original, q.Discount)
	}

	q2 := boundRenewalQuoteFor(18, 0, 1, "monthly")
	if q2.Original != 18 || q2.Discount != 0 {
		t.Errorf("查不到商品价时应无差价可示，实际 orig=%v disc=%v", q2.Original, q2.Discount)
	}
}

// TestBoundRenewalQuoteZeroPeriodCount 期数非法按 1 期处理（不能算出 0 元续费单）。
func TestBoundRenewalQuoteZeroPeriodCount(t *testing.T) {
	q := boundRenewalQuoteFor(18, 30, 0, "monthly")
	if q.Final != 18 {
		t.Errorf("期数 0 应按 1 期处理，实际 %v", q.Final)
	}
}

func floatPtr(v float64) *float64 { return &v }

// TestBoundRenewalPriceOnlyPositive 只有正数才算绑定：0/负数视为没绑过。
// 0 元续费会被算价守卫拒绝，当成"已绑定"会让这台机器永远续不了费。
func TestBoundRenewalPriceOnlyPositive(t *testing.T) {
	cases := []struct {
		name string
		raw  *float64
		want bool
	}{
		{"未绑定", nil, false},
		{"绑定 18", floatPtr(18), true},
		{"绑定 0", floatPtr(0), false},
		{"绑定负数", floatPtr(-5), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := boundRenewalPrice(&syncmodel.Instance{RenewalPrice: tc.raw})
			if (got != nil) != tc.want {
				t.Errorf("绑定判定 = %v, want %v", got != nil, tc.want)
			}
		})
	}
	if got := boundRenewalPrice(nil); got != nil {
		t.Errorf("nil 实例应返回 nil，实际 %v", *got)
	}
}
