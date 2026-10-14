//go:build live

package server

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"testing"
)

// 安全风控端到端——阈值聚合/设备变更/撞库/静默登录（doc91 §3）。

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
	// 按 IP 清理本次事件与登录日志。admin 是共享账号，按账号名清会误删演示环境
	// 的管理员事件，所以这里只动 IP 维度（见 cleanupRiskArtifacts 注释）。
	h.cleanupRiskArtifacts(nil, []string{ip})
	t.Cleanup(func() { h.cleanupRiskArtifacts(nil, []string{ip}) })

	// 聚合是跨 IP 的：FindOpenEvent 按 (user_id, username, rule_code, status) 匹配，
	// 窗口内若有其他 IP 挂的 open 设备变更事件，本用例的命中会被吸走聚合进去，
	// 按本用例 IP 统计恒为 0。把 admin 的既有 open 事件处置掉，隔离聚合竞争。
	h.closeAdminOpenEvents()

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
