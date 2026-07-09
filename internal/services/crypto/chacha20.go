package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
)

type ChaCha20Poly1305 struct {
	key []byte
}

func NewChaCha20Poly1305(key []byte) (*ChaCha20Poly1305, error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("crypto: ChaCha20-Poly1305 密钥长度必须为 %d 字节, 当前 %d", chacha20poly1305.KeySize, len(key))
	}
	return &ChaCha20Poly1305{key: key}, nil
}

func NewChaCha20Poly1305FromHex(hexKey string) (*ChaCha20Poly1305, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("crypto: 密钥 hex 解码失败: %w", err)
	}
	return NewChaCha20Poly1305(key)
}

func (c *ChaCha20Poly1305) Encrypt(plaintext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(c.key)
	if err != nil {
		return nil, fmt.Errorf("crypto: ChaCha20-Poly1305 初始化失败: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: 随机数生成失败: %w", err)
	}

	ciphertext := aead.Seal(nil, nonce, plaintext, nil)
	return append(nonce, ciphertext...), nil
}

func (c *ChaCha20Poly1305) Decrypt(ciphertext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(c.key)
	if err != nil {
		return nil, fmt.Errorf("crypto: ChaCha20-Poly1305 初始化失败: %w", err)
	}

	nonceSize := aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("crypto: 密文太短")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("crypto: 解密失败: %w", err)
	}

	return plaintext, nil
}

func (c *ChaCha20Poly1305) EncryptString(plaintext string) (string, error) {
	enc, err := c.Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(enc), nil
}

func (c *ChaCha20Poly1305) DecryptString(ciphertext string) (string, error) {
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
