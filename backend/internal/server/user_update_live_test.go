//go:build live

package server

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"testing"

	"golang.org/x/crypto/bcrypt"

	accountdto "hostsent/backend/internal/modules/admin/user/account/dto"
	accountrepo "hostsent/backend/internal/modules/admin/user/account/repository"
	securityrepo "hostsent/backend/internal/modules/admin/user/security/repository"
	securityservice "hostsent/backend/internal/modules/admin/user/security/service"
)

// 用户体系收口端到端联调——资料更新/列表性能/安全操作人/会话（doc104 §10.5、F5、会话下线）。
// TestLiveUserUpdatePartialAndConflict 覆盖 doc104 §10.5 的 R1 与 R2。
//
// R2 是本轮索引改造里风险最高的一项：迁移 051 把 idx_users_username /
// idx_users_email 换成了带 WHERE deleted_at IS NULL 的部分唯一索引，约束名变了。
// 冲突识别靠错误文本匹配（PG 的 23505 不带列名），认不出新名字就会把 409 退化成
// 500 —— 运营看到的是「服务器内部错误」，然后对着一个不存在的服务端故障排查半天。
//
// R1 与它相邻：唯一冲突只有在「只传部分字段」时才会被触发（只改邮箱不该动用户名），
// 两者一起测才能同时钉住「nil 不修改」与「冲突仍映射 409」。
func TestLiveUserUpdatePartialAndConflict(t *testing.T) {
	h := newLiveHarness(t)
	admin := h.loginAdmin()

	idA, nameA := h.registerUser("patcha")
	idB, nameB := h.registerUser("patchb")

	// 先把 A 的资料填满，便于验证「只传一个字段时其余不变」。
	status, body := h.do(http.MethodPut, fmt.Sprintf("/api/v1/admin/users/%d", idA), admin, map[string]any{
		"real_name": "张三",
		"phone":     "13800000001",
		"region":    "east",
	})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("填充资料失败: HTTP %d %v", status, body)
	}

	// R1：只传 email，其余字段必须原样保留。
	status, body = h.do(http.MethodPut, fmt.Sprintf("/api/v1/admin/users/%d", idA), admin, map[string]any{
		"email": nameA + "+new@example.com",
	})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("部分更新失败: HTTP %d %v", status, body)
	}
	var gotUsername, gotRealName, gotPhone, gotRegion, gotEmail string
	h.db.Raw("SELECT username, real_name, phone, region, email FROM users WHERE id = ?", idA).
		Row().Scan(&gotUsername, &gotRealName, &gotPhone, &gotRegion, &gotEmail) //nolint:errcheck // live 维护脚本：Scan 错误由后续字段断言暴露
	if gotUsername != nameA {
		t.Errorf("R1：只传 email 时 username 被改动：%q → %q", nameA, gotUsername)
	}
	if gotRealName != "张三" || gotPhone != "13800000001" || gotRegion != "east" {
		t.Errorf("R1：只传 email 时其余字段被清空：real_name=%q phone=%q region=%q",
			gotRealName, gotPhone, gotRegion)
	}
	if gotEmail != nameA+"+new@example.com" {
		t.Errorf("R1：email 未更新，实际 %q", gotEmail)
	}

	// R1 反向：显式空串 = 清空（nil 与 "" 语义必须可区分）。
	status, body = h.do(http.MethodPut, fmt.Sprintf("/api/v1/admin/users/%d", idA), admin, map[string]any{
		"phone": "",
	})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("清空字段失败: HTTP %d %v", status, body)
	}
	h.db.Raw("SELECT phone, real_name FROM users WHERE id = ?", idA).Row().Scan(&gotPhone, &gotRealName) //nolint:errcheck // live 维护脚本：Scan 错误由后续字段断言暴露
	if gotPhone != "" {
		t.Errorf("R1：显式空串应清空 phone，实际 %q", gotPhone)
	}
	if gotRealName != "张三" {
		t.Errorf("R1：清空 phone 不应影响 real_name，实际 %q", gotRealName)
	}

	// R2：撞用户名 → 409（不是 500），且库里没有被改动。
	status, body = h.do(http.MethodPut, fmt.Sprintf("/api/v1/admin/users/%d", idA), admin, map[string]any{
		"username": nameB,
	})
	if status != http.StatusConflict {
		t.Fatalf("R2：用户名冲突应返回 409，实际 HTTP %d %v（索引名未被 isUniqueViolation 识别？）", status, body)
	}
	h.db.Raw("SELECT username FROM users WHERE id = ?", idA).Scan(&gotUsername)
	if gotUsername != nameA {
		t.Errorf("R2：冲突请求改动了库中用户名：%q", gotUsername)
	}

	// R2：撞邮箱 → 409。
	status, body = h.do(http.MethodPut, fmt.Sprintf("/api/v1/admin/users/%d", idA), admin, map[string]any{
		"email": nameB + "@example.com",
	})
	if status != http.StatusConflict {
		t.Fatalf("R2：邮箱冲突应返回 409，实际 HTTP %d %v", status, body)
	}

	// R2 的边界：注销用户释放出来的账号名可以正常占用（部分唯一索引的核心收益）。
	// 用 B 的账号名先注销 B，再把 A 改成那个名字，应当成功而不是 409。
	status, body = h.do(http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%d", idB), admin,
		map[string]any{"reason": "联调：验证部分唯一索引", "force": true})
	if status != http.StatusOK {
		t.Fatalf("注销 B 失败: HTTP %d %v", status, body)
	}
	status, body = h.do(http.MethodPut, fmt.Sprintf("/api/v1/admin/users/%d", idA), admin, map[string]any{
		"username": nameB,
	})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("R2：注销用户的账号名应可被占用（部分唯一索引），实际 HTTP %d %v", status, body)
	}
}

