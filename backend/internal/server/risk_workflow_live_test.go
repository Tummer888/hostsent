//go:build live

package server

import (
	"fmt"
	"math/rand"
	"net/http"
	"testing"
)

// 安全风控端到端——风险事件处置工作流与会话失效精度（doc91 §4）。

func TestLiveRiskEventAdminWorkflow(t *testing.T) {
	h := newLiveHarness(t)
	adminToken := h.loginAdmin()
	_, username := h.registerUser("riskadm")
	ip := fmt.Sprintf("192.0.2.%d", 220+rand.Intn(30))
	h.cleanupRiskArtifacts([]string{username}, []string{ip})
	t.Cleanup(func() { h.cleanupRiskArtifacts([]string{username}, []string{ip}) })

	for i := 0; i < 5; i++ {
		h.loginRaw(username, "wrong-password", ip, "")
	}

	// 列表能查到，且处置人姓名字段存在（未处置时为空）。
	status, resp := h.do(http.MethodGet,
		fmt.Sprintf("/api/v1/admin/security/risk-events?page=1&page_size=10&keyword=%s", username), adminToken, nil)
	if status != http.StatusOK || digNumber(resp, "code") != 0 {
		t.Fatalf("风险事件列表失败: HTTP %d %v", status, resp)
	}
	items := digSlice(resp, "data", "items")
	if len(items) == 0 {
		t.Fatal("应能查到刚产生的风险事件")
	}
	row, _ := items[0].(map[string]any)
	eventID := uint64(row["id"].(float64))
	if subject, _ := row["subject_type"].(string); subject != "user" {
		t.Errorf("列表回显 subject_type = %q，期望 user", subject)
	}
	if _, ok := row["handled_by_name"]; !ok {
		t.Error("列表缺少 handled_by_name 字段（处置人列无法显示姓名）")
	}

	// 汇总接口：待处理至少 1 条。
	status, statsResp := h.do(http.MethodGet,
		fmt.Sprintf("/api/v1/admin/security/risk-events/stats?keyword=%s", username), adminToken, nil)
	if status != http.StatusOK {
		t.Fatalf("汇总接口失败: HTTP %d %v", status, statsResp)
	}
	pending := digNumber(statsResp, "data", "pending")
	if pending < 1 {
		t.Errorf("汇总 pending = %v，期望 >= 1", pending)
	}

	// 手动升级等级：critical。
	status, levelResp := h.do(http.MethodPost,
		fmt.Sprintf("/api/v1/admin/security/risk-events/%d/level", eventID), adminToken,
		map[string]any{"risk_level": "critical", "note": "联调：手工提级"})
	if status != http.StatusOK || digNumber(levelResp, "code") != 0 {
		t.Fatalf("升级风险等级失败: HTTP %d %v", status, levelResp)
	}
	if got := digString(levelResp, "data", "risk_level"); got != "critical" {
		t.Errorf("升级后等级 = %q，期望 critical", got)
	}
	// 处置人姓名应回填成 admin。
	if got := digString(levelResp, "data", "handled_by_name"); got != "admin" {
		t.Errorf("处置人姓名 = %q，期望 admin", got)
	}
	// 状态必须仍是 pending：升级等级不等于已处置，否则刚提级的事件
	// 会从「待处理」里消失，运营反而丢了待办。
	if got := digString(levelResp, "data", "status"); got != "pending" {
		t.Errorf("升级等级后状态 = %q，期望仍是 pending", got)
	}

	// 非法等级被拒。
	if status, _ := h.do(http.MethodPost,
		fmt.Sprintf("/api/v1/admin/security/risk-events/%d/level", eventID), adminToken,
		map[string]any{"risk_level": "banana"}); status == http.StatusOK {
		t.Error("非法等级应返回非 200")
	}

	// 拉黑这条事件命中的 IP，再登录应被拒（处置动作真的接线）。
	status, blkResp := h.do(http.MethodPost,
		fmt.Sprintf("/api/v1/admin/security/risk-events/%d/blacklist", eventID), adminToken,
		map[string]any{"note": "联调：由风险事件拉黑"})
	if status != http.StatusOK || digNumber(blkResp, "code") != 0 {
		t.Fatalf("风险事件拉黑失败: HTTP %d %v", status, blkResp)
	}
	waitBlacklistVisible()
	if _, body := h.loginRaw(username, liveUserPassword, ip, ""); digNumber(body, "code") != 20018 {
		t.Errorf("由风险事件拉黑后登录应被拒，实际 %v", body)
	}

	// —— 拉黑是管控动作，不改待办状态 ——
	//
	// 改造前这里会写出一条「已处置」的假象；现在的口径是「封了但还要盯」，
	// 状态仍是 pending，动作列亮出 blacklist。
	if got := h.riskEventStatus(eventID); got != "pending" {
		t.Errorf("拉黑后状态 = %q，期望仍是 pending（管控动作不等于结案）", got)
	}
	codes := h.actionCodes(eventID)
	if len(codes) != 2 || codes[0] != "level" || codes[1] != "blacklist" {
		t.Errorf("处置流水 = %v，期望 [level blacklist]", codes)
	}
	// 列表回显动作：这是「点完看不出效果」的直接修复点。
	if row := h.riskEventRowByID(adminToken, eventID); row != nil {
		actions, _ := row["actions"].([]any)
		if len(actions) == 0 {
			t.Error("列表未回显已做动作（actions 为空），运营点完拉黑看不出变化")
		}
		if summary, _ := row["action_summary"].(string); summary == "" {
			t.Error("列表未回显动作摘要（action_summary 为空）")
		}
	} else {
		t.Errorf("列表里找不到事件 #%d", eventID)
	}

	// —— 按已做动作筛 ——
	if n := h.riskEventMatches(adminToken,
		fmt.Sprintf("keyword=%s&action=blacklist", username)); n != 1 {
		t.Errorf("action=blacklist 应筛出 1 条，实际 %d 条", n)
	}
	if n := h.riskEventMatches(adminToken,
		fmt.Sprintf("keyword=%s&action=revoke_sessions", username)); n != 0 {
		t.Errorf("action=revoke_sessions 不应筛出任何事件，实际 %d 条", n)
	}

	// —— 处置时间线 ——
	status, actionsResp := h.do(http.MethodGet,
		fmt.Sprintf("/api/v1/admin/security/risk-events/%d/actions", eventID), adminToken, nil)
	if status != http.StatusOK || digNumber(actionsResp, "code") != 0 {
		t.Fatalf("处置时间线接口失败: HTTP %d %v", status, actionsResp)
	}
	actionItems := digSlice(actionsResp, "data", "items")
	if len(actionItems) != 2 {
		t.Fatalf("处置时间线应有 2 条，实际 %d 条", len(actionItems))
	}
	first, _ := actionItems[0].(map[string]any)
	if got, _ := first["operator_name"].(string); got != "admin" {
		t.Errorf("时间线操作人 = %q，期望 admin（应补全姓名而不是只回 ID）", got)
	}

	// —— 处置关单：一次点完「处置 + 拉黑」+ 强制下线 ——
	//
	// 这里刻意用同一个 IP 但换一个**新注册**的客户来跑，避免被上一步的黑名单挡住。
	_, closeUser := h.registerUser("riskclose")
	closeIP := fmt.Sprintf("192.0.2.%d", 100+rand.Intn(60))
	// 复用 cleanup：关闭用例结束后一并清掉。
	h.cleanupRiskArtifacts([]string{closeUser}, []string{closeIP})
	t.Cleanup(func() { h.cleanupRiskArtifacts([]string{closeUser}, []string{closeIP}) })
	// 先正常登录一次，保证有一条 active 会话可被踢。
	if status, body := h.loginRaw(closeUser, liveUserPassword, closeIP, "fp-close-a"); status != http.StatusOK {
		t.Fatalf("基线登录应成功: HTTP %d %v", status, body)
	}
	// 再连续失败 5 次触发事件（失败不改会话状态）。
	for i := 0; i < 5; i++ {
		h.loginRaw(closeUser, "wrong-password", closeIP, "fp-close-a")
	}
	closeEventID, _ := h.riskEventRow(adminToken, closeUser, "")
	if closeEventID == 0 {
		t.Fatal("应能查到刚产生的风险事件")
	}
	status, closeResp := h.do(http.MethodPost,
		fmt.Sprintf("/api/v1/admin/security/risk-events/%d/handle", closeEventID), adminToken,
		map[string]any{
			"note":            "联调：确认攻击，处置并管控",
			"blacklist":       true,
			"revoke_sessions": true,
		})
	if status != http.StatusOK || digNumber(closeResp, "code") != 0 {
		t.Fatalf("处置+管控失败: HTTP %d %v", status, closeResp)
	}
	if got := digString(closeResp, "data", "status"); got != "handled" {
		t.Errorf("处置后状态 = %q，期望 handled", got)
	}
	closeCodes := h.actionCodes(closeEventID)
	wantCodes := map[string]bool{"blacklist": false, "revoke_sessions": false, "handle": false}
	for _, code := range closeCodes {
		wantCodes[code] = true
	}
	for code, seen := range wantCodes {
		if !seen {
			t.Errorf("处置流水缺少动作 %q（实际 %v）", code, closeCodes)
		}
	}
	// 会话应真的被撤销（管控动作不是写个状态就完事）。
	var revoked int64
	h.db.Table("user_sessions").Where("username = ? AND status = ?", closeUser, "revoked").Count(&revoked)
	if revoked == 0 {
		t.Error("勾选了强制下线，但该账号没有会话被撤销")
	}
	// 黑名单应立即生效。
	waitBlacklistVisible()
	if _, body := h.loginRaw(closeUser, liveUserPassword, closeIP, "fp-close-b"); digNumber(body, "code") != 20018 {
		t.Errorf("处置时勾选的拉黑未生效，登录未被拒: %v", body)
	}

	// —— 管控失败则整体不结案（不留「已处置但其实没封上」的假象）——
	//
	// 把同一维度再拉黑一次会命中唯一索引；此时 closeRiskEvent 必须报错且不关单。
	_, dupUser := h.registerUser("riskdup")
	dupIP := fmt.Sprintf("192.0.2.%d", 20+rand.Intn(60))
	h.cleanupRiskArtifacts([]string{dupUser}, []string{dupIP})
	t.Cleanup(func() { h.cleanupRiskArtifacts([]string{dupUser}, []string{dupIP}) })
	h.mustCreateBlacklist(adminToken, "ip", dupIP, "联调：预置同维度黑名单", nil)
	waitBlacklistVisible()
	for i := 0; i < 5; i++ {
		h.loginRaw(dupUser, "wrong-password", dupIP, "")
	}
	dupEventID, _ := h.riskEventRow(adminToken, dupUser, "")
	if dupEventID == 0 {
		t.Fatal("应能查到刚产生的风险事件")
	}
	if status, _ := h.do(http.MethodPost,
		fmt.Sprintf("/api/v1/admin/security/risk-events/%d/handle", dupEventID), adminToken,
		map[string]any{"note": "联调：重复拉黑", "blacklist": true}); status == http.StatusOK {
		t.Error("重复拉黑应失败并返回非 200（否则会留下「已处置但没封上」的假象）")
	}
	if got := h.riskEventStatus(dupEventID); got != "pending" {
		t.Errorf("管控失败后事件状态 = %q，期望仍是 pending（不得标记为已处置）", got)
	}
}

