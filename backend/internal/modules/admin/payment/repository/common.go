package repository

import (
	"strings"
	"time"
)

func normalizePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func normalizePageSize(pageSize int) int {
	if pageSize <= 0 {
		return 10
	}
	if pageSize > 100 {
		return 100
	}
	return pageSize
}

// normalizeTime 解析形如 2006-01-02 15:04:05 的时间字符串，失败返回 nil。
func normalizeTime(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", raw, time.Local)
	if err != nil {
		return nil
	}
	return &t
}

// normalizePeriodRange 把账期（如 202609）换算为左闭右开时间区间。
func normalizePeriodRange(period string) (start, end time.Time, ok bool) {
	period = strings.TrimSpace(period)
	if len(period) != 6 {
		return time.Time{}, time.Time{}, false
	}
	t, err := time.ParseInLocation("200601", period, time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	return t, t.AddDate(0, 1, 0), true
}
