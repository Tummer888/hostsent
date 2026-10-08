//go:build live

// 异常行为监控与黑名单的端到端联调用例（doc06 §4.3/§4.4）。
//
// 这一组要的是**完整装配的 HTTP 栈**：登录链路 → 黑名单校验 → 拒绝/放行、
// 登录信号 → 规则引擎 → 风险事件落库、管理端接口 → 事件查询/升级/汇总。
// 只测服务层会漏掉「端口没注入」「规则没接上登录链路」这类问题 ——
// 而改造前这两处恰恰就是断的（黑名单从没被登录链路读过、rule_code 从没被写过）。
//
// 运行：
//
//	go test -tags live ./internal/server/ -run 'TestLiveRisk|TestLiveBlacklist' -v
//
// 前置：本机可直连 postgres / redis（见 liveDSN / liveConfig）。
// 副作用：会建/删若干 zzlive_* 账号与黑名单条目、写入风险事件，
// 每个用例结束清理自己造的数据。
package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// doFrom 发一次带自定义对端地址与请求头的请求。
//
// 必须自己拼 httptest.Request 而不是复用 h.do：这组用例要精确控制客户端 IP
// （黑名单按 IP 命中、撞库按 IP 统计不同账号），而 h.do 用的是 httptest 的默认
// 对端地址 192.0.2.1 —— 所有用例共用同一个 IP，计数会互相污染，
// 断言也就不可复现了。
//
// 直接写 RemoteAddr 而不是 X-Forwarded-For：可信代理未配置时 gin 会忽略转发头，
// 写 RemoteAddr 才是真正决定 ClientIP 的输入（也与 netutil 的注释一致）。
func (h *liveHarness) doFrom(method, path, token, ip string, headers map[string]string, body any) (int, map[string]any) {
	h.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("序列化请求体失败: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "hostsent-live-test/1.0")
	req.RemoteAddr = ip + ":45678"
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.srv.http.Handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return resp.StatusCode, out
}

// loginRaw 用指定 IP / 设备指纹尝试一次用户登录，返回状态码与信封。
func (h *liveHarness) loginRaw(username, password, ip, deviceFP string) (int, map[string]any) {
	h.t.Helper()
	headers := map[string]string{}
	if deviceFP != "" {
		headers["X-Device-Fingerprint"] = deviceFP
	}
	return h.doFrom(http.MethodPost, "/api/v1/uc/auth/login", "", ip, headers, map[string]any{
		"username": username, "password": password,
	})
}

// cleanupRiskArtifacts 清掉本次用例造的风控数据（风险事件 / 登录日志 / 黑名单）。
//
// 按 username 或 ip 定向清，不做全表清理：联调库里有真实演示数据，
// 一条 DELETE 打歪就会把演示环境的风险事件全删掉。
func (h *liveHarness) cleanupRiskArtifacts(usernames []string, ips []string) {
	if h.db == nil {
		return
	}
	if len(usernames) > 0 {
		h.db.Exec("DELETE FROM risk_events WHERE username IN ?", usernames)
		h.db.Exec("DELETE FROM login_logs WHERE username IN ?", usernames)
		// 黑名单里 target_value 是账号名（user 类型）。
		h.db.Exec("DELETE FROM blacklists WHERE type = 'user' AND target_value IN ?", usernames)
	}
	if len(ips) > 0 {
		h.db.Exec("DELETE FROM risk_events WHERE ip IN ?", ips)
		h.db.Exec("DELETE FROM login_logs WHERE ip IN ?", ips)
		h.db.Exec("DELETE FROM blacklists WHERE type = 'ip' AND target_value IN ?", ips)
	}
}

// mustCreateBlacklist 通过管理端接口新增一条黑名单并登记清理。
func (h *liveHarness) mustCreateBlacklist(token, kind, value, reason string, extra map[string]any) uint64 {
	h.t.Helper()
	body := map[string]any{
		"type": kind, "target_value": value, "status": "active", "source": "manual", "reason": reason,
	}
	for k, v := range extra {
		body[k] = v
	}
	status, resp := h.do(http.MethodPost, "/api/v1/admin/security/blacklists", token, body)
	if status != http.StatusOK || digNumber(resp, "code") != 0 {
		h.t.Fatalf("创建黑名单失败: HTTP %d %v", status, resp)
	}
	id := uint64(digNumber(resp, "data", "id"))
	if id == 0 {
		h.t.Fatalf("创建黑名单未返回 id: %v", resp)
	}
	return id
}

// waitBlacklistVisible 等待黑名单对登录链路生效。
//
// 黑名单校验直接查库、没有缓存，因此创建后立即生效；保留这个函数是为了在
// 未来引入缓存时给用例留出明确的等待点，而不是散落 sleep。
func waitBlacklistVisible() { time.Sleep(50 * time.Millisecond) }

// ---------- 用例 1：黑名单真的拦截登录 ----------

// TestLiveBlacklistBlocksUserLogin 新增 IP / 账号黑名单后，登录被拒绝。
//
// 这是改造前最核心的缺口：blacklists 表只在管理页读写，登录链路从不读它 ——
// 运营拉黑一个 IP 之后对方照样能登录。本用例就是这条断链的回归断言。
func TestLiveBlacklistBlocksUserLogin(t *testing.T) {
	h := newLiveHarness(t)
	adminToken := h.loginAdmin()
	_, username := h.registerUser("blk")
	ip := fmt.Sprintf("198.51.100.%d", 10+rand.Intn(200))
	h.cleanupRiskArtifacts([]string{username}, []string{ip})
	t.Cleanup(func() { h.cleanupRiskArtifacts([]string{username}, []string{ip}) })

	// 基线：未被拉黑时可以正常登录。
	if status, body := h.loginRaw(username, liveUserPassword, ip, ""); status != http.StatusOK || digString(body, "data", "token") == "" {
		t.Fatalf("基线登录应成功: HTTP %d %v", status, body)
	}

	// 拉黑这个 IP。
	h.mustCreateBlacklist(adminToken, "ip", ip, "联调：撞库来源", nil)
	waitBlacklistVisible()

	status, body := h.loginRaw(username, liveUserPassword, ip, "")
	if code := digNumber(body, "code"); code != 20018 {
		t.Fatalf("IP 黑名单命中应返回 20018，实际 HTTP %d code=%v body=%v", status, code, body)
	}

	// 换一个 IP 用同一账号：账号本身没被拉黑，应当能登录。
	otherIP := fmt.Sprintf("203.0.113.%d", 10+rand.Intn(200))
	h.cleanupRiskArtifacts(nil, []string{otherIP})
	t.Cleanup(func() { h.cleanupRiskArtifacts(nil, []string{otherIP}) })
	if status, body := h.loginRaw(username, liveUserPassword, otherIP, ""); status != http.StatusOK || digString(body, "data", "token") == "" {
		t.Fatalf("IP 维度黑名单不应影响其它来源: HTTP %d %v", status, body)
	}

	// 再拉黑这个账号，换回未拉黑的 IP 也应被拒。
	h.mustCreateBlacklist(adminToken, "user", username, "联调：账号撞库", nil)
	waitBlacklistVisible()
	if status, body := h.loginRaw(username, liveUserPassword, otherIP, ""); digNumber(body, "code") != 20018 {
		t.Fatalf("账号黑名单命中应返回 20018，实际 HTTP %d %v", status, body)
	}
}

// TestLiveBlacklistReleaseAndExpiry 停用 / 已过期的黑名单必须放行。
//
// 运营把黑名单「解除」或限时到期后，对方必须真的能登录回来 ——
// 只写文案不生效的实现同样会被这条用例抓出来。
func TestLiveBlacklistReleaseAndExpiry(t *testing.T) {
	h := newLiveHarness(t)
	adminToken := h.loginAdmin()
	_, username := h.registerUser("blkrel")
	ip := fmt.Sprintf("198.51.100.%d", 60+rand.Intn(100))
	h.cleanupRiskArtifacts([]string{username}, []string{ip})
	t.Cleanup(func() { h.cleanupRiskArtifacts([]string{username}, []string{ip}) })

	// 新建时限时黑名单的失效时间必须晚于当前：填过去的时间是配置错误，
	// 静默接受会让运营以为封上了，而实际从未生效过。
	past := time.Now().Add(-time.Hour).Format("2006-01-02 15:04:05")
	status, resp := h.do(http.MethodPost, "/api/v1/admin/security/blacklists", adminToken, map[string]any{
		"type": "ip", "target_value": ip, "status": "active", "reason": "联调：过期时间填错",
		"expired_at": past,
	})
	if digNumber(resp, "code") == 0 {
		t.Fatalf("新增时失效时间早于当前应被拒绝，实际 HTTP %d %v", status, resp)
	}

	// 正常的限时黑名单：未过期 → 拦截。
	future := time.Now().Add(time.Hour).Format("2006-01-02 15:04:05")
	id := h.mustCreateBlacklist(adminToken, "ip", ip, "联调：限时生效", map[string]any{"expired_at": future})
	waitBlacklistVisible()
	if _, body := h.loginRaw(username, liveUserPassword, ip, ""); digNumber(body, "code") != 20018 {
		t.Fatalf("未过期黑名单应拦截，实际 %v", body)
	}

	// 把失效时间改到过去 = 立刻失效（运营提前解除限时黑名单的做法）→ 放行。
	status, resp = h.do(http.MethodPut, fmt.Sprintf("/api/v1/admin/security/blacklists/%d", id), adminToken,
		map[string]any{"expired_at": past})
	if status != http.StatusOK || digNumber(resp, "code") != 0 {
		t.Fatalf("更新失效时间失败: HTTP %d %v", status, resp)
	}
	waitBlacklistVisible()
	if _, body := h.loginRaw(username, liveUserPassword, ip, ""); digNumber(body, "code") == 20018 {
		t.Fatalf("失效时间已过应放行，实际 %v", body)
	}

	// 停用 → 放行。
	status, resp = h.do(http.MethodPatch, fmt.Sprintf("/api/v1/admin/security/blacklists/%d/status", id), adminToken,
		map[string]any{"status": "inactive"})
	if status != http.StatusOK || digNumber(resp, "code") != 0 {
		t.Fatalf("停用黑名单失败: HTTP %d %v", status, resp)
	}
	waitBlacklistVisible()
	if _, body := h.loginRaw(username, liveUserPassword, ip, ""); digNumber(body, "code") == 20018 {
		t.Fatalf("已停用黑名单不应拦截，实际 %v", body)
	}

	// 运行态字段：停用的条目 runtime_status 必须是 inactive（页面状态列读的就是它）。
	_, listResp := h.do(http.MethodGet, "/api/v1/admin/security/blacklists?page=1&page_size=50", adminToken, nil)
	for _, item := range digSlice(listResp, "data", "items") {
		row, _ := item.(map[string]any)
		if uint64(digNumber(map[string]any{"v": row["id"]}, "v")) != id {
			continue
		}
		if got := digString(map[string]any{"v": row}, "v"); got == "" {
			continue
		}
		runtime, _ := row["runtime_status"].(string)
		if runtime != "expired" && runtime != "inactive" {
			t.Errorf("停用条目的 runtime_status = %q，期望 inactive/expired", runtime)
		}
	}
}

// TestLiveBlacklistHitCountAndTrace 命中时累加计数并留下登录日志。
//
// 「命中次数」列与「命中记录」抽屉都依赖这条链路：不写 login_logs 的话
// 抽屉永远是空的，运营无法解释「这条黑名单到底有没有在拦人」。
func TestLiveBlacklistHitCountAndTrace(t *testing.T) {
	h := newLiveHarness(t)
	adminToken := h.loginAdmin()
	id, username := h.registerUser("blkhit")
	ip := fmt.Sprintf("203.0.113.%d", 100+rand.Intn(100))
	h.cleanupRiskArtifacts([]string{username}, []string{ip})
	t.Cleanup(func() { h.cleanupRiskArtifacts([]string{username}, []string{ip}) })

	blkID := h.mustCreateBlacklist(adminToken, "ip", ip, "联调：命中计数", nil)
	waitBlacklistVisible()

	for i := 0; i < 3; i++ {
		h.loginRaw(username, liveUserPassword, ip, "")
	}

	// 计数：直接读库最可靠（列表接口的 hit_count 同源）。
	var hitCount int
	h.db.Raw("SELECT hit_count FROM blacklists WHERE id = ?", blkID).Scan(&hitCount)
	if hitCount != 3 {
		t.Errorf("命中 3 次后 hit_count = %d，期望 3", hitCount)
	}

	// 命中记录：按黑名单类型关联登录日志，ip 类型走 login_logs.ip。
	status, resp := h.do(http.MethodGet,
		fmt.Sprintf("/api/v1/admin/security/blacklists/%d/hits?page=1&page_size=10", blkID), adminToken, nil)
	if status != http.StatusOK {
		t.Fatalf("命中记录接口失败: HTTP %d %v", status, resp)
	}
	total := digNumber(resp, "data", "meta", "total")
	if total < 3 {
		t.Errorf("命中记录总数 = %v，期望 >= 3（命中未写登录日志？）", total)
	}
	// 至少有一条能看出是黑名单导致的失败。
	var reason string
	h.db.Raw("SELECT COALESCE(failure_reason, '') FROM login_logs WHERE ip = ? ORDER BY id DESC LIMIT 1", ip).Scan(&reason)
	if reason == "" {
		t.Errorf("命中未在 login_logs 留下失败原因（命中记录抽屉将无内容）")
	}
	_ = id
}

// TestLiveBlacklistPhoneAndEmailType 手机号 / 邮箱类型的黑名单对验证码登录生效。
//
// 这两类是 doc06 §4.4 要求、改造前后端枚举都没有的类型：短信/邮箱登录的账号
// 目标就是手机号与邮箱，撞库时封 IP 会连坐同出口的正常用户。
func TestLiveBlacklistPhoneAndEmailType(t *testing.T) {
	h := newLiveHarness(t)
	adminToken := h.loginAdmin()
	_, username := h.registerUser("blkpe")
	ip := fmt.Sprintf("192.0.2.%d", 20+rand.Intn(200))
	h.cleanupRiskArtifacts([]string{username}, []string{ip})
	t.Cleanup(func() { h.cleanupRiskArtifacts([]string{username}, []string{ip}) })

	// 邮箱类型：用邮箱作为登录目标。
	email := username + "@example.com"
	h.mustCreateBlacklist(adminToken, "email", email, "联调：邮箱拉黑", nil)
	waitBlacklistVisible()

	status, body := h.doFrom(http.MethodPost, "/api/v1/uc/auth/login", "", ip, nil, map[string]any{
		"login_type": "email", "email": email, "code": "123456",
	})
	if digNumber(body, "code") != 20018 {
		t.Fatalf("邮箱黑名单应拦截（在任何验证码校验之前），实际 HTTP %d %v", status, body)
	}
	t.Cleanup(func() {
		h.db.Exec("DELETE FROM blacklists WHERE type = 'email' AND target_value = ?", email)
	})

	// 非法类型必须被拒（避免前端传入未落地的类型静默写入一条无效黑名单）。
	status, resp := h.do(http.MethodPost, "/api/v1/admin/security/blacklists", adminToken, map[string]any{
		"type": "ua", "target_value": "Mozilla/5.0", "status": "active",
	})
	if status != http.StatusInternalServerError && digNumber(resp, "code") == 0 {
		t.Fatalf("未支持的黑名单类型应被拒绝，实际 HTTP %d %v", status, resp)
	}
}

// ---------- 用例 2：规则引擎真的产生风险事件 ----------

// TestLiveRiskEngineFailThresholdAndAggregate 连续失败触发事件，重复命中聚合。
func TestLiveRiskEngineFailThresholdAndAggregate(t *testing.T) {
	h := newLiveHarness(t)
	_, username := h.registerUser("riskfail")
	ip := fmt.Sprintf("198.51.100.%d", 120+rand.Intn(100))
	h.cleanupRiskArtifacts([]string{username}, []string{ip})
	t.Cleanup(func() { h.cleanupRiskArtifacts([]string{username}, []string{ip}) })

	// 前 4 次失败：未达阈值（默认 5），不应有事件。
	for i := 0; i < 4; i++ {
		h.loginRaw(username, "wrong-password", ip, "")
	}
	if n := h.countRiskEvents(username, ""); n != 0 {
		t.Fatalf("连续失败 4 次（阈值 5）不应产生事件，实际 %d 条", n)
	}

	// 第 5 次失败：达到阈值，产生事件。
	h.loginRaw(username, "wrong-password", ip, "")
	n := h.countRiskEvents(username, "LOGIN_FAIL_THRESHOLD")
	if n != 1 {
		t.Fatalf("连续失败 5 次应产生 1 条 LOGIN_FAIL_THRESHOLD 事件，实际 %d 条", n)
	}

	// 继续失败：同类事件聚合，不新增行、只累加次数。
	h.loginRaw(username, "wrong-password", ip, "")
	if n := h.countRiskEvents(username, "LOGIN_FAIL_THRESHOLD"); n != 1 {
		t.Fatalf("同类命中应聚合（仍 1 条），实际 %d 条 —— 告警风暴", n)
	}
	var occur int
	h.db.Raw("SELECT occur_count FROM risk_events WHERE username = ? AND rule_code = ? ORDER BY id DESC LIMIT 1",
		username, "LOGIN_FAIL_THRESHOLD").Scan(&occur)
	if occur < 2 {
		t.Errorf("聚合后 occur_count = %d，期望 >= 2", occur)
	}

	// 事件字段完整性：等级/主体域/风险类型都要落地。
	var ev struct {
		RiskLevel   string
		RiskType    string
		SubjectType string
	}
	h.db.Raw(`SELECT risk_level, risk_type, subject_type FROM risk_events
		WHERE username = ? ORDER BY id DESC LIMIT 1`, username).Scan(&ev)
	level, riskType, subject := ev.RiskLevel, ev.RiskType, ev.SubjectType
	if riskType != "brute_force" || level != "high" {
		t.Errorf("事件类型/等级 = %s / %s，期望 brute_force / high", riskType, level)
	}
	if subject != "user" {
		t.Errorf("事件主体域 = %q，期望 user（客户登录）", subject)
	}
}

// TestLiveRiskEngineUnknownAccountsNotCollapsed 不存在的账号名之间不能共用一条事件。
//
// 撞库打的全是不存在的账号，user_id 全是 0。若聚合只按 (规则, user_id) 匹配，
// 被扫的几十个账号名会塌成一条事件、行上挂着第一个名字，运营只看到「某账号失败 5 次」，
// 完全看不出「有人正在扫一批账号」。这条用例就是钉住「聚合必须按账号分开」。
func TestLiveRiskEngineUnknownAccountsNotCollapsed(t *testing.T) {
	h := newLiveHarness(t)
	ip := fmt.Sprintf("198.51.100.%d", 40+rand.Intn(60))
	h.cleanupRiskArtifacts(nil, []string{ip})
	t.Cleanup(func() { h.cleanupRiskArtifacts(nil, []string{ip}) })

	// 两个不存在的账号名，各自在同一 IP 上失败到阈值以上。
	names := []string{
		fmt.Sprintf("zzlive_unknown_a_%d", rand.Intn(1_000_000)),
		fmt.Sprintf("zzlive_unknown_b_%d", rand.Intn(1_000_000)),
	}
	t.Cleanup(func() { h.cleanupRiskArtifacts(names, nil) })
	for _, name := range names {
		for i := 0; i < 6; i++ {
			h.loginRaw(name, "wrong-password", ip, "")
		}
	}

	for _, name := range names {
		if n := h.countRiskEvents(name, "LOGIN_FAIL_THRESHOLD"); n != 1 {
			t.Errorf("账号 %s 应有 1 条 LOGIN_FAIL_THRESHOLD 事件，实际 %d 条", name, n)
		}
	}
	// 两条事件必须分别挂着各自的账号名，且 user_id 都是 0（账号不存在）。
	var rows []struct {
		Username string
		UserID   uint64
	}
	h.db.Raw(`SELECT username, user_id FROM risk_events
		WHERE rule_code = ? AND ip = ? ORDER BY id`, "LOGIN_FAIL_THRESHOLD", ip).Scan(&rows)
	if len(rows) != 2 {
		t.Fatalf("两个不同账号应各有一条事件，实际 %d 条（被折叠了）", len(rows))
	}
	if rows[0].Username == rows[1].Username {
		t.Errorf("两条事件挂着同一个账号名 %q，说明聚合没按账号分开", rows[0].Username)
	}
	for _, row := range rows {
		if row.UserID != 0 {
			t.Errorf("不存在的账号 user_id 应为 0，实际 %d", row.UserID)
		}
	}
}

// TestLiveRiskEngineDeviceChange 新设备登录产生设备变更事件（低危）。
func TestLiveRiskEngineDeviceChange(t *testing.T) {
	h := newLiveHarness(t)
	_, username := h.registerUser("riskdev")
	ip := fmt.Sprintf("192.0.2.%d", 120+rand.Intn(100))
	h.cleanupRiskArtifacts([]string{username}, []string{ip})
	t.Cleanup(func() { h.cleanupRiskArtifacts([]string{username}, []string{ip}) })

	// 第一次登录（已建设备）：不报 —— 没有「历史常用设备」可比。
	if status, body := h.loginRaw(username, liveUserPassword, ip, "fp-device-a"); status != http.StatusOK || digString(body, "data", "token") == "" {
		t.Fatalf("首次登录应成功: HTTP %d %v", status, body)
	}
	if n := h.countRiskEvents(username, "DEVICE_FINGERPRINT_CHANGED"); n != 0 {
		t.Fatalf("首次登录（无历史设备）不应报设备变更，实际 %d 条", n)
	}

	// 用同一设备再登录：不报。
	h.loginRaw(username, liveUserPassword, ip, "fp-device-a")
	if n := h.countRiskEvents(username, "DEVICE_FINGERPRINT_CHANGED"); n != 0 {
		t.Fatalf("同一设备不应报设备变更，实际 %d 条", n)
	}

	// 换设备：报。
	h.loginRaw(username, liveUserPassword, ip, "fp-device-b")
	if n := h.countRiskEvents(username, "DEVICE_FINGERPRINT_CHANGED"); n != 1 {
		t.Fatalf("换设备应产生 1 条设备变更事件，实际 %d 条", n)
	}
	var devEv struct {
		RiskLevel string
		RiskType  string
	}
	h.db.Raw(`SELECT risk_level, risk_type FROM risk_events
		WHERE username = ? AND rule_code = ? ORDER BY id DESC LIMIT 1`,
		username, "DEVICE_FINGERPRINT_CHANGED").Scan(&devEv)
	level, riskType := devEv.RiskLevel, devEv.RiskType
	if level != "low" || riskType != "device_change" {
		t.Errorf("设备变更事件 等级/类型 = %s / %s，期望 low / device_change", level, riskType)
	}

	// 设备指纹也要落进会话表（运营在会话列表里看到的就靠它区分同一 NAT 下的设备）。
	var fp string
	h.db.Raw("SELECT COALESCE(device_fingerprint, '') FROM user_sessions WHERE username = ? ORDER BY id DESC LIMIT 1", username).Scan(&fp)
	if fp != "fp-device-b" {
		t.Errorf("会话未记录设备指纹（实际 %q），会话列表无法区分设备", fp)
	}
}

// TestLiveAdminLoginDeviceChange 员工登录链路同样跑设备变更规则。
//
// 客户侧有 TestLiveRiskEngineDeviceChange 覆盖，但员工登录是另一条完全独立的
// 代码路径（admin/manager 自己写日志、自己上报信号）。此前该路径的成功登录
// 不写 device_fingerprint，导致员工「历史常用设备」永远是空的 —— 设备变更规则
// 在员工入口静默失效。这条用例专门钉住那条路径。
//
// 断言用**增量**而不是绝对值：admin 是共享账号，联调库里本来就积了历史设备，
// 「第一次登录不报」这种绝对断言会被既有数据打破（实测踩到过）。改用
// 「同设备再加一次不增长、换设备必增长」来验证规则确实在跑。
func TestLiveAdminLoginDeviceChange(t *testing.T) {
	h := newLiveHarness(t)
	ip := fmt.Sprintf("192.0.2.%d", 30+rand.Intn(60))
	// 只按 IP 清理：username 是 admin，按账号名清会误删演示环境的管理员事件。
	h.cleanupRiskArtifacts(nil, []string{ip})
	t.Cleanup(func() { h.cleanupRiskArtifacts(nil, []string{ip}) })

	adminLogin := func(fp string) {
		h.t.Helper()
		status, body := h.doFrom(http.MethodPost, "/api/v1/admin/auth/login", "", ip,
			map[string]string{"X-Device-Fingerprint": fp},
			map[string]any{"username": "admin", "password": "123456"})
		if status != http.StatusOK || digString(body, "data", "token") == "" {
			h.t.Fatalf("管理员登录应成功（fp=%s）: HTTP %d %v", fp, status, body)
		}
	}

	// 第一次登录：把 zzfp-admin-a 写进历史（本次之前是否有别的设备无关紧要）。
	adminLogin("zzfp-admin-a")
	baseline := h.riskHitCountByIPRule(ip, "DEVICE_FINGERPRINT_CHANGED")

	// 用同一设备再登录：不该再报（a 已在历史里）。
	adminLogin("zzfp-admin-a")
	if n := h.riskHitCountByIPRule(ip, "DEVICE_FINGERPRINT_CHANGED"); n != baseline {
		t.Fatalf("同设备重复登录不应新增设备变更命中，基线 %d → 实际 %d", baseline, n)
	}

	// 换设备：必须新增命中（同类命中会聚合进同一条事件，所以看 occur_count 的和，
	// 而不是行数），且明细里能看到新设备指纹。
	adminLogin("zzfp-admin-b")
	after := h.riskHitCountByIPRule(ip, "DEVICE_FINGERPRINT_CHANGED")
	if after <= baseline {
		t.Fatalf("换设备应新增设备变更命中（基线 %d，实际 %d）——员工链路的设备变更规则没跑", baseline, after)
	}
	var ev struct {
		SubjectType   string
		DetailPayload string
	}
	h.db.Raw(`SELECT subject_type, detail_payload FROM risk_events
		WHERE ip = ? AND rule_code = ? ORDER BY id DESC LIMIT 1`,
		ip, "DEVICE_FINGERPRINT_CHANGED").Scan(&ev)
	if ev.SubjectType != "admin" {
		t.Errorf("员工事件主体域 = %q，期望 admin", ev.SubjectType)
	}
	if !strings.Contains(ev.DetailPayload, "zzfp-admin-b") {
		t.Errorf("命中明细缺少新设备指纹: %s", ev.DetailPayload)
	}

	// 成功登录的设备指纹必须落在 login_logs 里（历史设备的唯一来源）。
	var fp string
	h.db.Raw(`SELECT COALESCE(device_fingerprint, '') FROM login_logs
		WHERE subject_type = 'admin' AND ip = ? AND result = 'success'
		ORDER BY id DESC LIMIT 1`, ip).Scan(&fp)
	if fp != "zzfp-admin-b" {
		t.Errorf("员工成功登录未记录设备指纹（实际 %q），设备变更规则将失去基线", fp)
	}
}

// TestLiveRiskEngineCredentialStuffing 同一 IP 打多个账号触发撞库事件（严重级）。
//
// 这是 login_guard 完全看不到的一类攻击：单账号连错有锁定兜底，
// 但「一个 IP 打 8 个不同账号」只有规则引擎能发现。
func TestLiveRiskEngineCredentialStuffing(t *testing.T) {
	h := newLiveHarness(t)
	ip := fmt.Sprintf("203.0.113.%d", 200+rand.Intn(50))
	h.cleanupRiskArtifacts(nil, []string{ip})
	t.Cleanup(func() { h.cleanupRiskArtifacts(nil, []string{ip}) })

	// 8 个互不相同的账号名（不需要真实存在：撞库打的就是一批未知账号）。
	usernames := make([]string, 0, 8)
	for i := 0; i < 8; i++ {
		name := fmt.Sprintf("zzlive_stuff_%d_%d", rand.Intn(1_000_000), i)
		usernames = append(usernames, name)
		h.loginRaw(name, "wrong-password", ip, "")
	}
	t.Cleanup(func() { h.cleanupRiskArtifacts(usernames, nil) })

	var found int
	for _, name := range usernames {
		found += h.countRiskEvents(name, "IP_MULTI_ACCOUNT_FAIL")
	}
	if found == 0 {
		t.Fatal("同 IP 打 8 个不同账号应产生撞库事件（IP_MULTI_ACCOUNT_FAIL），实际没有")
	}
	var stuffEv struct {
		RiskLevel string
		RiskType  string
	}
	h.db.Raw(`SELECT risk_level, risk_type FROM risk_events
		WHERE rule_code = ? AND ip = ? ORDER BY id DESC LIMIT 1`,
		"IP_MULTI_ACCOUNT_FAIL", ip).Scan(&stuffEv)
	level, riskType := stuffEv.RiskLevel, stuffEv.RiskType
	if level != "critical" {
		t.Errorf("撞库事件等级 = %q，期望 critical", level)
	}
	if riskType != "brute_force" {
		t.Errorf("撞库事件类型 = %q，期望 brute_force", riskType)
	}
}

// TestLiveRiskEngineHealthyLoginQuiet 正常登录不产生任何风险事件（防误报）。
func TestLiveRiskEngineHealthyLoginQuiet(t *testing.T) {
	h := newLiveHarness(t)
	_, username := h.registerUser("riskok")
	ip := fmt.Sprintf("198.51.100.%d", 220+rand.Intn(30))
	h.cleanupRiskArtifacts([]string{username}, []string{ip})
	t.Cleanup(func() { h.cleanupRiskArtifacts([]string{username}, []string{ip}) })

	for i := 0; i < 3; i++ {
		if status, body := h.loginRaw(username, liveUserPassword, ip, "fp-stable"); status != http.StatusOK || digString(body, "data", "token") == "" {
			t.Fatalf("正常登录应成功: HTTP %d %v", status, body)
		}
	}
	if n := h.countRiskEvents(username, ""); n != 0 {
		t.Fatalf("同一账号同一设备同一 IP 连续成功登录不应产生风险事件，实际 %d 条", n)
	}
}

// TestLiveRiskEventAdminWorkflow 管理端接口：汇总、详情、手动升级等级。
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
}

// countRiskEvents 统计某账号（或指定规则）的风险事件条数。
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