// TestLiveUserListNoNPlusOne 覆盖 doc104 §10.5 的 R3：列表无 N+1。
//
// 翻一页 10 行的角色查询必须是 **1 条**（批量 IN 查询），而不是 10 条。
// 断言方式：给 gorm 挂一个计数 logger，直接数 SQL 条数 —— 只检查代码里
// 「调了一次 RolesByUserIDs」是自我安慰，改动可能在任何一层把批量又拆回循环。
//
// 同时钉住本轮新增的两条批量查询（oauth 绑定、注销人姓名）也没有退化成逐行查询。
func TestLiveUserListNoNPlusOne(t *testing.T) {
	h := newLiveHarness(t)

	// 造 5 个用户，保证列表页至少有一行（空列表会让「1 条查询」变成「0 条」而失去意义）。
	var ids []uint64
	for i := 0; i < 5; i++ {
		id, _ := h.registerUser("nplus1")
		ids = append(ids, id)
	}
	if len(ids) != 5 {
		t.Fatalf("造数据失败，只有 %d 个用户", len(ids))
	}

	counting := liveCountingDB(t)
	repo := accountrepo.NewUserRepository(counting)

	resetLiveQueries()
	users, total, err := repo.List(context.Background(), accountdto.UserListQuery{
		Page: 1, PageSize: 10, IncludeDeleted: true,
	})
	if err != nil {
		t.Fatalf("列表查询失败: %v", err)
	}
	if total == 0 || len(users) == 0 {
		t.Fatal("列表返回空结果，无法验证 N+1（联调库应至少有种子用户）")
	}

	// 角色查询：必须只有 1 条，且带 IN 批量条件。
	if n := countLiveQueries("JOIN roles ON roles.id = user_roles.role_id"); n != 1 {
		t.Errorf("R3：角色查询发了 %d 条（期望 1 条批量查询）—— N+1 回归", n)
	}
	if n := countLiveQueries("user_roles.user_id IN"); n != 1 {
		t.Errorf("R3：角色批量查询未带 IN 条件（发了 %d 条）", n)
	}
	// 本轮新增的两条批量查询同样必须是 1 条。
	if n := countLiveQueries("FROM \"user_oauth_bindings\""); n > 1 {
		t.Errorf("R3：oauth 绑定查询发了 %d 条（期望 ≤1 条批量查询）", n)
	}
	if n := countLiveQueries("FROM \"admins\""); n > 1 {
		t.Errorf("R3：注销人姓名查询发了 %d 条（期望 ≤1 条批量查询）", n)
	}
	// 反向守卫：整页的查询总数不该随行数增长。5 行数据 + 4 条辅助查询，
	// 给到 12 条的上限已经非常宽松（真正退化成 N+1 时是 5+ 条额外查询）。
	if n := len(liveQueryCounter.items); n > 12 {
		t.Errorf("R3：单页共发 %d 条 SQL，疑似存在逐行查询（上限 12）", n)
	}
}

