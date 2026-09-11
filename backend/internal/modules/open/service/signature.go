// Package service 实现开放平台网关与业务服务（P6）。
//
// 签名规范（doc16 §8.2）：
//
//	stringToSign = METHOD \n path \n canonical_query \n sha256hex(body) \n timestamp \n nonce
//	signature    = hex( HMAC-SHA256( app_secret, stringToSign ) )
package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
	"time"
)

// 签名请求头。
const (
	HeaderAppID     = "X-App-Id"
	HeaderTimestamp = "X-Timestamp"
	HeaderNonce     = "X-Nonce"
	HeaderSignature = "X-Signature"
	// HeaderClientRequestID 写接口幂等键请求头（T6.3）。
	HeaderClientRequestID = "X-Client-Request-Id"
)

// SignatureWindow 签名时间窗：timestamp 与服务端时间允许的偏差（doc16 §8.2：±5 分钟）。
const SignatureWindow = 5 * time.Minute

// CanonicalQuery 规范化 query：按 key 字典序排序，k=v 用 & 连接，值做 URL 转义。
// 空 query 返回空串。同一请求无论参数如何编码，规范化结果一致。
func CanonicalQuery(query url.Values) string {
	if len(query) == 0 {
		return ""
	}
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		vals := query[k]
		if len(vals) == 0 {
			pairs = append(pairs, url.QueryEscape(k)+"=")
			continue
		}
		sorted := append([]string(nil), vals...)
		sort.Strings(sorted)
		for _, v := range sorted {
			pairs = append(pairs, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}
	return strings.Join(pairs, "&")
}

// StringToSign 组装签名串。body 以 SHA256 十六进制参与，避免超长 body 直接进入 HMAC 输入。
func StringToSign(method, path string, query url.Values, body []byte, timestamp, nonce string) string {
	sum := sha256.Sum256(body)
	return strings.Join([]string{
		strings.ToUpper(method),
		path,
		CanonicalQuery(query),
		hex.EncodeToString(sum[:]),
		timestamp,
		nonce,
	}, "\n")
}

// Sign 计算 HMAC-SHA256 签名（hex 小写）。
func Sign(secret, stringToSign string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(stringToSign))
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify 常量时间比较签名，防时序侧信道。
func Verify(secret, stringToSign, signature string) bool {
	expected := Sign(secret, stringToSign)
	return hmac.Equal([]byte(expected), []byte(strings.ToLower(signature)))
}
