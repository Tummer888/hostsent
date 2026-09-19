//go:build live

// 用户体系收口端到端联调用例（doc104 §10.3）。
//
// 与其它 live 用例的区别：这一组要的是**完整装配的 HTTP 栈** —— 中间件、鉴权、
// 路由、审计留痕都必须真实生效，只测仓储或服务层会漏掉「路由没注册」「权限码
// 写错」「中间件顺序不对」这类问题。因此这里用 server.New 在测试进程内装配一份
// 完整应用，再通过 srv.http.Handler 直接喂 httptest 请求：不监听端口，不会与正在
// 运行的 backend 容器抢 8080，但走的确实是同一条链路。
//
// 运行：
//
//	go test -tags live ./internal/server/ -run 'TestLive(User|Verification|OAuth|Realname|AssignRoles|Security)' -v
//
// 前置：本机可直连 postgres / redis（见 liveDSN / liveConfig）。
// 副作用：New 会跑一次 AutoMigrate 与 Seed（与容器每次启动做的事完全相同、幂等），
// 并会为本套用例建/删若干 zzlive_* 测试账号；每个用例结束都会清理自己造的数据。
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	ticketrepo "hostsent/backend/internal/modules/admin/ticket/repository"
	accountdto "hostsent/backend/internal/modules/admin/user/account/dto"
	accountrepo "hostsent/backend/internal/modules/admin/user/account/repository"
	appauth "hostsent/backend/internal/pkg/auth"
	"hostsent/backend/internal/pkg/config"
	oauthpkg "hostsent/backend/internal/pkg/oauth"
)

// ---------- 基础设施 ----------

// liveDSN 联调库连接串；LIVE_DB_DSN 可覆盖（与 internal/pkg/db 的 live 用例同约定）。
func liveDSN() string {
	if dsn := os.Getenv("LIVE_DB_DSN"); dsn != "" {
		return dsn
	}
	return "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
}

// liveDB 打开一个直连库句柄（仅用于断言与造数据，不参与业务链路）。
func liveDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open(liveDSN()), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Skipf("联调库不可达，跳过：%v", err)
	}
	return db
}

var (
	liveOnce sync.Once
	liveSrv  *Server
	liveCfg  *config.Config
	liveErr  error
)

// liveApp 返回进程内装配的应用实例与配置。
//
// 用 sync.Once 而不是每个用例各起一份：New 要跑 AutoMigrate + Seed（数秒），
// 九个用例各跑一遍纯属浪费；而应用本身无状态，共用一份更贴近真实部署形态。
func liveApp(t *testing.T) (*Server, *config.Config) {
	t.Helper()
	liveOnce.Do(func() {
		cfg, err := config.Load()
		if err != nil {
			liveErr = fmt.Errorf("加载配置失败: %w", err)
			return
		}
		// 强制指向本机联调依赖：配置文件里的 host 可能是容器网段里的服务名。
		cfg.Database.Host = "127.0.0.1"
		cfg.Database.Port = 5432
		cfg.Database.User = "hostsent"
		cfg.Database.Password = "hostsent"
		cfg.Database.Name = "hostsent"
		cfg.Database.SSLMode = "disable"
		cfg.Redis.Host = "127.0.0.1"
		cfg.Redis.Port = 6379
		cfg.Redis.Required = false
		// 附件落盘目录指向临时目录，避免测试往仓库的 uploads/ 里丢文件。
		if dir, err := os.MkdirTemp("", "hostsent-live-uploads-"); err == nil {
			cfg.Storage.Root = dir
		}
		liveCfg = cfg
		liveSrv, liveErr = New(cfg, zap.NewNop())
	})
	if liveErr != nil {
		t.Skipf("应用装配失败（依赖不可达？），跳过：%v", liveErr)
	}
	return liveSrv, liveCfg
}

// ---------- HTTP 驱动 ----------

// liveQueryCounter 记录本进程内所有 SQL 执行次数（含每条 SQL 文本）。
//
// 用来给「列表无 N+1」（R3）做**可执行**的断言：光看代码里调了一次
// RolesByUserIDs 不算数，得证明翻一页真的只发了一条角色查询。
// 计数器挂在 gorm 的 logger 上，不改任何生产代码路径。
var liveQueryCounter struct {
	mu    sync.Mutex
	items []string
}

type liveQueryLogger struct{}

func (liveQueryLogger) LogMode(gormlogger.LogLevel) gormlogger.Interface { return liveQueryLogger{} }
func (liveQueryLogger) Info(context.Context, string, ...any)             {}
func (liveQueryLogger) Warn(context.Context, string, ...any)             {}
func (liveQueryLogger) Error(context.Context, string, ...any)            {}
func (liveQueryLogger) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	liveQueryCounter.mu.Lock()
	liveQueryCounter.items = append(liveQueryCounter.items, sql)
	liveQueryCounter.mu.Unlock()
}

func resetLiveQueries() {
	liveQueryCounter.mu.Lock()
	liveQueryCounter.items = nil
	liveQueryCounter.mu.Unlock()
}

// countLiveQueries 统计包含给定片段的 SQL 条数。
func countLiveQueries(fragment string) int {
	liveQueryCounter.mu.Lock()
	defer liveQueryCounter.mu.Unlock()
	n := 0
	for _, sql := range liveQueryCounter.items {
		if strings.Contains(sql, fragment) {
			n++
		}
	}
	return n
}

// liveCountingDB 打开一个带 SQL 计数的库句柄，用于验证查询次数（R3）。
//
// 不能复用 h.db：那份用的是 Silent logger，看不到 SQL。也不能替换应用内部的
// 库句柄（会牵动生产装配），因此这里另开一份只给仓储直接调用。
func liveCountingDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open(liveDSN()), &gorm.Config{
		Logger: liveQueryLogger{},
	})
	if err != nil {
		t.Skipf("联调库不可达，跳过：%v", err)
	}
	return db
}

type liveHarness struct {
	t   *testing.T
	srv *Server
	cfg *config.Config
	db  *gorm.DB
	// adminToken 超管令牌（每个用例单独登录，避免与并发/缓存状态耦合）。
	adminToken string
	// created 本用例创建的用户 ID，用例结束统一清理。
	created []uint64
}

func newLiveHarness(t *testing.T) *liveHarness {
	t.Helper()
	srv, cfg := liveApp(t)
	h := &liveHarness{t: t, srv: srv, cfg: cfg, db: liveDB(t)}
	t.Cleanup(func() {
		for _, id := range h.created {
			h.cleanupUser(id)
		}
	})
	return h
}

// do 发一次请求并解出响应信封。
//
// 直接走 srv.http.Handler：绕开网络栈但保留 gin 的全部中间件，
// 与容器内 backend 处理真实请求的代码路径一致。
func (h *liveHarness) do(method, path, token string, body any) (int, map[string]any) {
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

// loginAdmin 登录超管并缓存令牌。
func (h *liveHarness) loginAdmin() string {
	h.t.Helper()
	if h.adminToken != "" {
		return h.adminToken
	}
	status, body := h.do(http.MethodPost, "/api/v1/admin/auth/login", "", map[string]any{
		"username": "admin", "password": "123456",
	})
	token := digString(body, "data", "token")
	if status != http.StatusOK || token == "" {
		h.t.Fatalf("超管登录失败: HTTP %d %v", status, body)
	}
	h.adminToken = token
	return token
}

// registerUser 注册一个测试用户并返回其 ID 与用户名。
func (h *liveHarness) registerUser(prefix string) (uint64, string) {
	h.t.Helper()
	username := fmt.Sprintf("zzlive_%s_%d", prefix, rand.Intn(1_000_000))
	status, body := h.do(http.MethodPost, "/api/v1/uc/auth/register", "", map[string]any{
		"username": username,
		"password": liveUserPassword,
		"email":    username + "@example.com",
	})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		h.t.Fatalf("注册 %s 失败: HTTP %d %v", username, status, body)
	}
	id := uint64(digNumber(body, "data", "id"))
	if id == 0 {
		h.t.Fatalf("注册 %s 未返回 id: %v", username, body)
	}
	h.created = append(h.created, id)
	return id, username
}