// TestLiveSecurityOperatorIdentity 覆盖 doc104 §10.3 第 9 行（F5 回归）。
//
// 非 admin 的管理员踢会话后，revoked_by 必须是其真实 ID 而不是硬编码的 1。
func TestLiveSecurityOperatorIdentity(t *testing.T) {
	h := newLiveHarness(t)
	h.loginAdmin()

	// 造一个临时运维管理员（演示账号 demo_tech_01 只在 900 演示种子里，
	// 全新引导库没有；按需创建并绑定持有 security:session:manage 的角色）。
	// ops_admin 是唯一持有该权限的非超管角色，按 code 动态取 ID（不硬编码）。
	var opsRoleID uint64
	h.db.Raw("SELECT id FROM roles WHERE code = 'ops_admin' LIMIT 1").Scan(&opsRoleID)
	if opsRoleID == 0 {
		t.Fatal("缺少 ops_admin 角色")
	}
	opName := fmt.Sprintf("zzlive_op_%d", rand.Intn(1_000_000))
	hash, err := bcrypt.GenerateFromPassword([]byte("Zzlive@123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	var operatorAdminID uint64
	if err := h.db.Exec(`INSERT INTO admins (username, email, password_hash, role, status, must_change_password)
		VALUES (?, ?, ?, 'ops_admin', 'active', false)`,
		opName, opName+"@example.com", string(hash)).Error; err != nil {
		t.Fatalf("造运维管理员失败: %v", err)
	}
	h.db.Raw("SELECT id FROM admins WHERE username = ?", opName).Scan(&operatorAdminID)
	if operatorAdminID == 0 {
		t.Fatal("造运维管理员后读不到 ID")
	}
	t.Cleanup(func() {
		h.db.Exec("DELETE FROM admin_roles WHERE admin_id = ?", operatorAdminID)
		h.db.Exec("DELETE FROM admins WHERE id = ?", operatorAdminID)
	})
	mustExec(t, h, "INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)", operatorAdminID, opsRoleID)

	status, body := h.do(http.MethodPost, "/api/v1/admin/auth/login", "", map[string]any{
		"username": opName, "password": "Zzlive@123",
	})
	opToken := digString(body, "data", "token")
	if status != http.StatusOK || opToken == "" {
		t.Fatalf("运维账号登录失败: HTTP %d %v", status, body)
	}

	// 造一条待踢的会话
	targetID, targetName := h.registerUser("sessiontarget")
	sessionKey := fmt.Sprintf("zzlive_session_%d", rand.Intn(1_000_000))
	mustExec(t, h, `INSERT INTO user_sessions
		(session_id, user_id, username, platform, ip, user_agent, login_at, last_active_at, status)
		VALUES (?, ?, ?, 'web', '127.0.0.1', 'live-test', NOW(), NOW(), 'active')`,
		sessionKey, targetID, targetName)
	var sessionRowID uint64
	h.db.Raw("SELECT id FROM user_sessions WHERE session_id = ?", sessionKey).Scan(&sessionRowID)
	if sessionRowID == 0 {
		t.Fatal("造会话行失败")
	}

	status, body = h.do(http.MethodPost,
		fmt.Sprintf("/api/v1/admin/security/sessions/%d/revoke", sessionRowID), opToken,
		map[string]any{"reason": "联调用例：强制下线"})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("踢会话失败: HTTP %d %v", status, body)
	}
	if got := digNumber(body, "data", "revoked_by"); uint64(got) != operatorAdminID {
		t.Errorf("revoked_by = %v，期望 %d（硬编码为 1 是 F5 回归）", got, operatorAdminID)
	}
	var storedBy uint64
	var storedStatus string
	h.db.Raw("SELECT COALESCE(revoked_by, 0), status FROM user_sessions WHERE id = ?", sessionRowID).
		Row().Scan(&storedBy, &storedStatus) //nolint:errcheck // live 维护脚本：Scan 错误由后续字段断言暴露
	if storedBy != operatorAdminID {
		t.Errorf("库中 revoked_by = %d，期望 %d", storedBy, operatorAdminID)
	}
	if storedStatus != "revoked" {
		t.Errorf("会话状态 = %q，期望 revoked", storedStatus)
	}
}

// TestLiveSessionRevokeKillsToken 覆盖「强制下线必须让令牌立即失效」。
//
// 这是本轮修复的核心回归点。此前 user_sessions.status 被撤销后，请求链路上
// 没有任何地方读它（JWT 无状态、没有 token_version），实测同一个令牌在
// 「撤销全部会话」之后照样能调通 /uc/auth/userinfo —— 安全页的强制下线
// 只是个装饰。现在令牌里带 sid，中间件每次请求校验会话有效性。
//
// 断言的是**行为**而不是实现：撤销后同一个令牌必须 401，且在线统计要跟着降。
func TestLiveSessionRevokeKillsToken(t *testing.T) {
	h := newLiveHarness(t)
	admin := h.loginAdmin()
	id, username := h.registerUser("revoketoken")
	token := h.loginUser(username, liveUserPassword)

	// 前置：令牌与它绑定的会话都可用。
	if status, body := h.do(http.MethodGet, "/api/v1/uc/auth/userinfo", token, nil); status != http.StatusOK {
		t.Fatalf("登录后令牌应可用: HTTP %d %v", status, body)
	}
	var sessionKey string
	h.db.Raw("SELECT session_id FROM user_sessions WHERE user_id = ? AND status = 'active' ORDER BY id DESC LIMIT 1", id).
		Scan(&sessionKey)
	if sessionKey == "" {
		t.Fatal("登录未写入 user_sessions（登录日志/会话记录缺失是 doc104 F15 的回归）")
	}
	if !h.onlineUserPresent(admin, id) {
		t.Fatal("登录后该用户应出现在在线统计里")
	}

	// 撤销该用户全部会话（管理员「强制下线」走的就是这个接口）。
	status, body := h.do(http.MethodPost, "/api/v1/admin/security/sessions/revoke-user-all", admin,
		map[string]any{"user_id": id, "reason": "联调用例：强制下线"})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("强制下线失败: HTTP %d %v", status, body)
	}

	// 核心断言：同一个令牌必须立刻不可用（不允许有缓存 TTL 的窗口）。
	if status, body := h.do(http.MethodGet, "/api/v1/uc/auth/userinfo", token, nil); status != http.StatusUnauthorized {
		t.Fatalf("撤销后令牌必须 401，实际 HTTP %d %v（强制下线未生效）", status, body)
	}
	// 在线统计必须同步下降：撤销的会话不该继续算作在线。
	if h.onlineUserPresent(admin, id) {
		t.Error("撤销后该用户仍出现在在线统计里（在线口径未按会话有效性过滤）")
	}
}

