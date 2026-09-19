package alipay

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/url"
	"strings"
	"testing"
)

// 生成一对 RSA 测试密钥（PKCS8 私钥 / PKIX 公钥 PEM），避免依赖测试夹具文件。
func testKeyPair(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey, string, string) {
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
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER}))
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))
	return priv, &priv.PublicKey, privPEM, pubPEM
}

// signNode 按支付宝规则给「响应节点原文」签名，供构造 mock 响应用。
func signNode(t *testing.T, priv *rsa.PrivateKey, nodeRaw string) string {
	t.Helper()
	digest := sha256.Sum256([]byte(nodeRaw))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	return base64.StdEncoding.EncodeToString(sig)
}

func TestParsePrivateKey(t *testing.T) {
	priv, _, privPEM, _ := testKeyPair(t)

	parsed, err := ParsePrivateKey(privPEM)
	if err != nil {
		t.Fatalf("解析 PKCS8 私钥失败: %v", err)
	}
	if parsed.N.Cmp(priv.N) != 0 {
		t.Fatal("解析出的私钥与原文不一致")
	}

	// PKCS1 编码同样要能解析（部分工具导出的是这种）。
	pkcs1PEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	}))
	if _, err := ParsePrivateKey(pkcs1PEM); err != nil {
		t.Fatalf("解析 PKCS1 私钥失败: %v", err)
	}

	if _, err := ParsePrivateKey("not-a-pem"); err == nil {
		t.Fatal("非 PEM 内容必须报错")
	}
}

func TestParsePublicKey(t *testing.T) {
	_, pub, _, pubPEM := testKeyPair(t)
	parsed, err := ParsePublicKey(pubPEM)
	if err != nil {
		t.Fatalf("解析 PKIX 公钥失败: %v", err)
	}
	if parsed.N.Cmp(pub.N) != 0 {
		t.Fatal("解析出的公钥与原文不一致")
	}
	if _, err := ParsePublicKey("-----BEGIN PUBLIC KEY-----\nzzz\n-----END PUBLIC KEY-----"); err == nil {
		t.Fatal("非法 base64 必须报错")
	}
}

// 单行 PEM（控制台复制粘贴常见）必须能自动规整——否则运营会反复排查「格式明明对」。
func TestParsePrivateKey_SingleLinePEM(t *testing.T) {
	_, _, privPEM, _ := testKeyPair(t)
	oneLine := strings.ReplaceAll(strings.TrimSpace(privPEM), "\n", "")
	if _, err := ParsePrivateKey(oneLine); err != nil {
		t.Fatalf("单行 PEM 应能被规整后解析: %v", err)
	}
}

// 待签名串：剔除 sign/sign_type 与空值，按 key 升序，& 连接。
// 这三个细节错了签名必然失败，但线上只会看到「验签失败」，所以必须直接断言。
func TestSignContent(t *testing.T) {
	got := SignContent(map[string]string{
		"b":         "2",
		"a":         "1",
		"sign":      "should-be-excluded",
		"sign_type": "RSA2",
		"empty":     "",
		"blank":     "   ",
	})
	if got != "a=1&b=2" {
		t.Fatalf("待签名串应为 a=1&b=2，实际 %q", got)
	}
}

