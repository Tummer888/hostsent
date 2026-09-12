package credentials

import (
	"strings"
	"testing"

	"hostsent/backend/internal/pkg/integration"
)

const testKey = "test-encrypt-key"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	schema := []integration.Field{
		{Key: "api_key", Type: integration.FieldTypeString},
		{Key: "api_secret", Type: integration.FieldTypePassword, Secret: true},
	}
	in := Map{"api_key": "user1", "api_secret": "super-secret"}
	enc, err := EncryptFields(in, schema, testKey)
	if err != nil {
		t.Fatalf("EncryptFields: %v", err)
	}
	if !strings.HasPrefix(enc["api_secret"], EncPrefix) {
		t.Fatalf("secret field not encrypted: %q", enc["api_secret"])
	}
	if enc["api_key"] != "user1" {
		t.Fatalf("non-secret field modified: %q", enc["api_key"])
	}
	dec, err := enc.DecryptFields(testKey)
	if err != nil {
		t.Fatalf("DecryptFields: %v", err)
	}
	if dec["api_secret"] != "super-secret" {
		t.Fatalf("round trip mismatch: %q", dec["api_secret"])
	}
}

func TestEncryptFieldsIdempotent(t *testing.T) {
	schema := []integration.Field{{Key: "api_secret", Secret: true}}
	once, err := EncryptFields(Map{"api_secret": "s"}, schema, testKey)
	if err != nil {
		t.Fatalf("first encrypt: %v", err)
	}
	twice, err := EncryptFields(once, schema, testKey)
	if err != nil {
		t.Fatalf("second encrypt: %v", err)
	}
	if once["api_secret"] != twice["api_secret"] {
		t.Fatalf("re-encrypt changed ciphertext: %q -> %q", once["api_secret"], twice["api_secret"])
	}
}

// L8 核心：带前缀但解密失败必须报错，绝不回退明文。
func TestDecryptFieldsFailsOnCorruptedCiphertext(t *testing.T) {
	_, err := Map{"api_secret": EncPrefix + "bm90LXZhbGlk"}.DecryptFields(testKey)
	if err == nil {
		t.Fatal("expected error for corrupted ciphertext")
	}
	if !strings.Contains(err.Error(), "解密失败") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// 兼容存量：无前缀视为历史明文，原样返回。
func TestDecryptFieldsPassesThroughPlaintext(t *testing.T) {
	dec, err := Map{"api_secret": "legacy-plain"}.DecryptFields(testKey)
	if err != nil {
		t.Fatalf("DecryptFields: %v", err)
	}
	if dec["api_secret"] != "legacy-plain" {
		t.Fatalf("legacy plaintext altered: %q", dec["api_secret"])
	}
}

func TestDecryptFieldsWrongKey(t *testing.T) {
	enc, err := EncryptFields(Map{"api_secret": "s"}, []integration.Field{{Key: "api_secret", Secret: true}}, testKey)
	if err != nil {
		t.Fatalf("EncryptFields: %v", err)
	}
	if _, err := enc.DecryptFields("another-key"); err == nil {
		t.Fatal("expected error when key rotated")
	}
}

func TestMaskOnlySecretFields(t *testing.T) {
	schema := []integration.Field{{Key: "api_key"}, {Key: "api_secret", Secret: true}}
	masked := Map{"api_key": "user1", "api_secret": "1234567890"}.Mask(schema)
	if masked["api_key"] != "user1" {
		t.Fatalf("non-secret masked: %q", masked["api_key"])
	}
	if masked["api_secret"] != "12****90" {
		t.Fatalf("secret mask mismatch: %q", masked["api_secret"])
	}
}

func TestDecodeEmptyAndNull(t *testing.T) {
	for _, raw := range []string{"", "null"} {
		m, err := Decode(raw)
		if err != nil || len(m) != 0 {
			t.Fatalf("Decode(%q) = %v, %v", raw, m, err)
		}
	}
}
