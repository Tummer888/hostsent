package captcha

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// randomHex 生成 n 字节的 crypto/rand 十六进制串。
// 图形码 key、验证票据、OTP code 等安全相关随机量一律走此函数，
// 不用 math/rand（有可预测性先例，doc91 §4.1 第 5 条）。
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成随机串失败: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// RandomNumeric 生成 n 位数字串，首位不为 0（避免前导零被短信通道吞掉）。
func RandomNumeric(n int) (string, error) {
	if n <= 0 {
		return "", nil
	}
	out := make([]byte, n)
	// 首位 1–9。
	b := make([]byte, 1)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成验证码失败: %w", err)
	}
	out[0] = byte('1' + int(b[0])%9)
	rest := make([]byte, n-1)
	if _, err := rand.Read(rest); err != nil {
		return "", fmt.Errorf("生成验证码失败: %w", err)
	}
	for i, v := range rest {
		out[i+1] = byte('0' + int(v)%10)
	}
	return string(out), nil
}
