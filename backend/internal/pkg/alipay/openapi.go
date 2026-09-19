// Package alipay 提供支付宝开放平台网关的协议实现：公共参数拼装、RSA2 签名与验签。
//
// 为什么单独成包而不是塞进 pkg/transport 的 signer 注册表：
// transport 的 signer 是跨域通用签名（none/bearer/form/token/ak_sk/tc3），
// 而这里的 RSA2 是支付宝特有的报文约定（参数排序拼接 + 原始响应子串验签），
// 放进通用注册表会把「某家网关的报文格式」写进跨域契约（doc104 §5.6）。
//
// 本包只依赖标准库，被 realname 与 oauth 两个域共用以避免重复实现。
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
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

// ErrInvalidKey 私钥/公钥格式不可用（PEM 解析失败）。
var ErrInvalidKey = errors.New("支付宝密钥格式不正确（需要 PEM 编码）")

// ErrVerifySign 响应验签失败——报文被篡改或公钥不匹配。
var ErrVerifySign = errors.New("支付宝响应验签失败")

// DefaultGateway 生产网关地址；沙箱环境由凭证里的 gateway_url 覆盖。
const DefaultGateway = "https://openapi.alipay.com/gateway.do"

// CommonParams 一次开放平台调用的公共参数。
type CommonParams struct {
	AppID     string
	Method    string
	SignType  string // RSA2（默认）/ RSA
	Timestamp string // 2006-01-02 15:04:05
	Version   string // 1.0
	NotifyURL string
	ReturnURL string
	Charset   string
	Format    string
}

// ParsePrivateKey 解析 PEM 私钥，支持 PKCS8 与 PKCS1 两种编码。
//
// 支付宝控制台给出的是 PKCS8（BEGIN PRIVATE KEY）；部分工具导出 PKCS1
// （BEGIN RSA PRIVATE KEY）。两种都接受，避免运营因格式差异反复排查。
func ParsePrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(normalizePEM(pemStr)))
	if block == nil {
		return nil, ErrInvalidKey
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
		return nil, fmt.Errorf("%w: 私钥不是 RSA 类型", ErrInvalidKey)
	}
	if rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return rsaKey, nil
	}
	return nil, ErrInvalidKey
}

// ParsePublicKey 解析 PEM 公钥，支持 PKIX 与 PKCS1 两种编码。
func ParsePublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(normalizePEM(pemStr)))
	if block == nil {
		return nil, ErrInvalidKey
	}
	if pub, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if rsaPub, ok := pub.(*rsa.PublicKey); ok {
			return rsaPub, nil
		}
		return nil, fmt.Errorf("%w: 公钥不是 RSA 类型", ErrInvalidKey)
	}
	if rsaPub, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return rsaPub, nil
	}
	return nil, ErrInvalidKey
}

// normalizePEM 容忍控制台复制粘贴时产生的常见瑕疵：
// 首尾空白、CRLF、以及被折叠成单行的 PEM（支付宝控制台复制常见）。
//
// 单行还原不能简单地 ReplaceAll("-----", "-----\n")：标记自身就是由 `-----` 包起来的，
// 那样会把 `-----BEGIN` 切成 `-----` + 换行 + `BEGIN`，pem.Decode 反而找不到标记
// （错误只会是「密钥格式不正确」，很难定位）。因此这里按 BEGIN/END 标记定位后重组。
func normalizePEM(raw string) string {
	s := strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n"))
	if strings.Contains(s, "\n") {
		return s
	}
	const beginTag, endTag = "-----BEGIN", "-----END"
	begin := strings.Index(s, beginTag)
	endMarker := strings.LastIndex(s, endTag)
	if begin < 0 || endMarker <= begin {
		return s
	}
	beginClose := markerClose(s, begin, beginTag)
	endClose := markerClose(s, endMarker, endTag)
	if beginClose < 0 || endClose < 0 {
		return s
	}
	header := s[begin : beginClose+5]
	body := strings.TrimSpace(s[beginClose+5 : endMarker])
	footer := s[endMarker : endClose+5]
	if body == "" {
		return s
	}
	return header + "\n" + body + "\n" + footer
}

// markerClose 返回从 at 处开始的标记里、`-----` 收尾段的起始下标（找不到返回 -1）。
func markerClose(s string, at int, tag string) int {
	rest := s[at+len(tag):]
	idx := strings.Index(rest, "-----")
	if idx < 0 {
		return -1
	}
	return at + len(tag) + idx
}

