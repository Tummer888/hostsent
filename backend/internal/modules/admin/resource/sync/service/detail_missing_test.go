package service

import "testing"

// TestIsDetailMissing 适配器"本次没拿到详情"的标记识别。
//
// 为什么需要这个判据（实测故障）：上游 prodetail 是逐商品同步组装，批量越大越慢，
// 撞上 15s HTTP 超时后该批商品详情为空。此前这类商品会从同步结果里整体消失，
// 全量对账据此判定"上游已下架"，把 200 个在售商品误置为未在售。
// 正确语义是：商品仍在上游清单里（不能判下架），只是这次没取到价格与开通参数
// （不能覆盖本地已有值，否则 config_groups 被抹掉，商品从此开不了通）。
func TestIsDetailMissing(t *testing.T) {
	cases := []struct {
		name string
		raw  map[string]interface{}
		want bool
	}{
		{"适配器标记 true", map[string]interface{}{"detail_missing": true}, true},
		{"字符串 true（JSON 往返形态）", map[string]interface{}{"detail_missing": "true"}, true},
		{"字符串 TRUE（大小写不敏感）", map[string]interface{}{"detail_missing": "TRUE"}, true},
		{"显式 false", map[string]interface{}{"detail_missing": false}, false},
		{"无该键（正常详情）", map[string]interface{}{"gid": 6, "config_groups": []interface{}{}}, false},
		{"空规格", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isDetailMissing(tc.raw); got != tc.want {
				t.Fatalf("isDetailMissing() = %v, want %v", got, tc.want)
			}
		})
	}
}
