// Package mask 提供敏感字段脱敏与文本截断，供审计日志与上游接口日志共用
// （doc92 §3.3）。原先这套逻辑内嵌在 middleware/admin_audit.go，上游调用日志
// 也要脱敏（魔方云登录请求体带管理员密码、魔方财务 query 里带 jwt），
// 抽到独立包避免两处各写一份以致口径漂移。
package mask

import (
	"encoding/json"
	"strings"
)

// SensitiveKeys 命中即替换为 *** 的键名（子串匹配，不区分大小写）。
//
// 覆盖：password/passwd、token/access-token、secret/api_secret、captcha、
// jwt、session、authorization、api_key/access-key、sign/signature。
var SensitiveKeys = []string{
	"password", "passwd", "token", "secret", "captcha",
	"jwt", "session", "authorization", "api_key", "apikey",
	"access_key", "secret_key", "signature",
}

// Masked 脱敏后的占位值。
const Masked = "***"

// Truncate 截断字符串到 max 字节（按字节安全截断，避免切出半个 UTF-8）。
func Truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	// 从 max 往前退到合法的 UTF-8 边界。
	cut := max
	for cut > 0 && (s[cut]&0xC0) == 0x80 {
		cut--
	}
	return s[:cut]
}

// MaskKeyValue 对 "key=value" / "key":"value" / "key: value" 形式的敏感键做替换。
//
// 用于非 JSON 的表单体（上游请求体就是 urlencoded）。
func MaskKeyValue(text, key string) string {
	lower := strings.ToLower(text)
	idx := 0
	for {
		if idx >= len(lower) {
			return text
		}
		pos := strings.Index(lower[idx:], key)
		if pos < 0 {
			return text
		}
		start := idx + pos + len(key)
		for start < len(text) && (text[start] == '"' || text[start] == '=' || text[start] == ':' || text[start] == ' ') {
			start++
		}
		end := start
		for end < len(text) && text[end] != '&' && text[end] != '"' && text[end] != ',' && text[end] != ' ' {
			end++
		}
		if end > start {
			text = text[:start] + Masked + text[end:]
			lower = strings.ToLower(text)
		}
		idx = start + len(Masked)
		if idx >= len(text) {
			return text
		}
	}
}

// MaskText 纯文本脱敏：逐个敏感键做键值替换，最后按 max 截断（max<=0 不截断）。
func MaskText(text string, max int) string {
	result := text
	for _, key := range SensitiveKeys {
		result = MaskKeyValue(result, key)
	}
	return Truncate(result, max)
}

// MaskJSON 对 JSON 对象做一层浅脱敏（键名子串命中即整值替换）。
//
// 解析失败时退化为 MaskText。返回值同时报告是否命中了任何敏感键，
// 便于调用方决定是否需要标记「已脱敏」。
func MaskJSON(payload []byte, max int) (string, bool) {
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" {
		return "", false
	}
	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return MaskText(trimmed, max), HasSensitiveKey(trimmed)
	}
	hit := false
	for key := range data {
		if IsSensitiveKey(key) {
			data[key] = Masked
			hit = true
		}
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return MaskText(trimmed, max), hit
	}
	return Truncate(string(encoded), max), hit
}

// MaskGenericBody 脱敏任意响应体：能解析成 JSON 对象就走 MaskJSON，
// 否则按文本处理（上游响应可能是纯 token 字符串、HTML 错误页等）。
func MaskGenericBody(body string, max int) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "{") {
		out, _ := MaskJSON([]byte(trimmed), max)
		return out
	}
	return MaskText(trimmed, max)
}

// IsSensitiveKey 键名是否命中敏感词。
func IsSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, s := range SensitiveKeys {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

// HasSensitiveKey 文本里是否出现敏感键名。
//
// 注意：这里只做「是否出现」判断，用于给日志打「疑似含凭据」标记，
// 不用于决定是否脱敏 —— 脱敏一律执行，不依赖这个探测结果。
func HasSensitiveKey(text string) bool {
	lower := strings.ToLower(text)
	for _, s := range SensitiveKeys {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

// MaskURL 脱敏 URL 的 query（魔方财务有 ?jwt= 的用法，token 可能在 query 里）。
//
// 保留 path 与参数名，仅替换命中的参数值；解析失败时对整个串做文本脱敏。
func MaskURL(raw string) string {
	idx := strings.Index(raw, "?")
	if idx < 0 {
		return raw
	}
	head, query := raw[:idx], raw[idx+1:]
	parts := strings.Split(query, "&")
	changed := false
	for i, part := range parts {
		eq := strings.Index(part, "=")
		if eq <= 0 {
			continue
		}
		if IsSensitiveKey(part[:eq]) {
			parts[i] = part[:eq] + "=" + Masked
			changed = true
		}
	}
	if !changed {
		return raw
	}
	return head + "?" + strings.Join(parts, "&")
}
