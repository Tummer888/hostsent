package qq

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"hostsent/backend/internal/pkg/oauth"
)

// 授权地址：QQ 互联用 client_id（不是 appid），scope 默认 get_user_info。
func TestAuthorizeURL(t *testing.T) {
	p, err := oauth.New(Type, oauth.ProviderConfig{Type: Type})
	if err != nil {
		t.Fatalf("构造 provider 失败: %v", err)
	}
	raw := p.AuthorizeURL(oauth.ProviderConfig{
		Type:        Type,
		Credentials: map[string]string{"app_id": "101234567"},
	}, "state-1", "https://example.com/api/v1/uc/oauth/qq/callback")

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("授权地址不是合法 URL: %v", err)
	}
	q := u.Query()
	if q.Get("client_id") != "101234567" {
		t.Fatalf("QQ 用 client_id 传 AppID，实际 %s", raw)
	}
	if q.Get("scope") != "get_user_info" {
		t.Fatalf("scope 默认应为 get_user_info，实际 %q", q.Get("scope"))
	}
	if q.Get("state") != "state-1" || q.Get("response_type") != "code" {
		t.Fatalf("state / response_type 未带上: %s", raw)
	}
}

// QQ 的 token 接口正常时返回 query string 而非 JSON；出错时返回 error=xxx。
// 适配器必须两种都能处理，且 /oauth2.0/me 的 JSONP 必须剥壳。
func TestExchange_QueryStringThenJSONP(t *testing.T) {
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.URL.Path)
		switch r.URL.Path {
		case "/oauth2.0/token":
			// 正常返回 query string。
			w.Write([]byte("access_token=AT-1&expires_in=7776000&refresh_token=RT-1"))
		case "/oauth2.0/me":
			// JSONP 包裹体，且要求带 User-Agent。
			if r.Header.Get("User-Agent") == "" {
				t.Error("/oauth2.0/me 必须带 User-Agent，否则部分场景返回空体")
			}
			w.Write([]byte(`callback( {"openid":"OID-QQ-1","unionid":"UID-QQ-1"} );`))
		case "/user/get_user_info":
			w.Write([]byte(`{"ret":0,"msg":"","nickname":"企鹅","figureurl_qq_2":"https://example.com/qq2.png"}`))
		default:
			t.Errorf("未预期的请求路径 %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	p := &Provider{client: srv.Client()}
	// 用 Endpoint 覆盖：适配器内部的 endpoint 常量不可注入，因此直接改包级变量。
	restore := overrideEndpoints(srv.URL)
	defer restore()

	token, err := p.Exchange(context.Background(), oauth.ProviderConfig{
		Type:        Type,
		Credentials: map[string]string{"app_id": "101234567", "app_key": "key"},
	}, "code-1", "https://example.com/cb")
	if err != nil {
		t.Fatalf("换令牌失败: %v", err)
	}
	if token.AccessToken != "AT-1" {
		t.Fatalf("access_token 解析错误: %q", token.AccessToken)
	}
	if token.OpenID != "OID-QQ-1" {
		t.Fatalf("openid 应从 JSONP 剥壳得到，实际 %q", token.OpenID)
	}
	if token.UnionID != "UID-QQ-1" {
		t.Fatalf("unionid 解析错误: %q", token.UnionID)
	}
	if len(hits) != 2 {
		t.Fatalf("Exchange 应调用 token 与 me 两个接口，实际 %v", hits)
	}

	user, err := p.UserInfo(context.Background(), oauth.ProviderConfig{
		Type:        Type,
		Credentials: map[string]string{"app_id": "101234567", "app_key": "key"},
	}, token)
	if err != nil {
		t.Fatalf("拉取资料失败: %v", err)
	}
	if user.Nickname != "企鹅" || user.Avatar != "https://example.com/qq2.png" {
		t.Fatalf("资料解析错误: %+v", user)
	}
}

// 出错时返回 query string 形式的 error，必须转成 ErrExchangeFailed 而不是静默继续。
func TestExchange_ErrorQueryString(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("error=100016&error_description=access+token+expired"))
	}))
	defer srv.Close()

	p := &Provider{client: srv.Client()}
	restore := overrideEndpoints(srv.URL)
	defer restore()

	_, err := p.Exchange(context.Background(), oauth.ProviderConfig{
		Type:        Type,
		Credentials: map[string]string{"app_id": "101234567", "app_key": "key"},
	}, "code", "")
	if err == nil {
		t.Fatal("上游返回 error 必须报错")
	}
	if !strings.Contains(err.Error(), "100016") {
		t.Fatalf("错误信息应带上游业务码，实际: %v", err)
	}
}

// 缺 AppID / AppKey 时必须先报配置错误，不发出请求。
func TestExchange_MissingCredentials(t *testing.T) {
	p, _ := oauth.New(Type, oauth.ProviderConfig{Type: Type})
	_, err := p.Exchange(context.Background(), oauth.ProviderConfig{
		Type:        Type,
		Credentials: map[string]string{"app_id": "101234567"},
	}, "code", "")
	if err == nil {
		t.Fatal("缺 AppKey 必须报错")
	}
	if !strings.Contains(err.Error(), "AppKey") {
		t.Fatalf("错误信息应指明缺失字段，实际: %v", err)
	}
}

// Test 判据：凭证齐全 + 授权地址可拼装（不发起真实授权）。
func TestProvider_Test(t *testing.T) {
	p, _ := oauth.New(Type, oauth.ProviderConfig{Type: Type})
	if err := p.Test(context.Background(), oauth.ProviderConfig{
		Type:        Type,
		Credentials: map[string]string{"app_id": "101234567", "app_key": "key"},
	}); err != nil {
		t.Fatalf("凭证齐全时应通过: %v", err)
	}
	if err := p.Test(context.Background(), oauth.ProviderConfig{
		Type:        Type,
		Credentials: map[string]string{"app_id": "101234567"},
	}); err == nil {
		t.Fatal("缺 AppKey 必须报错")
	}
}

// overrideEndpoints 把包级 endpoint 常量临时指向 mock 服务，返回还原函数。
//
// 适配器把 endpoint 写成包级常量（生产地址不可配置），测试用这种方式注入
// 本地服务，避免为了可测性在生产代码里加一个只有测试会用的配置项。
func overrideEndpoints(base string) func() {
	token, openID, userInfo := tokenEndpoint, openIDEndpoint, userInfoEndpoint
	tokenEndpoint = base + "/oauth2.0/token"
	openIDEndpoint = base + "/oauth2.0/me"
	userInfoEndpoint = base + "/user/get_user_info"
	return func() {
		tokenEndpoint, openIDEndpoint, userInfoEndpoint = token, openID, userInfo
	}
}