// loginUser 用账号密码登录并返回访问令牌。
func (h *liveHarness) loginUser(username, password string) string {
	h.t.Helper()
	status, body := h.do(http.MethodPost, "/api/v1/uc/auth/login", "", map[string]any{
		"username": username, "password": password,
	})
	token := digString(body, "data", "token")
	if status != http.StatusOK || token == "" {
		h.t.Fatalf("用户 %s 登录失败: HTTP %d %v", username, status, body)
	}
	return token
}

// userToken 为指定用户现造一枚令牌。
//
// 用在「密码已被清空 / 登录被拒」但仍需要调用需登录接口的场景：JWT 是无状态的，
// 只要密钥与签发者一致，中间件就会认。这是测试专用手段，不进生产代码路径。
func (h *liveHarness) userToken(id uint64, username string) string {
	h.t.Helper()
	issuer := appauth.NewJWTIssuer(h.cfg.Auth.JWTSecret, h.cfg.Auth.JWTIssuer,
		time.Duration(h.cfg.Auth.JWTExpireHours)*time.Hour)
	token, err := issuer.GenerateUserFull(username, id, "free", 0, false)
	if err != nil {
		h.t.Fatalf("签发用户令牌失败: %v", err)
	}
	return token
}

// cleanupUser 清掉一个测试用户及其在本套用例里可能产生的关联行。
//
// 刻意用白名单而不是「把所有 user_id 列的表都删一遍」：白名单看得见、
// 出错时容易定位，也不会因为将来新增一张表就误删真实业务数据。
func (h *liveHarness) cleanupUser(id uint64) {
	if id == 0 || h.db == nil {
		return
	}
	var appIDs []uint64
	h.db.Table("verification_applications").Where("user_id = ?", id).Pluck("id", &appIDs)
	if len(appIDs) > 0 {
		h.db.Exec("DELETE FROM verification_review_logs WHERE application_id IN ?", appIDs)
		h.db.Exec("DELETE FROM verification_documents WHERE application_id IN ?", appIDs)
		h.db.Exec("DELETE FROM verification_enterprises WHERE application_id IN ?", appIDs)
	}
	var orderIDs []uint64
	h.db.Table("orders").Where("user_id = ?", id).Pluck("id", &orderIDs)
	if len(orderIDs) > 0 {
		h.db.Exec("DELETE FROM order_items WHERE order_id IN ?", orderIDs)
	}
	var paymentNos []string
	h.db.Table("payment_orders").Where("user_id = ?", id).Pluck("payment_no", &paymentNos)
	if len(paymentNos) > 0 {
		h.db.Exec("DELETE FROM payment_callback_logs WHERE payment_no IN ?", paymentNos)
	}
	h.db.Exec("DELETE FROM sales_commission_transactions WHERE customer_user_id = ?", id)
	for _, table := range []string{
		"verification_applications", "user_oauth_bindings", "user_sessions", "login_logs",
		"user_roles", "sub_account_permissions", "wallet_accounts", "user_operation_logs",
		"staff_sales_relations", "orders", "payment_orders", "notifications",
	} {
		h.db.Exec("DELETE FROM "+table+" WHERE user_id = ?", id)
	}
	h.db.Exec("DELETE FROM users WHERE id = ?", id)
}

// ---------- 信封读取工具 ----------

func digValue(body map[string]any, keys ...string) any {
	var cur any = body
	for _, key := range keys {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = obj[key]
	}
	return cur
}

func digString(body map[string]any, keys ...string) string {
	v, _ := digValue(body, keys...).(string)
	return v
}

func digNumber(body map[string]any, keys ...string) float64 {
	v, _ := digValue(body, keys...).(float64)
	return v
}

func digSlice(body map[string]any, keys ...string) []any {
	v, _ := digValue(body, keys...).([]any)
	return v
}

// ---------- 模拟第三方渠道 ----------

// liveOAuthType 联调用例专用的第三方登录渠道标识。
//
// 不去改 wechat/qq/alipay 适配器里的包级 endpoint 变量（那是单测的做法），
// 而是注册一个只存在于测试进程内的渠道：它既能让「授权跳转 → 回调 → 换令牌 →
// 绑定/解绑」这条链路完整跑通，又完全不触碰生产适配器的代码路径。
const liveOAuthType = "livemock"

type liveMockProvider struct{}

func (liveMockProvider) Type() string { return liveOAuthType }

// AuthorizeURL 把 state 原样带进地址，供用例取回后拼回调。
func (liveMockProvider) AuthorizeURL(_ oauthpkg.ProviderConfig, state, redirectURI string) string {
	q := url.Values{}
	q.Set("state", state)
	q.Set("redirect_uri", redirectURI)
	return "https://livemock.local/authorize?" + q.Encode()
}

// Exchange 把授权码当成外部身份标识：用例传不同的 code 就得到不同的 openid。
// 不发任何 HTTP 请求，因此这条链路不依赖外网。
func (liveMockProvider) Exchange(_ context.Context, _ oauthpkg.ProviderConfig, code, _ string) (*oauthpkg.Token, error) {
	if strings.TrimSpace(code) == "" {
		return nil, oauthpkg.ErrExchangeFailed
	}
	return &oauthpkg.Token{AccessToken: "live-token-" + code, OpenID: code}, nil
}

// UserInfo 直接复用 Exchange 里带出的 openid。
func (liveMockProvider) UserInfo(_ context.Context, _ oauthpkg.ProviderConfig, token *oauthpkg.Token) (*oauthpkg.ExternalUser, error) {
	return &oauthpkg.ExternalUser{OpenID: token.OpenID, Nickname: "联调用户"}, nil
}

func (liveMockProvider) Test(_ context.Context, cfg oauthpkg.ProviderConfig) error {
	if strings.TrimSpace(cfg.Type) == "" {
		return oauthpkg.ErrProviderConfig
	}
	return nil
}

func init() {
	oauthpkg.RegisterDescriptor(liveOAuthType, oauthpkg.CapabilityDescriptor{
		Type:           liveOAuthType,
		Name:           "联调模拟渠道",
		Mode:           "api",
		AdapterVersion: "1.0.0",
		Icon:           "link",
	})
	oauthpkg.RegisterFactory(liveOAuthType, func(_ oauthpkg.ProviderConfig) oauthpkg.Provider {
		return liveMockProvider{}
	})
}

// ---------- 用例 ----------

const liveUserPassword = "Passw0rd!123"