// SignContent 按支付宝规则拼装待签名字符串：排除 sign 与空值，按 key 升序，k=v 以 & 连接。
//
// 导出供单测直接断言排序与空值剔除行为（这两个细节错了签名必然失败，但错误信息
// 只会是「验签失败」，很难定位）。
func SignContent(params map[string]string) string {
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
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(params[k])
	}
	return b.String()
}

// Sign 用应用私钥对参数签名，返回 base64 签名串。
func Sign(params map[string]string, privateKey *rsa.PrivateKey) (string, error) {
	content := SignContent(params)
	digest := sha256.Sum256([]byte(content))
	sig, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("支付宝签名失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// VerifyResponseSign 校验开放平台响应的签名。
//
// 关键点：待验签内容是**响应节点在原始报文中的子串**，不是重新序列化的 JSON。
// 重新序列化会改变键序与转义方式，导致验签恒失败——这是支付宝接入最常见的坑。
// 因此这里从原始 body 里定位 `"<node_name>":{...}` 的起止位置截取原文。
func VerifyResponseSign(rawBody []byte, nodeName string, publicKey *rsa.PublicKey) error {
	content, err := extractNodeRaw(rawBody, nodeName)
	if err != nil {
		return err
	}
	var signStr string
	var envelope struct {
		Sign string `json:"sign"`
	}
	if err := json.Unmarshal(rawBody, &envelope); err != nil {
		return fmt.Errorf("解析支付宝响应失败: %w", err)
	}
	signStr = envelope.Sign
	if signStr == "" {
		return fmt.Errorf("%w: 响应缺少 sign 字段", ErrVerifySign)
	}
	sig, err := base64.StdEncoding.DecodeString(signStr)
	if err != nil {
		return fmt.Errorf("%w: 签名不是合法 base64", ErrVerifySign)
	}
	digest := sha256.Sum256([]byte(content))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], sig); err != nil {
		return ErrVerifySign
	}
	return nil
}

// extractNodeRaw 从原始 JSON 报文中截取指定节点的原文（含大括号，不含键名与冒号）。
func extractNodeRaw(rawBody []byte, nodeName string) (string, error) {
	key := `"` + nodeName + `"`
	idx := strings.Index(string(rawBody), key)
	if idx < 0 {
		return "", fmt.Errorf("支付宝响应中不存在节点 %s", nodeName)
	}
	rest := string(rawBody)[idx+len(key):]
	start := strings.Index(rest, "{")
	if start < 0 {
		return "", fmt.Errorf("支付宝响应节点 %s 不是对象", nodeName)
	}
	// 逐字符配对花括号，跳过字符串字面量内的括号。
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(rest); i++ {
		ch := rest[i]
		if escaped {
			escaped = false
			continue
		}
		switch ch {
		case '\\':
			if inString {
				escaped = true
			}
		case '"':
			inString = !inString
		case '{':
			if !inString {
				depth++
			}
		case '}':
			if !inString {
				depth--
				if depth == 0 {
					return rest[start : i+1], nil
				}
			}
		}
	}
	return "", fmt.Errorf("支付宝响应节点 %s 括号不闭合", nodeName)
}

// BuildForm 构造带签名的表单参数（含 sign），供 POST 到网关。
func BuildForm(cp CommonParams, bizContent string, privateKey *rsa.PrivateKey, extras map[string]string) (url.Values, error) {
	params := map[string]string{
		"app_id":      cp.AppID,
		"method":      cp.Method,
		"format":      orDefault(cp.Format, "JSON"),
		"charset":     orDefault(cp.Charset, "utf-8"),
		"sign_type":   orDefault(cp.SignType, "RSA2"),
		"timestamp":   orDefault(cp.Timestamp, time.Now().Format("2006-01-02 15:04:05")),
		"version":     orDefault(cp.Version, "1.0"),
		"biz_content": bizContent,
	}
	if cp.NotifyURL != "" {
		params["notify_url"] = cp.NotifyURL
	}
	if cp.ReturnURL != "" {
		params["return_url"] = cp.ReturnURL
	}
	for k, v := range extras {
		params[k] = v
	}
	sign, err := Sign(params, privateKey)
	if err != nil {
		return nil, err
	}
	params["sign"] = sign

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	return form, nil
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
