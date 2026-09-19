// Package alipay 提供支付宝开放平台的「第三方应用授权登录」适配器（doc104 §6.3）。
//
// 流程：publicAppAuthorize（用户授权）→ alipay.system.oauth.token（code 换 token，
// 返回 user_id）→ alipay.user.info.share（拉昵称头像）。
//
// 与支付/实名共用 pkg/alipay 的网关协议（参数排序拼接 + RSA2 签名 + 原始子串验签）。
// 注意：登录场景的 openid 就是支付宝的 user_id（2088 开头的 16 位数字）。
package alipay

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	alipaypkg "hostsent/backend/internal/pkg/alipay"
	"hostsent/backend/internal/pkg/integration"
	"hostsent/backend/internal/pkg/oauth"
)

// Type 渠道标识。
const Type = "alipay"

const (
	authorizeEndpoint = "https://openauth.alipay.com/oauth2/publicAppAuthorize.htm"
	methodToken       = "alipay.system.oauth.token"
	methodUserInfo    = "alipay.user.info.share"
	nodeToken         = "alipay_system_oauth_token_response"
	nodeUserInfo      = "alipay_user_info_share_response"
)

func init() {
	oauth.RegisterDescriptor(Type, oauth.CapabilityDescriptor{
		Type:           Type,
		Name:           "支付宝",
		Mode:           "api",
		AdapterVersion: "1.0.0",
		Icon:           "alipay",
		DocURL:         "https://opendocs.alipay.com/open/289/105656",
		DefaultScopes:  "auth_user",
		CredentialSchema: []integration.Field{
			{Key: "app_id", Label: "应用 AppID", Type: integration.FieldTypeString, Required: true,
				Placeholder: "支付宝开放平台应用的 AppID"},
			{Key: "private_key", Label: "应用私钥", Type: integration.FieldTypeTextarea, Required: true, Secret: true,
				Help: "PKCS8 或 PKCS1 PEM 格式"},
			{Key: "alipay_public_key", Label: "支付宝公钥", Type: integration.FieldTypeTextarea, Required: true, Secret: true,
				Help: "用于校验支付宝响应签名，与「应用公钥」不是同一把"},
			{Key: "gateway_url", Label: "网关地址", Type: integration.FieldTypeString, Required: false,
				Default: alipaypkg.DefaultGateway, Help: "沙箱环境填 https://openapi.alipaydev.com/gateway.do"},
			{Key: "sign_type", Label: "签名算法", Type: integration.FieldTypeSelect, Required: false, Default: "RSA2",
				Options: []integration.FieldOption{{Label: "RSA2", Value: "RSA2"}, {Label: "RSA", Value: "RSA"}}},
		},
	})
	oauth.RegisterFactory(Type, func(cfg oauth.ProviderConfig) oauth.Provider {
		return &Provider{client: oauth.HTTPClient(0)}
	})
}

// Provider 支付宝登录适配器。
type Provider struct{ client *http.Client }

// Type 返回渠道标识。
func (p *Provider) Type() string { return Type }

// AuthorizeURL 拼装用户授权地址。
func (p *Provider) AuthorizeURL(cfg oauth.ProviderConfig, state, redirectURI string) string {
	q := url.Values{}
	q.Set("app_id", creds(cfg)["app_id"])
	q.Set("scope", orDefault(cfg.Scopes, "auth_user"))
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	return authorizeEndpoint + "?" + q.Encode()
}

// Exchange 用授权码换令牌。
func (p *Provider) Exchange(ctx context.Context, cfg oauth.ProviderConfig, code, _ string) (*oauth.Token, error) {
	key, err := p.key(cfg)
	if err != nil {
		return nil, err
	}
	// alipay.system.oauth.token 用 grant_type=authorization_code 换用户令牌。
	extras := map[string]string{
		"grant_type": "authorization_code",
		"code":       code,
	}
	var resp struct {
		AccessToken  string      `json:"access_token"`
		UserID       string      `json:"user_id"`
		ExpiresIn    json.Number `json:"expires_in"`
		RefreshToken string      `json:"refresh_token"`
		Code         string      `json:"code"`
		Msg          string      `json:"msg"`
		SubCode      string      `json:"sub_code"`
		SubMsg       string      `json:"sub_msg"`
		Error        string      `json:"error"`
	}
	if err := p.call(ctx, key, methodToken, nodeToken, "", extras, &resp); err != nil {
		return nil, err
	}
	if resp.AccessToken == "" || resp.UserID == "" {
		return nil, fmt.Errorf("%w: 支付宝未返回 access_token/user_id（%s %s）", oauth.ErrExchangeFailed, resp.SubCode, resp.SubMsg)
	}
	expires := 0
	if n, err := resp.ExpiresIn.Int64(); err == nil {
		expires = int(n)
	}
	return &oauth.Token{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		OpenID:       resp.UserID,
		ExpiresIn:    expires,
	}, nil
}

