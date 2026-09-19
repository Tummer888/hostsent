package alipay

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	alipaypkg "hostsent/backend/internal/pkg/alipay"
	"hostsent/backend/internal/pkg/realname"
)

// 生成一对 RSA 测试密钥，避免依赖测试夹具文件。
// 与 pkg/alipay 的测试用同一手法，但这里必须自建：跨包无法复用未导出的辅助函数。
func testKeys(t *testing.T) (priv *rsa.PrivateKey, privPEM, pubPEM string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("序列化私钥失败: %v", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("序列化公钥失败: %v", err)
	}
	privPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER}))
	pubPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))
	return priv, privPEM, pubPEM
}

// mockGateway 起一个支付宝网关桩：按 method 返回节点原文，并用 priv 对节点签名。
//
// 返回的 *httptest.Server 关闭由 t.Cleanup 负责；lastForm 让用例断言请求参数
// （例如 biz_content 里是否真的带了明文证件号、outer_order_no 是否稳定）。
func mockGateway(t *testing.T, priv *rsa.PrivateKey, nodes map[string]string) (*httptest.Server, *map[string]string) {
	t.Helper()
	last := map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, err := parseForm(string(body))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		for k, v := range form {
			last[k] = v
		}
		method := form["method"]
		nodeName := nodeFor(method)
		nodeRaw, ok := nodes[method]
		if !ok {
			// 未登记的 method 也返回一个合法节点，便于断言「未知方法被当业务错误」。
			nodeRaw = `{"code":"40004","msg":"Business Failed","sub_code":"isv.invalid-method","sub_msg":"未知方法"}`
		}
		sig := signNode(t, priv, nodeRaw)
		w.Header().Set("Content-Type", "application/json;charset=utf-8")
		_, _ = w.Write([]byte(`{"` + nodeName + `":` + nodeRaw + `,"sign":"` + sig + `"}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &last
}

func nodeFor(method string) string {
	switch method {
	case methodInitialize:
		return nodeInitialize
	case methodQuery:
		return nodeQuery
	default:
		// certify 是跳转式接口，本适配器只拼 URL 不调用；其余归到 query 节点。
		return nodeQuery
	}
}

func signNode(t *testing.T, priv *rsa.PrivateKey, nodeRaw string) string {
	t.Helper()
	digest := sha256.Sum256([]byte(nodeRaw))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	return base64.StdEncoding.EncodeToString(sig)
}

// parseForm 解析 x-www-form-urlencoded 请求体。
func parseForm(body string) (map[string]string, error) {
	out := map[string]string{}
	for _, pair := range strings.Split(body, "&") {
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, "=", 2)
		k, err := urlQueryUnescape(kv[0])
		if err != nil {
			return nil, err
		}
		v := ""
		if len(kv) == 2 {
			v, err = urlQueryUnescape(kv[1])
			if err != nil {
				return nil, err
			}
		}
		out[k] = v
	}
	return out, nil
}

func urlQueryUnescape(s string) (string, error) {
	s = strings.ReplaceAll(s, "+", " ")
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			b.WriteByte(s[i])
			continue
		}
		if i+2 >= len(s) {
			return "", errors.New("非法百分号转义")
		}
		var v byte
		for j := 1; j <= 2; j++ {
			c := s[i+j]
			switch {
			case c >= '0' && c <= '9':
				v = v<<4 | (c - '0')
			case c >= 'a' && c <= 'f':
				v = v<<4 | (c - 'a' + 10)
			case c >= 'A' && c <= 'F':
				v = v<<4 | (c - 'A' + 10)
			default:
				return "", errors.New("非法百分号转义")
			}
		}
		b.WriteByte(v)
		i += 2
	}
	return b.String(), nil
}

// 描述符必须声明 redirect 形态：支付宝没有无跳转核验接口，
// 配置页要据此给出「跳转式」说明，而不是让运营以为配错了。
func TestDescriptor_IsRedirectMode(t *testing.T) {
	d, ok := realname.Descriptor(Type)
	if !ok {
		t.Fatal("alipay 描述符未登记")
	}
	if d.Mode != "redirect" {
		t.Fatalf("Mode 应为 redirect，实际 %q", d.Mode)
	}
	// Descriptor() 返回的是登记时的快照，Implemented 恒为 false；
	// 后台看到的状态必须取自 AllDescriptors()（运行时按工厂注册计算）。
	// 这个区别曾导致配置页把「已接入的适配器」显示成未接入。
	if d.Implemented {
		t.Fatal("Descriptor() 返回的快照里 Implemented 应为 false（由 AllDescriptors 计算）")
	}
	var found bool
	for _, item := range realname.AllDescriptors() {
		if item.Type != Type {
			continue
		}
		found = true
		if !item.Implemented {
			t.Fatal("工厂已注册时 AllDescriptors 应把 Implemented 算成 true")
		}
	}
	if !found {
		t.Fatal("AllDescriptors 里应包含 alipay")
	}
	for _, f := range d.CredentialSchema {
		if f.Key == "app_id" || f.Key == "private_key" || f.Key == "alipay_public_key" {
			if !f.Required {
				t.Fatalf("凭证字段 %s 必须为必填", f.Key)
			}
		}
		if f.Key == "private_key" || f.Key == "alipay_public_key" {
			if !f.Secret {
				t.Fatalf("凭证字段 %s 必须标记为 secret", f.Key)
			}
		}
	}
}

// DirectVerify 必须明确返回 ErrAdapterNotImplemented，服务层据此回落人工审核。
// 若这里返回一个「失败」结果，用户提交会被误判为核验不通过而直接驳回。
func TestDirectVerify_ReturnsNotImplemented(t *testing.T) {
	p, err := realname.New(Type, realname.ProviderConfig{})
	if err != nil {
		t.Fatalf("构造 provider 失败: %v", err)
	}
	_, err = p.DirectVerify(context.Background(), realname.ProviderConfig{}, realname.VerifyRequest{})
	if !errors.Is(err, realname.ErrAdapterNotImplemented) {
		t.Fatalf("DirectVerify 应返回 ErrAdapterNotImplemented，实际 %v", err)
	}
}

// Initialize：先 initialize 拿 certify_id，再签出 certify 跳转地址。
func TestInitialize_BuildsSignedCertifyURL(t *testing.T) {
	priv, privPEM, pubPEM := testKeys(t)
	srv, lastForm := mockGateway(t, priv, map[string]string{
		methodInitialize: `{"code":"10000","msg":"Success","certify_id":"cert-abc"}`,
	})
	p := &Provider{cfg: realname.ProviderConfig{}, client: srv.Client()}

	challenge, err := p.Initialize(context.Background(), realname.ProviderConfig{
		Credentials: map[string]string{
			"app_id":            "2021000000000000",
			"private_key":       privPEM,
			"alipay_public_key": pubPEM,
			"gateway_url":       srv.URL,
		},
	}, realname.VerifyRequest{
		ApplicationID: 77,
		RealName:      "张三",
		IDNumber:      "110101199001011234",
		CertifyMode:   "FACE",
	})
	if err != nil {
		t.Fatalf("Initialize 失败: %v", err)
	}
	if challenge.TxnNo != "cert-abc" {
		t.Fatalf("TxnNo 应为 certify_id，实际 %q", challenge.TxnNo)
	}
	if !strings.HasPrefix(challenge.AuthURL, srv.URL+"?") {
		t.Fatalf("AuthURL 应指向配置的网关，实际 %q", challenge.AuthURL)
	}

	// 跳转地址必须是**带签名的** certify 请求：参数齐全且签名可被公钥验证。
	query := challenge.AuthURL[strings.Index(challenge.AuthURL, "?")+1:]
	form, err := parseForm(query)
	if err != nil {
		t.Fatalf("解析跳转参数失败: %v", err)
	}
	if form["method"] != methodCertify {
		t.Fatalf("跳转 method 应为 %s，实际 %q", methodCertify, form["method"])
	}
	sign := form["sign"]
	if sign == "" {
		t.Fatal("跳转地址缺少 sign")
	}
	var biz struct {
		CertifyID string `json:"certify_id"`
	}
	if err := json.Unmarshal([]byte(form["biz_content"]), &biz); err != nil {
		t.Fatalf("biz_content 不是合法 JSON: %v", err)
	}
	if biz.CertifyID != "cert-abc" {
		t.Fatalf("biz_content.certify_id 应为 cert-abc，实际 %q", biz.CertifyID)
	}

	// 用配置的公钥验证跳转地址签名（跳转签名与响应验签同一套规则）。
	// parseForm 已把表单转义还原（%2B → +），可直接按 base64 解。
	delete(form, "sign")
	digest := sha256.Sum256([]byte(alipaypkg.SignContent(form)))
	sig, err := base64.StdEncoding.DecodeString(sign)
	if err != nil {
		t.Fatalf("跳转 sign 不是合法 base64: %v", err)
	}
	if err := rsa.VerifyPKCS1v15(mustPublic(t, pubPEM), crypto.SHA256, digest[:], sig); err != nil {
		t.Fatalf("跳转地址签名验证失败: %v", err)
	}

	// initialize 的请求体必须带明文姓名与证件号（三方接口不收脱敏值），
	// 且 outer_order_no 以申请 ID 开头，便于按申请排查。
	initBiz := (*lastForm)["biz_content"]
	if !strings.Contains(initBiz, "张三") || !strings.Contains(initBiz, "110101199001011234") {
		t.Fatalf("initialize 的 biz_content 应含明文姓名与证件号，实际 %q", initBiz)
	}
	if !strings.Contains(initBiz, `"outer_order_no":"va-77-`) {
		t.Fatalf("outer_order_no 应以 va-<申请ID>- 开头，实际 %q", initBiz)
	}
}

func mustPublic(t *testing.T, pemStr string) *rsa.PublicKey {
	t.Helper()
	pub, err := alipaypkg.ParsePublicKey(pemStr)
	if err != nil {
		t.Fatalf("解析公钥失败: %v", err)
	}
	return pub
}

// Initialize 缺少姓名/证件号时必须报配置类错误，且**不发请求**。
func TestInitialize_RequiresRealNameAndIDNumber(t *testing.T) {
	priv, privPEM, pubPEM := testKeys(t)
	srv, _ := mockGateway(t, priv, nil)
	p := &Provider{client: srv.Client()}
	cfg := realname.ProviderConfig{Credentials: map[string]string{
		"app_id": "1", "private_key": privPEM, "alipay_public_key": pubPEM, "gateway_url": srv.URL,
	}}
	if _, err := p.Initialize(context.Background(), cfg, realname.VerifyRequest{RealName: "张三"}); !errors.Is(err, realname.ErrProviderConfig) {
		t.Fatalf("缺证件号应返回 ErrProviderConfig，实际 %v", err)
	}
	if _, err := p.Initialize(context.Background(), cfg, realname.VerifyRequest{IDNumber: "110101199001011234"}); !errors.Is(err, realname.ErrProviderConfig) {
		t.Fatalf("缺姓名应返回 ErrProviderConfig，实际 %v", err)
	}
}

// initialize 未返回 certify_id 时必须报错：拿不到 certify_id 就没法拼跳转地址，
// 静默返回空 URL 会让用户点到一个空白页。
func TestInitialize_MissingCertifyIDIsError(t *testing.T) {
	priv, privPEM, pubPEM := testKeys(t)
	srv, _ := mockGateway(t, priv, map[string]string{
		methodInitialize: `{"code":"40004","msg":"Business Failed","sub_code":"isv.invalid-param","sub_msg":"参数错误"}`,
	})
	p := &Provider{client: srv.Client()}
	_, err := p.Initialize(context.Background(), realname.ProviderConfig{Credentials: map[string]string{
		"app_id": "1", "private_key": privPEM, "alipay_public_key": pubPEM, "gateway_url": srv.URL,
	}}, realname.VerifyRequest{RealName: "张三", IDNumber: "110101199001011234"})
	if !errors.Is(err, realname.ErrProviderConfig) {
		t.Fatalf("缺 certify_id 应返回 ErrProviderConfig，实际 %v", err)
	}
	if !strings.Contains(err.Error(), "isv.invalid-param") {
		t.Fatalf("错误应带上支付宝的业务子码便于排查，实际 %v", err)
	}
}

// Query 的三种 passed 取值：T 通过 / F 未通过 / 空 = 用户还没做完。
// 空值绝不能当「失败」，否则用户刚发起认证就被系统驳回。
func TestQuery_PassedSemantics(t *testing.T) {
	cases := []struct {
		name        string
		node        string
		wantPassed  bool
		wantBizCode string
	}{
		{"T 通过", `{"code":"10000","msg":"Success","passed":"T"}`, true, ""},
		{"F 未通过", `{"code":"10000","msg":"Success","passed":"F"}`, false, ""},
		{"空值表示未完成", `{"code":"10000","msg":"Success","passed":""}`, false, "PENDING"},
		{"缺 passed 字段同样按未完成", `{"code":"10000","msg":"Success"}`, false, "PENDING"},
		{"小写 t 也要认", `{"code":"10000","msg":"Success","passed":"t"}`, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			priv, privPEM, pubPEM := testKeys(t)
			srv, _ := mockGateway(t, priv, map[string]string{methodQuery: tc.node})
			p := &Provider{client: srv.Client()}
			res, err := p.Query(context.Background(), realname.ProviderConfig{Credentials: map[string]string{
				"app_id": "1", "private_key": privPEM, "alipay_public_key": pubPEM, "gateway_url": srv.URL,
			}}, "cert-abc")
			if err != nil {
				t.Fatalf("Query 失败: %v", err)
			}
			if res.Passed != tc.wantPassed {
				t.Fatalf("Passed 应为 %v，实际 %v", tc.wantPassed, res.Passed)
			}
			if tc.wantBizCode != "" && res.BizCode != tc.wantBizCode {
				t.Fatalf("BizCode 应为 %s，实际 %q", tc.wantBizCode, res.BizCode)
			}
			if res.TxnNo != "cert-abc" {
				t.Fatalf("TxnNo 应原样回填，实际 %q", res.TxnNo)
			}
			if res.Message == "" {
				t.Fatal("Message 必须有值供前端展示")
			}
		})
	}
}

