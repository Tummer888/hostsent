package transport

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"hostsent/backend/internal/pkg/upstream"
)

// ============================================================================
// 签名策略（契约③ Signer）
//
// 目标：把"各适配器自己拼签名"收敛为可声明、可复用的策略集，
// 由 CapabilityDescriptor.SignerType 指定。本期实现五种：
//   none   —— 无签名（内网/白名单）
//   bearer —— Authorization: Bearer <token>（魔方财务 JWT 即此形态）
//   form   —— 表单参数签名（魔方系 form 风格，按 key 排序后拼 secret 取摘要）
//   ak_sk  —— 阿里云 RPC 风格 HMAC-SHA1（AccessKeyId/AccessKeySecret）
//   tc3    —— 腾讯云 TC3-HMAC-SHA256（SecretId/SecretKey）
//
// 边界（17 §8）：不改写 mofangyun/mofangfinance 的现有传输实现，
// 本文件只提供新适配器（阿里云/腾讯云等）可直接使用的构件。
// ============================================================================

// Credentials 凭证键值对（来源为 resource_providers.credentials 解密后的 JSON）。
type Credentials map[string]string

// Get 读取凭证，缺失返回空串。
func (c Credentials) Get(key string) string {
	if c == nil {
		return ""
	}
	return c[key]
}

// Signer 签名策略：就地修改请求（写头或写 query），不发送请求。
type Signer interface {
	// Type 返回策略标识（与 upstream.Signer* 常量一致）。
	Type() string
	// Sign 对请求签名；body 为完整请求体（GET 传 nil）。
	Sign(req *http.Request, cred Credentials, body []byte) error
	// Scheme 返回可读的鉴权说明，供后台能力矩阵展示。
	Scheme() string
}

var signers = map[string]Signer{
	upstream.SignerNone:   noneSigner{},
	upstream.SignerBearer: bearerSigner{},
	upstream.SignerForm:   formSigner{},
	upstream.SignerAKSK:   aliyunRPCSigner{},
	upstream.SignerTC3:    tencentTC3Signer{},
}

// GetSigner 按标识取签名策略。
func GetSigner(signerType string) (Signer, bool) {
	s, ok := signers[signerType]
	return s, ok
}

// SignerTypes 返回全部已实现策略标识（升序）。
func SignerTypes() []string {
	types := make([]string, 0, len(signers))
	for t := range signers {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}

// ErrMissingCredential 凭证缺失。
var ErrMissingCredential = errors.New("缺少必要凭证字段")

// ---------- none ----------

type noneSigner struct{}

func (noneSigner) Type() string   { return upstream.SignerNone }
func (noneSigner) Scheme() string { return "无签名（依赖网络白名单）" }
func (noneSigner) Sign(*http.Request, Credentials, []byte) error {
	return nil
}

// ---------- bearer ----------

// bearerSigner 设置 Authorization: Bearer <token>；token 取 credential.token，
// 未显式提供时回退 api_key（与魔方财务 jwt 存储方式对应）。
type bearerSigner struct{}

func (bearerSigner) Type() string   { return upstream.SignerBearer }
func (bearerSigner) Scheme() string { return "Authorization: Bearer" }

func (bearerSigner) Sign(req *http.Request, cred Credentials, _ []byte) error {
	token := cred.Get("token")
	if token == "" {
		token = cred.Get("api_key")
	}
	if token == "" {
		return fmt.Errorf("%w: token", ErrMissingCredential)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

// ---------- form ----------

// formSigner 表单签名：把请求表单参数按 key 升序拼成 k=v&...，末尾拼 secret 后取
// MD5 十六进制，作为 sign 参数附加。魔方系接口的典型形态（此处为通用预埋实现，
// 现有 mofangyun/mofangfinance 适配器走登录换 token，不依赖本策略）。
type formSigner struct{}

func (formSigner) Type() string   { return upstream.SignerForm }
func (formSigner) Scheme() string { return "表单参数 sign=MD5(排序参数+secret)" }

func (formSigner) Sign(req *http.Request, cred Credentials, body []byte) error {
	secret := cred.Get("api_secret")
	if secret == "" {
		return fmt.Errorf("%w: api_secret", ErrMissingCredential)
	}
	values, err := formValues(req, body)
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		if k == "sign" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(values.Get(k))
	}
	sb.WriteString(secret)
	sum := md5.Sum([]byte(sb.String()))
	signed := hex.EncodeToString(sum[:])

	if req.Method == http.MethodGet {
		q := req.URL.Query()
		q.Set("sign", signed)
		req.URL.RawQuery = q.Encode()
		return nil
	}
	// 表单请求体需整体重写：先取原值，追加 sign 后替换。
	values.Set("sign", signed)
	encoded := values.Encode()
	req.Body = io.NopCloser(strings.NewReader(encoded))
	req.ContentLength = int64(len(encoded))
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(encoded)), nil }
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return nil
}

// formValues 取请求参数：GET 读 query，其余优先用传入的 body 解析表单。
func formValues(req *http.Request, body []byte) (url.Values, error) {
	if req.Method == http.MethodGet {
		return req.URL.Query(), nil
	}
	if len(body) > 0 {
		v, err := url.ParseQuery(string(body))
		if err != nil {
			return nil, fmt.Errorf("解析表单请求体失败: %w", err)
		}
		return v, nil
	}
	return url.Values{}, nil
}

