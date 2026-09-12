package billingcycle

import (
	"testing"
	"time"
)

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"month": Monthly, "monthly": Monthly, "Monthly": Monthly,
		"quarterly": Quarterly, "quarter": Quarterly,
		"semiannually": Semiannually, "semiannual": Semiannually,
		"year": Annually, "yearly": Annually, "annually": Annually,
		"biennially": Biennially, "triennially": Triennially,
		"onetime": Onetime, "fixed": Onetime,
		"hour": Hourly, "day": Daily,
		"": "", "quarter-of-year": "", "啥": "",
	}
	for raw, want := range cases {
		if got := Normalize(raw); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", raw, got, want)
		}
	}
	if got := NormalizeOrDefault("unknown"); got != Monthly {
		t.Errorf("NormalizeOrDefault(unknown) = %q, want monthly", got)
	}
}

func TestNormalizeListOrdersAndDedupes(t *testing.T) {
	got := NormalizeList([]string{"annually", "month", "yearly", "bad", "quarterly", "monthly"})
	want := []string{Monthly, Quarterly, Annually}
	if len(got) != len(want) {
		t.Fatalf("NormalizeList = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NormalizeList = %v, want %v", got, want)
		}
	}
}

func TestAdvance(t *testing.T) {
	base := time.Date(2026, 1, 31, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		cycle string
		count int
		want  time.Time
	}{
		// Go 的 AddDate 归一化溢出日：1/31 + 1 月 = 3/3。
		{Monthly, 1, time.Date(2026, 3, 3, 10, 0, 0, 0, time.UTC)},
		{Quarterly, 1, time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{Semiannually, 1, time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)},
		{Annually, 1, time.Date(2027, 1, 31, 10, 0, 0, 0, time.UTC)},
		{Biennially, 1, time.Date(2028, 1, 31, 10, 0, 0, 0, time.UTC)},
		{Triennially, 1, time.Date(2029, 1, 31, 10, 0, 0, 0, time.UTC)},
		{Daily, 3, time.Date(2026, 2, 3, 10, 0, 0, 0, time.UTC)},
		{Hourly, 5, time.Date(2026, 1, 31, 15, 0, 0, 0, time.UTC)},
		// 未知周期回落按月。
		{"weird", 1, time.Date(2026, 3, 3, 10, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		if got := Advance(base, c.cycle, c.count); !got.Equal(c.want) {
			t.Errorf("Advance(%q,%d) = %v, want %v", c.cycle, c.count, got, c.want)
		}
	}
	// periodCount <= 0 视为 1。
	if got := Advance(base, Monthly, 0); !got.Equal(time.Date(2026, 3, 3, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("Advance 0 count = %v", got)
	}
}

func TestSemiannualNotFallingBackToMonthly(t *testing.T) {
	// 修复前的关键缺陷：半年付被当 1 个月。这里锁死 6 个月。
	base := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	if got := Advance(base, Semiannually, 1); got.Month() != time.July || got.Year() != 2026 {
		t.Fatalf("半年付应推进到 7 月，got %v", got)
	}
}

func TestMultiplier(t *testing.T) {
	if got := Multiplier(Annually); got != 12 {
		t.Errorf("Multiplier(annually) = %d, want 12", got)
	}
	if got := Multiplier(Quarterly); got != 3 {
		t.Errorf("Multiplier(quarterly) = %d, want 3", got)
	}
	if got := Multiplier(Hourly); got != 0 {
		t.Errorf("Multiplier(hourly) = %d, want 0", got)
	}
}