// 响应验签必须用「节点在原始报文中的子串」而不是重新序列化的 JSON：
// 重新序列化会改键序与转义，导致验签恒失败（支付宝接入最常见的坑）。
func TestVerifyResponseSign_UsesRawSubstring(t *testing.T) {
	priv, pub, _, _ := testKeyPair(t)
	const node = "alipay_user_certify_open_query_response"
	// 故意让键序非字母序、并带转义字符，验证不是靠重新序列化。
	nodeRaw := `{"passed":"T","sub_msg":"含\"引号\"与空格 的中文","code":"10000"}`
	body := []byte(`{"` + node + `":` + nodeRaw + `,"sign":"` + signNode(t, priv, nodeRaw) + `"}`)

	if err := VerifyResponseSign(body, node, pub); err != nil {
		t.Fatalf("原始报文验签应通过: %v", err)
	}

	// 篡改节点内容后必须验签失败。
	tampered := []byte(strings.Replace(string(body), `"passed":"T"`, `"passed":"F"`, 1))
	if err := VerifyResponseSign(tampered, node, pub); err == nil {
		t.Fatal("报文被篡改后必须验签失败")
	}

	// 缺 sign 字段必须报错，不能静默放过。
	noSign := []byte(`{"` + node + `":` + nodeRaw + `}`)
	if err := VerifyResponseSign(noSign, node, pub); err == nil {
		t.Fatal("响应缺少 sign 必须报错")
	}

	// 节点不存在必须报错。
	if err := VerifyResponseSign(body, "no_such_node", pub); err == nil {
		t.Fatal("节点不存在必须报错")
	}
}

// 花括号配对要跳过字符串字面量内的括号，否则含 JSON 片段的字段会把截取提前截断。
func TestExtractNodeRaw_NestedBracesAndStrings(t *testing.T) {
	raw := []byte(`{"node":{"a":{"b":1},"c":"}{ not a brace }","d":[{"e":2}]},"sign":"x"}`)
	got, err := extractNodeRaw(raw, "node")
	if err != nil {
		t.Fatalf("截取失败: %v", err)
	}
	want := `{"a":{"b":1},"c":"}{ not a brace }","d":[{"e":2}]}`
	if got != want {
		t.Fatalf("截取结果不符\nwant: %s\n got: %s", want, got)
	}
	if !json.Valid([]byte(got)) {
		t.Fatal("截取结果应为合法 JSON")
	}
}

// BuildForm 必须带上全部公共参数与 sign，且 biz_content 原样透传。
func TestBuildForm(t *testing.T) {
	priv, pub, _, _ := testKeyPair(t)
	biz := `{"certify_id":"c-1"}`

	form, err := BuildForm(CommonParams{
		AppID:     "2021000000000000",
		Method:    "alipay.user.certify.open.query",
		NotifyURL: "https://example.com/notify",
	}, biz, priv, nil)
	if err != nil {
		t.Fatalf("构造表单失败: %v", err)
	}
	for _, key := range []string{"app_id", "method", "format", "charset", "sign_type", "timestamp", "version", "biz_content", "notify_url", "sign"} {
		if form.Get(key) == "" {
			t.Fatalf("表单缺少参数 %s", key)
		}
	}
	if form.Get("biz_content") != biz {
		t.Fatalf("biz_content 应原样透传，实际 %q", form.Get("biz_content"))
	}
	if form.Get("sign_type") != "RSA2" {
		t.Fatalf("sign_type 默认应为 RSA2，实际 %q", form.Get("sign_type"))
	}

	// 用表单里除 sign 外的参数重算签名，必须与表单里的 sign 一致。
	params := map[string]string{}
	for k, v := range form {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	sign := params["sign"]
	delete(params, "sign")
	want, err := Sign(params, priv)
	if err != nil {
		t.Fatalf("重算签名失败: %v", err)
	}
	if sign != want {
		t.Fatal("表单里的 sign 与按规则重算的结果不一致")
	}
	if _, err := url.ParseQuery(form.Encode()); err != nil {
		t.Fatalf("表单应能编码为合法 query: %v", err)
	}
	_ = pub
}

// sign_type=RSA 时必须用 SHA1，不能悄悄回落到 SHA256（否则支付宝验签必失败）。
func TestSignContent_ExcludesSignType(t *testing.T) {
	got := SignContent(map[string]string{"sign_type": "RSA2", "method": "x"})
	if strings.Contains(got, "sign_type") {
		t.Fatalf("sign_type 必须参与排序前被剔除，实际 %q", got)
	}
}
