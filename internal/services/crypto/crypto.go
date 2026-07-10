package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
)

// Cipher 定义加密/解密接口
type Cipher interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
	EncryptString(plaintext string) (string, error)
	DecryptString(ciphertext string) (string, error)
}

// AESGCM 封装 AES-GCM 加密，密钥长度支持 16/24/32 字节
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

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
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

func NewAESGCMFromPassword(password string, salt []byte) (*AESGCM, error) {
	if salt == nil {
		salt = make([]byte, 16)
		if _, err := rand.Read(salt); err != nil {
			return nil, fmt.Errorf("crypto: salt 生成失败: %w", err)
		}
	}
	key := deriveKey(password, salt, 32)
	return NewAESGCM(key)
}

type RSAOAEP struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

func NewRSAOAEPFromPrivateKey(pemStr string) (*RSAOAEP, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("crypto: PEM 解码失败")
	}

	var priv *rsa.PrivateKey
	var err error

	if block.Type == "RSA PRIVATE KEY" {
		priv, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	} else {
		key, parseErr := x509.ParsePKCS8PrivateKey(block.Bytes)
		if parseErr != nil {
			return nil, fmt.Errorf("crypto: 私钥解析失败: %w", parseErr)
		}
		var ok bool
		priv, ok = key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("crypto: 密钥不是 RSA 私钥")
		}
		_ = err
	}
	if err != nil {
		return nil, fmt.Errorf("crypto: 私钥解析失败: %w", err)
	}

	return &RSAOAEP{
		privateKey: priv,
		publicKey:  &priv.PublicKey,
	}, nil
}

func NewRSAOAEPFromPublicKey(pemStr string) (*RSAOAEP, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("crypto: PEM 解码失败")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("crypto: 公钥解析失败: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("crypto: 密钥不是 RSA 公钥")
	}

	return &RSAOAEP{publicKey: rsaPub}, nil
}

func (c *RSAOAEP) Encrypt(plaintext []byte) ([]byte, error) {
	if c.publicKey == nil {
		return nil, errors.New("crypto: 未设置公钥")
	}
	return rsa.EncryptOAEP(sha256.New(), rand.Reader, c.publicKey, plaintext, nil)
}

func (c *RSAOAEP) Decrypt(ciphertext []byte) ([]byte, error) {
	if c.privateKey == nil {
		return nil, errors.New("crypto: 未设置私钥")
	}
	return rsa.DecryptOAEP(sha256.New(), rand.Reader, c.privateKey, ciphertext, nil)
}

func (c *RSAOAEP) EncryptString(plaintext string) (string, error) {
	enc, err := c.Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(enc), nil
}

func (c *RSAOAEP) DecryptString(ciphertext string) (string, error) {
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