// TestLiveRiskEventRevokeSessionsPrecision 失效会话必须精确到事件的主体域与用户。
//
// 改造前 RevokeSessionsFromRisk 走的是 ListSessions(UserID: event.UserID)，
// 既不带 subject_type 也没有 user_id=0 的保护。两个后果：
//   - 员工事件（user_id 是 admins.id）会去踢 ID 相同的**客户**会话（撞号）；
//   - 账号不存在的撞库事件（user_id=0）会把全站所有 user_id=0 的会话当成目标。
//
// 现在的口径：员工域直接拒绝（管理端令牌不带 sid，没有可踢的会话）、
// user_id=0 直接拒绝、客户域按 (subject_type, user_id) 精确匹配。
func TestLiveRiskEventRevokeSessionsPrecision(t *testing.T) {
	h := newLiveHarness(t)
	adminToken := h.loginAdmin()

	// a) 员工事件：必须明确拒绝，而不是返回一个空的「成功」。
	adminIP := fmt.Sprintf("192.0.2.%d", 150+rand.Intn(50))
	h.doFrom(http.MethodPost, "/api/v1/admin/auth/login", "", adminIP,
		map[string]string{"X-Device-Fingerprint": "fp-revoke-admin"},
		map[string]any{"username": "admin", "password": "123456"})
	h.doFrom(http.MethodPost, "/api/v1/admin/auth/login", "", adminIP,
		map[string]string{"X-Device-Fingerprint": "fp-revoke-admin-2"},
		map[string]any{"username": "admin", "password": "123456"})
	var adminEventID uint64
	h.db.Table("risk_events").
		Where("subject_type = ? AND ip = ? AND rule_code = ?", "admin", adminIP, "DEVICE_FINGERPRINT_CHANGED").
		Order("id DESC").Limit(1).Pluck("id", &adminEventID)
	if adminEventID == 0 {
		t.Skip("未能造出员工设备变更事件，跳过员工域断言")
	}
	t.Cleanup(func() { h.cleanupRiskArtifacts(nil, []string{adminIP}) })

	status, body := h.do(http.MethodPost,
		fmt.Sprintf("/api/v1/admin/security/risk-events/%d/revoke-sessions", adminEventID), adminToken,
		map[string]any{"note": "联调：员工域应被拒"})
	if status == http.StatusOK {
		t.Errorf("员工事件失效会话应被拒绝，实际 HTTP 200 %v —— 静默假成功会让运营以为已踢下线", body)
	}

	// b) 撞库事件（账号不存在，user_id=0）：同样必须拒绝，
	//    否则会把全站所有 user_id=0 的会话当成目标。
	stuffIP := fmt.Sprintf("203.0.113.%d", 30+rand.Intn(60))
	h.cleanupRiskArtifacts(nil, []string{stuffIP})
	t.Cleanup(func() { h.cleanupRiskArtifacts(nil, []string{stuffIP}) })
	stufferNames := make([]string, 0, 8)
	for i := 0; i < 8; i++ {
		name := fmt.Sprintf("zzlive_nostuff_%d_%d", rand.Intn(1_000_000), i)
		stufferNames = append(stufferNames, name)
		h.loginRaw(name, "wrong-password", stuffIP, "")
	}
	t.Cleanup(func() { h.cleanupRiskArtifacts(stufferNames, nil) })
	var stuffEventID uint64
	var stuffUserID uint64
	h.db.Table("risk_events").Where("rule_code = ? AND ip = ?", "IP_MULTI_ACCOUNT_FAIL", stuffIP).
		Order("id DESC").Limit(1).Pluck("id", &stuffEventID)
	if stuffEventID == 0 {
		t.Skip("未能造出撞库事件，跳过 user_id=0 断言")
	}
	h.db.Table("risk_events").Where("id = ?", stuffEventID).Pluck("user_id", &stuffUserID)
	if stuffUserID != 0 {
		t.Skipf("撞库事件的 user_id = %d（期望 0），环境与用例前提不符，跳过", stuffUserID)
	}
	status, body = h.do(http.MethodPost,
		fmt.Sprintf("/api/v1/admin/security/risk-events/%d/revoke-sessions", stuffEventID), adminToken,
		map[string]any{"note": "联调：无账号事件应被拒"})
	if status == http.StatusOK {
		t.Errorf("user_id=0 的事件失效会话应被拒绝，实际 HTTP 200 %v", body)
	}

	// c) 客户事件：精确踢同主体域的会话，且不影响 staff 域。
	_, username := h.registerUser("riskprec")
	ip := fmt.Sprintf("198.51.100.%d", 150+rand.Intn(60))
	h.cleanupRiskArtifacts([]string{username}, []string{ip})
	t.Cleanup(func() { h.cleanupRiskArtifacts([]string{username}, []string{ip}) })
	if status, body := h.loginRaw(username, liveUserPassword, ip, "fp-prec-a"); status != http.StatusOK {
		t.Fatalf("基线登录应成功: HTTP %d %v", status, body)
	}
	for i := 0; i < 5; i++ {
		h.loginRaw(username, "wrong-password", ip, "fp-prec-a")
	}
	eventID, _ := h.riskEventRow(adminToken, username, "")
	if eventID == 0 {
		t.Fatal("应能查到风险事件")
	}
	var clientSessions int64
	h.db.Table("user_sessions").Where("username = ? AND status = ?", username, "active").Count(&clientSessions)
	if clientSessions == 0 {
		t.Fatal("基线登录应留下至少一条有效会话")
	}
	status, body = h.do(http.MethodPost,
		fmt.Sprintf("/api/v1/admin/security/risk-events/%d/revoke-sessions", eventID), adminToken,
		map[string]any{"note": "联调：精确失效"})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("客户事件失效会话失败: HTTP %d %v", status, body)
	}
	if got := digNumber(body, "data", "meta", "total"); got <= 0 {
		t.Errorf("应撤销至少 1 条客户会话，实际 %v 条", got)
	}
	var stillActive int64
	h.db.Table("user_sessions").Where("username = ? AND status = ?", username, "active").Count(&stillActive)
	if stillActive != 0 {
		t.Errorf("仍有 %d 条会话处于 active，失效未生效", stillActive)
	}
	// staff 域会话（subject_type=admin）不得被这条客户事件波及 ——
	// 联调库里 admin 演示会话与客户 1 号可能撞 ID，这正是要防的越界。
	var staffRevokedByThis int64
	h.db.Table("user_sessions").
		Where("subject_type = ? AND status = ? AND revoked_reason LIKE ?", "admin", "revoked", "联调：精确失效%").
		Count(&staffRevokedByThis)
	if staffRevokedByThis != 0 {
		t.Errorf("客户事件的失效会话误伤了 %d 条员工会话（主体域未过滤）", staffRevokedByThis)
	}
}

// countRiskEvents 统计某账号（或指定规则）的风险事件条数。
