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
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	appauth "hostsent/backend/internal/pkg/auth"
	"hostsent/backend/internal/pkg/config"
	oauthpkg "hostsent/backend/internal/pkg/oauth"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// ---------- 基础设施 ----------

// liveDSN 联调库连接串；LIVE_DB_DSN 可覆盖（与 internal/pkg/db 的 live 用例同约定）。
func liveDSN() string {
	if dsn := os.Getenv("LIVE_DB_DSN"); dsn != "" {
		return dsn
	}
	return "host=127.0.0.1 port=5432 user=hostsent password=hostsent dbname=hostsent sslmode=disable"
}

// applyLiveDSN 把 libpq key=value 形式的 DSN 解析进 DatabaseConfig（仅解析
// liveApp 需要的 host/port/user/password/dbname/sslmode 六键）。
func applyLiveDSN(dst *config.DatabaseConfig, dsn string) {
	for _, part := range strings.Fields(dsn) {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch k {
		case "host":
			dst.Host = v
		case "port":
			if p, err := strconv.Atoi(v); err == nil {
				dst.Port = p
			}
		case "user":
			dst.User = v
		case "password":
			dst.Password = v
		case "dbname":
			dst.Name = v
		case "sslmode":
			dst.SSLMode = v
		}
	}
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
		// 数据库目标与 liveDSN() 同源：LIVE_DB_DSN 显式指定时按其解析
		// （否则 app 写默认库、断言读指定库，两边对不上全盘假失败）。
		if dsn := os.Getenv("LIVE_DB_DSN"); dsn != "" {
			applyLiveDSN(&cfg.Database, dsn)
		} else {
			cfg.Database.Host = "127.0.0.1"
			cfg.Database.Port = 5432
			cfg.Database.User = "hostsent"
			cfg.Database.Password = "hostsent"
			cfg.Database.Name = "hostsent"
			cfg.Database.SSLMode = "disable"
		}
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
	defer func() { _ = resp.Body.Close() }()
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
//
// 必须同时插一条 user_sessions：令牌里的 sid 会被 UserAuth 拿去查会话，
// 会话不存在（或不是 active）就 401 —— 只签令牌不建会话，接口一律不可用。
func (h *liveHarness) userToken(id uint64, username string) string {
	h.t.Helper()
	issuer := appauth.NewJWTIssuer(h.cfg.Auth.JWTSecret, h.cfg.Auth.JWTIssuer,
		time.Duration(h.cfg.Auth.JWTExpireHours)*time.Hour)
	sessionID := fmt.Sprintf("test_sess_%d_%d", id, time.Now().UnixNano())
	now := time.Now()
	expiredAt := now.Add(issuer.ExpireIn())
	if h.db != nil {
		if err := h.db.Exec(`INSERT INTO user_sessions
			(session_id, user_id, username, platform, ip, user_agent, login_at, last_active_at, expired_at, status, created_at, updated_at)
			VALUES (?, ?, ?, 'web', '127.0.0.1', 'live-test', ?, ?, ?, 'active', NOW(), NOW())`,
			sessionID, id, username, now, now, expiredAt).Error; err != nil {
			h.t.Fatalf("写入测试会话失败: %v", err)
		}
	}
	token, err := issuer.GenerateUserSession(username, id, "free", 0, false, sessionID)
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
		"user_roles", "sub_account_permissions", "user_operation_logs",
		"staff_sales_relations", "orders", "payment_orders", "notifications",
		// 人工充值链路产生的三张表（充值单 / 资金流水 / 账单），一并按 user_id 清。
		"wallet_transactions", "wallet_accounts", "bills", "recharges",
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

func digBool(body map[string]any, keys ...string) bool {
	v, _ := digValue(body, keys...).(bool)
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
