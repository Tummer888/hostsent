package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	openmodel "hostsent/backend/internal/modules/open/model"
	openrepo "hostsent/backend/internal/modules/open/repository"
	"hostsent/backend/internal/pkg/crypto"
)

// stubAppRepo 内存版应用仓储，覆盖网关测试所需路径。
type stubAppRepo struct {
	apps map[string]*openrepo.ResolvedApp
	err  error
}

func (s *stubAppRepo) ResolveByAppID(_ context.Context, appID string) (*openrepo.ResolvedApp, error) {
	if s.err != nil {
		return nil, s.err
	}
	if app, ok := s.apps[appID]; ok {
		return app, nil
	}
	return nil, openrepo.ErrAppNotFound
}

func (s *stubAppRepo) CreateApp(_ context.Context, _ *openmodel.OpenApp, _ []string) error {
	return errors.New("not implemented")
}
func (s *stubAppRepo) ListApps(_ context.Context) ([]openmodel.OpenApp, error) {
	return nil, errors.New("not implemented")
}
func (s *stubAppRepo) GetByAppID(_ context.Context, _ string) (*openmodel.OpenApp, error) {
	return nil, errors.New("not implemented")
}
func (s *stubAppRepo) UpdateStatus(_ context.Context, _ string, _ int) error {
	return errors.New("not implemented")
}
func (s *stubAppRepo) RotateSecret(_ context.Context, _ string, _ string) error {
	return errors.New("not implemented")
}
func (s *stubAppRepo) SetNotify(_ context.Context, _ string, _ string, _ string) error {
	return errors.New("not implemented")
}
func (s *stubAppRepo) AddIPRules(_ context.Context, _ string, _ []string) error {
	return errors.New("not implemented")
}
func (s *stubAppRepo) DeleteIPRules(_ context.Context, _ string) error {
	return errors.New("not implemented")
}
func (s *stubAppRepo) ListIPRules(_ context.Context, _ string) ([]openmodel.OpenAppIPRule, error) {
	return nil, errors.New("not implemented")
}

func (s *stubAppRepo) WriteLog(_ context.Context, _ *openmodel.OpenAPILog) error {
	return nil
}

const testEncryptKey = "unit-test-encrypt-key"

type fixture struct {
	appID     string
	secret    string // 明文（客户端持有）
	repo      *stubAppRepo
	now       time.Time // 服务端时钟（签名窗口与限流判定基准）
	clientNow time.Time // 客户端时钟（签名头时间戳来源），默认与 now 一致
}

func newFixture(t *testing.T, mutate func(app *openmodel.OpenApp, res *openrepo.ResolvedApp)) *fixture {
	t.Helper()
	secret := "unit-test-app-secret"
	enc, err := crypto.Encrypt(secret, testEncryptKey)
	if err != nil {
		t.Fatalf("encrypt secret: %v", err)
	}
	res := openrepo.NewResolvedApp(openmodel.OpenApp{
		ID: 1, AppID: "app_test", AppSecret: enc, Status: openmodel.OpenAppStatusEnabled,
		APIVersion: "v1", OwnerUserID: 100,
	}, []openmodel.OpenAppScope{{AppID: 1, Scope: "catalog:read"}}, nil)
	if mutate != nil {
		mutate(&res.App, res)
	}
	return &fixture{
		appID:     res.App.AppID,
		secret:    secret,
		repo:      &stubAppRepo{apps: map[string]*openrepo.ResolvedApp{res.App.AppID: res}},
		now:       time.Now(),
		clientNow: time.Now(),
	}
}

func (f *fixture) gateway() *Gateway {
	return NewGateway(GatewayDeps{AppRepo: f.repo, EncryptKey: testEncryptKey, Now: func() time.Time { return f.now }})
}

// signedRequest 以 fixture 内的明文 secret 生成完整签名请求。
func (f *fixture) signedRequest(method, path string, query string, body []byte, mangle func(*http.Request)) *http.Request {
	u := path
	if query != "" {
		u += "?" + query
	}
	req := httptest.NewRequest(method, u, strings.NewReader(string(body)))
	ts := fmt.Sprintf("%d", f.clientNow.Unix())
	nonce := fmt.Sprintf("nonce-%d", time.Now().UnixNano()) // nonce 只需唯一，签名窗口看 timestamp
	req.Header.Set(HeaderAppID, f.appID)
	req.Header.Set(HeaderTimestamp, ts)
	req.Header.Set(HeaderNonce, nonce)
	req.Header.Set(HeaderSignature, Sign(f.secret, StringToSign(method, path, req.URL.Query(), body, ts, nonce)))
	if mangle != nil {
		mangle(req)
	}
	return req
}

