package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	openmodel "hostsent/backend/internal/modules/open/model"
	openrepo "hostsent/backend/internal/modules/open/repository"
	"hostsent/backend/internal/pkg/crypto"
	"hostsent/backend/internal/pkg/response"
)

// gin 上下文键。
const (
	ctxKeyApp        = "openApp"
	ctxKeyBodySHA256 = "openBodySHA256"
)

// AppFromContext 取鉴权后的应用视图；未鉴权返回 nil。
func AppFromContext(c *gin.Context) *openrepo.ResolvedApp {
	if v, ok := c.Get(ctxKeyApp); ok {
		if app, ok := v.(*openrepo.ResolvedApp); ok {
			return app
		}
	}
	return nil
}

const (
	defaultRatePerMin  = 600 // open_apps.rate_limit=0 时的默认限速
	nonceTTL           = 10 * time.Minute
	appCacheTTL        = 30 * time.Second
	maxDigestBodyBytes = 2048 // 审计摘要截断
)

// GatewayDeps 网关依赖。
type GatewayDeps struct {
	AppRepo    openrepo.AppRepository
	LogRepo    openrepo.APILogRepository
	EncryptKey string // app_secret 解密密钥（= cfg.App.EncryptKey）
	Logger     *zap.Logger
	// Now 便于测试注入时钟；nil 用 time.Now。
	Now func() time.Time
}

// Gateway 开放平台网关：签名验证 → 应用校验 → IP 白名单 → nonce 防重放 → 限流 → 审计。
type Gateway struct {
	deps     GatewayDeps
	nonces   *nonceCache
	limiters *limiterStore
	apps     *appCache
}

// NewGateway 构造网关。
func NewGateway(deps GatewayDeps) *Gateway {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Logger == nil {
		deps.Logger = zap.NewNop()
	}
	return &Gateway{
		deps:     deps,
		nonces:   newNonceCache(nonceTTL),
		limiters: newLimiterStore(defaultRatePerMin),
		apps:     newAppCache(appCacheTTL),
	}
}

// Middleware 鉴权链。任何失败以固定错误码终止请求（响应信封与全站一致，HTTP 200 + code）。
func (g *Gateway) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.GetHeader(HeaderAppID)
		ts := c.GetHeader(HeaderTimestamp)
		nonce := c.GetHeader(HeaderNonce)
		sig := c.GetHeader(HeaderSignature)
		if appID == "" || ts == "" || nonce == "" || sig == "" {
			abortOpen(c, CodeOpenMissingAuth, "缺少认证头")
			return
		}

		tsVal, err := strconv.ParseInt(ts, 10, 64)
		if err != nil {
			abortOpen(c, CodeOpenTimestamp, "时间戳格式错误")
			return
		}
		now := g.deps.Now()
		drift := now.Unix() - tsVal
		if drift < 0 {
			drift = -drift
		}
		if drift > int64(SignatureWindow/time.Second) {
			abortOpen(c, CodeOpenTimestamp, "时间戳超出窗口")
			return
		}

		app, err := g.loadApp(c.Request.Context(), appID)
		if err != nil {
			g.deps.Logger.Warn("open: resolve app failed", zap.String("app_id", appID), zap.Error(err))
			abortOpen(c, CodeOpenAppInvalid, "应用不存在或已停用")
			return
		}
		if app.App.Status != openmodel.OpenAppStatusEnabled {
			abortOpen(c, CodeOpenAppInvalid, "应用不存在或已停用")
			return
		}

		if ipDenied(app, c.ClientIP()) {
			abortOpen(c, CodeOpenIPDenied, "来源 IP 不在白名单")
			return
		}

		// body 读取后回填，后续 handler 正常解析；SHA256 摘要供签名与审计共用。
		var body []byte
		if c.Request.Body != nil {
			body, err = io.ReadAll(c.Request.Body)
			if err != nil {
				abortOpen(c, CodeOpenParam, "读取请求体失败")
				return
			}
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
		}
		bodySum := sha256.Sum256(body)

		secret, err := crypto.Decrypt(app.App.AppSecret, g.deps.EncryptKey)
		if err != nil {
			g.deps.Logger.Error("open: decrypt app secret failed", zap.String("app_id", appID), zap.Error(err))
			abortOpen(c, CodeOpenInternal, "服务内部错误")
			return
		}
		sts := StringToSign(c.Request.Method, c.Request.URL.Path, c.Request.URL.Query(), body, ts, nonce)
		if !Verify(secret, sts, sig) {
			abortOpen(c, CodeOpenSignInvalid, "签名不正确")
			return
		}

		if g.nonces.seen(appID + ":" + nonce) {
			abortOpen(c, CodeOpenNonceReplay, "nonce 已被使用")
			return
		}

		if !g.limiters.allow(app.App.ID, app.App.RateLimit, now) {
			abortOpen(c, CodeOpenRateLimited, "请求过于频繁")
			return
		}

		c.Set(ctxKeyApp, app)
		c.Set(ctxKeyBodySHA256, hex.EncodeToString(bodySum[:]))
		c.Next()
	}
}