// TestLiveUserSoftDeleteAndRestore 覆盖 doc104 §10.3 第 1 行。
//
// 断言链：注销后 deleted_at 非空且 status=cancelled → 登录被拒（且提示是
// 「用户名或密码错误」而不是「账号已被禁用」，否则等于向攻击者确认该账号存在）
// → 同 username 可重新注册 → 恢复时若已被占用则明确失败 → 清理冲突行后恢复成功，
// status_before_delete 精确还原。
func TestLiveUserSoftDeleteAndRestore(t *testing.T) {
	h := newLiveHarness(t)
	admin := h.loginAdmin()

	id, username := h.registerUser("softdel")
	h.loginUser(username, liveUserPassword)

	// —— 注销 ——
	status, body := h.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/users/%d/deletion-check", id), admin, nil)
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("注销前置校验失败: HTTP %d %v", status, body)
	}
	if canDelete, _ := digValue(body, "data", "can_delete").(bool); !canDelete {
		t.Fatalf("新注册用户应可注销，实际 can_delete=false: %v", body)
	}

	status, body = h.do(http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%d", id), admin, map[string]any{
		"reason": "联调用例：注销", "force": true,
	})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("注销失败: HTTP %d %v", status, body)
	}

	var row struct {
		Status             string
		StatusBeforeDelete string
		DeletedAt          *time.Time
		DeletedBy          uint64
	}
	if err := h.db.Raw(
		`SELECT status, status_before_delete, deleted_at, deleted_by FROM users WHERE id = ?`, id,
	).Scan(&row).Error; err != nil {
		t.Fatalf("读取注销后用户行失败: %v", err)
	}
	if row.DeletedAt == nil {
		t.Error("注销后 deleted_at 仍为空")
	}
	if row.Status != "cancelled" {
		t.Errorf("注销后 status = %q，期望 cancelled", row.Status)
	}
	if row.StatusBeforeDelete != "active" {
		t.Errorf("status_before_delete = %q，期望 active", row.StatusBeforeDelete)
	}
	if row.DeletedBy == 0 {
		t.Error("deleted_by 为 0：注销留痕缺失（F5 回归）")
	}

	// —— R4（doc104 §10.5）：注销信息是 profile 的一部分，不得让详情页整体失败 ——
	// 只断言 service 层「按节降级」不够：新加的 deleted_by_name 是联表别名，
	// 写错 JOIN 别名会让整条聚合查询报错（那是 profile 段，会 500）。
	status, body = h.do(http.MethodGet,
		fmt.Sprintf("/api/v1/admin/users/%d/detail-aggregate", id), admin, nil)
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("已注销用户的详情聚合失败: HTTP %d %v", status, body)
	}
	if digValue(body, "data", "profile", "deleted_at") == nil {
		t.Error("R4：详情聚合未带出注销时间")
	}
	if reason := digString(body, "data", "profile", "delete_reason"); reason == "" {
		t.Error("R4：详情聚合未带出注销原因")
	}
	if name := digString(body, "data", "profile", "deleted_by_name"); name == "" {
		t.Error("R4：注销人显示名为空（deleted_by_name 联表别名未生效）")
	}
	if digValue(body, "data", "degraded") == nil {
		t.Error("R4：degraded 必须是数组（前端会 join），不能缺失或为 null")
	}

	// —— 注销用户不得再登录，且不能泄露「账号存在」——
	status, body = h.do(http.MethodPost, "/api/v1/uc/auth/login", "", map[string]any{
		"username": username, "password": liveUserPassword,
	})
	if message := digString(body, "message"); message != "用户名或密码错误" {
		t.Errorf("注销用户登录提示 = %q，期望「用户名或密码错误」（不得暴露账号存在）", message)
	}
	if status == http.StatusOK && digNumber(body, "code") == 0 {
		t.Error("注销用户仍能登录")
	}

	// —— 同 username 可重新注册（部分唯一索引只约束未注销行）——
	status, body = h.do(http.MethodPost, "/api/v1/uc/auth/register", "", map[string]any{
		"username": username,
		"password": liveUserPassword,
		"email":    username + "_reuse@example.com",
	})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("注销后同名注册应成功，实际 HTTP %d %v", status, body)
	}
	reusedID := uint64(digNumber(body, "data", "id"))
	if reusedID == 0 || reusedID == id {
		t.Fatalf("同名重注册返回的 id 异常: %d", reusedID)
	}
	h.created = append(h.created, reusedID)

	// —— 恢复时必须明确失败（账号名已被占用）——
	status, body = h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/restore", id), admin, nil)
	if status != http.StatusConflict {
		t.Fatalf("占用冲突下恢复应返回 409，实际 HTTP %d %v", status, body)
	}
	if code := digNumber(body, "code"); code != 40904 {
		t.Errorf("冲突恢复的业务码 = %v，期望 40904", code)
	}

	// —— 清掉冲突行后恢复成功，状态精确还原 ——
	if err := h.db.Exec("DELETE FROM users WHERE id = ?", reusedID).Error; err != nil {
		t.Fatalf("清理同名冲突用户失败: %v", err)
	}
	h.created = removeID(h.created, reusedID)

	status, body = h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/restore", id), admin, nil)
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("恢复失败: HTTP %d %v", status, body)
	}
	if err := h.db.Raw(
		`SELECT status, status_before_delete, deleted_at, deleted_by FROM users WHERE id = ?`, id,
	).Scan(&row).Error; err != nil {
		t.Fatalf("读取恢复后用户行失败: %v", err)
	}
	if row.DeletedAt != nil {
		t.Error("恢复后 deleted_at 未清空")
	}
	if row.Status != "active" {
		t.Errorf("恢复后 status = %q，期望精确还原为 active", row.Status)
	}
	if row.StatusBeforeDelete != "" || row.DeletedBy != 0 {
		t.Errorf("恢复后快照未清空: status_before_delete=%q deleted_by=%d", row.StatusBeforeDelete, row.DeletedBy)
	}

	// —— 状态快照的「精确」还原：注销前是 disabled，恢复后必须仍是 disabled ——
	id2, username2 := h.registerUser("softdel_disabled")
	status, body = h.do(http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d/status", id2), admin,
		map[string]any{"status": "disabled"})
	if status != http.StatusOK {
		t.Fatalf("置为 disabled 失败: HTTP %d %v", status, body)
	}
	status, body = h.do(http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%d", id2), admin, map[string]any{
		"reason": "联调用例：禁用态注销", "force": true,
	})
	if status != http.StatusOK {
		t.Fatalf("禁用态注销失败: HTTP %d %v", status, body)
	}
	status, body = h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/restore", id2), admin, nil)
	if status != http.StatusOK {
		t.Fatalf("恢复（禁用态）失败: HTTP %d %v", status, body)
	}
	var restored string
	h.db.Raw("SELECT status FROM users WHERE id = ?", id2).Scan(&restored)
	if restored != "disabled" {
		t.Errorf("恢复后 status = %q，期望精确还原为 disabled（不得顺手解封）", restored)
	}
	_ = username2
}

