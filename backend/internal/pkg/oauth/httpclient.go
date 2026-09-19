package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultTimeout 三方接口调用的默认超时。
//
// 5 秒：授权码换令牌与拉取用户资料都是轻量调用，三方抖动不应拖垮平台请求链路
// （对齐 pkg/captcha/provider/netease 的短超时口径）。
const DefaultTimeout = 5 * time.Second

// maxRespBytes 响应体上限，防止异常大包打爆内存。
const maxRespBytes = 64 * 1024

// HTTPClient 构造带超时的客户端。
func HTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &http.Client{Timeout: timeout}
}

// PostForm 以 application/x-www-form-urlencoded 发起 POST，返回响应体。
//
// 错误一律包成 ErrExchangeFailed/ErrUserInfoFailed 之外的**类型化摘要**，
// 绝不把三方原始报文拼进错误信息回显给前端。
func PostForm(ctx context.Context, client *http.Client, endpoint string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	req.Header.Set("Accept", "application/json")
	return do(client, req)
}

// GetBytes 发起 GET 并返回响应体。extraHeaders 用于 QQ 的 User-Agent 等要求。
func GetBytes(ctx context.Context, client *http.Client, endpoint string, extraHeaders map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	return do(client, req)
}

// GetJSON 发起 GET 并把响应体解为 JSON。
func GetJSON(ctx context.Context, client *http.Client, endpoint string, out any) error {
	body, err := GetBytes(ctx, client, endpoint, nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("解析三方响应失败: %w", err)
	}
	return nil
}

// ParseJSON 解析响应体为 JSON 对象。
func ParseJSON(body []byte, out any) error {
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("解析三方响应失败: %w", err)
	}
	return nil
}

// ParseJSONP 剥离 JSONP 外壳后解析（QQ 的 /oauth2.0/me 返回 `callback( {...} );`）。
//
// QQ 这个接口是唯一以 JSONP 形式返回 JSON 的常用接口，且外壳格式不稳定
// （有无分号、有无空格都可能），因此这里做容错剥离而不是写死正则。
func ParseJSONP(body []byte, out any) error {
	s := strings.TrimSpace(string(body))
	if s == "" {
		return fmt.Errorf("三方返回空响应")
	}
	// 形如 callback( {...} ); 或 callback({...}) 或纯 JSON。
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end < 0 || end <= start {
		return fmt.Errorf("三方响应格式不可识别")
	}
	return ParseJSON([]byte(s[start:end+1]), out)
}

// ParseQueryString 把 `access_token=xxx&expires_in=7200` 形式的响应解析成 map。
//
// 微信与 QQ 的 token 接口在出错时返回的是 query string 而非 JSON，
// 正常返回时也常用 query string，故两种都要能解。
func ParseQueryString(body []byte) map[string]string {
	out := map[string]string{}
	for _, pair := range strings.Split(strings.TrimSpace(string(body)), "&") {
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, "=", 2)
		key := strings.TrimSpace(kv[0])
		val := ""
		if len(kv) == 2 {
			if decoded, err := url.QueryUnescape(kv[1]); err == nil {
				val = decoded
			} else {
				val = kv[1]
			}
		}
		if key != "" {
			out[key] = val
		}
	}
	return out
}

func do(client *http.Client, req *http.Request) ([]byte, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("三方接口不可达: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	if err != nil {
		return nil, fmt.Errorf("读取三方响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("三方接口返回 HTTP %d", resp.StatusCode)
	}
	return body, nil
}