// TestLiveLogoutRevokesOwnSession 覆盖登出只结束当前会话。
//
// 登出必须真正撤销服务端会话：只清本地令牌的话，那条会话会一直挂在安全页的
// 「在线用户」里直到 JWT 自然过期（默认 24 小时），看起来像用户还在线。
func TestLiveLogoutRevokesOwnSession(t *testing.T) {
	h := newLiveHarness(t)
	id, username := h.registerUser("logoutrevoke")
	token := h.loginUser(username, liveUserPassword)

	if status, body := h.do(http.MethodPost, "/api/v1/uc/auth/logout", token, nil); status != http.StatusOK {
		t.Fatalf("登出失败: HTTP %d %v", status, body)
	}

	var status2 string
	h.db.Raw("SELECT status FROM user_sessions WHERE user_id = ? ORDER BY id DESC LIMIT 1", id).Scan(&status2)
	if status2 != "revoked" {
		t.Errorf("登出后会话状态 = %q，期望 revoked", status2)
	}
	if status, _ := h.do(http.MethodGet, "/api/v1/uc/auth/userinfo", token, nil); status != http.StatusUnauthorized {
		t.Errorf("登出后旧令牌必须 401，实际 HTTP %d", status)
	}
}

// TestLiveExpiredSessionIsOffline 覆盖「过期会话不算在线、也不能用」。
//
// 会话过期有两条独立路径，都要成立：
//   - 令牌仍然合法（JWT 未到期），但会话行已过期 → 中间件按会话判据拒绝；
//   - 在线统计按 expired_at 过滤，不把过期会话算成在线。
//
// 用例直接把会话行的 expired_at 改到过去来模拟「已过期但 status 还没被回写」，
// 这正是回写调度器上线前会长期存在的真实状态。
func TestLiveExpiredSessionIsOffline(t *testing.T) {
	h := newLiveHarness(t)
	admin := h.loginAdmin()
	id, username := h.registerUser("expiredsess")
	token := h.loginUser(username, liveUserPassword)

	// 先把该用户的会话标记为「已过期但状态仍 active」。
	mustExec(t, h, `UPDATE user_sessions SET expired_at = NOW() - INTERVAL '1 hour'
		WHERE user_id = ? AND status = 'active'`, id)

	if status, body := h.do(http.MethodGet, "/api/v1/uc/auth/userinfo", token, nil); status != http.StatusUnauthorized {
		t.Fatalf("过期会话的令牌必须 401，实际 HTTP %d %v", status, body)
	}
	if h.onlineUserPresent(admin, id) {
		t.Error("过期会话不应计入在线用户")
	}

	// 会话列表按「在线」筛选时同样不该出现（口径必须与中间件一致）。
	status, body := h.do(http.MethodGet, "/api/v1/admin/security/sessions?user_id="+fmt.Sprint(id)+"&status=active", admin, nil)
	if status != http.StatusOK {
		t.Fatalf("查询会话列表失败: HTTP %d %v", status, body)
	}
	if n := len(digSlice(body, "data", "items")); n != 0 {
		t.Errorf("按「在线」筛选返回了 %d 条已过期会话，期望 0 条", n)
	}
}

