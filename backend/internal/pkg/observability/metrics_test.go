package observability

import (
	"errors"
	"strings"
	"testing"
)

func TestIncAndSnapshot(t *testing.T) {
	Inc("ut_inc_a", 1)
	Inc("ut_inc_a", 2)
	Inc("ut_inc_b", 10)
	snap := Snapshot()
	if !strings.Contains(snap, "ut_inc_a 3\n") {
		t.Fatalf("expected ut_inc_a 3, got snapshot:\n%s", snap)
	}
	if !strings.Contains(snap, "ut_inc_b 10\n") {
		t.Fatalf("expected ut_inc_b 10, got snapshot:\n%s", snap)
	}
}

func TestTimed(t *testing.T) {
	if err := Timed("ut_timed_ok", func() error { return nil }); err != nil {
		t.Fatalf("Timed ok returned err: %v", err)
	}
	if err := Timed("ut_timed_err", func() error { return errors.New("boom") }); err == nil {
		t.Fatalf("Timed expected error, got nil")
	}
	snap := Snapshot()
	if !strings.Contains(snap, "ut_timed_ok_duration_ms") {
		t.Errorf("missing ut_timed_ok_duration_ms in snapshot:\n%s", snap)
	}
	if !strings.Contains(snap, "ut_timed_err_errors_total 1\n") {
		t.Errorf("expected ut_timed_err_errors_total 1, got snapshot:\n%s", snap)
	}
}