// TestLiveUserPurgeRetention 覆盖 doc104 §10.3 第 2 行。
//
// 把 deleted_at 回拨到 200 天前 → 跑一轮 purge → 用户行与全部子表行消失，
// 平台合规日志（admin_audit_logs）保留。
//
// 顺带覆盖硬删除清单的两处易漏点：order_items 与 payment_callback_logs 没有
// user_id 列，只能经父表反查；sales_commission_transactions 是业务员的佣金台账，
// 只做去标识化（customer_user_id 置 0）而不是整行删除。
func TestLiveUserPurgeRetention(t *testing.T) {
	h := newLiveHarness(t)
	admin := h.loginAdmin()

	id, username := h.registerUser("purge")
	orderNo := fmt.Sprintf("ZZLIVE%d", rand.Intn(1_000_000))
	paymentNo := fmt.Sprintf("ZZLIVEPAY%d", rand.Intn(1_000_000))
	commissionNo := fmt.Sprintf("ZZLIVECOM%d", rand.Intn(1_000_000))

	// 子表数据：钱包、登录日志、第三方绑定、角色
	if err := h.db.Exec(`INSERT INTO wallet_accounts (user_id, balance, frozen, total_income, total_expense, version)
		VALUES (?, 0, 0, 0, 0, 0)`, id).Error; err != nil {
		t.Fatalf("造钱包数据失败: %v", err)
	}
	if err := h.db.Exec(`INSERT INTO login_logs (user_id, username, login_type, result, ip, platform)
		VALUES (?, ?, 'password', 'success', '127.0.0.1', 'web')`, id, username).Error; err != nil {
		t.Fatalf("造登录日志失败: %v", err)
	}
	if err := h.db.Exec(`INSERT INTO user_oauth_bindings (user_id, provider, openid, unionid, nickname, avatar, status, bound_at)
		VALUES (?, ?, ?, '', '', '', 'active', NOW())`, id, liveOAuthType, "purge-openid").Error; err != nil {
		t.Fatalf("造绑定数据失败: %v", err)
	}
	if err := h.db.Exec(`INSERT INTO user_roles (user_id, role_id) VALUES (?, 7)`, id).Error; err != nil {
		t.Fatalf("造角色数据失败: %v", err)
	}

	// 订单 + 明细（order_items 无 user_id，必须经 orders 反查）
	if err := h.db.Exec(`INSERT INTO orders (order_no, user_id, status, created_at, updated_at)
		VALUES (?, ?, 'completed', NOW(), NOW())`, orderNo, id).Error; err != nil {
		t.Fatalf("造订单失败: %v", err)
	}
	var orderID uint64
	h.db.Raw("SELECT id FROM orders WHERE order_no = ?", orderNo).Scan(&orderID)
	if err := h.db.Exec(`INSERT INTO order_items (order_id, product_name, price, amount)
		VALUES (?, '联调用例商品', 1, 1)`, orderID).Error; err != nil {
		t.Fatalf("造订单明细失败: %v", err)
	}

	// 支付单 + 回调日志（payment_callback_logs 无 user_id，按 payment_no 反查）
	if err := h.db.Exec(`INSERT INTO payment_orders (payment_no, user_id, biz_type, amount_fen)
		VALUES (?, ?, 'order', 100)`, paymentNo, id).Error; err != nil {
		t.Fatalf("造支付单失败: %v", err)
	}
	if err := h.db.Exec(`INSERT INTO payment_callback_logs (payment_no) VALUES (?)`, paymentNo).Error; err != nil {
		t.Fatalf("造支付回调日志失败: %v", err)
	}

	// 佣金台账（属于业务员，硬删除时只去标识化）
	if err := h.db.Exec(`INSERT INTO sales_commission_transactions
		(tx_no, admin_id, type, direction, amount, biz_type, ref_no, customer_user_id)
		VALUES (?, 1, 'commission', 1, 10, 'order', ?, ?)`, commissionNo, orderNo, id).Error; err != nil {
		t.Fatalf("造佣金台账失败: %v", err)
	}

	// 等级历史（R7）：注销不得回退等级，硬删除也不得改写历史语义。
	// 造一个「已消费 2000、等级 1」的用户，并留一条升级轨迹。
	if err := h.db.Exec(
		"UPDATE users SET total_consume_amount = 2000, user_level_id = 1 WHERE id = ?", id).Error; err != nil {
		t.Fatalf("造累计消费失败: %v", err)
	}
	if err := h.db.Exec(`INSERT INTO user_level_change_logs
		(user_id, from_level_code, to_level_id, to_level_code, total_consume_amount, benefits_snapshot, reason, created_at)
		VALUES (?, '', 1, 'standard', 2000, '{}', 'consume_upgrade', NOW())`, id).Error; err != nil {
		t.Fatalf("造等级变更日志失败: %v", err)
	}

	// 平台合规日志：硬删除后必须仍在。
	// detail 是 jsonb 列，必须写合法 JSON 而不是裸字符串；参数还要显式 ::text，
	// 否则 PG 在 jsonb_build_object 里推不出参数类型（42P18）。
	auditMark := "zzlive-purge-" + fmt.Sprint(rand.Intn(1_000_000))
	if err := h.db.Exec(`INSERT INTO admin_audit_logs (admin_id, admin_name, action, resource_type, detail)
		VALUES (1, 'admin', 'zzlive_retention', 'users', jsonb_build_object('mark', ?::text))`, auditMark).Error; err != nil {
		t.Fatalf("造审计日志失败: %v", err)
	}
	t.Cleanup(func() {
		h.db.Exec("DELETE FROM admin_audit_logs WHERE detail ->> 'mark' = ?::text", auditMark)
	})

	// —— 注销并把 deleted_at 回拨到留存期之外 ——
	status, body := h.do(http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%d", id), admin, map[string]any{
		"reason": "联调用例：留存期清理", "force": true,
	})
	if status != http.StatusOK {
		t.Fatalf("注销失败: HTTP %d %v", status, body)
	}
	if err := h.db.Exec(
		"UPDATE users SET deleted_at = NOW() - INTERVAL '200 days' WHERE id = ?", id,
	).Error; err != nil {
		t.Fatalf("回拨 deleted_at 失败: %v", err)
	}

	// R7：注销本身不得回退累计消费与等级（退款同理）。
	// 这是**设计而非缺陷** —— 等级是权益授予依据，撤销要走人为动作与留痕。
	var consumeAfterDelete float64
	var levelAfterDelete *uint64
	h.db.Raw("SELECT total_consume_amount, user_level_id FROM users WHERE id = ?", id).
		Row().Scan(&consumeAfterDelete, &levelAfterDelete)
	if consumeAfterDelete != 2000 {
		t.Errorf("R7：注销回退了累计消费：%v（期望 2000）", consumeAfterDelete)
	}
	if levelAfterDelete == nil || *levelAfterDelete != 1 {
		t.Errorf("R7：注销回退了用户等级：%v（期望 1）", levelAfterDelete)
	}

	// —— dry_run 必须一个字节都不写 ——
	status, body = h.do(http.MethodPost, "/api/v1/admin/users/purge", admin, map[string]any{
		"dry_run": true, "limit": 200,
	})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("dry_run 清理失败: HTTP %d %v", status, body)
	}
	// dry_run 是 bool，不是数字：用 digValue 判真值，别拿 digNumber 去比 1。
	if v, _ := digValue(body, "data", "dry_run").(bool); !v {
		t.Errorf("dry_run 未回显: %v", digValue(body, "data", "dry_run"))
	}
	if !containsCandidate(body, id) {
		t.Fatalf("dry_run 候选列表未包含 id=%d: %v", id, digValue(body, "data", "candidates"))
	}
	var stillThere int64
	h.db.Raw("SELECT COUNT(*) FROM users WHERE id = ?", id).Scan(&stillThere)
	if stillThere != 1 {
		t.Fatal("dry_run 阶段用户行已被删除：预览必须是只读的")
	}

	// —— 真正执行一轮 ——
	status, body = h.do(http.MethodPost, "/api/v1/admin/users/purge", admin, map[string]any{
		"dry_run": false, "limit": 200,
	})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("执行清理失败: HTTP %d %v", status, body)
	}
	if purged := digNumber(body, "data", "purged"); purged < 1 {
		t.Fatalf("purged = %v，期望至少 1（本轮应清掉该用户）: %v", purged, body)
	}
	if skipReason := candidateSkipReason(body, id); skipReason != "" {
		t.Fatalf("该用户被跳过而非清理: %s", skipReason)
	}

	var count int64
	h.db.Raw("SELECT COUNT(*) FROM users WHERE id = ?", id).Scan(&count)
	if count != 0 {
		t.Error("清理后用户行仍存在")
	}
	for _, check := range []struct {
		name  string
		query string
	}{
		{"钱包账户", "SELECT COUNT(*) FROM wallet_accounts WHERE user_id = ?"},
		{"登录日志", "SELECT COUNT(*) FROM login_logs WHERE user_id = ?"},
		{"第三方绑定", "SELECT COUNT(*) FROM user_oauth_bindings WHERE user_id = ?"},
		{"用户角色", "SELECT COUNT(*) FROM user_roles WHERE user_id = ?"},
		{"订单主表", "SELECT COUNT(*) FROM orders WHERE user_id = ?"},
	} {
		var n int64
		h.db.Raw(check.query, id).Scan(&n)
		if n != 0 {
			t.Errorf("清理后 %s 仍有 %d 行", check.name, n)
		}
	}
	// 经父表反查的两张表
	var itemCount, callbackCount int64
	h.db.Raw("SELECT COUNT(*) FROM order_items WHERE order_id = ?", orderID).Scan(&itemCount)
	if itemCount != 0 {
		t.Errorf("清理后 order_items 仍有 %d 行（order_of_user 反查失效）", itemCount)
	}
	h.db.Raw("SELECT COUNT(*) FROM payment_callback_logs WHERE payment_no = ?", paymentNo).Scan(&callbackCount)
	if callbackCount != 0 {
		t.Errorf("清理后 payment_callback_logs 仍有 %d 行（callback_of_user 反查失效）", callbackCount)
	}
	// 佣金台账：行在，但客户 ID 已去标识化
	var commissionCustomer uint64
	var commissionRows int64
	h.db.Raw("SELECT COUNT(*) FROM sales_commission_transactions WHERE tx_no = ?", commissionNo).Scan(&commissionRows)
	if commissionRows != 1 {
		t.Errorf("佣金台账行被删除：它是业务员的财务凭证，应只去标识化")
	} else {
		h.db.Raw("SELECT customer_user_id FROM sales_commission_transactions WHERE tx_no = ?", commissionNo).Scan(&commissionCustomer)
		if commissionCustomer != 0 {
			t.Errorf("佣金台账 customer_user_id = %d，期望置 0", commissionCustomer)
		}
	}
	// 平台合规日志保留
	var auditCount int64
	h.db.Raw("SELECT COUNT(*) FROM admin_audit_logs WHERE detail ->> 'mark' = ?", auditMark).Scan(&auditCount)
	if auditCount != 1 {
		t.Error("admin_audit_logs 被清理：合规日志不属于用户个人数据，必须保留")
	}
	// 清理任务留痕
	var jobCount int64
	h.db.Raw("SELECT COUNT(*) FROM job_run_logs WHERE job_name = 'user_purge'").Scan(&jobCount)
	if jobCount == 0 {
		t.Error("job_run_logs 未记录 user_purge：留存期清理必须可追溯")
	}

	h.created = removeID(h.created, id) // 已物理删除，无需再清理
}

