package alipay

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	alipaypkg "hostsent/backend/internal/pkg/alipay"
	"hostsent/backend/internal/pkg/oauth"
)

// 生成测试密钥对：私钥用于「平台签名」（冒充支付宝），公钥填进凭证里供适配器验签。
func oauthTestKeys(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("序列化公钥失败: %v", err)
	}
	return priv, string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))
}

// signNodeRaw 对响应节点原文签名（与 alipaypkg.VerifyResponseSign 的口径一致）。
func signNodeRaw(t *testing.T, priv *rsa.PrivateKey, nodeRaw string) string {
	t.Helper()
	digest := sha256.Sum256([]byte(nodeRaw))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	return base64.StdEncoding.EncodeToString(sig)
}

// mockGateway 起一个假网关：校验请求是合法的签名表单，然后返回带签名的响应节点。
func mockGateway(t *testing.T, priv *rsa.PrivateKey, node, nodeRaw string, assertBiz func(t *testing.T, biz string)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Errorf("请求体不是合法表单: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if form.Get("app_id") == "" || form.Get("method") == "" || form.Get("sign") == "" {
			t.Errorf("请求缺少公共参数: %v", form)
		}
		if assertBiz != nil {
			assertBiz(t, form.Get("biz_content"))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"%s":%s,"sign":"%s"}`, node, nodeRaw, signNodeRaw(t, priv, nodeRaw))
	}))
}

func TestProviderType(t *testing.T) {
	p, err := oauth.New(Type, oauth.ProviderConfig{Type: Type})
	if err != nil {
		t.Fatalf("构造 provider 失败: %v", err)
	}
	if p.Type() != Type {
		t.Fatalf("Type 应为 %s", Type)
	}
}

// 授权地址必须带 app_id / redirect_uri / state，scope 默认 auth_user。
func TestAuthorizeURL(t *testing.T) {
	p, _ := oauth.New(Type, oauth.ProviderConfig{Type: Type})
	raw := p.AuthorizeURL(oauth.ProviderConfig{
		Type:        Type,
		Credentials: map[string]string{"app_id": "2021000000000000"},
	}, "state-token", "https://example.com/api/v1/uc/oauth/alipay/callback")

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("授权地址不是合法 URL: %v", err)
	}
	q := u.Query()
	if q.Get("app_id") != "2021000000000000" {
		t.Fatalf("app_id 未带上: %s", raw)
	}
	if q.Get("state") != "state-token" {
		t.Fatalf("state 未带上: %s", raw)
	}
	if q.Get("scope") != "auth_user" {
		t.Fatalf("scope 默认应为 auth_user，实际 %q", q.Get("scope"))
	}
	if !strings.Contains(q.Get("redirect_uri"), "/uc/oauth/alipay/callback") {
		t.Fatalf("redirect_uri 未带上: %s", raw)
	}
}

// 授权码换令牌：走 mock 网关，返回带合法签名的响应。
func TestExchange_Success(t *testing.T) {
	priv, pubPEM := oauthTestKeys(t)
	const node = "alipay_system_oauth_token_response"
	nodeRaw := `{"access_token":"AT-1","user_id":"2088000000000001","expires_in":3600,"refresh_token":"RT-1","re_expires_in":7200}`

	srv := mockGateway(t, priv, node, nodeRaw, func(t *testing.T, biz string) {
		if biz != "" {
			t.Errorf("token 调用不应带 biz_content，实际 %q", biz)
		}
	})
	defer srv.Close()

	p, _ := oauth.New(Type, oauth.ProviderConfig{Type: Type})
	token, err := p.Exchange(context.Background(), oauth.ProviderConfig{
		Type: Type,
		Credentials: map[string]string{
			"app_id": "2021000000000000", "private_key": testPrivPEM(t), "alipay_public_key": pubPEM,
			"gateway_url": srv.URL,
		},
	}, "auth-code-1", "")
	if err != nil {
		t.Fatalf("换令牌失败: %v", err)
	}
	if token.AccessToken != "AT-1" {
		t.Fatalf("access_token 解析错误: %q", token.AccessToken)
	}
	// 登录场景的 openid 就是支付宝 user_id（2088 开头的 16 位数字）。
	if token.OpenID != "2088000000000001" {
		t.Fatalf("openid 应取 user_id，实际 %q", token.OpenID)
	}
	if token.ExpiresIn != 3600 {
		t.Fatalf("expires_in 解析错误: %d", token.ExpiresIn)
	}
}

// 响应验签失败必须报错——这是「响应确实来自支付宝」的唯一保证。
func TestExchange_RejectsUnsignedResponse(t *testing.T) {
	_, pubPEM := oauthTestKeys(t)
	// 用另一把私钥签名：公钥对不上，验签必须失败。
	otherPriv, _ := oauthTestKeys(t)
	const node = "alipay_system_oauth_token_response"
	nodeRaw := `{"access_token":"AT-1","user_id":"2088000000000001"}`

	srv := mockGateway(t, otherPriv, node, nodeRaw, nil)
	defer srv.Close()

	p, _ := oauth.New(Type, oauth.ProviderConfig{Type: Type})
	_, err := p.Exchange(context.Background(), oauth.ProviderConfig{
		Type: Type,
		Credentials: map[string]string{
			"app_id": "2021000000000000", "private_key": testPrivPEM(t), "alipay_public_key": pubPEM,
			"gateway_url": srv.URL,
		},
	}, "code", "")
	if err == nil {
		t.Fatal("验签失败必须报错")
	}
	if !strings.Contains(err.Error(), "验签") {
		t.Fatalf("错误信息应指明验签问题，实际: %v", err)
	}
}

// 拉取用户资料：biz_content 必须带 auth_token。
func TestUserInfo_Success(t *testing.T) {
	priv, pubPEM := oauthTestKeys(t)
	const node = "alipay_user_info_share_response"
	nodeRaw := `{"code":"10000","msg":"Success","user_id":"2088000000000001","nick_name":"张三","avatar":"https://example.com/a.png"}`

	srv := mockGateway(t, priv, node, nodeRaw, func(t *testing.T, biz string) {
		if !strings.Contains(biz, "auth_token") {
			t.Errorf("userinfo 的 biz_content 应带 auth_token，实际 %q", biz)
		}
	})
	defer srv.Close()

	p, _ := oauth.New(Type, oauth.ProviderConfig{Type: Type})
	user, err := p.UserInfo(context.Background(), oauth.ProviderConfig{
		Type: Type,
		Credentials: map[string]string{
			"app_id": "2021000000000000", "private_key": testPrivPEM(t), "alipay_public_key": pubPEM,
			"gateway_url": srv.URL,
		},
	}, &oauth.Token{AccessToken: "AT-1", OpenID: "2088000000000001"})
	if err != nil {
		t.Fatalf("拉取资料失败: %v", err)
	}
	if user.Nickname != "张三" {
		t.Fatalf("昵称解析错误: %q", user.Nickname)
	}
	if user.OpenID != "2088000000000001" {
		t.Fatalf("openid 应回落 user_id，实际 %q", user.OpenID)
	}
}

// 凭证不全时必须明确报配置错误，不能发出请求（否则会把缺配置当成上游故障）。
func TestExchange_MissingCredentials(t *testing.T) {
	p, _ := oauth.New(Type, oauth.ProviderConfig{Type: Type})
	_, err := p.Exchange(context.Background(), oauth.ProviderConfig{
		Type:        Type,
		Credentials: map[string]string{"app_id": "2021000000000000"},
	}, "code", "")
	if err == nil {
		t.Fatal("缺私钥/公钥必须报错")
	}
	if !strings.Contains(err.Error(), "必填") {
		t.Fatalf("错误信息应说明缺哪些凭证，实际: %v", err)
	}
}

// 网关不可达时返回配置错误摘要，绝不把上游原始报文回显。
func TestExchange_GatewayUnreachable(t *testing.T) {
	_, pubPEM := oauthTestKeys(t)
	p, _ := oauth.New(Type, oauth.ProviderConfig{Type: Type})
	_, err := p.Exchange(context.Background(), oauth.ProviderConfig{
		Type: Type,
		Credentials: map[string]string{
			"app_id": "2021000000000000", "private_key": testPrivPEM(t), "alipay_public_key": pubPEM,
			// 保留端口不可达的地址。
			"gateway_url": "http://127.0.0.1:1/gateway.do",
		},
	}, "code", "")
	if err == nil {
		t.Fatal("网关不可达必须报错")
	}
	if !strings.Contains(err.Error(), "不可达") {
		t.Fatalf("错误信息应为「网关不可达」摘要，实际: %v", err)
	}
}

// Test 连通性：拿到业务码即算通（用假 code 探测，测的是「凭证配得对不对」）。
func TestProvider_Test(t *testing.T) {
	priv, pubPEM := oauthTestKeys(t)
	const node = "alipay_system_oauth_token_response"
	nodeRaw := `{"code":"40002","msg":"Invalid Arguments","sub_code":"isv.code-invalid"}`

	srv := mockGateway(t, priv, node, nodeRaw, nil)
	defer srv.Close()

	p, _ := oauth.New(Type, oauth.ProviderConfig{Type: Type})
	if err := p.Test(context.Background(), oauth.ProviderConfig{
		Type: Type,
		Credentials: map[string]string{
			"app_id": "2021000000000000", "private_key": testPrivPEM(t), "alipay_public_key": pubPEM,
			"gateway_url": srv.URL,
		},
	}); err != nil {
		t.Fatalf("拿到业务码时应判定连通: %v", err)
	}
}

// testPrivPEM 生成一把 PKCS8 私钥 PEM，供凭证使用（内容无需与 mock 网关的公钥配对：
// 私钥只用于平台侧签名，验签用的是响应里的 alipay_public_key）。
func testPrivPEM(t *testing.T) string {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成私钥失败: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("序列化私钥失败: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

// 确保 alipaypkg 被引用（签名口径与适配器共用同一实现，测试里显式锚定）。
var _ = alipaypkg.DefaultGateway
