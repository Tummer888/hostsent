// Package wechat 提供微信开放平台「网站应用」扫码登录适配器（doc104 §6.3）。
//
// 流程：qrconnect（扫码授权）→ sns/oauth2/access_token（code 换 token，返回 openid）
//
//	→ sns/userinfo（拉昵称头像）。
//
// 与微信「公众平台」网页授权（snsapi_base/userinfo，走 /connect/oauth2/authorize）
// 的区别：开放平台网站应用走 qrconnect 且 scope 为 snsapi_login，本适配器实现的是
// PC 站扫码登录这一形态，与登录页的「微信登录」图标语义一致。
package wechat

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"hostsent/backend/internal/pkg/integration"
	"hostsent/backend/internal/pkg/oauth"
)

// Type 渠道标识。
const Type = "wechat"

const (
	authorizeEndpoint = "https://open.weixin.qq.com/connect/qrconnect"
	tokenEndpoint     = "https://api.weixin.qq.com/sns/oauth2/access_token"
	userInfoEndpoint  = "https://api.weixin.qq.com/sns/userinfo"
)

func init() {
	oauth.RegisterDescriptor(Type, oauth.CapabilityDescriptor{
		Type:           Type,
		Name:           "微信",
		Mode:           "qr",
		AdapterVersion: "1.0.0",
		Icon:           "wechat",
		DocURL:         "https://developers.weixin.qq.com/doc/oplatform/Website_App/WeChat_Login/Wechat_Login.html",
		DefaultScopes:  "snsapi_login",
		CredentialSchema: []integration.Field{
			{Key: "app_id", Label: "AppID", Type: integration.FieldTypeString, Required: true,
				Placeholder: "微信开放平台网站应用的 AppID"},
			{Key: "app_secret", Label: "AppSecret", Type: integration.FieldTypePassword, Required: true, Secret: true},
			{Key: "scope", Label: "授权范围", Type: integration.FieldTypeString, Required: false,
				Default: "snsapi_login", Help: "网站应用扫码登录固定为 snsapi_login"},
		},
	})
	oauth.RegisterFactory(Type, func(cfg oauth.ProviderConfig) oauth.Provider {
		return &Provider{client: oauth.HTTPClient(0)}
	})
}

// Provider 微信登录适配器。
type Provider struct{ client *http.Client }

// Type 返回渠道标识。
func (p *Provider) Type() string { return Type }

// AuthorizeURL 拼装扫码授权地址。
func (p *Provider) AuthorizeURL(cfg oauth.ProviderConfig, state, redirectURI string) string {
	q := url.Values{}
	q.Set("appid", creds(cfg)["app_id"])
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", orDefault(creds(cfg)["scope"], cfg.Scopes, "snsapi_login"))
	q.Set("state", state)
	// 微信要求带 #wechat_redirect 锚点，否则在微信内打开可能被拦。
	return authorizeEndpoint + "?" + q.Encode() + "#wechat_redirect"
}

// Exchange 用授权码换令牌。
func (p *Provider) Exchange(ctx context.Context, cfg oauth.ProviderConfig, code, _ string) (*oauth.Token, error) {
	c := creds(cfg)
	if c["app_id"] == "" || c["app_secret"] == "" {
		return nil, fmt.Errorf("%w: AppID / AppSecret 必填", oauth.ErrProviderConfig)
	}
	q := url.Values{}
	q.Set("appid", c["app_id"])
	q.Set("secret", c["app_secret"])
	q.Set("code", code)
	q.Set("grant_type", "authorization_code")

	body, err := oauth.GetBytes(ctx, p.client, tokenEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", oauth.ErrExchangeFailed, err)
	}
	var resp struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    int    `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		OpenID       string `json:"openid"`
		UnionID      string `json:"unionid"`
		ErrCode      int    `json:"errcode"`
		ErrMsg       string `json:"errmsg"`
	}
	if err := oauth.ParseJSON(body, &resp); err != nil {
		return nil, fmt.Errorf("%w: %v", oauth.ErrExchangeFailed, err)
	}
	if resp.ErrCode != 0 || resp.AccessToken == "" {
		return nil, fmt.Errorf("%w: 微信返回 errcode=%d", oauth.ErrExchangeFailed, resp.ErrCode)
	}
	return &oauth.Token{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		OpenID:       resp.OpenID,
		UnionID:      resp.UnionID,
		ExpiresIn:    resp.ExpiresIn,
	}, nil
}

// UserInfo 拉取微信用户资料。
func (p *Provider) UserInfo(ctx context.Context, cfg oauth.ProviderConfig, token *oauth.Token) (*oauth.ExternalUser, error) {
	q := url.Values{}
	q.Set("access_token", token.AccessToken)
	q.Set("openid", token.OpenID)
	q.Set("lang", "zh_CN")

	body, err := oauth.GetBytes(ctx, p.client, userInfoEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", oauth.ErrUserInfoFailed, err)
	}
	var resp struct {
		OpenID   string `json:"openid"`
		Nickname string `json:"nickname"`
		HeadImg  string `json:"headimgurl"`
		UnionID  string `json:"unionid"`
		ErrCode  int    `json:"errcode"`
		ErrMsg   string `json:"errmsg"`
	}
	if err := oauth.ParseJSON(body, &resp); err != nil {
		return nil, fmt.Errorf("%w: %v", oauth.ErrUserInfoFailed, err)
	}
	if resp.ErrCode != 0 {
		return nil, fmt.Errorf("%w: 微信返回 errcode=%d", oauth.ErrUserInfoFailed, resp.ErrCode)
	}
	openID := resp.OpenID
	if openID == "" {
		openID = token.OpenID
	}
	unionID := resp.UnionID
	if unionID == "" {
		unionID = token.UnionID
	}
	return &oauth.ExternalUser{
		OpenID:   openID,
		UnionID:  unionID,
		Nickname: resp.Nickname,
		Avatar:   resp.HeadImg,
	}, nil
}

// Test 连通性测试：校验凭证齐全且授权地址可拼装。
//
// 不发起真实授权（需要用户在微信侧操作），判据是「凭证齐全 + 关键字段非空」，
// 与 pkg/payment/manual 的 HealthCheck 口径一致。
func (p *Provider) Test(_ context.Context, cfg oauth.ProviderConfig) error {
	c := creds(cfg)
	if err := oauth.ValidateCredentials(mustDescriptor(), c); err != nil {
		return err
	}
	if !strings.HasPrefix(c["app_id"], "wx") {
		// 微信开放平台 AppID 一律以 wx 开头；不匹配时给出提示而不是硬失败，
		// 避免把非标准但可用的自建平台 ID 拦掉。
		return fmt.Errorf("%w: AppID 通常以 wx 开头，请确认填的是微信开放平台网站应用的 AppID", oauth.ErrProviderConfig)
	}
	u := p.AuthorizeURL(cfg, "probe", "https://example.com/callback")
	if !strings.Contains(u, "appid=") {
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

// 保留 strconv 的显式引用：errcode 在排查日志里需要以十进制展示。
var _ = strconv.Itoa