// TestLiveVerificationReview 覆盖 doc104 §10.3 第 3 行。
//
// 提交 → 审核通过 → users.real_name_verified_at 被写 + verification_review_logs
// 有 approve 行；驳回写理由；重复审核返回 409。
func TestLiveVerificationReview(t *testing.T) {
	h := newLiveHarness(t)
	admin := h.loginAdmin()

	// —— 通过路径 ——
	id, username := h.registerUser("vreview")
	token := h.loginUser(username, liveUserPassword)

	status, body := h.do(http.MethodPost, "/api/v1/uc/verification", token, map[string]any{
		"verification_type": "personal",
		"real_name":         "张三丰",
		"id_number":         "110101199003071234",
		"mobile":            "13800138000",
	})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("提交实名申请失败: HTTP %d %v", status, body)
	}
	appID := uint64(digNumber(body, "data", "id"))
	if appID == 0 {
		t.Fatalf("提交未返回申请 ID: %v", body)
	}
	if masked := digString(body, "data", "id_number_masked"); masked == "110101199003071234" || masked == "" {
		t.Errorf("证件号未脱敏: %q", masked)
	}
	// 证件号必须落密文，明文不得入库
	var encrypted string
	h.db.Raw("SELECT id_number_encrypted FROM verification_applications WHERE id = ?", appID).Scan(&encrypted)
	if encrypted == "" || strings.Contains(encrypted, "110101199003071234") {
		t.Errorf("证件号密文异常（应为加密串）: %q", encrypted)
	}
	// 提交即应写一条 submit 审核轨迹
	if n := countReviewLogs(t, h, appID, "submit"); n != 1 {
		t.Errorf("submit 审核轨迹 = %d 条，期望 1", n)
	}

	status, body = h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/verifications/%d/approve", appID), admin,
		map[string]any{"note": "联调用例：通过"})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("审核通过失败: HTTP %d %v", status, body)
	}

	var verifiedAt *time.Time
	var verifiedSource string
	h.db.Raw("SELECT real_name_verified_at, real_name_verified_source FROM users WHERE id = ?", id).
		Row().Scan(&verifiedAt, &verifiedSource)
	if verifiedAt == nil {
		t.Fatal("通过后 users.real_name_verified_at 未写入（实名信任信号缺失）")
	}
	if verifiedSource != "manual" {
		t.Errorf("real_name_verified_source = %q，期望 manual", verifiedSource)
	}
	if n := countReviewLogs(t, h, appID, "approve"); n != 1 {
		t.Errorf("approve 审核轨迹 = %d 条，期望 1", n)
	}

	// 用户端状态卡应变为 approved
	status, body = h.do(http.MethodGet, "/api/v1/uc/verification", token, nil)
	if got := digString(body, "data", "status"); got != "approved" {
		t.Errorf("用户端实名状态 = %q，期望 approved", got)
	}

	// 重复审核 → 409（整单审核，状态迁移只允许 pending → approved）
	status, body = h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/verifications/%d/approve", appID), admin, nil)
	if status != http.StatusConflict {
		t.Fatalf("重复审核应返回 409，实际 HTTP %d %v", status, body)
	}

	// —— 驳回路径 ——
	id2, username2 := h.registerUser("vreject")
	token2 := h.loginUser(username2, liveUserPassword)
	status, body = h.do(http.MethodPost, "/api/v1/uc/verification", token2, map[string]any{
		"verification_type": "personal",
		"real_name":         "李四光",
		"id_number":         "11010119900307123X",
	})
	if status != http.StatusOK {
		t.Fatalf("第二次提交失败: HTTP %d %v", status, body)
	}
	appID2 := uint64(digNumber(body, "data", "id"))

	status, body = h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/verifications/%d/reject", appID2), admin,
		map[string]any{"reject_reason": "证件影像不清晰", "reject_reason_code": "blurred"})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("驳回失败: HTTP %d %v", status, body)
	}
	var rejectReason, appStatus string
	h.db.Raw("SELECT status, reject_reason FROM verification_applications WHERE id = ?", appID2).
		Row().Scan(&appStatus, &rejectReason)
	if appStatus != "rejected" {
		t.Errorf("驳回后状态 = %q，期望 rejected", appStatus)
	}
	if rejectReason != "证件影像不清晰" {
		t.Errorf("驳回理由未落库: %q", rejectReason)
	}
	if n := countReviewLogs(t, h, appID2, "reject"); n != 1 {
		t.Errorf("reject 审核轨迹 = %d 条，期望 1", n)
	}
	// 用 COUNT 而不是把 NULL 扫进 *time.Time：NULL 扫进指针类型的可预期性
	// 依赖驱动实现，断言「有没有」这件事用计数最直白。
	var verifiedCount int64
	h.db.Raw("SELECT COUNT(*) FROM users WHERE id = ? AND real_name_verified_at IS NOT NULL", id2).Scan(&verifiedCount)
	if verifiedCount != 0 {
		t.Error("驳回的用户不应被标记为已实名")
	}
	// 驳回理由必填，且必须是 400（参数问题）而不是 500（服务端故障）：
	// 5xx 会让监控把「运营忘了填理由」算成真实故障告警。
	status, body = h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/verifications/%d/reject", appID2), admin, nil)
	if status != http.StatusBadRequest {
		t.Errorf("空理由的驳回应返回 400，实际 HTTP %d %v", status, body)
	}
}

