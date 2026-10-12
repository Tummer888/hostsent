package model

import "testing"

// TestPoolCategoryOf 池类别判定：区域/可用区是位置型，其余都按容量型。
//
// 这条口径决定了两件事：位置型池不进容量汇总（否则区域行的 0 会稀释 CPU 总量），
// 以及页面在哪一类视图里展示它。判定用排除法，是为了让新接入的容量型池类型
// （如 storage）默认落在容量维度，不需要每加一种类型就改这里。
func TestPoolCategoryOf(t *testing.T) {
	cases := map[string]string{
		// 位置型：只有位置，没有容量
		"region": PoolCategoryLocation,
		"REGION": PoolCategoryLocation,
		" zone ": PoolCategoryLocation,
		"Zone":   PoolCategoryLocation,
		// 容量型：真正消耗 CPU/内存/磁盘
		"node":      PoolCategoryCapacity,
		"storage":   PoolCategoryCapacity,
		"compute":   PoolCategoryCapacity,
		"memory":    PoolCategoryCapacity,
		"":          PoolCategoryCapacity, // 未标类型的老数据按容量型兜底
		"something": PoolCategoryCapacity, // 未知类型默认容量型（排除法）
	}
	for in, want := range cases {
		if got := PoolCategoryOf(in); got != want {
			t.Errorf("PoolCategoryOf(%q) = %q, want %q", in, got, want)
		}
	}
}
