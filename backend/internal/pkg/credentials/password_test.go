package credentials

import (
	"errors"
	"strings"
	"testing"
)

// TestGeneratePasswordShape 密码必须满足上游面板的强度策略：
// 12 位、含大小写与数字、无易混淆字符（0/O/o/1/l/I）。
func TestGeneratePasswordShape(t *testing.T) {
	for i := 0; i < 200; i++ {
		p := GeneratePassword()
		if len(p) != passwordLength {
			t.Fatalf("长度应为 %d，实得 %d（%s）", passwordLength, len(p), p)
		}
		var hasLower, hasUpper, hasDigit bool
		for _, r := range p {
			switch {
			case r >= 'a' && r <= 'z':
				hasLower = true
			case r >= 'A' && r <= 'Z':
				hasUpper = true
			case r >= '0' && r <= '9':
				hasDigit = true
			}
			if strings.ContainsRune("0Oo1lI", r) {
				t.Fatalf("出现易混淆字符 %q：%s", r, p)
			}
		}
		if !hasLower || !hasUpper || !hasDigit {
			t.Fatalf("必须同时含小写/大写/数字，实得 %s", p)
		}
	}
}

// TestGeneratePasswordUnique 连续生成的密码不得重复（随机源可用性的最简证明）。
func TestGeneratePasswordUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 500; i++ {
		p := GeneratePassword()
		if seen[p] {
			t.Fatalf("生成重复密码：%s", p)
		}
		seen[p] = true
	}
}

// TestSecretRoundTrip 加密→解密必须还原明文，且密文带 enc:v1: 前缀（绝不落明文）。
func TestSecretRoundTrip(t *testing.T) {
	const key = "test-encrypt-key"
	plain := GeneratePassword()
	enc, err := EncryptSecret(plain, key)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	if !strings.HasPrefix(enc, EncPrefix) {
		t.Fatalf("密文应带 %s 前缀，实得 %s", EncPrefix, enc)
	}
	if strings.Contains(enc, plain) {
		t.Fatal("密文中不得出现明文")
	}
	got, err := DecryptSecret(enc, key)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	if got != plain {
		t.Fatalf("解密结果不符：want %s got %s", plain, got)
	}
}

// TestEncryptSecretIdempotent 已是密文的值不得重复加密（重复提交同一份数据是常态）。
func TestEncryptSecretIdempotent(t *testing.T) {
	enc, err := EncryptSecret("Abc123456789", "k")
	if err != nil {
		t.Fatal(err)
	}
	again, err := EncryptSecret(enc, "k")
	if err != nil {
		t.Fatal(err)
	}
	if again != enc {
		t.Fatal("重复加密应原样返回，否则每次提交都会套一层新密文")
	}
}

// TestEncryptSecretEmpty 空明文返回空串、不报错（"没有密码"是合法状态，不是错误）。
func TestEncryptSecretEmpty(t *testing.T) {
	for _, in := range []string{"", "   "} {
		out, err := EncryptSecret(in, "k")
		if err != nil || out != "" {
			t.Fatalf("空明文应返回空串，实得 (%q, %v)", out, err)
		}
	}
}

// TestDecryptSecretRules 覆盖地雷 L8 的三种情况：
// 空值原样空、历史明文原样返回、密文解密失败必须显式报错（绝不回退密文）。
func TestDecryptSecretRules(t *testing.T) {
	const key = "test-encrypt-key"

	if got, err := DecryptSecret("", key); err != nil || got != "" {
		t.Fatalf("空值应原样返回空串，实得 (%q, %v)", got, err)
	}

	// 历史明文（迁移 100 之前的存量行）：原样返回，供只读兼容。
	if got, err := DecryptSecret("legacy-plain-password", key); err != nil || got != "legacy-plain-password" {
		t.Fatalf("历史明文应原样返回，实得 (%q, %v)", got, err)
	}

	// 密文 + 错误密钥：必须报错，绝不能把 base64 密文当明文交出去。
	enc, err := EncryptSecret("RealSecret123", key)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := DecryptSecret(enc, "wrong-key"); !errors.Is(err, ErrDecryptFailed) {
		t.Fatalf("错误密钥应返回 ErrDecryptFailed，实得 (%q, %v)", got, err)
	}

	// 密文 + 空密钥（未配置加密能力）：同样必须报错而非回退。
	if got, err := DecryptSecret(enc, ""); !errors.Is(err, ErrDecryptFailed) {
		t.Fatalf("空密钥应返回 ErrDecryptFailed，实得 (%q, %v)", got, err)
	}
}