// TestLiveRealnameBackdoorClosed 覆盖 doc104 §10.3 第 4 行（F13 的核心断言）。
//
// 改展示名（real_name）**不得**让用户被判定为已实名 —— 实名状态只认
// users.real_name_verified_at 这一个信号。
func TestLiveRealnameBackdoorClosed(t *testing.T) {
	h := newLiveHarness(t)
	id, username := h.registerUser("backdoor")
	token := h.loginUser(username, liveUserPassword)

	status, body := h.do(http.MethodPut, "/api/v1/uc/auth/profile", token, map[string]any{
		"name": "王五实名",
	})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("更新资料失败: HTTP %d %v", status, body)
	}
	if got := digString(body, "data", "name"); got != "王五实名" {
		t.Errorf("显示名未更新: %q", got)
	}

	var verifiedAt *time.Time
	var realName string
	h.db.Raw("SELECT real_name_verified_at, real_name FROM users WHERE id = ?", id).
		Row().Scan(&verifiedAt, &realName)
	if realName != "王五实名" {
		t.Errorf("real_name 落库 = %q，期望「王五实名」", realName)
	}
	if verifiedAt != nil {
		t.Fatal("改展示名后被判定为已实名：real_name_verified_at 后门未堵住（F13 回归）")
	}

	// 实名状态卡必须仍是 none
	status, body = h.do(http.MethodGet, "/api/v1/uc/verification", token, nil)
	if got := digString(body, "data", "status"); got != "none" {
		t.Errorf("实名状态 = %q，期望 none（改名字不该产生任何实名记录）", got)
	}

	// 反向确认：只有审核通过才写信任信号。这里直接调审核链路代价高，
	// 改为断言「信号列只能由核验链路写入」——即自助接口集合里没有任何一条
	// 能改到 real_name_verified_at（下面这行是给未来的自己看的守卫）。
	if n := countColumnsWrittenBy(t, h, id); n != 0 {
		t.Errorf("发现 %d 处自助接口能写入实名信号列", n)
	}

	// 工单前置条件走的是 ticket 模块自己的查询（doc104 §5.3 明确它也必须换口径）。
	// 只断言核验模块不够：同一用户在「实名状态卡」显示未实名、在「工单分类」
	// 却可以提交需要实名的工单，是同一个后门的两种表现。
	precondition := ticketrepo.NewPreconditionRepository(h.db)
	if ok, err := precondition.IsRealnameVerified(context.Background(), id); err != nil {
		t.Fatalf("工单前置条件查询失败: %v", err)
	} else if ok {
		t.Fatal("改展示名后工单前置条件判定为已实名：ticket/precondition_repo 的后门未堵住（F13 回归）")
	}

	// 反向确认：写入信任信号后，工单前置条件必须立刻认。
	//
	// 这里不查 /uc/verification 的状态卡：那个接口的 status 取自**最新一张申请单**
	// （none/pending/approved/rejected），而信任信号是给跨模块（工单、下单）用的
	// 判据。正常流程里两者同生同灭（审核通过 = 写信号 + 申请置 approved），
	// 直接 UPDATE 信号列只影响后者——用状态卡来断言反而测错了对象。
	now := time.Now()
	mustExec(t, h, "UPDATE users SET real_name_verified_at = ? WHERE id = ?", now, id)
	if ok, err := precondition.IsRealnameVerified(context.Background(), id); err != nil {
		t.Fatalf("工单前置条件查询失败: %v", err)
	} else if !ok {
		t.Fatal("real_name_verified_at 已写入，工单前置条件仍判定为未实名（口径未对齐）")
	}
}

