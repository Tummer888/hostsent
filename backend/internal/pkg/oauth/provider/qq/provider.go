// Package qq 提供 QQ 互联开放平台的网站应用登录适配器（doc104 §6.3）。
//
// 流程：authorize（授权码）→ /oauth2.0/token（换令牌，返回 query string）
//
//	→ /oauth2.0/me（拿 openid，返回 **JSONP 包裹体**）
//	→ /user/get_user_info（拉昵称头像）。
//
// 两个 QQ 特有的坑，都在本适配器内处理：
//  1. token 接口正常与出错时都返回 `access_token=...&expires_in=...` 形式的
//     query string，不是 JSON，因此用 ParseQueryString 解析；
//  2. /oauth2.0/me 返回 `callback( {"openid":"..."} );`，必须剥壳（ParseJSONP），
//     且该接口要求带 User-Agent，否则部分场景返回空。
package qq

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"hostsent/backend/internal/pkg/integration"
	"hostsent/backend/internal/pkg/oauth"
)

// Type 渠道标识。
const Type = "qq"

// QQ 互联的接口地址。写成包级变量而不是常量，唯一原因是让单测能把它们
// 指向 httptest 起的本地服务（QQ 的 token 接口返回 query string、/me 返回
// JSONP 两个坑都必须靠真实 HTTP 往返才能覆盖）；生产装配不做任何改写。
var (
	authorizeEndpoint = "https://graph.qq.com/oauth2.0/authorize"
	tokenEndpoint     = "https://graph.qq.com/oauth2.0/token"
	openIDEndpoint    = "https://graph.qq.com/oauth2.0/me"
	userInfoEndpoint  = "https://graph.qq.com/user/get_user_info"
)

func init() {
	oauth.RegisterDescriptor(Type, oauth.CapabilityDescriptor{
		Type:           Type,
		Name:           "QQ",
		Mode:           "api",
		AdapterVersion: "1.0.0",
		Icon:           "qq",
		DocURL:         "https://wiki.connect.qq.com/",
		DefaultScopes:  "get_user_info",
		CredentialSchema: []integration.Field{
			{Key: "app_id", Label: "AppID", Type: integration.FieldTypeString, Required: true,
				Placeholder: "QQ 互联应用的 AppID（数字）"},
			{Key: "app_key", Label: "AppKey", Type: integration.FieldTypePassword, Required: true, Secret: true},
			{Key: "scope", Label: "授权范围", Type: integration.FieldTypeString, Required: false,
				Default: "get_user_info"},
		},
	})
	oauth.RegisterFactory(Type, func(cfg oauth.ProviderConfig) oauth.Provider {
		return &Provider{client: oauth.HTTPClient(0)}
	})
}

// Provider QQ 登录适配器。
type Provider struct{ client *http.Client }

// Type 返回渠道标识。
func (p *Provider) Type() string { return Type }

// AuthorizeURL 拼装授权地址。
func (p *Provider) AuthorizeURL(cfg oauth.ProviderConfig, state, redirectURI string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", creds(cfg)["app_id"])
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	q.Set("scope", orDefault(creds(cfg)["scope"], cfg.Scopes, "get_user_info"))
	return authorizeEndpoint + "?" + q.Encode()
}

// Exchange 用授权码换令牌。
func (p *Provider) Exchange(ctx context.Context, cfg oauth.ProviderConfig, code, redirectURI string) (*oauth.Token, error) {
	c := creds(cfg)
	if c["app_id"] == "" || c["app_key"] == "" {
		return nil, fmt.Errorf("%w: AppID / AppKey 必填", oauth.ErrProviderConfig)
	}
	q := url.Values{}
	q.Set("grant_type", "authorization_code")
	q.Set("client_id", c["app_id"])
	q.Set("client_secret", c["app_key"])
	q.Set("code", code)
	q.Set("redirect_uri", redirectURI)
	q.Set("fmt", "json")
	// QQ 需要 client_id 的十进制值参与签名，带上返回格式参数以减少解析歧义。

	body, err := oauth.GetBytes(ctx, p.client, tokenEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", oauth.ErrExchangeFailed, err)
	}
	// 出错时返回 `error=xxx&error_description=yyy`；正常时返回 query string。
	kv := oauth.ParseQueryString(body)
	if msg := kv["error"]; msg != "" {
		return nil, fmt.Errorf("%w: QQ 返回 error=%s", oauth.ErrExchangeFailed, msg)
	}
	token := kv["access_token"]
	if token == "" {
		// 少数情况返回 JSON，兜底再试一次解析。
		var j struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int    `json:"expires_in"`
			Error       int    `json:"error"`
		}
		if err := oauth.ParseJSON(body, &j); err != nil || j.AccessToken == "" {
			return nil, fmt.Errorf("%w: QQ 未返回 access_token", oauth.ErrExchangeFailed)
		}
		token = j.AccessToken
	}
	openID, unionID, err := p.fetchOpenID(ctx, token)
	if err != nil {
		return nil, err
	}
	return &oauth.Token{AccessToken: token, OpenID: openID, UnionID: unionID}, nil
}

