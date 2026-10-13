//go:build live

package server

import (
	"fmt"
	"net/http"
)

func (h *liveHarness) countRiskEvents(username, ruleCode string) int {
	h.t.Helper()
	if h.db == nil {
		return 0
	}
	var n int64
	q := h.db.Table("risk_events").Where("username = ?", username)
	if ruleCode != "" {
		q = q.Where("rule_code = ?", ruleCode)
	}
	q.Count(&n)
	return int(n)
}

// riskHitCountByIPRule 按来源 IP + 规则统计**命中次数**（occur_count 之和，不是行数）。
//
// 员工登录用的账号名是固定的「admin」，按账号名清/查会碰到演示环境里管理员的
// 既有事件；用本次用例专用的 IP 才隔离得干净。
//
// 求和而不是数行：同类命中会聚合进同一条未处置事件（只累加 occur_count），
// 数行会得到恒为 1 的结果，「换设备又多命中了一次」就断言不出来。
func (h *liveHarness) riskHitCountByIPRule(ip, ruleCode string) int {
	h.t.Helper()
	if h.db == nil {
		return 0
	}
	var total int64
	q := h.db.Table("risk_events").Where("ip = ?", ip)
	if ruleCode != "" {
		q = q.Where("rule_code = ?", ruleCode)
	}
	q.Select("COALESCE(SUM(occur_count), 0)").Scan(&total)
	return int(total)
}

// actionCodes 取某事件的处置动作码（按流水顺序）。
func (h *liveHarness) actionCodes(eventID uint64) []string {
	h.t.Helper()
	if h.db == nil {
		return nil
	}
	var codes []string
	h.db.Table("risk_event_actions").Where("event_id = ?", eventID).
		Order("id").Pluck("action", &codes)
	return codes
}

// riskEventStatus 取某事件当前的处置状态。
func (h *liveHarness) riskEventStatus(eventID uint64) string {
	h.t.Helper()
	if h.db == nil {
		return ""
	}
	var status string
	h.db.Table("risk_events").Where("id = ?", eventID).Pluck("status", &status)
	return status
}

// riskEventByKeyword 取列表接口中第一条匹配关键词的事件，返回其 id 与响应体。
func (h *liveHarness) riskEventRow(token, keyword string, query string) (uint64, map[string]any) {
	h.t.Helper()
	path := fmt.Sprintf("/api/v1/admin/security/risk-events?page=1&page_size=10&keyword=%s", keyword)
	if query != "" {
		path += "&" + query
	}
	status, resp := h.do(http.MethodGet, path, token, nil)
	if status != http.StatusOK || digNumber(resp, "code") != 0 {
		h.t.Fatalf("查询风险事件失败: HTTP %d %v", status, resp)
	}
	items := digSlice(resp, "data", "items")
	if len(items) == 0 {
		return 0, nil
	}
	row, _ := items[0].(map[string]any)
	id := uint64(0)
	if raw, ok := row["id"].(float64); ok {
		id = uint64(raw)
	}
	return id, row
}

// riskEventRowByID 在列表里按事件 ID 取那一行。
//
// 不能用「关键词命中的第一条」：同一账号在用例过程中可能产生多条事件
// （黑名单命中也会计一次登录失败），而列表按 id DESC 排，第一条未必是
// 被测的那条 —— 实测踩到过，断言会莫名失败。
func (h *liveHarness) riskEventRowByID(token string, eventID uint64) map[string]any {
	h.t.Helper()
	status, resp := h.do(http.MethodGet, "/api/v1/admin/security/risk-events?page=1&page_size=50", token, nil)
	if status != http.StatusOK || digNumber(resp, "code") != 0 {
		h.t.Fatalf("查询风险事件失败: HTTP %d %v", status, resp)
	}
	for _, item := range digSlice(resp, "data", "items") {
		row, _ := item.(map[string]any)
		if raw, ok := row["id"].(float64); ok && uint64(raw) == eventID {
			return row
		}
	}
	return nil
}

// riskEventMatches 列表接口在给定筛选下命中的事件数。
func (h *liveHarness) riskEventMatches(token, query string) int {
	h.t.Helper()
	status, resp := h.do(http.MethodGet, "/api/v1/admin/security/risk-events?page=1&page_size=50&"+query, token, nil)
	if status != http.StatusOK || digNumber(resp, "code") != 0 {
		h.t.Fatalf("查询风险事件失败: HTTP %d %v", status, resp)
	}
	return len(digSlice(resp, "data", "items"))
}
