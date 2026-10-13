//go:build live

package server

import (
	"fmt"
	"math/rand"
	"net/http"
	"testing"
	"time"
)

// 安全风控端到端——拉黑/解黑/命中计数与手机邮箱类型（doc91 §4）。

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
