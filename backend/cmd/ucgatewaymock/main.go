// Command ucgatewaymock 是 scripts/e2e_user_closure.py 专用的支付宝开放平台模拟器。
//
// 存在的理由：第三方登录与实名核验的支付宝适配器都要调用 https://openapi.alipay.com，
// 端到端脚本不可能连真实网关；而这两个适配器最需要被证明的恰恰是「RSA2 签名与验签
// 能不能互通」——只测配置错误分支等于没测。因此这里实现一个最小网关：验请求签名、
// 返回带签名的响应节点，让脚本走完真实适配器的完整链路。
//
// 它不是生产代码，也不在任何运行时路径上：只有 e2e 脚本会把它编译成静态二进制，
// 挂进 backend_default 网络里的临时容器运行（后端容器无法访问宿主机，只能访问同网络容器）。
// 放在 cmd/ 下是为了让 go build ./... 与 go vet ./... 顺带覆盖它（仅依赖标准库）。
package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
)

// 响应节点名（支付宝按 method 下划线化后加 _response 后缀）。
const (
	methodOAuthToken   = "alipay.system.oauth.token"
	methodOAuthInfo    = "alipay.user.info.share"
	methodCertifyInit  = "alipay.user.certify.open.initialize"
	methodCertifyQuery = "alipay.user.certify.open.query"

	nodeOAuthToken   = "alipay_system_oauth_token_response"
	nodeOAuthInfo    = "alipay_user_info_share_response"
	nodeCertifyInit  = "alipay_user_certify_open_initialize_response"
	nodeCertifyQuery = "alipay_user_certify_open_query_response"
)

type server struct {
	appPublicKey *rsa.PublicKey
	mockPrivate  *rsa.PrivateKey
	statePath    string

	mu    sync.Mutex
	state mockState
}

// mockState 落在挂载目录里，供脚本在断言阶段读取。
//
// 记录 request_sign_ok 而不是直接 500：请求签名校验失败说明平台的签名实现
// 与网关约定不一致，用 HTTP 500 表达只会让脚本看到「网关返回 500」，
// 定位不到真正原因。
type mockState struct {
	Calls         map[string]int `json:"calls"`
	RequestSignOK bool           `json:"request_sign_ok"`
	SignFailures  []string       `json:"sign_failures"`
}

func main() {
	addr := flag.String("addr", ":18099", "监听地址")
	statePath := flag.String("state", "/state/state.json", "状态文件路径")
	appPubPath := flag.String("app-public-key", "/keys/app_public.pem", "应用公钥（校验请求签名）")
	mockPrivPath := flag.String("mock-private-key", "/keys/mock_private.pem", "模拟网关私钥（签响应）")
	flag.Parse()

	s := &server{
		appPublicKey: mustPublicKey(*appPubPath),
		mockPrivate:  mustPrivateKey(*mockPrivPath),
		statePath:    *statePath,
		state:        mockState{Calls: map[string]int{}, RequestSignOK: true},
	}
	s.flush()

	mux := http.NewServeMux()
	mux.HandleFunc("/gateway.do", s.handle)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 便于脚本探活（certify 跳转地址是 GET 到网关）。
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, "ucgatewaymock")
			return
		}
		s.handle(w, r)
	})
	log.Printf("ucgatewaymock listening on %s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("listen failed: %v", err)
	}
}

