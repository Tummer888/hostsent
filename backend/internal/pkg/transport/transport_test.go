package transport

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"hostsent/backend/internal/pkg/upstream"
)

func TestKindOf(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want ErrKind
	}{
		{"nil", nil, ""},
		{"deadline", context.DeadlineExceeded, ErrNetwork},
		{"not implemented", upstream.ErrNotImplemented{}, ErrNotSupported},
		{"http 401", &upstream.ProviderError{StatusCode: 401}, ErrAuthFailed},
		{"http 403", &upstream.ProviderError{StatusCode: 403}, ErrAuthFailed},
		{"http 404", &upstream.ProviderError{StatusCode: 404}, ErrNotFound},
		{"http 429", &upstream.ProviderError{StatusCode: 429}, ErrRateLimited},
		{"http 400", &upstream.ProviderError{StatusCode: 400}, ErrInvalidParam},
		{"http 500", &upstream.ProviderError{StatusCode: 500}, ErrUpstreamInternal},
		{"business code 405 token", &upstream.ProviderError{Code: 405, Msg: "token 失效"}, ErrAuthFailed},
		{"msg rate limit", &upstream.ProviderError{Msg: "too many requests"}, ErrRateLimited},
		{"msg not found", &upstream.ProviderError{Msg: "实例不存在"}, ErrNotFound},
		{"msg unsupported", &upstream.ProviderError{Msg: "不支持该操作"}, ErrNotSupported},
		{"msg exists", &upstream.ProviderError{Msg: "已存在"}, ErrNotFound},
		{"net error", &net.OpError{Op: "dial", Err: errors.New("refused")}, ErrNetwork},
		{"unknown", errors.New("boom"), ErrUnknown},
	}
	for _, c := range cases {
		if got := KindOf(c.err); got != c.want {
			t.Errorf("%s: KindOf = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestRetryableAndPause(t *testing.T) {
	if !Retryable(ErrRateLimited) || !Retryable(ErrNetwork) || !Retryable(ErrUpstreamInternal) {
		t.Fatal("transient kinds should be retryable")
	}
	if Retryable(ErrAuthFailed) || Retryable(ErrInvalidParam) || Retryable(ErrNotSupported) {
		t.Fatal("permanent kinds must not be retryable")
	}
	if !ShouldPauseProvider(ErrAuthFailed) || !ShouldPauseProvider(ErrNotSupported) {
		t.Fatal("auth/not-supported should pause provider")
	}
	if ShouldPauseProvider(ErrRateLimited) {
		t.Fatal("rate limit should back off, not pause")
	}
}

func TestRetryPolicyStopsOnPermanentError(t *testing.T) {
	calls := 0
	p := RetryPolicy{Timeout: time.Second, MaxRetries: 3, BaseDelay: time.Millisecond}
	err := p.Do(context.Background(), func(context.Context) error {
		calls++
		return &upstream.ProviderError{StatusCode: 401}
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Fatalf("auth failure must not retry, calls = %d", calls)
	}
}

func TestRetryPolicyRetriesTransient(t *testing.T) {
	calls := 0
	p := RetryPolicy{Timeout: time.Second, MaxRetries: 2, BaseDelay: time.Millisecond}
	err := p.Do(context.Background(), func(context.Context) error {
		calls++
		if calls < 3 {
			return &upstream.ProviderError{StatusCode: 503}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestRetryPolicyHonorsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := RetryPolicy{Timeout: time.Second, MaxRetries: 5, BaseDelay: time.Second}
	err := p.Do(ctx, func(context.Context) error {
		return &upstream.ProviderError{StatusCode: 503}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestNormalizeFillsDefaults(t *testing.T) {
	p := RetryPolicy{}.Normalize()
	if p.Timeout != DefaultTimeout || p.BaseDelay != DefaultBaseDelay || p.MaxRetries != 0 {
		t.Fatalf("unexpected normalized policy: %+v", p)
	}
}

func TestBackoffGrowsAndCaps(t *testing.T) {
	base := 100 * time.Millisecond
	first := Backoff(ErrUnknown, 1, base)
	later := Backoff(ErrUnknown, 5, base)
	if first <= 0 || later <= first {
		t.Fatalf("backoff should grow: %v -> %v", first, later)
	}
	capped := Backoff(ErrUnknown, 100, base)
	if capped > 3*time.Minute {
		t.Fatalf("backoff not capped: %v", capped)
	}
	// RateLimited 基线更长。
	if Backoff(ErrRateLimited, 1, base) <= Backoff(ErrUpstreamInternal, 1, base) {
		t.Fatal("rate limited should use a longer base delay")
	}
}

func TestNilLimiterNeverBlocks(t *testing.T) {
	var l *Limiter
	wait, err := l.Wait(context.Background())
	if err != nil || wait != 0 {
		t.Fatalf("nil limiter should pass through, got %v, %v", wait, err)
	}
	if NewLimiter(RateLimitSpec{QPS: 0}) != nil {
		t.Fatal("qps<=0 should disable limiting")
	}
}

func TestLimiterThrottlesSecondCall(t *testing.T) {
	l := NewLimiter(RateLimitSpec{QPS: 10, Burst: 1})
	if l == nil {
		t.Fatal("limiter should be created")
	}
	if _, err := l.Wait(context.Background()); err != nil {
		t.Fatalf("first wait: %v", err)
	}
	// 桶容量 1，第二次必须等待约 100ms。
	start := time.Now()
	if _, err := l.Wait(context.Background()); err != nil {
		t.Fatalf("second wait: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 50*time.Millisecond {
		t.Fatalf("expected throttling, elapsed = %v", elapsed)
	}
}

func TestLimiterRespectsContext(t *testing.T) {
	l := NewLimiter(RateLimitSpec{QPS: 1, Burst: 1})
	if _, err := l.Wait(context.Background()); err != nil {
		t.Fatalf("first wait: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := l.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline, got %v", err)
	}
}

func TestGetSigner(t *testing.T) {
	for _, typ := range []string{
		upstream.SignerNone, upstream.SignerBearer, upstream.SignerForm,
		upstream.SignerAKSK, upstream.SignerTC3,
	} {
		s, ok := GetSigner(typ)
		if !ok {
			t.Fatalf("signer %q missing", typ)
		}
		if s.Type() != typ {
			t.Fatalf("signer %q reports type %q", typ, s.Type())
		}
	}
	if _, ok := GetSigner("nope"); ok {
		t.Fatal("unknown signer should not resolve")
	}
}

func TestBearerSigner(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err := mustSigner(t, upstream.SignerBearer).Sign(req, Credentials{"token": "abc"}, nil); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer abc" {
		t.Fatalf("Authorization = %q", got)
	}
	// 回退 api_key。
	req2, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err := mustSigner(t, upstream.SignerBearer).Sign(req2, Credentials{"api_key": "k"}, nil); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	// 缺失凭证必须报错。
	req3, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err := mustSigner(t, upstream.SignerBearer).Sign(req3, Credentials{}, nil); err == nil {
		t.Fatal("expected missing credential error")
	}
}

func TestFormSignerRewritesBody(t *testing.T) {
	body := []byte("a=1&b=2")
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", strings.NewReader(string(body)))
	s := mustSigner(t, upstream.SignerForm)
	if err := s.Sign(req, Credentials{"api_secret": "sec"}, body); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if req.ContentLength <= int64(len(body)) {
		t.Fatalf("body should grow with sign param, len = %d", req.ContentLength)
	}
	if req.GetBody == nil {
		t.Fatal("GetBody should be set for retryable requests")
	}
}

func TestFormSignerMissingSecret(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err := mustSigner(t, upstream.SignerForm).Sign(req, Credentials{}, nil); err == nil {
		t.Fatal("expected missing secret error")
	}
}

func TestAliyunRPCSigner(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://ecs.aliyuncs.com/?Action=DescribeInstances", nil)
	cred := Credentials{"access_key_id": "AK", "access_key_secret": "SK"}
	if err := mustSigner(t, upstream.SignerAKSK).Sign(req, cred, nil); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	q := req.URL.Query()
	for _, key := range []string{"AccessKeyId", "Signature", "SignatureMethod", "SignatureNonce", "Timestamp"} {
		if q.Get(key) == "" {
			t.Fatalf("missing query param %s", key)
		}
	}
	if q.Get("AccessKeyId") != "AK" {
		t.Fatalf("AccessKeyId = %q", q.Get("AccessKeyId"))
	}
}

func TestAliyunPercentEncode(t *testing.T) {
	// 阿里云要求空格编码为 %20、* 为 %2A、~ 保持原样。
	got := aliyunPercentEncode("a b*c~d")
	if !strings.Contains(got, "%20") || !strings.Contains(got, "%2A") || !strings.Contains(got, "~") {
		t.Fatalf("aliyun encoding = %q", got)
	}
	if strings.Contains(got, "+") {
		t.Fatalf("space must not encode as +: %q", got)
	}
}

func TestTencentTC3Signer(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://cvm.tencentcloudapi.com/", strings.NewReader("{}"))
	req.Header.Set("X-TC-Action", "DescribeInstances")
	cred := Credentials{"secret_id": "SID", "secret_key": "SKEY", "service": "cvm"}
	if err := mustSigner(t, upstream.SignerTC3).Sign(req, cred, []byte("{}")); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	auth := req.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "TC3-HMAC-SHA256 Credential=SID/") {
		t.Fatalf("Authorization = %q", auth)
	}
	if !strings.Contains(auth, "SignedHeaders=content-type;host") {
		t.Fatalf("Authorization = %q", auth)
	}
	if req.Header.Get("X-TC-Timestamp") == "" {
		t.Fatal("X-TC-Timestamp missing")
	}
}

func TestTencentTC3RequiresAction(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://cvm.tencentcloudapi.com/", nil)
	cred := Credentials{"secret_id": "SID", "secret_key": "SKEY", "service": "cvm"}
	if err := mustSigner(t, upstream.SignerTC3).Sign(req, cred, nil); err == nil {
		t.Fatal("expected error when X-TC-Action missing")
	}
}

func mustSigner(t *testing.T, typ string) Signer {
	t.Helper()
	s, ok := GetSigner(typ)
	if !ok {
		t.Fatalf("signer %q not found", typ)
	}
	return s
}