// UserInfo 拉取支付宝用户资料。
func (p *Provider) UserInfo(ctx context.Context, cfg oauth.ProviderConfig, token *oauth.Token) (*oauth.ExternalUser, error) {
	key, err := p.key(cfg)
	if err != nil {
		return nil, err
	}
	bizRaw, _ := json.Marshal(map[string]any{"auth_token": token.AccessToken})
	var resp struct {
		UserID   string `json:"user_id"`
		NickName string `json:"nick_name"`
		Avatar   string `json:"avatar"`
		Code     string `json:"code"`
		SubCode  string `json:"sub_code"`
		SubMsg   string `json:"sub_msg"`
	}
	if err := p.call(ctx, key, methodUserInfo, nodeUserInfo, string(bizRaw), nil, &resp); err != nil {
		return nil, err
	}
	openID := resp.UserID
	if openID == "" {
		openID = token.OpenID
	}
	return &oauth.ExternalUser{
		OpenID:   openID,
		Nickname: resp.NickName,
		Avatar:   resp.Avatar,
	}, nil
}

// Test 连通性测试：校验密钥可解析、网关可达（不发起真实授权）。
func (p *Provider) Test(ctx context.Context, cfg oauth.ProviderConfig) error {
	key, err := p.key(cfg)
	if err != nil {
		return err
	}
	var resp struct {
		Code    string `json:"code"`
		Msg     string `json:"msg"`
		SubCode string `json:"sub_code"`
		SubMsg  string `json:"sub_msg"`
	}
	// 用必然业务失败的空 code 探测：只要拿到业务码（而非网络/验签错误）即说明链路通。
	if err := p.call(ctx, key, methodToken, nodeToken, "", map[string]string{"grant_type": "authorization_code", "code": "connectivity-probe"}, &resp); err != nil {
		return err
	}
	if resp.Code == "" && resp.SubCode == "" && resp.Msg == "" {
		return fmt.Errorf("%w: 网关未返回业务码，请核对网关地址", oauth.ErrProviderConfig)
	}
	return nil
}

type alipayKey struct {
	appID      string
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	gateway    string
	signType   string
}

func (p *Provider) key(cfg oauth.ProviderConfig) (*alipayKey, error) {
	c := creds(cfg)
	appID := strings.TrimSpace(c["app_id"])
	privRaw := strings.TrimSpace(c["private_key"])
	pubRaw := strings.TrimSpace(c["alipay_public_key"])
	if appID == "" || privRaw == "" || pubRaw == "" {
		return nil, fmt.Errorf("%w: 应用 AppID / 应用私钥 / 支付宝公钥 均为必填", oauth.ErrProviderConfig)
	}
	priv, err := alipaypkg.ParsePrivateKey(privRaw)
	if err != nil {
		return nil, fmt.Errorf("%w: 应用私钥解析失败", oauth.ErrProviderConfig)
	}
	pub, err := alipaypkg.ParsePublicKey(pubRaw)
	if err != nil {
		return nil, fmt.Errorf("%w: 支付宝公钥解析失败", oauth.ErrProviderConfig)
	}
	gateway := strings.TrimSpace(c["gateway_url"])
	if gateway == "" {
		gateway = strings.TrimSpace(cfg.Endpoint)
	}
	if gateway == "" {
		gateway = alipaypkg.DefaultGateway
	}
	return &alipayKey{appID: appID, privateKey: priv, publicKey: pub, gateway: gateway, signType: c["sign_type"]}, nil
}

// call 发起一次开放平台调用：签名 → POST → 验签 → 解出响应节点。
func (p *Provider) call(ctx context.Context, key *alipayKey, method, node, bizContent string, extras map[string]string, out any) error {
	form, err := alipaypkg.BuildForm(alipaypkg.CommonParams{
		AppID:    key.appID,
		Method:   method,
		SignType: key.signType,
	}, bizContent, key.privateKey, extras)
	if err != nil {
		return fmt.Errorf("%w: %v", oauth.ErrProviderConfig, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, key.gateway, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: 支付宝网关不可达", oauth.ErrProviderConfig)
	}
	defer resp.Body.Close()
	body, err := readLimited(resp.Body)
	if err != nil {
		return fmt.Errorf("%w: 读取支付宝响应失败", oauth.ErrProviderConfig)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: 支付宝网关返回 HTTP %d", oauth.ErrProviderConfig, resp.StatusCode)
	}
	if err := alipaypkg.VerifyResponseSign(body, node, key.publicKey); err != nil {
		return fmt.Errorf("%w: 响应验签未通过", oauth.ErrProviderConfig)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("%w: 支付宝响应不是合法 JSON", oauth.ErrProviderConfig)
	}
	raw, ok := envelope[node]
	if !ok {
		return fmt.Errorf("%w: 支付宝响应缺少节点 %s", oauth.ErrProviderConfig, node)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%w: 解析支付宝响应节点失败", oauth.ErrProviderConfig)
	}
	return nil
}

func readLimited(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, 64*1024))
}

func creds(cfg oauth.ProviderConfig) map[string]string {
	if cfg.Credentials == nil {
		return map[string]string{}
	}
	return cfg.Credentials
}

func orDefault(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