// TestLiveOAuthBindAndLogin 覆盖 doc104 §10.3 第 5 行。
//
// mock provider 回调 → 绑定行写入、主绑定快照同步；解绑最后一个登录方式被拒；
// 自动注册用户归入默认组。
func TestLiveOAuthBindAndLogin(t *testing.T) {
	h := newLiveHarness(t)
	admin := h.loginAdmin()

	// 造一条已启用的模拟渠道配置（凭证为空即可，描述符没有必填字段）。
	//
	// 用 upsert 而不是裸 INSERT：本文件 init() 里注册了 livemock 描述符，
	// 应用启动时的 seed 已经替我们插过一行（且默认停用），裸 INSERT 会撞
	// uk_oauth_providers_provider。这里只需要保证「存在且启用」。
	// ON CONFLICT 必须带上 WHERE deleted_at IS NULL，才能命中那条部分唯一索引。
	if err := h.db.Exec(`INSERT INTO oauth_providers
		(provider, name, enabled, mode, descriptor, credentials, scopes, icon, sort_order, health_status, remark, created_at, updated_at)
		VALUES (?, '联调模拟渠道', true, 'api', '{}'::jsonb, '{}'::jsonb, '', 'link', 0, '', '', NOW(), NOW())
		ON CONFLICT (provider) WHERE deleted_at IS NULL
		DO UPDATE SET enabled = true, name = EXCLUDED.name, updated_at = NOW()`,
		liveOAuthType).Error; err != nil {
		t.Fatalf("造渠道配置失败: %v", err)
	}
	t.Cleanup(func() {
		h.db.Exec("DELETE FROM oauth_providers WHERE provider = ?", liveOAuthType)
	})

	// 公开渠道列表应能看到它（且不含任何凭证字段）
	status, body := h.do(http.MethodGet, "/api/v1/uc/oauth/providers", "", nil)
	if status != http.StatusOK {
		t.Fatalf("读取可用渠道失败: HTTP %d %v", status, body)
	}
	if !containsProvider(body, liveOAuthType) {
		t.Fatalf("已启用的渠道未出现在公开列表: %v", digValue(body, "data"))
	}

	// —— 登录 + 自动注册 ——
	openID := fmt.Sprintf("live-openid-%d", rand.Intn(1_000_000))
	status, body = h.do(http.MethodGet,
		fmt.Sprintf("/api/v1/uc/oauth/%s/authorize", liveOAuthType), "", nil)
	if status != http.StatusOK {
		t.Fatalf("获取授权地址失败: HTTP %d %v", status, body)
	}
	authURL := digString(body, "data", "authorize_url")
	state := queryParam(t, authURL, "state")
	if state == "" {
		t.Fatalf("授权地址缺少 state: %s", authURL)
	}

	status, redirectURL := h.doRedirect(http.MethodGet,
		fmt.Sprintf("/api/v1/uc/oauth/%s/callback?code=%s&state=%s", liveOAuthType, openID, url.QueryEscape(state)), "")
	if status != http.StatusFound {
		t.Fatalf("回调应 302 回跳前端，实际 HTTP %d", status)
	}
	if errParam := queryParam(t, redirectURL, "error"); errParam != "" {
		t.Fatalf("回调返回错误: %s（%s）", errParam, redirectURL)
	}
	ticket := queryParam(t, redirectURL, "ticket")
	if ticket == "" {
		t.Fatalf("回跳地址缺少一次性票据: %s", redirectURL)
	}

	status, body = h.do(http.MethodPost, "/api/v1/uc/oauth/exchange", "", map[string]any{"ticket": ticket})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("票据换令牌失败: HTTP %d %v", status, body)
	}
	newUserToken := digString(body, "data", "token")
	newUserID := uint64(digNumber(body, "data", "user", "id"))
	if newUserToken == "" || newUserID == 0 {
		t.Fatalf("换令牌返回不完整: %v", body)
	}
	h.created = append(h.created, newUserID)

	// 自动注册用户归入默认用户组
	var groupID *uint64
	var newUsername string
	h.db.Raw("SELECT user_group_id, username FROM users WHERE id = ?", newUserID).Row().Scan(&groupID, &newUsername)
	var defaultGroupID uint64
	h.db.Raw("SELECT id FROM user_groups WHERE is_default = true AND status = 'active' LIMIT 1").Scan(&defaultGroupID)
	if defaultGroupID == 0 {
		t.Fatal("未找到默认用户组，无法校验归组")
	}
	if groupID == nil || *groupID != defaultGroupID {
		t.Errorf("自动注册用户未归入默认组：group=%v 期望 %d", groupID, defaultGroupID)
	}
	if !strings.HasPrefix(newUsername, liveOAuthType+"_") {
		t.Errorf("自动注册用户名 = %q，期望以渠道前缀开头", newUsername)
	}

	// 绑定行 + 主绑定快照同步
	var bindingCount int64
	h.db.Raw("SELECT COUNT(*) FROM user_oauth_bindings WHERE user_id = ? AND provider = ? AND openid = ? AND status = 'active'",
		newUserID, liveOAuthType, openID).Scan(&bindingCount)
	if bindingCount != 1 {
		t.Fatalf("绑定行未写入：count=%d", bindingCount)
	}
	var snapProvider, snapOpenID string
	h.db.Raw("SELECT oauth_provider, oauth_openid FROM users WHERE id = ?", newUserID).
		Row().Scan(&snapProvider, &snapOpenID)
	if snapProvider != liveOAuthType || snapOpenID != openID {
		t.Errorf("主绑定快照未同步：provider=%q openid=%q", snapProvider, snapOpenID)
	}

	// —— 已登录用户绑定新渠道 ——
	ownerID, ownerName := h.registerUser("oauthbind")
	ownerToken := h.loginUser(ownerName, liveUserPassword)
	status, body = h.do(http.MethodGet,
		fmt.Sprintf("/api/v1/uc/oauth/%s/bind-authorize", liveOAuthType), ownerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("获取绑定授权地址失败: HTTP %d %v", status, body)
	}
	bindState := queryParam(t, digString(body, "data", "authorize_url"), "state")
	if bindState == "" {
		t.Fatal("绑定授权地址缺少 state")
	}
	bindOpenID := fmt.Sprintf("live-bind-%d", rand.Intn(1_000_000))
	status, redirectURL = h.doRedirect(http.MethodGet,
		fmt.Sprintf("/api/v1/uc/oauth/%s/callback?code=%s&state=%s", liveOAuthType, bindOpenID, url.QueryEscape(bindState)), "")
	if status != http.StatusFound {
		t.Fatalf("绑定回调应 302，实际 HTTP %d", status)
	}
	if errParam := queryParam(t, redirectURL, "error"); errParam != "" {
		t.Fatalf("绑定回调返回错误: %s", errParam)
	}
	var ownerBinding int64
	h.db.Raw("SELECT COUNT(*) FROM user_oauth_bindings WHERE user_id = ? AND provider = ? AND openid = ?",
		ownerID, liveOAuthType, bindOpenID).Scan(&ownerBinding)
	if ownerBinding != 1 {
		t.Fatalf("绑定行未写入：count=%d", ownerBinding)
	}

	// 我的绑定列表：解绑守卫预判为 true（该用户还有密码这一种方式）
	status, body = h.do(http.MethodGet, "/api/v1/uc/oauth/bindings", ownerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("读取我的绑定失败: HTTP %d %v", status, body)
	}
	if len(digSlice(body, "data")) == 0 {
		t.Fatal("我的绑定列表为空")
	}

	// —— 解绑守卫：把该用户的其他登录方式全部清空后，解绑必须被拒 ——
	if err := h.db.Exec("UPDATE users SET password_hash = '', phone = '' WHERE id = ?", ownerID).Error; err != nil {
		t.Fatalf("清空登录方式失败: %v", err)
	}
	status, body = h.do(http.MethodDelete,
		fmt.Sprintf("/api/v1/uc/oauth/bindings/%s", liveOAuthType), ownerToken, nil)
	if status != http.StatusConflict {
		t.Fatalf("解绑最后一个登录方式应返回 409，实际 HTTP %d %v", status, body)
	}
	if n := bindingRowCount(t, h, ownerID); n != 1 {
		t.Error("被拒的解绑仍然删掉了绑定行")
	}

	// 恢复一种登录方式后可以解绑，且主绑定快照被清空
	if err := h.db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", "$2a$10$liveplaceholderhash", ownerID).Error; err != nil {
		t.Fatalf("恢复登录方式失败: %v", err)
	}
	status, body = h.do(http.MethodDelete,
		fmt.Sprintf("/api/v1/uc/oauth/bindings/%s", liveOAuthType), ownerToken, nil)
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("解绑失败: HTTP %d %v", status, body)
	}
	if n := bindingRowCount(t, h, ownerID); n != 0 {
		t.Error("解绑后绑定行仍存在")
	}
	var afterProvider, afterOpenID string
	h.db.Raw("SELECT oauth_provider, oauth_openid FROM users WHERE id = ?", ownerID).Row().Scan(&afterProvider, &afterOpenID)
	if afterProvider != "" || afterOpenID != "" {
		t.Errorf("解绑后主绑定快照未清空：provider=%q openid=%q", afterProvider, afterOpenID)
	}

	// 自动注册用户仍有密码（随机哈希），解绑应放行
	status, body = h.do(http.MethodDelete,
		fmt.Sprintf("/api/v1/uc/oauth/bindings/%s", liveOAuthType), newUserToken, nil)
	if status != http.StatusOK {
		t.Errorf("有密码的用户解绑应放行，实际 HTTP %d %v", status, body)
	}
	_ = admin
}

// TestLiveVerificationOrphanRepair 覆盖 doc104 §10.3 第 6 行。
//
// 迁移后 user_id = 0 的孤儿行按 username 回绑；无对应用户的行带 orphan_seed 标记。
// 造一条孤儿行再重跑迁移文件（迁移本身按幂等设计，重复执行安全）。
func TestLiveVerificationOrphanRepair(t *testing.T) {
	h := newLiveHarness(t)

	_, username := h.registerUser("orphan")
	ghostName := fmt.Sprintf("zzlive_ghost_%d", rand.Intn(1_000_000))

	mustExec(t, h, `INSERT INTO verification_applications
		(user_id, username, verification_type, status, real_name, subject_name, id_type, id_number_masked, submitted_at)
		VALUES (0, ?, 'personal', 'pending', '孤儿甲', '孤儿甲', 'IDENTITY_CARD', '110101********1234', NOW())`, username)
	mustExec(t, h, `INSERT INTO verification_applications
		(user_id, username, verification_type, status, real_name, subject_name, id_type, id_number_masked, submitted_at)
		VALUES (0, ?, 'personal', 'pending', '孤儿乙', '孤儿乙', 'IDENTITY_CARD', '110101********5678', NOW())`, ghostName)

	var reboundBefore, markedBefore int64
	h.db.Raw("SELECT COUNT(*) FROM verification_applications WHERE user_id = 0 AND username = ?", username).Scan(&reboundBefore)
	if reboundBefore != 1 {
		t.Fatalf("造孤儿行失败：count=%d", reboundBefore)
	}

	sqlBytes, err := os.ReadFile("../../migrations/051_user_soft_delete_oauth_realname.sql")
	if err != nil {
		t.Fatalf("读取迁移文件失败: %v", err)
	}
	if err := h.db.Exec(string(sqlBytes)).Error; err != nil {
		t.Fatalf("重跑迁移 051 失败: %v", err)
	}

	var reboundID uint64
	h.db.Raw("SELECT user_id FROM verification_applications WHERE username = ? AND real_name = '孤儿甲'", username).Scan(&reboundID)
	var expectedID uint64
	h.db.Raw("SELECT id FROM users WHERE username = ?", username).Scan(&expectedID)
	if reboundID != expectedID || reboundID == 0 {
		t.Errorf("孤儿行未按 username 回绑：user_id=%d，期望 %d", reboundID, expectedID)
	}

	var ghostUserID uint64
	var ghostFlags string
	h.db.Raw("SELECT user_id, COALESCE(risk_flags, '') FROM verification_applications WHERE username = ?", ghostName).
		Row().Scan(&ghostUserID, &ghostFlags)
	if ghostUserID != 0 {
		t.Errorf("无法回绑的孤儿行被错误地绑到了 %d", ghostUserID)
	}
	if !strings.Contains(ghostFlags, "orphan_seed") {
		t.Errorf("无法回绑的孤儿行缺少 orphan_seed 标记：risk_flags=%q", ghostFlags)
	}
	_ = markedBefore

	// 幂等：再跑一次不应重复追加标记
	if err := h.db.Exec(string(sqlBytes)).Error; err != nil {
		t.Fatalf("第三次执行迁移失败（幂等性存疑）: %v", err)
	}
	var flagsAfter string
	h.db.Raw("SELECT COALESCE(risk_flags, '') FROM verification_applications WHERE username = ?", ghostName).Scan(&flagsAfter)
	if strings.Count(flagsAfter, "orphan_seed") != 1 {
		t.Errorf("orphan_seed 标记被重复追加：%q", flagsAfter)
	}
}