// serve 构造挂了网关与 scope 保护的引擎并执行请求，返回响应记录器。
func serve(t *testing.T, gw *Gateway, req *http.Request, scope string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/open/v1/ping", gw.Middleware(), func(c *gin.Context) {
		app := AppFromContext(c)
		c.JSON(http.StatusOK, gin.H{"code": 0, "app": app.App.AppID})
	})
	if scope != "" {
		r.GET("/open/v1/scoped", gw.Middleware(), gw.RequireScope(scope), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"code": 0})
		})
	}
	r.POST("/open/v1/ping", gw.Middleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0})
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func codeOf(t *testing.T, body string) int {
	t.Helper()
	var resp struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("parse response %q: %v", body, err)
	}
	return resp.Code
}

func TestGatewayMissingHeaders(t *testing.T) {
	f := newFixture(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/open/v1/ping", nil)
	w := serve(t, f.gateway(), req, "")
	if codeOf(t, w.Body.String()) != CodeOpenMissingAuth {
		t.Fatalf("missing headers: body=%s", w.Body.String())
	}
}

func TestGatewayHappyPath(t *testing.T) {
	f := newFixture(t, nil)
	w := serve(t, f.gateway(), f.signedRequest(http.MethodGet, "/open/v1/ping", "", nil, nil), "")
	if codeOf(t, w.Body.String()) != 0 {
		t.Fatalf("happy path rejected: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "app_test") {
		t.Fatalf("ping should echo app id: %s", w.Body.String())
	}
}

func TestGatewayBadSignature(t *testing.T) {
	f := newFixture(t, nil)
	req := f.signedRequest(http.MethodGet, "/open/v1/ping", "", nil, func(r *http.Request) {
		r.Header.Set(HeaderSignature, strings.Repeat("0", 64))
	})
	w := serve(t, f.gateway(), req, "")
	if codeOf(t, w.Body.String()) != CodeOpenSignInvalid {
		t.Fatalf("bad signature: %s", w.Body.String())
	}
}

func TestGatewayBodyTamperFails(t *testing.T) {
	f := newFixture(t, nil)
	// 签名基于 body {"a":1}，实际发送 {"a":2} → 签名必须失败。
	r := httptest.NewRequest(http.MethodPost, "/open/v1/ping", strings.NewReader(`{"a":2}`))
	ts := fmt.Sprintf("%d", f.now.Unix())
	nonce := "tamper-test"
	r.Header.Set(HeaderAppID, f.appID)
	r.Header.Set(HeaderTimestamp, ts)
	r.Header.Set(HeaderNonce, nonce)
	r.Header.Set(HeaderSignature, Sign(f.secret, StringToSign(http.MethodPost, "/open/v1/ping", r.URL.Query(), []byte(`{"a":1}`), ts, nonce)))
	w := serve(t, f.gateway(), r, "")
	if codeOf(t, w.Body.String()) != CodeOpenSignInvalid {
		t.Fatalf("body tamper must fail signature: %s", w.Body.String())
	}
}

func TestGatewayTimestampDrift(t *testing.T) {
	f := newFixture(t, nil)
	f.now = f.now.Add(6 * time.Minute) // 服务端时间与签名时间偏差超窗
	w := serve(t, f.gateway(), f.signedRequest(http.MethodGet, "/open/v1/ping", "", nil, nil), "")
	if codeOf(t, w.Body.String()) != CodeOpenTimestamp {
		t.Fatalf("timestamp drift: %s", w.Body.String())
	}
}

func TestGatewayDisabledApp(t *testing.T) {
	f := newFixture(t, func(app *openmodel.OpenApp, _ *openrepo.ResolvedApp) {
		app.Status = openmodel.OpenAppStatusDisabled
	})
	w := serve(t, f.gateway(), f.signedRequest(http.MethodGet, "/open/v1/ping", "", nil, nil), "")
	if codeOf(t, w.Body.String()) != CodeOpenAppInvalid {
		t.Fatalf("disabled app: %s", w.Body.String())
	}
}

func TestGatewayUnknownApp(t *testing.T) {
	f := newFixture(t, nil)
	req := f.signedRequest(http.MethodGet, "/open/v1/ping", "", nil, func(r *http.Request) {
		r.Header.Set(HeaderAppID, "app_missing")
	})
	w := serve(t, f.gateway(), req, "")
	if codeOf(t, w.Body.String()) != CodeOpenAppInvalid {
		t.Fatalf("unknown app: %s", w.Body.String())
	}
}

func TestGatewayNonceReplay(t *testing.T) {
	f := newFixture(t, nil)
	gw := f.gateway()
	makeReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/open/v1/ping", nil)
		ts := fmt.Sprintf("%d", f.now.Unix())
		nonce := "fixed-nonce"
		req.Header.Set(HeaderAppID, f.appID)
		req.Header.Set(HeaderTimestamp, ts)
		req.Header.Set(HeaderNonce, nonce)
		req.Header.Set(HeaderSignature, Sign(f.secret, StringToSign(http.MethodGet, "/open/v1/ping", req.URL.Query(), nil, ts, nonce)))
		return req
	}
	w1 := serve(t, gw, makeReq(), "")
	if codeOf(t, w1.Body.String()) != 0 {
		t.Fatalf("first request should pass: %s", w1.Body.String())
	}
	w2 := serve(t, gw, makeReq(), "")
	if codeOf(t, w2.Body.String()) != CodeOpenNonceReplay {
		t.Fatalf("nonce replay must be rejected: %s", w2.Body.String())
	}
}

func TestGatewayIPWhitelist(t *testing.T) {
	f := newFixture(t, func(_ *openmodel.OpenApp, res *openrepo.ResolvedApp) {
		res.IPRules = []openmodel.OpenAppIPRule{{CIDR: "10.0.0.0/8"}}
	})
	// httptest 客户端 IP 默认 192.0.2.1，不在白名单。
	w := serve(t, f.gateway(), f.signedRequest(http.MethodGet, "/open/v1/ping", "", nil, nil), "")
	if codeOf(t, w.Body.String()) != CodeOpenIPDenied {
		t.Fatalf("ip deny: %s", w.Body.String())
	}
}

func TestGatewayScopeDenied(t *testing.T) {
	f := newFixture(t, nil)
	w := serve(t, f.gateway(), f.signedRequest(http.MethodGet, "/open/v1/scoped", "", nil, nil), "order:create")
	if codeOf(t, w.Body.String()) != CodeOpenScopeDenied {
		t.Fatalf("scope denied: %s", w.Body.String())
	}
}

func TestGatewayScopeAllowed(t *testing.T) {
	f := newFixture(t, nil)
	w := serve(t, f.gateway(), f.signedRequest(http.MethodGet, "/open/v1/scoped", "", nil, nil), "catalog:read")
	if codeOf(t, w.Body.String()) != 0 {
		t.Fatalf("scope allowed: %s", w.Body.String())
	}
}

func TestGatewayRateLimited(t *testing.T) {
	f := newFixture(t, func(app *openmodel.OpenApp, _ *openrepo.ResolvedApp) {
		app.RateLimit = 1 // 每分钟 1 次
	})
	gw := f.gateway()
	w1 := serve(t, gw, f.signedRequest(http.MethodGet, "/open/v1/ping", "", nil, nil), "")
	if codeOf(t, w1.Body.String()) != 0 {
		t.Fatalf("first request should pass: %s", w1.Body.String())
	}
	w2 := serve(t, gw, f.signedRequest(http.MethodGet, "/open/v1/ping", "", nil, nil), "")
	if codeOf(t, w2.Body.String()) != CodeOpenRateLimited {
		t.Fatalf("rate limited: %s", w2.Body.String())
	}
}

func TestIPDeniedEdgeCases(t *testing.T) {
	app := &openrepo.ResolvedApp{}
	if ipDenied(app, "1.2.3.4") {
		t.Fatalf("no rules must allow")
	}
	app.IPRules = []openmodel.OpenAppIPRule{{CIDR: "not-a-cidr"}}
	if !ipDenied(app, "1.2.3.4") {
		t.Fatalf("unparsable rules must deny (fail closed)")
	}
	app.IPRules = []openmodel.OpenAppIPRule{{CIDR: "1.2.3.0/24"}}
	if ipDenied(app, "1.2.3.4") {
		t.Fatalf("in-range ip must allow")
	}
	if !ipDenied(app, "5.6.7.8") {
		t.Fatalf("out-of-range ip must deny")
	}
}

func TestRateLimiterRefill(t *testing.T) {
	l := &rateLimiter{}
	start := time.Now()
	// 桶初始满（60 个），连续调用抽干。
	drained := false
	for i := 0; i < 100; i++ {
		if !l.allow(start, 60) {
			drained = true
			break
		}
	}
	if !drained {
		t.Fatalf("bucket of 60 must drain within 100 calls")
	}
	if l.allow(start.Add(time.Millisecond), 60) {
		t.Fatalf("empty bucket must deny immediately")
	}
	// 60/min = 1 token/秒：2 秒补给 2 个，可放行。
	if !l.allow(start.Add(2*time.Second), 60) {
		t.Fatalf("2s refill must grant tokens at 60/min")
	}
}

func TestParseEnvelopeCode(t *testing.T) {
	if got := parseEnvelopeCode(`{"code":40102,"message":"x"}`); got != 40102 {
		t.Fatalf("parse code = %d", got)
	}
	if got := parseEnvelopeCode(`{"code":0,"message":"ok"}`); got != 0 {
		t.Fatalf("parse zero code = %d", got)
	}
	if got := parseEnvelopeCode(`not-json`); got != 0 {
		t.Fatalf("non-json must be 0, got %d", got)
	}
}