// TestLiveExpireStaleSessionsRewritesStatus 覆盖过期回写把 status 收成 expired。
//
// 没有这条回写，expired_at 到点后 status 会永远停在 active，
// 安全页按状态筛选/导出、用户详情的「有效会话数」都会长期虚高。
func TestLiveExpireStaleSessionsRewritesStatus(t *testing.T) {
	h := newLiveHarness(t)
	id, username := h.registerUser("expirewrite")
	h.loginUser(username, liveUserPassword)
	mustExec(t, h, `UPDATE user_sessions SET expired_at = NOW() - INTERVAL '1 hour'
		WHERE user_id = ? AND status = 'active'`, id)

	// 直接用仓储 + 服务跑回写，而不是拿应用内部那个实例：
	// liveApp 用 sync.Once 共享一份应用，往里塞测试数据再触发它的调度器
	// 会影响同进程内其它用例。这里关心的是回写逻辑本身（仓储 SQL + 状态判定），
	// 用同一张表、同一个库，结论等价。缓存失效由 TestLiveExpiredSessionIsOffline 覆盖。
	svc := securityservice.NewSecurityService(securityrepo.NewSecurityRepository(h.db))
	n, err := svc.ExpireStaleSessions(context.Background(), 500)
	if err != nil {
		t.Fatalf("过期回写失败: %v", err)
	}
	if n == 0 {
		t.Fatal("应至少回写一条过期会话")
	}
	var status2 string
	h.db.Raw("SELECT status FROM user_sessions WHERE user_id = ? ORDER BY id DESC LIMIT 1", id).Scan(&status2)
	if status2 != "expired" {
		t.Errorf("回写后状态 = %q，期望 expired", status2)
	}

	// 反向守卫：revoked 的行不能被这个批处理覆盖成 expired —— 那会丢掉
	// 「谁在什么时候踢的」这条审计线索。
	otherID, otherName := h.registerUser("expirekeeprevoked")
	key := fmt.Sprintf("zzlive_keeprevoked_%d", rand.Intn(1_000_000))
	mustExec(t, h, `INSERT INTO user_sessions
		(session_id, user_id, username, platform, ip, user_agent, login_at, last_active_at, expired_at, status, revoked_reason, revoked_at)
		VALUES (?, ?, ?, 'web', '127.0.0.1', 'live-test', NOW(), NOW(), NOW() - INTERVAL '1 hour', 'revoked', '人工撤销', NOW())`,
		key, otherID, otherName)
	if _, err := svc.ExpireStaleSessions(context.Background(), 500); err != nil {
		t.Fatalf("第二轮过期回写失败: %v", err)
	}
	var keptStatus, keptReason string
	h.db.Raw("SELECT status, COALESCE(revoked_reason, '') FROM user_sessions WHERE session_id = ?", key).
		Row().Scan(&keptStatus, &keptReason) //nolint:errcheck // live 维护脚本：Scan 错误由后续字段断言暴露
	if keptStatus != "revoked" || keptReason != "人工撤销" {
		t.Errorf("revoked 行被回写覆盖: status=%q reason=%q，期望 revoked/人工撤销", keptStatus, keptReason)
	}
}

// TestLiveManualRechargeCreatesBill 覆盖「用户列表 → 人工代充值」的完整落账。
//
// 此前这个入口只调钱包人工调账，写一条 type=adjust 的流水就结束：充值单表没有行、
// 账单里没有充值额，运营在「充值管理」与「账单」两个页面都查不到刚充的钱。
// 本用例锁死四件事必须同时发生：
//  1. 生成充值单（success + manual），充值单号回给调用方；
//  2. 写 type=recharge 的资金流水，ref_no 即充值单号（幂等键一致）；
//  3. **每笔充值各开一张**独立账单：bill_type=recharge、source_no=充值单号、
//     状态已结清、recharge_amount 等于该笔金额，且不计入 total_amount 应结；
//  4. 按期账单不承载充值额（recharge_amount 恒为 0）。
//
// 反向守卫（同一用例内）：负数金额是人工扣减，不该生成充值单，也不该开账单。