// ---------- ak_sk（阿里云 RPC 风格） ----------

// aliyunRPCSigner 阿里云 RPC 签名（HMAC-SHA1 + Base64）：
// 公共参数 AccessKeyId/SignatureMethod/SignatureVersion/SignatureNonce/Timestamp 后，
// 对全部参数按 key 升序做 percentEncode 拼接，StringToSign = "GET&%2F&" + encode(query)，
// 以 AccessKeySecret + "&" 为密钥 HMAC-SHA1，结果 Base64 作为 Signature。
type aliyunRPCSigner struct{}

func (aliyunRPCSigner) Type() string   { return upstream.SignerAKSK }
func (aliyunRPCSigner) Scheme() string { return "阿里云 RPC HMAC-SHA1" }

func (aliyunRPCSigner) Sign(req *http.Request, cred Credentials, body []byte) error {
	ak := cred.Get("access_key_id")
	sk := cred.Get("access_key_secret")
	if ak == "" || sk == "" {
		return fmt.Errorf("%w: access_key_id/access_key_secret", ErrMissingCredential)
	}
	q := req.URL.Query()
	for k, v := range formValuesOrEmpty(req, body) {
		for _, item := range v {
			q.Set(k, item)
		}
	}
	q.Set("AccessKeyId", ak)
	q.Set("Format", "JSON")
	q.Set("SignatureMethod", "HMAC-SHA1")
	q.Set("SignatureVersion", "1.0")
	q.Set("SignatureNonce", randomNonce())
	q.Set("Timestamp", time.Now().UTC().Format("2006-01-02T15:04:05Z"))

	canonical := aliyunCanonicalQuery(q)
	stringToSign := req.Method + "&%2F&" + aliyunPercentEncode(canonical)
	mac := hmac.New(sha1.New, []byte(sk+"&"))
	mac.Write([]byte(stringToSign))
	q.Set("Signature", base64.StdEncoding.EncodeToString(mac.Sum(nil)))

	req.URL.RawQuery = q.Encode()
	return nil
}

// aliyunCanonicalQuery 按 RFC3986 规范拼接待签串。
func aliyunCanonicalQuery(q url.Values) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		for _, v := range q[k] {
			parts = append(parts, aliyunPercentEncode(k)+"="+aliyunPercentEncode(v))
		}
	}
	return strings.Join(parts, "&")
}

// aliyunPercentEncode 阿里云要求的 percent 编码（空格→%20，*→%2A，%7E→~）。
func aliyunPercentEncode(s string) string {
	encoded := url.QueryEscape(s)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}

// ---------- tc3（腾讯云） ----------

// tencentTC3Signer 腾讯云 TC3-HMAC-SHA256 签名。
// 需 Host、X-TC-Action、X-TC-Timestamp、X-TC-Version（由调用方设置），
// SecretId/SecretKey/Service 从凭证取；payload 为请求体原文。
type tencentTC3Signer struct{}

func (tencentTC3Signer) Type() string   { return upstream.SignerTC3 }
func (tencentTC3Signer) Scheme() string { return "腾讯云 TC3-HMAC-SHA256" }

func (tencentTC3Signer) Sign(req *http.Request, cred Credentials, body []byte) error {
	secretID := cred.Get("secret_id")
	secretKey := cred.Get("secret_key")
	service := cred.Get("service")
	if secretID == "" || secretKey == "" || service == "" {
		return fmt.Errorf("%w: secret_id/secret_key/service", ErrMissingCredential)
	}
	action := req.Header.Get("X-TC-Action")
	if action == "" {
		return errors.New("TC3 签名需要 X-TC-Action")
	}
	timestamp := time.Now().Unix()
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")

	req.Header.Set("X-TC-Timestamp", fmt.Sprintf("%d", timestamp))
	if req.Header.Get("X-TC-Version") == "" {
		req.Header.Set("X-TC-Version", "2017-03-12")
	}
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	req.Header.Set("Host", host)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	payload := body
	hashedPayload := sha256Hex(payload)

	canonicalHeaders := "content-type:" + req.Header.Get("Content-Type") + "\n" +
		"host:" + host + "\n"
	signedHeaders := "content-type;host"
	canonicalRequest := strings.Join([]string{
		req.Method,
		req.URL.EscapedPath(),
		req.URL.RawQuery,
		canonicalHeaders,
		signedHeaders,
		hashedPayload,
	}, "\n")

	credentialScope := date + "/" + service + "/tc3_request"
	stringToSign := strings.Join([]string{
		"TC3-HMAC-SHA256",
		fmt.Sprintf("%d", timestamp),
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	secretDate := hmacSHA256([]byte("TC3"+secretKey), date)
	secretService := hmacSHA256(secretDate, service)
	secretSigning := hmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))

	auth := fmt.Sprintf("TC3-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		secretID, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", auth)
	return nil
}

// ---------- 工具 ----------

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func randomNonce() string {
	return fmt.Sprintf("%d%d", time.Now().UnixNano(), rand.Int63())
}

func formValuesOrEmpty(req *http.Request, body []byte) url.Values {
	v, err := formValues(req, body)
	if err != nil {
		return url.Values{}
	}
	return v
}
