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

type Cipher interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
	EncryptString(plaintext string) (string, error)
	DecryptString(ciphertext string) (string, error)
}

type AESGCM struct {
	key []byte
}

func NewAESGCM(key []byte) (*AESGCM, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, fmt.Errorf("crypto: AES 密钥长度必须为 16/24/32 字节, 当前 %d", len(key))
	}
	return &AESGCM{key: key}, nil
}

func NewAESGCMFromHex(hexKey string) (*AESGCM, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("crypto: 密钥 hex 解码失败: %w", err)
	}
	return NewAESGCM(key)
}

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

func (c *AESGCM) EncryptString(plaintext string) (string, error) {
	enc, err := c.Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(enc), nil
}

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

type ChaCha20 struct {
	key []byte
}

func NewChaCha20(key []byte) (*ChaCha20, error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("crypto: ChaCha20-Poly1305 密钥长度必须为 %d 字节, 当前 %d", chacha20poly1305.KeySize, len(key))
	}
	return &ChaCha20{key: key}, nil
}

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

func (c *ChaCha20) EncryptString(plaintext string) (string, error) {
	enc, err := c.Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(enc), nil
}

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

func deriveKey(password string, salt []byte, keyLen int) []byte {
	h := sha256.Sum256(append([]byte(password), salt...))
	key := h[:]
	for len(key) < keyLen {
		h = sha256.Sum256(append(h[:], salt...))
		key = append(key, h[:]...)
	}
	return key[:keyLen]
}
