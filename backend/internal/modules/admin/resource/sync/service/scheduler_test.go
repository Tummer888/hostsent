package service

import (
	"testing"
	"time"

	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
)

func i16(v int16) *int16 { return &v }

// TestWithinWindow 验证执行时间窗判定：不限 / 同日 / 跨夜 / start==end。
func TestWithinWindow(t *testing.T) {
	at := func(hour int) time.Time {
		return time.Date(2026, 9, 11, hour, 30, 0, 0, time.Local)
	}
	cases := []struct {
		name       string
		start, end *int16
		now        time.Time
		want       bool
	}{
		{"两个界限都为空则不限", nil, nil, at(3), true},
		{"仅一个为空也视为不限", i16(1), nil, at(3), true},
		{"同日窗口内", i16(1), i16(6), at(3), true},
		{"同日窗口上界不含", i16(1), i16(6), at(6), false},
		{"同日窗口下界包含", i16(1), i16(6), at(1), true},
		{"跨夜窗口深夜内", i16(22), i16(6), at(23), true},
		{"跨夜窗口凌晨内", i16(22), i16(6), at(3), true},
		{"跨夜窗口白天外", i16(22), i16(6), at(12), false},
		{"start==end 视为该整点", i16(5), i16(5), at(5), true},
		{"start==end 其它小时不执行", i16(5), i16(5), at(6), false},
	}
	for _, c := range cases {
		item := syncmodel.SyncSchedule{WindowStart: c.start, WindowEnd: c.end}
		if got := withinWindow(item, c.now); got != c.want {
			t.Errorf("%s: withinWindow = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestNextWindowStart 未到点取当天、已过点顺延到次日。
func TestNextWindowStart(t *testing.T) {
	base := time.Date(2026, 9, 11, 10, 0, 0, 0, time.Local)
	next := nextWindowStart(base, i16(6), i16(12))
	if next.Day() != 12 || next.Hour() != 6 {
		t.Errorf("已过 6 点应顺延到次日 6 点，got %v", next)
	}
	next = nextWindowStart(base, i16(20), i16(23))
	if next.Day() != 11 || next.Hour() != 20 {
		t.Errorf("未到 20 点应取当天 20 点，got %v", next)
	}
	if got := nextWindowStart(base, nil, nil); !got.Equal(base) {
		t.Errorf("无窗口起点应原样返回，got %v", got)
	}
}

// TestSafeInterval 非法间隔回落到默认 1 小时，避免每轮扫描都到期。
func TestSafeInterval(t *testing.T) {
	if got := safeInterval(0); got != 3600 {
		t.Errorf("safeInterval(0) = %d, want 3600", got)
	}
	if got := safeInterval(-5); got != 3600 {
		t.Errorf("safeInterval(-5) = %d, want 3600", got)
	}
	if got := safeInterval(300); got != 300 {
		t.Errorf("safeInterval(300) = %d, want 300", got)
	}
}