// fetchOpenID 调 /oauth2.0/me 拿 openid（JSONP 剥壳）。
func (p *Provider) fetchOpenID(ctx context.Context, accessToken string) (string, string, error) {
	endpoint := openIDEndpoint + "?access_token=" + url.QueryEscape(accessToken) + "&fmt=json&unionid=1"
	// QQ 的 /me 在缺少 User-Agent 时可能返回空体，必须显式带上。
	body, err := oauth.GetBytes(ctx, p.client, endpoint, map[string]string{"User-Agent": "HostSent/1.0"})
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", oauth.ErrExchangeFailed, err)
	}
	var resp struct {
		OpenID  string `json:"openid"`
		UnionID string `json:"unionid"`
		Error   int    `json:"error"`
	}
	if err := oauth.ParseJSONP(body, &resp); err != nil {
		return "", "", fmt.Errorf("%w: %v", oauth.ErrExchangeFailed, err)
	}
	if resp.Error != 0 || resp.OpenID == "" {
		return "", "", fmt.Errorf("%w: QQ 未返回 openid", oauth.ErrExchangeFailed)
	}
	return resp.OpenID, resp.UnionID, nil
}

// UserInfo 拉取 QQ 用户资料。
func (p *Provider) UserInfo(ctx context.Context, cfg oauth.ProviderConfig, token *oauth.Token) (*oauth.ExternalUser, error) {
	q := url.Values{}
	q.Set("access_token", token.AccessToken)
	q.Set("oauth_consumer_key", creds(cfg)["app_id"])
	q.Set("openid", token.OpenID)

	body, err := oauth.GetBytes(ctx, p.client, userInfoEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", oauth.ErrUserInfoFailed, err)
	}
	var resp struct {
		Ret       int    `json:"ret"`
		Msg       string `json:"msg"`
		Nickname  string `json:"nickname"`
		FigureURL string `json:"figureurl_qq_2"`
		Figure2   string `json:"figureurl_qq_1"`
	}
	if err := oauth.ParseJSON(body, &resp); err != nil {
		return nil, fmt.Errorf("%w: %v", oauth.ErrUserInfoFailed, err)
	}
	if resp.Ret != 0 {
		return nil, fmt.Errorf("%w: QQ 返回 ret=%d", oauth.ErrUserInfoFailed, resp.Ret)
	}
	avatar := resp.FigureURL
	if avatar == "" {
		avatar = resp.Figure2
	}
	return &oauth.ExternalUser{
		OpenID:   token.OpenID,
		UnionID:  token.UnionID,
		Nickname: resp.Nickname,
		Avatar:   avatar,
	}, nil
}

// Test 连通性测试：校验凭证齐全且授权地址可拼装。
func (p *Provider) Test(_ context.Context, cfg oauth.ProviderConfig) error {
	c := creds(cfg)
	if err := oauth.ValidateCredentials(mustDescriptor(), c); err != nil {
		return err
	}
	if strings.TrimSpace(c["app_id"]) == "" {
		return fmt.Errorf("%w: AppID 必填", oauth.ErrProviderConfig)
	}
	if !strings.Contains(p.AuthorizeURL(cfg, "probe", "https://example.com/callback"), "client_id=") {
		return fmt.Errorf("%w: 授权地址拼装失败", oauth.ErrProviderConfig)
	}
	return nil
}

func creds(cfg oauth.ProviderConfig) map[string]string {
	if cfg.Credentials == nil {
		return map[string]string{}
	}
	return cfg.Credentials
}

func mustDescriptor() oauth.CapabilityDescriptor {
	d, _ := oauth.Descriptor(Type)
	return d
}

func orDefault(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
