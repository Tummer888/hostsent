package service

import (
	"testing"
)

// TestUpstreamOptionLocksQuantityNotLocked 数量型配置项不得被标成「固定配置」。
//
// 实测问题：锁定判定原先是"可见取值 ≤ 1 即锁定"。数量型配置项（数据盘等）
// 通常只有一个价档，于是被标成锁定 —— 客户以为这项改不了，实际它能填 0~1024G。
// 判定必须排除数量型（上游对这类项收的是数量，不是候选值）。
func TestUpstreamOptionLocksQuantityNotLocked(t *testing.T) {
	groups := []upstreamConfigGroup{{
		Name: "美国CN2 配置项组",
		Options: []upstreamConfigOption{
			// 数量型：单个价档，但客户能填 0~1024。
			{OptionName: "data_disk_size|数据盘", OptionType: 14, QtyMinimum: 0, QtyMaximum: 1024,
				Sub: []upstreamConfigSub{{OptionName: "数据盘"}}},
			// 数量型：两个价档（系统盘 80~1024）。
			{OptionName: "system_disk_size|系统盘", OptionType: 19, QtyMinimum: 80, QtyMaximum: 1024,
				Sub: []upstreamConfigSub{{OptionName: "系统盘"}, {OptionName: "系统盘"}}},
			// 真正的固定配置：单值枚举（区域）。
			{OptionName: "area|区域", OptionType: 12,
				Sub: []upstreamConfigSub{{OptionName: "11|US^美国精品CN2机房"}}},
			// 多值枚举（操作系统）不算锁定。
			{OptionName: "os|操作系统", OptionType: 5,
				Sub: []upstreamConfigSub{{OptionName: "27|CentOS"}, {OptionName: "78|Ubuntu"}}},
		},
	}}

	got := upstreamOptionLocksFromRaw(groups)
	byKey := map[string]bool{}
	vals := map[string]string{}
	for _, o := range got {
		byKey[o.Key] = o.Locked
		vals[o.Key] = o.Value
	}

	if byKey["data_disk_size"] {
		t.Error("数据盘是数量型（可填 0~1024），不应标为固定配置/锁定")
	}
	if byKey["system_disk_size"] {
		t.Error("系统盘是数量型（可填 80~1024），不应标为锁定")
	}
	if !byKey["area"] {
		t.Error("单值枚举（区域）仍是固定配置，应标为锁定")
	}
	if byKey["os"] {
		t.Error("多值枚举（操作系统）不应标为锁定")
	}
	// 展示值对数量型给可填区间（比价档名"数据盘"有信息量）。
	if vals["data_disk_size"] != "最大 1024" {
		t.Errorf("数据盘展示值 = %q, want %q", vals["data_disk_size"], "最大 1024")
	}
	if vals["system_disk_size"] != "80 – 1024" {
		t.Errorf("系统盘展示值 = %q, want %q", vals["system_disk_size"], "80 – 1024")
	}
	if vals["area"] != "11|US^美国精品CN2机房" {
		t.Errorf("区域展示值 = %q", vals["area"])
	}
}

// TestQuantityRangeText 区间文案的边界形态。
func TestQuantityRangeText(t *testing.T) {
	cases := []struct {
		min, max int
		want     string
	}{
		{80, 1024, "80 – 1024"},
		{0, 1024, "最大 1024"},
		{1, 0, "最小 1"},
		{0, 0, ""},
	}
	for _, c := range cases {
		if got := quantityRangeText(c.min, c.max); got != c.want {
			t.Errorf("quantityRangeText(%d,%d) = %q, want %q", c.min, c.max, got, c.want)
		}
	}
}
