package service

import (
	"math"
	"testing"

	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
)

// TestPriceChangeRatio 变动幅度；old=0 时非零新值按 100% 处理（避免除零）。
func TestPriceChangeRatio(t *testing.T) {
	cases := []struct {
		old, new, want float64
	}{
		{100, 150, 0.5},
		{100, 50, 0.5}, // 降价同样取绝对值
		{0, 0, 0},
		{0, 10, 1},
		{100, 100, 0},
	}
	for _, c := range cases {
		if got := priceChangeRatio(c.old, c.new); math.Abs(got-c.want) > 0.00001 {
			t.Errorf("priceChangeRatio(%v,%v) = %v, want %v", c.old, c.new, got, c.want)
		}
	}
}

// TestSameFloat decimal 往返产生的 4 位以内误差不应被判为变更。
func TestSameFloat(t *testing.T) {
	if !sameFloat(100, 100.00001) {
		t.Error("4 位以内误差应视为相同")
	}
	if sameFloat(100, 100.01) {
		t.Error("1 分钱差异应视为不同")
	}
}

// TestDispositionFor 超阈挂起 → pending，阈值内 → applied。
func TestDispositionFor(t *testing.T) {
	if got := dispositionFor(syncmodel.PriceChangePending); got != "pending" {
		t.Errorf("pending 事件 disposition = %q, want pending", got)
	}
	if got := dispositionFor("auto_applied"); got != "applied" {
		t.Errorf("auto_applied 事件 disposition = %q, want applied", got)
	}
}
