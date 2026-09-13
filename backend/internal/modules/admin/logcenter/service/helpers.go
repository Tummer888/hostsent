package service

import (
	"encoding/json"
	"strings"
	"time"
)

// encodeSummary 摘要编码：空摘要写空串（落库时转 NULL），不写 "{}"。
func encodeSummary(summary map[string]any) string {
	if len(summary) == 0 {
		return ""
	}
	raw, err := json.Marshal(summary)
	if err != nil {
		return ""
	}
	return string(raw)
}

// decodeSummary 摘要解码（前端展示用）。
func decodeSummary(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

// encodeStrings 字符串数组 → jsonb 列文本；空切片写空串（落库转 NULL）。
func encodeStrings(items []string) string {
	if len(items) == 0 {
		return ""
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return ""
	}
	return string(raw)
}

// decodeStrings jsonb 文本 → 字符串数组。
func decodeStrings(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

// encodeUint64s uint64 数组 → jsonb 文本。
func encodeUint64s(items []uint64) string {
	if len(items) == 0 {
		return ""
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return ""
	}
	return string(raw)
}

// decodeUint64s jsonb 文本 → uint64 数组。
func decodeUint64s(raw string) []uint64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out []uint64
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

// truncate 截断字符串（列宽保护）。
func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && (s[cut]&0xC0) == 0x80 {
		cut--
	}
	return s[:cut]
}

// fmtTime 统一时间格式（doc92 §9.3：前端不做时区换算，后端给本地时间串）。
func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

// fmtTimePtr 可空时间格式化。
func fmtTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return fmtTime(*t)
}