// RequireScope 按路由声明所需能力位；未持有以 40301 终止。
func (g *Gateway) RequireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		app := AppFromContext(c)
		if app == nil {
			abortOpen(c, CodeOpenMissingAuth, "缺少认证头")
			return
		}
		if !app.HasScope(scope) {
			abortOpen(c, CodeOpenScopeDenied, "缺少能力位："+scope)
			return
		}
		c.Next()
	}
}

// Audit 请求/响应摘要落 open_api_logs（失败仅记日志，不影响主流程）。
// 注册为分组最外层：鉴权失败（401xx）同样留痕。
func (g *Gateway) Audit() gin.HandlerFunc {
	if g.deps.LogRepo == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		start := g.deps.Now()
		cw := &digestWriter{ResponseWriter: c.Writer}
		c.Writer = cw
		c.Next()

		app := AppFromContext(c)
		var appID uint64
		if app != nil {
			appID = app.App.ID
		}
		row := &openmodel.OpenAPILog{
			AppID:           appID,
			Method:          c.Request.Method,
			Path:            c.Request.URL.Path,
			Query:           truncateStr(c.Request.URL.RawQuery, maxDigestBodyBytes),
			ClientRequestID: c.GetHeader(HeaderClientRequestID),
			StatusCode:      c.Writer.Status(),
			ErrorCode:       parseEnvelopeCode(cw.digest()),
			DurationMS:      g.deps.Now().Sub(start).Milliseconds(),
			RequestDigest:   bodySHA256Of(c),
			ResponseDigest:  cw.digest(),
			IP:              c.ClientIP(),
		}
		// 异步落库：不阻塞响应；独立 context 避免请求结束后被取消。
		logRepo := g.deps.LogRepo
		logger := g.deps.Logger
		go func() {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 5*time.Second)
			defer cancel()
			if err := logRepo.WriteLog(ctx, row); err != nil {
				logger.Warn("open: write api log failed", zap.String("path", row.Path), zap.Error(err))
			}
		}()
	}
}

func (g *Gateway) loadApp(ctx context.Context, appID string) (*openrepo.ResolvedApp, error) {
	if app, ok := g.apps.get(appID); ok {
		return app, nil
	}
	app, err := g.deps.AppRepo.ResolveByAppID(ctx, appID)
	if err != nil {
		return nil, err
	}
	g.apps.put(appID, app)
	return app, nil
}

// ipDenied 判断来源 IP 是否被白名单拒绝：无规则放行；有规则时需命中任一 CIDR。
func ipDenied(app *openrepo.ResolvedApp, clientIP string) bool {
	if len(app.IPRules) == 0 {
		return false
	}
	addr, err := netip.ParseAddr(clientIP)
	if err != nil {
		return true
	}
	for _, rule := range app.IPRules {
		prefix, err := netip.ParsePrefix(rule.CIDR)
		if err != nil {
			continue
		}
		if prefix.Contains(addr.Unmap()) {
			return false
		}
	}
	return true
}

// abortOpen 以统一信封返回业务错误码并终止请求链。
func abortOpen(c *gin.Context, code int, message string) {
	c.AbortWithStatusJSON(http.StatusOK, response.Body{Code: code, Message: message, Timestamp: time.Now().Unix()})
}