func (s *server) handle(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	params := map[string]string{}
	for k, v := range r.Form {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	method := params["method"]

	s.mu.Lock()
	s.state.Calls[method]++
	s.mu.Unlock()

	if err := s.verifyRequestSign(params); err != nil {
		s.mu.Lock()
		s.state.RequestSignOK = false
		s.state.SignFailures = append(s.state.SignFailures, method+": "+err.Error())
		s.mu.Unlock()
	}

	node, payload := s.respond(method, params)
	body, err := s.envelope(node, payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.flush()
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	_, _ = w.Write(body)
}

// respond 按 method 生成响应节点内容。
func (s *server) respond(method string, params map[string]string) (string, map[string]any) {
	switch method {
	case methodOAuthToken:
		// 把授权码原样编进 user_id：脚本用不同的 code 就能扮演不同的外部账号。
		code := strings.TrimSpace(params["code"])
		// code/msg 不可省：真实网关的每个响应节点都带业务码，平台的连通性测试
		// 正是靠「有没有业务码」区分「链路通」与「网关地址填错了」。
		return nodeOAuthToken, map[string]any{
			"access_token":  "mock-access-" + code,
			"user_id":       "mockuid-" + code,
			"expires_in":    3600,
			"refresh_token": "mock-refresh-" + code,
			"code":          "10000",
			"msg":           "Success",
		}
	case methodOAuthInfo:
		// auth_token 走 biz_content（JSON），不是顶层表单参数——照顶层取会恒拿不到值，
		// 表现为「每个账号的 openid 都变成 unknown」，看起来像平台没传 openid。
		var biz struct {
			AuthToken string `json:"auth_token"`
		}
		_ = json.Unmarshal([]byte(params["biz_content"]), &biz)
		token := strings.TrimPrefix(biz.AuthToken, "mock-access-")
		if token == biz.AuthToken || token == "" {
			token = "unknown"
		}
		return nodeOAuthInfo, map[string]any{
			"user_id":   "mockuid-" + token,
			"nick_name": "联调用户-" + token,
			"avatar":    "https://mock.local/avatar.png",
			"code":      "10000",
			"msg":       "Success",
		}
	case methodCertifyInit:
		var biz struct {
			OuterOrderNo string `json:"outer_order_no"`
			BizCode      string `json:"biz_code"`
		}
		_ = json.Unmarshal([]byte(params["biz_content"]), &biz)
		return nodeCertifyInit, map[string]any{
			"certify_id": "mockcertify-" + biz.OuterOrderNo,
			"code":       "10000",
			"msg":        "Success",
		}
	case methodCertifyQuery:
		return nodeCertifyQuery, map[string]any{
			"passed": "T",
			"code":   "10000",
			"msg":    "Success",
		}
	default:
		return strings.ReplaceAll(method, ".", "_") + "_response", map[string]any{
			"code":    "40004",
			"msg":     "Business Failed",
			"sub_msg": "unsupported method " + method,
		}
	}
}

// envelope 拼出 `{"<node>":{...},"sign":"..."}`。
//
// 签名对象是**节点在报文里的原始子串**——与 alipay.VerifyResponseSign 的
// 验签口径严格一致（重新序列化会让键序与转义变化，导致验签恒失败）。
func (s *server) envelope(node string, payload map[string]any) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.mockPrivate, crypto.SHA256, digest[:])
	if err != nil {
		return nil, err
	}
	body := fmt.Sprintf(`{"%s":%s,"sign":"%s"}`,
		node, raw, base64.StdEncoding.EncodeToString(sig))
	return []byte(body), nil
}

// verifyRequestSign 校验平台请求的 RSA2 签名。
func (s *server) verifyRequestSign(params map[string]string) error {
	signStr := strings.TrimSpace(params["sign"])
	if signStr == "" {
		return fmt.Errorf("请求缺少 sign")
	}
	sig, err := base64.StdEncoding.DecodeString(signStr)
	if err != nil {
		return fmt.Errorf("sign 不是合法 base64")
	}
	digest := sha256.Sum256([]byte(signContent(params)))
	if err := rsa.VerifyPKCS1v15(s.appPublicKey, crypto.SHA256, digest[:], sig); err != nil {
		return fmt.Errorf("验签未通过")
	}
	return nil
}

// signContent 与 alipay.SignContent 同规则：剔除 sign/sign_type 与空值，按 key 升序拼接。
func signContent(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || k == "sign_type" {
			continue
		}
		if strings.TrimSpace(v) == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	return strings.Join(parts, "&")
}

func (s *server) flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.statePath, raw, 0o644)
}

func mustPrivateKey(path string) *rsa.PrivateKey {
	raw, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("读取私钥 %s 失败: %v", path, err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		log.Fatalf("私钥 %s 不是 PEM 格式", path)
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey
		}
	}
	rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		log.Fatalf("私钥 %s 解析失败: %v", path, err)
	}
	return rsaKey
}

func mustPublicKey(path string) *rsa.PublicKey {
	raw, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("读取公钥 %s 失败: %v", path, err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		log.Fatalf("公钥 %s 不是 PEM 格式", path)
	}
	if pub, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if rsaPub, ok := pub.(*rsa.PublicKey); ok {
			return rsaPub
		}
	}
	rsaPub, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		log.Fatalf("公钥 %s 解析失败: %v", path, err)
	}
	return rsaPub
}
