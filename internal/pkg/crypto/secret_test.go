package crypto

import (
	"errors"
	"strings"
	"testing"
)

func testKey() string {
	return strings.Repeat("ab", 32)
}

func TestNewCipherValidation(t *testing.T) {
	if _, err := NewCipher(""); !errors.Is(err, ErrKeyRequired) {
		t.Fatalf("空密钥应返回 ErrKeyRequired，实际 %v", err)
	}
	if _, err := NewCipher("zz"); err == nil {
		t.Fatal("非法十六进制应报错")
	}
	if _, err := NewCipher(strings.Repeat("ab", 16)); err == nil {
		t.Fatal("短密钥应报错")
	}
	if _, err := NewCipher(testKey()); err != nil {
		t.Fatalf("合法密钥创建失败: %v", err)
	}
}

func TestEncryptDecrypt(t *testing.T) {
	c, err := NewCipher(testKey())
	if err != nil {
		t.Fatal(err)
	}
	secret := "n8n-webhook-secret-中文"
	encrypted, err := c.Encrypt(secret)
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == secret || !strings.HasPrefix(encrypted, "v1:") {
		t.Fatalf("密文格式异常: %q", encrypted)
	}
	if again, _ := c.Encrypt(secret); again == encrypted {
		t.Fatal("相同明文应产生不同密文（随机 nonce）")
	}
	plain, err := c.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if plain != secret {
		t.Fatalf("解密结果 = %q，期望 %q", plain, secret)
	}
}

func TestDecryptLegacyPlaintext(t *testing.T) {
	c, _ := NewCipher(testKey())
	plain, err := c.Decrypt("legacy-plain")
	if err != nil || plain != "legacy-plain" {
		t.Fatalf("历史明文应原样返回，实际 %q / %v", plain, err)
	}
}

func TestDecryptTampered(t *testing.T) {
	c, _ := NewCipher(testKey())
	encrypted, _ := c.Encrypt("secret")
	tampered := encrypted[:len(encrypted)-2] + "AA"
	if _, err := c.Decrypt(tampered); err == nil {
		t.Fatal("篡改密文应解密失败")
	}
}

func TestDecryptWithoutKey(t *testing.T) {
	c, _ := NewCipher(testKey())
	encrypted, _ := c.Encrypt("secret")
	if _, err := (*Cipher)(nil).Decrypt(encrypted); !errors.Is(err, ErrKeyRequired) {
		t.Fatalf("无密钥解密密文应返回 ErrKeyRequired，实际 %v", err)
	}
	if _, err := c.Encrypt(""); err != nil {
		t.Fatal(err)
	}
}
