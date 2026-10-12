package service

import (
	"math"
	"testing"

	resourceproductmodel "hostsent/backend/internal/modules/admin/resource/product/model"
)

// TestClonePriceBaseline 售价基线取上游标价（history: 旧口径按代理价 × pct，售价偏低）。
func TestClonePriceBaseline(t *testing.T) {
	cases := []struct {
		name string
		rp   *resourceproductmodel.ResourceProduct
		want float64
	}{
		{"标价与代理价都有", &resourceproductmodel.ResourceProduct{SalePrice: 180, CostPrice: 126}, 180},
		{"只有代理价（历史数据）", &resourceproductmodel.ResourceProduct{CostPrice: 126}, 126},
		{"都没有", &resourceproductmodel.ResourceProduct{}, 0},
		{"nil", nil, 0},
	}
	for _, c := range cases {
		if got := clonePriceBaseline(c.rp); got != c.want {
			t.Errorf("%s: clonePriceBaseline = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestCloneCycleRatio 周期矩阵倍率 = 标价/代理价 × pct/100：
// 100% 时按上游原价卖，成本行仍是代理价（利润 = 标价 − 代理价）。
func TestCloneCycleRatio(t *testing.T) {
	rp := &resourceproductmodel.ResourceProduct{SalePrice: 180, CostPrice: 126}
	cases := []struct {
		pct  float64
		want float64
	}{
		{100, 180.0 / 126.0},   // 按上游原价
		{0, 180.0 / 126.0},     // 0 视为 100
		{120, 1.2 * 180 / 126}, // 加价 20%
		{80, 0.8 * 180 / 126},  // 让利 20%
	}
	for _, c := range cases {
		got := cloneCycleRatio(rp, c.pct)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("cloneCycleRatio(pct=%v) = %v, want %v", c.pct, got, c.want)
		}
	}
	// 周期成本 126 → 售价应等于 180 × pct/100（与商品售价同口径）。
	if got := 126 * cloneCycleRatio(rp, 100); math.Abs(got-180) > 0.001 {
		t.Errorf("周期售价 = %v, want 180（按上游标价）", got)
	}
	// 成本缺失（无代理价）时退化为 pct/100，不产生 0 元档。
	if got := cloneCycleRatio(&resourceproductmodel.ResourceProduct{SalePrice: 180}, 100); got != 1 {
		t.Errorf("缺代理价时倍率 = %v, want 1", got)
	}
}