// 响应验签失败必须整体报错：不能因为「报文看着像通过」就采信。
func TestQuery_RejectsTamperedResponse(t *testing.T) {
	_, privPEM, pubPEM := testKeys(t)
	// 用一个「别的私钥」签名，模拟篡改/伪造。
	otherPriv, _, _ := testKeys(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nodeRaw := `{"code":"10000","msg":"Success","passed":"T"}`
		sig := signNode(t, otherPriv, nodeRaw)
		_, _ = w.Write([]byte(`{"` + nodeQuery + `":` + nodeRaw + `,"sign":"` + sig + `"}`))
	}))
	t.Cleanup(srv.Close)

	p := &Provider{client: srv.Client()}
	_, err := p.Query(context.Background(), realname.ProviderConfig{Credentials: map[string]string{
		"app_id": "1", "private_key": privPEM, "alipay_public_key": pubPEM, "gateway_url": srv.URL,
	}}, "cert-abc")
	if !errors.Is(err, realname.ErrProviderConfig) {
		t.Fatalf("验签失败应返回 ErrProviderConfig，实际 %v", err)
	}
	if !strings.Contains(err.Error(), "验签") {
		t.Fatalf("错误信息应指明验签失败，实际 %v", err)
	}
}

// 网关返回非 200 必须报错，且错误里带状态码。
func TestCall_Non200IsError(t *testing.T) {
	priv, privPEM, pubPEM := testKeys(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	p := &Provider{client: srv.Client()}
	_, err := p.Query(context.Background(), realname.ProviderConfig{Credentials: map[string]string{
		"app_id": "1", "private_key": privPEM, "alipay_public_key": pubPEM, "gateway_url": srv.URL,
	}}, "cert-abc")
	if !errors.Is(err, realname.ErrProviderConfig) {
		t.Fatalf("非 200 应返回 ErrProviderConfig，实际 %v", err)
	}
	if !strings.Contains(err.Error(), "502") {
		t.Fatalf("错误应带上 HTTP 状态码，实际 %v", err)
	}
	_ = priv
}

// 凭证不全 / 密钥非法必须在**发请求之前**失败，错误信息要能指到具体字段。
func TestCredentials_Validation(t *testing.T) {
	_, privPEM, pubPEM := testKeys(t)
	p := &Provider{}
	cases := []struct {
		name  string
		creds map[string]string
		want  string
	}{
		{"缺 app_id", map[string]string{"private_key": privPEM, "alipay_public_key": pubPEM}, "必填"},
		{"缺私钥", map[string]string{"app_id": "1", "alipay_public_key": pubPEM}, "必填"},
		{"缺公钥", map[string]string{"app_id": "1", "private_key": privPEM}, "必填"},
		{"私钥非法", map[string]string{"app_id": "1", "private_key": "garbage", "alipay_public_key": pubPEM}, "私钥"},
		{"公钥非法", map[string]string{"app_id": "1", "private_key": privPEM, "alipay_public_key": "garbage"}, "公钥"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := p.Test(context.Background(), realname.ProviderConfig{Credentials: tc.creds})
			if !errors.Is(err, realname.ErrProviderConfig) {
				t.Fatalf("应返回 ErrProviderConfig，实际 %v", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("错误信息应包含 %q，实际 %v", tc.want, err)
			}
		})
	}
}

// 网关地址缺省时回落：凭证没填 → Endpoint → 官方默认网关。
func TestCredentials_GatewayFallback(t *testing.T) {
	_, privPEM, pubPEM := testKeys(t)
	p := &Provider{}
	key, err := p.credentials(realname.ProviderConfig{
		Endpoint: "https://endpoint.example/gateway.do",
		Credentials: map[string]string{
			"app_id": "1", "private_key": privPEM, "alipay_public_key": pubPEM,
		},
	})
	if err != nil {
		t.Fatalf("解析凭证失败: %v", err)
	}
	if key.gateway != "https://endpoint.example/gateway.do" {
		t.Fatalf("应回落到 Endpoint，实际 %q", key.gateway)
	}

	key, err = p.credentials(realname.ProviderConfig{Credentials: map[string]string{
		"app_id": "1", "private_key": privPEM, "alipay_public_key": pubPEM,
	}})
	if err != nil {
		t.Fatalf("解析凭证失败: %v", err)
	}
	if key.gateway != alipaypkg.DefaultGateway {
		t.Fatalf("应回落到官方网关，实际 %q", key.gateway)
	}
}

// Test 只做「凭证可解析 + 网关有业务码」的连通性判断，不发真实核验。
func TestTest_SucceedsOnBusinessCode(t *testing.T) {
	priv, privPEM, pubPEM := testKeys(t)
	srv, lastForm := mockGateway(t, priv, map[string]string{
		methodQuery: `{"code":"40004","msg":"Business Failed","sub_code":"isv.invalid-certify-id","sub_msg":"certify_id 无效"}`,
	})
	p := &Provider{client: srv.Client()}
	if err := p.Test(context.Background(), realname.ProviderConfig{Credentials: map[string]string{
		"app_id": "1", "private_key": privPEM, "alipay_public_key": pubPEM, "gateway_url": srv.URL,
	}}); err != nil {
		t.Fatalf("有业务码即视为连通，实际报错 %v", err)
	}
	if !strings.Contains((*lastForm)["biz_content"], "connectivity-probe") {
		t.Fatalf("连通性探测应使用固定的占位 certify_id，实际 %q", (*lastForm)["biz_content"])
	}
}

// 网关返回 200 但既无 code 也无 sub_code（例如被反代成 HTML）必须判失败，
// 否则运营会以为「配置正确」而实际一个请求都发不出去。
func TestTest_NoBusinessCodeIsError(t *testing.T) {
	priv, privPEM, pubPEM := testKeys(t)
	srv, _ := mockGateway(t, priv, map[string]string{methodQuery: `{}`})
	p := &Provider{client: srv.Client()}
	err := p.Test(context.Background(), realname.ProviderConfig{Credentials: map[string]string{
		"app_id": "1", "private_key": privPEM, "alipay_public_key": pubPEM, "gateway_url": srv.URL,
	}})
	if !errors.Is(err, realname.ErrProviderConfig) {
		t.Fatalf("无业务码应返回 ErrProviderConfig，实际 %v", err)
	}
}

// 适配器必须实现 Initializer，否则服务层无法为跳转式核验生成入口。
func TestProvider_ImplementsInitializer(t *testing.T) {
	p, err := realname.New(Type, realname.ProviderConfig{})
	if err != nil {
		t.Fatalf("构造 provider 失败: %v", err)
	}
	if _, ok := p.(realname.Initializer); !ok {
		t.Fatal("支付宝适配器必须实现 realname.Initializer")
	}
	if p.Type() != Type {
		t.Fatalf("Type() 应为 %s，实际 %s", Type, p.Type())
	}
}
