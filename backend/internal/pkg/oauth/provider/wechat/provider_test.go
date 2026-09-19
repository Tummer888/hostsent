package wechat

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"hostsent/backend/internal/pkg/oauth"
)

// 授权地址：微信开放平台网站应用走 qrconnect + snsapi_login，
// 且必须带 #wechat_redirect 锚点，否则在微信内打开可能被拦。
func TestAuthorizeURL(t *testing.T) {
	p, err := oauth.New(Type, oauth.ProviderConfig{Type: Type})
	if err != nil {
		t.Fatalf("构造 provider 失败: %v", err)
	}
	raw := p.AuthorizeURL(oauth.ProviderConfig{
		Type:        Type,
		Credentials: map[string]string{"app_id": "wx1234567890abcdef"},
	}, "state-1", "https://example.com/api/v1/uc/oauth/wechat/callback")

	if !strings.HasSuffix(raw, "#wechat_redirect") {
		t.Fatalf("授权地址必须带 #wechat_redirect，实际 %s", raw)
	}
	u, err := url.Parse(strings.TrimSuffix(raw, "#wechat_redirect"))
	if err != nil {
		t.Fatalf("授权地址不是合法 URL: %v", err)
	}
	if u.Host != "open.weixin.qq.com" || u.Path != "/connect/qrconnect" {
		t.Fatalf("应使用开放平台 qrconnect，实际 %s", u.String())
	}
	q := u.Query()
	if q.Get("appid") != "wx1234567890abcdef" {
		t.Fatalf("appid 未带上: %s", raw)
	}
	if q.Get("scope") != "snsapi_login" {
		t.Fatalf("网站应用扫码登录 scope 应为 snsapi_login，实际 %q", q.Get("scope"))
	}
	if q.Get("state") != "state-1" || q.Get("response_type") != "code" {
		t.Fatalf("state / response_type 未带上: %s", raw)
	}
}

// 凭证里的 scope 优先于默认值（便于个别应用调整授权范围）。
func TestAuthorizeURL_CustomScope(t *testing.T) {
	p, _ := oauth.New(Type, oauth.ProviderConfig{Type: Type})
	raw := p.AuthorizeURL(oauth.ProviderConfig{
		Type:        Type,
		Credentials: map[string]string{"app_id": "wx1", "scope": "snsapi_login snsapi_userinfo"},
	}, "s", "https://example.com/cb")
	q, err := url.Parse(strings.TrimSuffix(raw, "#wechat_redirect"))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if got := q.Query().Get("scope"); got != "snsapi_login snsapi_userinfo" {
		t.Fatalf("scope 应取凭证值，实际 %q", got)
	}
}

// Test 不发起真实授权（需要用户扫码），判据是「凭证齐全 + 授权地址可拼装」。
func TestProvider_Test(t *testing.T) {
	p, _ := oauth.New(Type, oauth.ProviderConfig{Type: Type})

	if err := p.Test(context.Background(), oauth.ProviderConfig{
		Type:        Type,
		Credentials: map[string]string{"app_id": "wx1234567890abcdef", "app_secret": "secret"},
	}); err != nil {
		t.Fatalf("凭证齐全时应通过: %v", err)
	}

	err := p.Test(context.Background(), oauth.ProviderConfig{
		Type:        Type,
		Credentials: map[string]string{"app_id": "wx1234567890abcdef"},
	})
	if err == nil {
		t.Fatal("缺 AppSecret 必须报错")
	}
	if !strings.Contains(err.Error(), "AppSecret") {
		t.Fatalf("错误信息应指明缺失字段，实际: %v", err)
	}
}

// 非 wx 开头的 AppID 给出提示而不是硬失败：不把非标准但可用的自建平台 ID 拦掉。
func TestProvider_Test_NonStandardAppIDIsHint(t *testing.T) {
	p, _ := oauth.New(Type, oauth.ProviderConfig{Type: Type})
	err := p.Test(context.Background(), oauth.ProviderConfig{
		Type:        Type,
		Credentials: map[string]string{"app_id": "custom_app", "app_secret": "secret"},
	})
	if err == nil {
		t.Fatal("非 wx 开头应给出提示")
	}
	if !strings.Contains(err.Error(), "wx 开头") {
		t.Fatalf("提示应说明 AppID 通常以 wx 开头，实际: %v", err)
	}
}
