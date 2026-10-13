package credentials

import (
	crand "crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"hostsent/backend/internal/pkg/crypto"
)

// 实例登录密码的生成策略：
//   - 字母表在上游适配器自有生成器（internal/pkg/upstream/{dcimcloud,zjmf} 的
//     randPassword，已剔除 l/I/0/1）基础上**再剔除 o 与 O**：运营和客户会照着界面
//     手抄密码，0/O/o 与 1/l/I 是"密码明明对却登不上"的两大来源；
//   - 长度 12、且必含小写/大写/数字各一位：满足魔方云面板的密码强度校验（实测 12 位可用），
//     魔方财务侧同样接受。
//
// 生成走 crypto/rand（适配器内部用的是 math/rand，仅作兜底）：这是真正下发给客户机器的
// 凭据，强度不能依赖可预测的伪随机源。

const (
	pwLower = "abcdefghijkmnpqrstuvwxyz"
	pwUpper = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	pwDigit = "23456789"
)

// passwordLength 生成长度：与上游适配器保持一致的 12 位。
const passwordLength = 12

// randInt 取 [0,n) 的密码学安全随机数；随机源不可用时 panic 由进程级处理（极罕见）。
func randInt(n int) int {
	v, err := crand.Int(crand.Reader, big.NewInt(int64(n)))
	if err != nil {
		// crypto/rand 不可用意味着系统熵源故障，生成弱密码的危害大于直接失败。
		panic("credentials: 系统随机源不可用: " + err.Error())
	}
	return int(v.Int64())
}

// pick 从字符集中随机取一个字符。
func pick(set string) byte { return set[randInt(len(set))] }

// GeneratePassword 生成实例登录初始密码（12 位，必含大小写与数字，无易混淆字符）。
func GeneratePassword() string {
	all := pwLower + pwUpper + pwDigit
	b := make([]byte, passwordLength)
	b[0] = pick(pwLower)
	b[1] = pick(pwUpper)
	b[2] = pick(pwDigit)
	for i := 3; i < passwordLength; i++ {
		b[i] = pick(all)
	}
	// 洗牌：否则前三位的类型固定，密码空间被无谓缩小。
	for i := len(b) - 1; i > 0; i-- {
		j := randInt(i + 1)
		b[i], b[j] = b[j], b[i]
	}
	return string(b)
}

// EncryptSecret 把明文秘密加密为带 `enc:v1:` 前缀的密文（空串原样返回）。
//
// 用于实例登录密码这类"由本系统生成、需要长期留存回看"的凭据。与
// EncryptFields 的分工：那个按渠道凭证 schema 全量处理，这里面向单值。
func EncryptSecret(plaintext, secretKey string) (string, error) {
	v := strings.TrimSpace(plaintext)
	if v == "" {
		return "", nil
	}
	if strings.HasPrefix(v, EncPrefix) {
		return v, nil // 已是密文，幂等
	}
	enc, err := crypto.Encrypt(v, secretKey)
	if err != nil {
		return "", fmt.Errorf("加密失败: %w", err)
	}
	return EncPrefix + enc, nil
}

// DecryptSecret 解密 EncryptSecret 的产物，按凭证同一套规则区分三种情况（地雷 L8）：
//   - 空串        → 返回空，不报错（上游从未回传密码属正常态）；
//   - 无 enc: 前缀 → 视为历史明文，原样返回（兼容存量）；
//   - 有前缀但解密失败 → **返回错误**，绝不把 base64 密文当明文交出去。
//
// key 为空时视为未配置加密能力：此时仍拒绝把密文当明文返回（返回错误）。
func DecryptSecret(stored, secretKey string) (string, error) {
	v := strings.TrimSpace(stored)
	if v == "" {
		return "", nil
	}
	if !strings.HasPrefix(v, EncPrefix) {
		return v, nil
	}
	if strings.TrimSpace(secretKey) == "" {
		return "", ErrDecryptFailed
	}
	plain, err := crypto.Decrypt(strings.TrimPrefix(v, EncPrefix), secretKey)
	if err != nil {
		return "", ErrDecryptFailed
	}
	return plain, nil
}
