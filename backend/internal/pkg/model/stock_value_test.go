package model

import "testing"

// TestStockValue 上游库存解析口径：stock_control=1 才认 qty，其余一律不限量（-1）。
//
// 为什么这条口径重要（实测数据）：上游 245 个商品里 200 个 stock_control=0，
// 其中部分 qty 是 1~3 这类无意义残值；若直接拿 qty 当库存，会凭空造出
// "只剩 1 台"的假象，把正常商品卖成售罄。
func TestStockValue(t *testing.T) {
	cases := []struct {
		name string
		raw  map[string]interface{}
		want int
	}{
		{"未开启库存控制 → 不限量（忽略 qty 残值）", map[string]interface{}{"stock_control": 0, "qty": 3}, -1},
		{"无库存字段 → 不限量", map[string]interface{}{"gid": 6}, -1},
		{"空规格 → 不限量", nil, -1},
		{"限量且有余量", map[string]interface{}{"stock_control": 1, "qty": 188}, 188},
		{"限量且售罄 → 0（不是不限量）", map[string]interface{}{"stock_control": 1, "qty": 0}, 0},
		{"限量但余量为负 → 归零", map[string]interface{}{"stock_control": 1, "qty": -5}, 0},
		{"字段是字符串（上游 JSON 形态不定）", map[string]interface{}{"stock_control": "1", "qty": "11"}, 11},
		{"限量但无 qty → 0", map[string]interface{}{"stock_control": 1}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &StandardProduct{RawSpecs: tc.raw}
			if got := p.StockValue(); got != tc.want {
				t.Fatalf("StockValue() = %d, want %d", got, tc.want)
			}
		})
	}
	// 空指针不应 panic（同步链路里 RawSpecs 可能整体缺失）。
	var nilProduct *StandardProduct
	if got := nilProduct.StockValue(); got != -1 {
		t.Fatalf("nil 商品应返回 -1，got %d", got)
	}
}
