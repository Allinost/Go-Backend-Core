// Package crypto 提供应用层敏感字段加密。
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

const cipherPrefix = "v1:"

// ErrKeyRequired 未配置主密钥
var ErrKeyRequired = errors.New("crypto: 未配置 master_key_hex")

// Cipher AES-256-GCM 加解密器，用于连接器密钥等敏感字段。
type Cipher struct {
	aead cipher.AEAD
}

// NewCipher 从 32 字节十六进制主密钥创建加解密器。
func NewCipher(masterKeyHex string) (*Cipher, error) {
	raw := strings.TrimSpace(masterKeyHex)
	if raw == "" {
		return nil, ErrKeyRequired
	}
	key, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("crypto: master_key_hex 不是合法十六进制: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("crypto: master_key_hex 必须为 32 字节（64 个十六进制字符），实际 %d 字节", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// Encrypt 加密明文，返回 v1:<base64(nonce+ciphertext)>。
func (c *Cipher) Encrypt(plain string) (string, error) {
	if c == nil || c.aead == nil {
		return "", ErrKeyRequired
	}
	if plain == "" {
		return "", nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: 生成 nonce 失败: %w", err)
	}
	sealed := c.aead.Seal(nil, nonce, []byte(plain), nil)
	return cipherPrefix + base64.RawURLEncoding.EncodeToString(append(nonce, sealed...)), nil
}

// Decrypt 解密 v1 前缀密文；无前缀内容视为历史明文直接返回。
func (c *Cipher) Decrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, cipherPrefix) {
		return value, nil
	}
	if c == nil || c.aead == nil {
		return "", ErrKeyRequired
	}
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, cipherPrefix))
	if err != nil {
		return "", fmt.Errorf("crypto: 密文格式错误: %w", err)
	}
	nonceSize := c.aead.NonceSize()
	if len(data) <= nonceSize {
		return "", errors.New("crypto: 密文长度不足")
	}
	plain, err := c.aead.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("crypto: 解密失败: %w", err)
	}
	return string(plain), nil
}
