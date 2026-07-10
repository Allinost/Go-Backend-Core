package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
)

// Cipher 加密解密接口，支持字节数组和字符串操作
type Cipher interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
	EncryptString(plaintext string) (string, error)
	DecryptString(ciphertext string) (string, error)
}

// AESGCM AES-GCM 加密实现
type AESGCM struct {
	key []byte // AES 密钥（16/24/32 字节）
}

// NewAESGCM 根据字节密钥创建 AES-GCM 加密器
func NewAESGCM(key []byte) (*AESGCM, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, fmt.Errorf("crypto: AES 密钥长度必须为 16/24/32 字节, 当前 %d", len(key))
	}
	return &AESGCM{key: key}, nil
}

// NewAESGCMFromHex 根据 hex 编码的密钥字符串创建 AES-GCM 加密器
func NewAESGCMFromHex(hexKey string) (*AESGCM, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("crypto: 密钥 hex 解码失败: %w", err)
	}
	return NewAESGCM(key)
}

// Encrypt 使用 AES-GCM 加密明文，返回 nonce+密文
func (c *AESGCM) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("crypto: AES 初始化失败: %w", err)
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: GCM 初始化失败: %w", err)
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: 随机数生成失败: %w", err)
	}
	ciphertext := aesGCM.Seal(nil, nonce, plaintext, nil)
	return append(nonce, ciphertext...), nil
}

// Decrypt 使用 AES-GCM 解密密文（前 nonceSize 字节为 nonce）
func (c *AESGCM) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("crypto: AES 初始化失败: %w", err)
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: GCM 初始化失败: %w", err)
	}
	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("crypto: 密文太短")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("crypto: 解密失败: %w", err)
	}
	return plaintext, nil
}

// EncryptString 加密字符串并返回 hex 编码结果
func (c *AESGCM) EncryptString(plaintext string) (string, error) {
	enc, err := c.Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(enc), nil
}

// DecryptString 解密 hex 编码的密文并返回原始字符串
func (c *AESGCM) DecryptString(ciphertext string) (string, error) {
	data, err := hex.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("crypto: hex 解码失败: %w", err)
	}
	dec, err := c.Decrypt(data)
	if err != nil {
		return "", err
	}
	return string(dec), nil
}

// ChaCha20 ChaCha20-Poly1305 加密实现
type ChaCha20 struct {
	key []byte // 密钥（固定 32 字节）
}

// NewChaCha20 根据字节密钥创建 ChaCha20-Poly1305 加密器
func NewChaCha20(key []byte) (*ChaCha20, error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("crypto: ChaCha20-Poly1305 密钥长度必须为 %d 字节, 当前 %d", chacha20poly1305.KeySize, len(key))
	}
	return &ChaCha20{key: key}, nil
}

// Encrypt 使用 ChaCha20-Poly1305 加密明文，返回 nonce+密文
func (c *ChaCha20) Encrypt(plaintext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(c.key)
	if err != nil {
		return nil, fmt.Errorf("crypto: ChaCha20 初始化失败: %w", err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: 随机数生成失败: %w", err)
	}
	ciphertext := aead.Seal(nil, nonce, plaintext, nil)
	return append(nonce, ciphertext...), nil
}

// Decrypt 使用 ChaCha20-Poly1305 解密密文（前 nonceSize 字节为 nonce）
func (c *ChaCha20) Decrypt(ciphertext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(c.key)
	if err != nil {
		return nil, fmt.Errorf("crypto: ChaCha20 初始化失败: %w", err)
	}
	nonceSize := aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("crypto: 密文太短")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("crypto: 解密失败: %w", err)
	}
	return plaintext, nil
}

// EncryptString 加密字符串并返回 hex 编码结果
func (c *ChaCha20) EncryptString(plaintext string) (string, error) {
	enc, err := c.Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(enc), nil
}

// DecryptString 解密 hex 编码的密文并返回原始字符串
func (c *ChaCha20) DecryptString(ciphertext string) (string, error) {
	data, err := hex.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("crypto: hex 解码失败: %w", err)
	}
	dec, err := c.Decrypt(data)
	if err != nil {
		return "", err
	}
	return string(dec), nil
}

// deriveKey 基于密码和盐值通过多次 SHA256 迭代派生指定长度的密钥
func deriveKey(password string, salt []byte, keyLen int) []byte {
	h := sha256.Sum256(append([]byte(password), salt...))
	key := h[:]
	for len(key) < keyLen {
		h = sha256.Sum256(append(h[:], salt...))
		key = append(key, h[:]...)
	}
	return key[:keyLen]
}
