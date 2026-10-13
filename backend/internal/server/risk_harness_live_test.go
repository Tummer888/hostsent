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
	"io"
	"net/http"
	"net/http/httptest"
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
	defer func() { _ = resp.Body.Close() }()
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

// cleanupRiskArtifacts 清掉本次用例造的风控数据（风险事件 / 处置流水 / 登录日志 / 黑名单）。
//
// 按 username 或 ip 定向清，不做全表清理：联调库里有真实演示数据，
// 一条 DELETE 打歪就会把演示环境的风险事件全删掉。
func (h *liveHarness) cleanupRiskArtifacts(usernames []string, ips []string) {
	if h.db == nil {
		return
	}
	if len(usernames) > 0 {
		h.db.Exec("DELETE FROM risk_event_actions WHERE event_id IN (SELECT id FROM risk_events WHERE username IN ?)", usernames)
		h.db.Exec("DELETE FROM risk_events WHERE username IN ?", usernames)
		h.db.Exec("DELETE FROM login_logs WHERE username IN ?", usernames)
		// 黑名单里 target_value 是账号名（user 类型）。
		h.db.Exec("DELETE FROM blacklists WHERE type = 'user' AND target_value IN ?", usernames)
	}
	if len(ips) > 0 {
		h.db.Exec("DELETE FROM risk_event_actions WHERE event_id IN (SELECT id FROM risk_events WHERE ip IN ?)", ips)
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