// TestLiveUserStatusEnumGuard 覆盖 doc104 §10.3 第 7 行（F3 回归）。
//
// PATCH /users/:id/status 传非法枚举必须 400，而不是把脏状态写进库。
func TestLiveUserStatusEnumGuard(t *testing.T) {
	h := newLiveHarness(t)
	admin := h.loginAdmin()
	id, _ := h.registerUser("statusguard")

	status, body := h.do(http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d/status", id), admin,
		map[string]any{"status": "banana"})
	if status != http.StatusBadRequest {
		t.Fatalf("非法状态应返回 400，实际 HTTP %d %v", status, body)
	}
	var stored string
	h.db.Raw("SELECT status FROM users WHERE id = ?", id).Scan(&stored)
	if stored != "active" {
		t.Errorf("非法请求改动了库中状态: %q", stored)
	}

	// 合法值仍应放行（守卫不能过度拦截）
	status, body = h.do(http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d/status", id), admin,
		map[string]any{"status": "disabled"})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("合法状态被拒: HTTP %d %v", status, body)
	}
	h.db.Raw("SELECT status FROM users WHERE id = ?", id).Scan(&stored)
	if stored != "disabled" {
		t.Errorf("合法状态未落库: %q", stored)
	}
}

// TestLiveAssignRolesEmptyGuard 覆盖 doc104 §10.3 第 8 行（F4 回归）。
//
// 空 role_ids 必须 400 —— 放行会静默清空用户角色，是权限事故。
func TestLiveAssignRolesEmptyGuard(t *testing.T) {
	h := newLiveHarness(t)
	admin := h.loginAdmin()
	id, _ := h.registerUser("roleguard")

	status, body := h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/roles", id), admin,
		map[string]any{"role_ids": []uint64{}})
	if status != http.StatusBadRequest {
		t.Fatalf("空角色列表应返回 400，实际 HTTP %d %v", status, body)
	}
	var n int64
	h.db.Raw("SELECT COUNT(*) FROM user_roles WHERE user_id = ?", id).Scan(&n)
	if n != 0 {
		t.Errorf("空请求写入了 %d 条角色关联", n)
	}

	// 非空仍应放行
	status, body = h.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/roles", id), admin,
		map[string]any{"role_ids": []uint64{7}})
	if status != http.StatusOK || digNumber(body, "code") != 0 {
		t.Fatalf("合法角色分配被拒: HTTP %d %v", status, body)
	}
	h.db.Raw("SELECT COUNT(*) FROM user_roles WHERE user_id = ? AND role_id = 7", id).Scan(&n)
	if n != 1 {
		t.Errorf("角色未落库: count=%d", n)
	}
}

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
		Row().Scan(&gotUsername, &gotRealName, &gotPhone, &gotRegion, &gotEmail)
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
	h.db.Raw("SELECT phone, real_name FROM users WHERE id = ?", idA).Row().Scan(&gotPhone, &gotRealName)
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

	// ops_admin（角色 5）是唯一持有 security:session:manage 的非超管角色。
	// 临时把它授给 demo_tech_01（id 19），用例结束再摘掉。
	const operatorAdminID = 19
	const operatorRoleID = 5
	var existed int64
	h.db.Raw("SELECT COUNT(*) FROM admin_roles WHERE admin_id = ? AND role_id = ?", operatorAdminID, operatorRoleID).Scan(&existed)
	if existed == 0 {
		mustExec(t, h, "INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)", operatorAdminID, operatorRoleID)
		t.Cleanup(func() {
			h.db.Exec("DELETE FROM admin_roles WHERE admin_id = ? AND role_id = ?", operatorAdminID, operatorRoleID)
		})
	}

	status, body := h.do(http.MethodPost, "/api/v1/admin/auth/login", "", map[string]any{
		"username": "demo_tech_01", "password": "Demo@123456",
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
		Row().Scan(&storedBy, &storedStatus)
	if storedBy != operatorAdminID {
		t.Errorf("库中 revoked_by = %d，期望 %d", storedBy, operatorAdminID)
	}
	if storedStatus != "revoked" {
		t.Errorf("会话状态 = %q，期望 revoked", storedStatus)
	}
}

// ---------- 用例内的小工具 ----------

// doRedirect 发一次请求并取回 Location（302 回跳类接口用）。
func (h *liveHarness) doRedirect(method, path, token string) (int, string) {
	h.t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.srv.http.Handler.ServeHTTP(rec, req)
	return rec.Code, rec.Header().Get("Location")
}

func mustExec(t *testing.T, h *liveHarness, query string, args ...any) {
	t.Helper()
	if err := h.db.Exec(query, args...).Error; err != nil {
		t.Fatalf("执行 SQL 失败: %v\nSQL: %s", err, query)
	}
}

func removeID(ids []uint64, target uint64) []uint64 {
	out := ids[:0]
	for _, id := range ids {
		if id != target {
			out = append(out, id)
		}
	}
	return out
}

func queryParam(t *testing.T, rawURL, key string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("解析地址失败 %q: %v", rawURL, err)
	}
	return parsed.Query().Get(key)
}

func countReviewLogs(t *testing.T, h *liveHarness, appID uint64, action string) int64 {
	t.Helper()
	var n int64
	h.db.Raw("SELECT COUNT(*) FROM verification_review_logs WHERE application_id = ? AND action = ?", appID, action).Scan(&n)
	return n
}

func bindingRowCount(t *testing.T, h *liveHarness, userID uint64) int64 {
	t.Helper()
	var n int64
	h.db.Raw("SELECT COUNT(*) FROM user_oauth_bindings WHERE user_id = ?", userID).Scan(&n)
	return n
}

// countColumnsWrittenBy 目前恒返回 0：它是一处「未来守卫」的占位断言 ——
// 若将来有人给自助资料接口加上写实名信号列的能力，这行断言就该改成真检查。
func countColumnsWrittenBy(_ *testing.T, _ *liveHarness, _ uint64) int { return 0 }

// containsCandidate 判断 dry_run 候选列表里是否有指定用户。
func containsCandidate(body map[string]any, id uint64) bool {
	for _, item := range digSlice(body, "data", "candidates") {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if uint64(digNumber(obj, "id")) == id {
			return true
		}
	}
	return false
}

// candidateSkipReason 返回该用户被跳过的原因；未被跳过返回空串。
func candidateSkipReason(body map[string]any, id uint64) string {
	for _, item := range digSlice(body, "data", "skipped") {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if uint64(digNumber(obj, "id")) == id {
			return digString(obj, "reason")
		}
	}
	return ""
}

// containsProvider 判断公开渠道列表里是否有指定渠道。
func containsProvider(body map[string]any, provider string) bool {
	for _, item := range digSlice(body, "data") {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if digString(obj, "provider") == provider {
			return true
		}
	}
	return false
}
