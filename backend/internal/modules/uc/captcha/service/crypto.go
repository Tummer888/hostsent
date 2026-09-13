package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// hashTarget 计算目标（邮箱/手机号）的确定性哈希，用于频控查询。
//
// 确定性是必需的（同一目标多次请求要落到同一 key），因此 pepper 取应用级常量
// 而非每行随机 salt；明文 target 仍单独存储供运营排查，展示时打码。
func hashTarget(pepper, target string) string {
	sum := sha256.Sum256([]byte(pepper + "|" + strings.ToLower(strings.TrimSpace(target))))
	return hex.EncodeToString(sum[:])
}

// hashCode 计算验证码哈希：sha256(code + per-row salt)，绝不存明文。
func hashCode(code, salt string) string {
	sum := sha256.Sum256([]byte(code + salt))
	return hex.EncodeToString(sum[:])
}

// randomSalt 生成 16 字节随机 salt（hex）。
func randomSalt() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "0123456789abcdef"
	}
	return hex.EncodeToString(b)
}

// randomTicket 生成 32 字节随机票据（hex）。
func randomTicket() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// randomNumericCode 生成 n 位数字验证码，首位不为 0（避免前导零被短信通道吞掉）。
// 使用 crypto/rand：math/rand 有可预测性先例（doc91 §4.1 第 5 条）。
func randomNumericCode(n int) string {
	out := make([]byte, n)
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return strings.Repeat("9", n)
	}
	for i := range out {
		if i == 0 {
			out[i] = byte('1' + int(b[i])%9)
			continue
		}
		out[i] = byte('0' + int(b[i])%10)
	}
	return string(out)
}

// maskTarget 目标打码展示：邮箱保留首字符与域名，手机保留前 3 后 4。
func maskTarget(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	if at := strings.Index(target, "@"); at > 0 {
		name := target[:at]
		domain := target[at:]
		if len(name) <= 1 {
			return "*" + domain
		}
		return string(name[0]) + strings.Repeat("*", len(name)-1) + domain
	}
	r := []rune(target)
	if len(r) <= 4 {
		return strings.Repeat("*", len(r))
	}
	if len(r) >= 7 {
		return string(r[:3]) + strings.Repeat("*", len(r)-7) + string(r[len(r)-4:])
	}
	return string(r[:2]) + strings.Repeat("*", len(r)-4) + string(r[len(r)-2:])
}