// ---------------------------------------------------------------------------
// nonce 防重放缓存（进程内；单实例部署下成立，多副本需换集中存储）
// ---------------------------------------------------------------------------

type nonceCache struct {
	mu  sync.Mutex
	m   map[string]time.Time
	ttl time.Duration
}

func newNonceCache(ttl time.Duration) *nonceCache {
	return &nonceCache{m: make(map[string]time.Time), ttl: ttl}
}

// seen 返回 true 表示 nonce 在 TTL 内出现过（重放）；否则登记并放行。
func (n *nonceCache) seen(key string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	now := time.Now()
	if ts, ok := n.m[key]; ok && now.Sub(ts) < n.ttl {
		return true
	}
	// 顺手清理过期项，避免长期运行内存膨胀。
	if len(n.m) >= 1<<16 {
		for k, ts := range n.m {
			if now.Sub(ts) >= n.ttl {
				delete(n.m, k)
			}
		}
	}
	n.m[key] = now
	return false
}

// ---------------------------------------------------------------------------
// 每 app 令牌桶限流（进程内）
// ---------------------------------------------------------------------------

type rateLimiter struct {
	perMin int
	tokens float64
	last   time.Time
	mu     sync.Mutex
}

func (l *rateLimiter) allow(now time.Time, perMin int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if perMin <= 0 {
		perMin = defaultRatePerMin
	}
	// 应用限速被调整后重建桶。
	if l.perMin != perMin {
		l.perMin = perMin
		l.tokens = float64(perMin)
		l.last = now
	}
	elapsed := now.Sub(l.last).Seconds()
	l.last = now
	l.tokens += elapsed * float64(perMin) / 60
	if max := float64(perMin); l.tokens > max {
		l.tokens = max
	}
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}

type limiterStore struct {
	mu sync.Mutex
	m  map[uint64]*rateLimiter
}

func newLimiterStore(def int) *limiterStore {
	return &limiterStore{m: make(map[uint64]*rateLimiter)}
}

func (s *limiterStore) allow(appID uint64, perMin int, now time.Time) bool {
	s.mu.Lock()
	l, ok := s.m[appID]
	if !ok {
		l = &rateLimiter{perMin: -1}
		s.m[appID] = l
	}
	s.mu.Unlock()
	return l.allow(now, perMin)
}

// ---------------------------------------------------------------------------
// 应用短 TTL 缓存：签名链每请求查库代价高，TTL 内复用解析结果
// ---------------------------------------------------------------------------

type appCache struct {
	mu  sync.Mutex
	m   map[string]appCacheEntry
	ttl time.Duration
}

type appCacheEntry struct {
	app       *openrepo.ResolvedApp
	expiresAt time.Time
}

func newAppCache(ttl time.Duration) *appCache {
	return &appCache{m: make(map[string]appCacheEntry), ttl: ttl}
}

func (c *appCache) get(appID string) (*openrepo.ResolvedApp, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[appID]
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.app, true
}

func (c *appCache) put(appID string, app *openrepo.ResolvedApp) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[appID] = appCacheEntry{app: app, expiresAt: time.Now().Add(c.ttl)}
}

// parseEnvelopeCode 从响应摘要解析业务 code（HTTP 200 + code!=0 也是失败响应）。
func parseEnvelopeCode(digest string) int {
	var body struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal([]byte(digest), &body); err != nil {
		return 0
	}
	if body.Code < 0 {
		return 0
	}
	return body.Code
}

func bodySHA256Of(c *gin.Context) string {
	if v, ok := c.Get(ctxKeyBodySHA256); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// digestWriter 捕获响应体片段（截断），供审计与错误码解析。
type digestWriter struct {
	gin.ResponseWriter
	buf bytes.Buffer
}

func (w *digestWriter) Write(b []byte) (int, error) {
	if w.buf.Len() < maxDigestBodyBytes {
		remain := maxDigestBodyBytes - w.buf.Len()
		if len(b) < remain {
			remain = len(b)
		}
		w.buf.Write(b[:remain])
	}
	return w.ResponseWriter.Write(b)
}

func (w *digestWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *digestWriter) digest() string { return w.buf.String() }

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
