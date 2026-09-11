// Package credentials 提供上游渠道"凭证 JSON + 字段级加密"的读写（契约③，T2.3）。
//
// 设计（docs/实施计划/16 §5、17 §P2/T2.3）：
//   - 凭证以 JSON 对象落库（resource_providers.credentials），字段由渠道类型的
//     CapabilityDescriptor.CredentialSchema 声明，新增一家上游不再加数据库列。
//   - 声明为 secret 的字段逐字段 AES-256-GCM 加密，密文带 `enc:v1:` 前缀以便与
//     历史明文区分。
//
// 地雷 L8（解密静默回退）：旧 provider_service.decryptValue 在任何解密失败时
//
//	原样返回存储值，导致密钥轮换后把 base64 密文当明文发给上游，表现为"莫名其妙的
//	认证失败"。本包区分三种情况：
//	  ① 空值            → 返回空，不报错；
//	  ② 无 enc: 前缀     → 视为历史明文，原样返回（兼容存量，只读一版）；
//	  ③ 有 enc: 前缀但解密失败 → **返回错误**，由上层把渠道标记异常，绝不回退明文。
package credentials

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"hostsent/backend/internal/pkg/crypto"
	"hostsent/backend/internal/pkg/upstream"
)

// EncPrefix 密文标记前缀（含版本号，便于将来换算法）。
const EncPrefix = "enc:v1:"

// ErrDecryptFailed 密文解密失败——密钥变更或数据损坏，必须显式失败而非回退。
var ErrDecryptFailed = errors.New("凭证明文解密失败（密钥可能已变更，请重新录入凭证）")

// Map 凭证键值对。
type Map map[string]string

// Decode 解析 credentials 列（JSON 文本）；空串返回空 Map，不报错。
func Decode(raw string) (Map, error) {
	m := Map{}
	s := strings.TrimSpace(raw)
	if s == "" || s == "null" {
		return m, nil
	}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, fmt.Errorf("解析凭证 JSON 失败: %w", err)
	}
	return m, nil
}

// Encode 序列化凭证为 JSON 文本；空 Map 输出 "{}"。
func (m Map) Encode() (string, error) {
	if m == nil {
		m = Map{}
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// EncryptFields 按字段描述符加密：仅处理 Secret=true 且非空的字段。
// 已是密文（带前缀）的值原样保留，保证幂等（重复提交不重复加密）。
func EncryptFields(values Map, schema []upstream.Field, secretKey string) (Map, error) {
	out := Map{}
	for k, v := range values {
		out[k] = v
	}
	for _, f := range schema {
		if !f.Secret {
			continue
		}
		v := strings.TrimSpace(out[f.Key])
		if v == "" || strings.HasPrefix(v, EncPrefix) {
			continue
		}
		enc, err := crypto.Encrypt(v, secretKey)
		if err != nil {
			return nil, fmt.Errorf("加密凭证字段 %s 失败: %w", f.Key, err)
		}
		out[f.Key] = EncPrefix + enc
	}
	return out, nil
}

// DecryptFields 解密全部凭证值；非密文（历史明文）原样返回。
// 任一密文解密失败即整体报错（L8：不回退明文）。
func (m Map) DecryptFields(secretKey string) (Map, error) {
	out := Map{}
	for k, v := range m {
		if !strings.HasPrefix(v, EncPrefix) {
			out[k] = v
			continue
		}
		plain, err := crypto.Decrypt(strings.TrimPrefix(v, EncPrefix), secretKey)
		if err != nil {
			return nil, fmt.Errorf("%w: 字段 %s", ErrDecryptFailed, k)
		}
		out[k] = plain
	}
	return out, nil
}

// Mask 生成脱敏快照：secret 字段仅保留首尾各 2 位；非密文字段原样。
// 用于后台回显，绝不输出明文或完整密文。
func (m Map) Mask(schema []upstream.Field) Map {
	secretKeys := map[string]bool{}
	for _, f := range schema {
		if f.Secret {
			secretKeys[f.Key] = true
		}
	}
	out := Map{}
	for k, v := range m {
		if secretKeys[k] {
			out[k] = MaskSecret(v)
			continue
		}
		out[k] = v
	}
	return out
}

// MaskSecret 通用脱敏：首尾各 2 位，中间 ****；短值整体替换。
func MaskSecret(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) <= 4 {
		return "****"
	}
	return string(r[:2]) + "****" + string(r[len(r)-2:])
}

// IsMaskedEcho 判断前端回传值是否为脱敏回显（即用户未修改）。
func IsMaskedEcho(storedMasked, incoming string) bool {
	return storedMasked != "" && storedMasked == incoming
}
