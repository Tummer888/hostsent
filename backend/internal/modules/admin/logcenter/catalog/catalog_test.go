package catalog

import (
	"regexp"
	"testing"
)

// 标识符白名单：表名/列名/筛选列只能是小写字母+下划线。
// 执行期所有 SQL 都靠这些常量拼接（doc92 §0.3 硬约束 1），所以正则是最外层的防线。
var identRe = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

func TestSourceIdentifiersAreSafe(t *testing.T) {
	seen := map[string]bool{}
	for _, src := range All() {
		if src.Key == "" {
			t.Fatalf("source with empty key: %+v", src)
		}
		if seen[src.Key] {
			t.Fatalf("duplicate source key: %s", src.Key)
		}
		seen[src.Key] = true

		if !identRe.MatchString(src.Table) {
			t.Errorf("%s: unsafe table name %q", src.Key, src.Table)
		}
		if !identRe.MatchString(src.TimeColumn) {
			t.Errorf("%s: unsafe time column %q", src.Key, src.TimeColumn)
		}
		for _, c := range src.SelectColumns {
			if !identRe.MatchString(c.Key) {
				t.Errorf("%s: unsafe select column %q", src.Key, c.Key)
			}
			if c.Label == "" {
				t.Errorf("%s: column %s missing label", src.Key, c.Key)
			}
		}
		for _, s := range src.SearchColumns {
			if !identRe.MatchString(s) {
				t.Errorf("%s: unsafe search column %q", src.Key, s)
			}
		}
		for param, column := range src.FilterColumns {
			if !identRe.MatchString(param) || !identRe.MatchString(column) {
				t.Errorf("%s: unsafe filter %q -> %q", src.Key, param, column)
			}
		}
	}
}

// 时间列必须出现在 SelectColumns 里，否则前端无法按时间排序/展示。
func TestTimeColumnIsSelectable(t *testing.T) {
	for _, src := range All() {
		found := false
		for _, c := range src.SelectColumns {
			if c.Key == src.TimeColumn {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: time column %s not in select columns", src.Key, src.TimeColumn)
		}
	}
}

// Class 与 Cleanable 必须一致：只有 ops 才可能可清理，audit 一律不可清理。
func TestClassAndCleanableConsistent(t *testing.T) {
	for _, src := range All() {
		switch src.Class {
		case ClassOps:
			if !src.Cleanable {
				t.Errorf("%s: ops source must be cleanable", src.Key)
			}
		case ClassAudit:
			if src.Cleanable {
				t.Errorf("%s: audit source must not be cleanable", src.Key)
			}
		default:
			t.Errorf("%s: unknown class %q", src.Key, src.Class)
		}
	}
}

// 注册表里绝不能出现账本/凭据表（doc92 §0.3 硬约束 2）。
func TestNoLedgerTablesRegistered(t *testing.T) {
	forbidden := []string{
		"orders", "wallet_transactions", "sales_commission_transactions",
		"referral_transactions", "points", "point_transactions", "payment_orders",
		"bills", "log_cleanup_jobs", "log_export_files",
	}
	for _, src := range All() {
		for _, f := range forbidden {
			if src.Table == f {
				t.Errorf("forbidden table registered as log source: %s (%s)", f, src.Key)
			}
		}
	}
}

// 保留期硬地板：注册表默认值不得低于 7 天，站内信与已读记录固定 365 天。
func TestRetentionDefaults(t *testing.T) {
	for _, src := range All() {
		if got := src.DefaultRetention(); got < MinRetentionDays {
			t.Errorf("%s: default retention %d below floor %d", src.Key, got, MinRetentionDays)
		}
	}
	for _, key := range []string{"notify_inbox", "notify_read"} {
		src := Get(key)
		if src == nil {
			t.Fatalf("missing source %s", key)
		}
		if src.DefaultRetention() != 365 {
			t.Errorf("%s: expected 365-day retention, got %d", key, src.DefaultRetention())
		}
	}
}

// 26 个源必须齐备（doc92 §4.2 清单）。
func TestSourceCount(t *testing.T) {
	if got := len(All()); got != 26 {
		t.Errorf("expected 26 sources, got %d", got)
	}
	if got := len(CleanableKeys()); got != 14 {
		t.Errorf("expected 14 cleanable sources, got %d", got)
	}
}

// 删除守卫只允许出现在 Cleanable 源上。
func TestDeleteGuardOnlyOnCleanable(t *testing.T) {
	for _, src := range All() {
		if src.DeleteGuard != "" && !src.Cleanable {
			t.Errorf("%s: delete guard on non-cleanable source", src.Key)
		}
	}
}

func TestGetUnknownKey(t *testing.T) {
	if Get("orders") != nil {
		t.Error("Get(orders) must return nil: ledger tables are not log sources")
	}
	if Get("log_cleanup_jobs") != nil {
		t.Error("Get(log_cleanup_jobs) must return nil: cleanup bookkeeping is never cleanable")
	}
	if Get("") != nil {
		t.Error("Get(\"\") must return nil")
	}
}
