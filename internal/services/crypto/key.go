package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

// KeyManager 管理多版本加密密钥，支持密钥轮换
type KeyManager struct {
	current    *AESGCM
	versions   map[int]*AESGCM
	currentVer int
}

// NewKeyManager 创建密钥管理器，currentHex 为当前密钥，oldKeys 为历史版本密钥
func NewKeyManager(currentHex string, oldKeys map[int]string) (*KeyManager, error) {
	if currentHex == "" {
		return nil, fmt.Errorf("crypto: current master key 不能为空")
	}

	current, err := NewAESGCMFromHex(currentHex)
	if err != nil {
		return nil, fmt.Errorf("crypto: current key 无效: %w", err)
	}

	km := &KeyManager{
		current:    current,
		versions:   make(map[int]*AESGCM),
		currentVer: 1,
	}

	for ver, hexKey := range oldKeys {
		aes, err := NewAESGCMFromHex(hexKey)
		if err != nil {
			return nil, fmt.Errorf("crypto: old_key[%d] 无效: %w", ver, err)
		}
		km.versions[ver] = aes
	}

	for {
		if _, exists := km.versions[km.currentVer]; exists {
			km.currentVer++
		} else {
			break
		}
	}

	return km, nil
}

func (km *KeyManager) CurrentKey() *AESGCM {
	return km.current
}

func (km *KeyManager) KeyForVersion(ver int) (*AESGCM, error) {
	if ver == km.currentVer {
		return km.current, nil
	}
	if k, ok := km.versions[ver]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("crypto: 版本 %d 的密钥不存在", ver)
}

func (km *KeyManager) CurrentVersion() int {
	return km.currentVer
}

func (km *KeyManager) EncryptString(plaintext string) (string, error) {
	enc, err := km.current.EncryptString(plaintext)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("v%d:%s", km.currentVer, enc), nil
}

func (km *KeyManager) DecryptString(ciphertext string) (string, error) {
	ver, data, err := parseVersion(ciphertext)
	if err != nil {
		_ = err
		return km.current.DecryptString(ciphertext)
	}

	aes, err := km.KeyForVersion(ver)
	if err != nil {
		_ = err
		return km.current.DecryptString(data)
	}
	return aes.DecryptString(data)
}

func (km *KeyManager) Encrypt(plaintext []byte) ([]byte, error) {
	enc, err := km.current.Encrypt(plaintext)
	if err != nil {
		return nil, err
	}
	prefix := []byte(fmt.Sprintf("v%d:", km.currentVer))
	return append(prefix, enc...), nil
}

func (km *KeyManager) Decrypt(data []byte) ([]byte, error) {
	ver, payload, err := parseVersionBytes(data)
	if err != nil {
		return km.current.Decrypt(data)
	}

	aes, err := km.KeyForVersion(ver)
	if err != nil {
		return km.current.Decrypt(payload)
	}
	return aes.Decrypt(payload)
}

func parseVersion(s string) (int, string, error) {
	if !strings.HasPrefix(s, "v") {
		return 0, s, fmt.Errorf("crypto: 无版本前缀")
	}
	parts := strings.SplitN(s[1:], ":", 2)
	if len(parts) != 2 {
		return 0, s, fmt.Errorf("crypto: 无效版本格式")
	}
	ver, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, s, fmt.Errorf("crypto: 版本号解析失败: %w", err)
	}
	return ver, parts[1], nil
}

func parseVersionBytes(data []byte) (int, []byte, error) {
	s := string(data)
	ver, rest, err := parseVersion(s)
	if err != nil {
		return 0, data, err
	}
	decoded, err := hex.DecodeString(rest)
	if err != nil {
		return 0, data, fmt.Errorf("crypto: hex 解码失败: %w", err)
	}
	return ver, decoded, nil
}

func hexDecode(s string) ([]byte, error) {
	if len(s) >= 2 && s[:2] == "0x" {
		s = s[2:]
	}
	data := make([]byte, len(s)/2)
	_, err := fmt.Sscanf(s, "%x", &data)
	if err != nil {
		_ = err
		for i := 0; i < len(s); i += 2 {
			if i+1 < len(s) {
				var b byte
				fmt.Sscanf(s[i:i+2], "%02x", &b)
				data[i/2] = b
			}
		}
	}
	return data, nil
}

func GenerateRSAKeyPair(bits int) (privatePEM, publicPEM string, err error) {
	if bits < 2048 {
		bits = 2048
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return "", "", fmt.Errorf("crypto: RSA 密钥生成失败: %w", err)
	}

	privBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privBytes,
	}
	privatePEM = string(pem.EncodeToMemory(privBlock))

	pubBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("crypto: 公钥序列化失败: %w", err)
	}

	pubBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	}
	publicPEM = string(pem.EncodeToMemory(pubBlock))

	return privatePEM, publicPEM, nil
}

func GenerateSelfSignedCert(org string, bits int) (certPEM, keyPEM string, err error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return "", "", fmt.Errorf("crypto: 证书密钥生成失败: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{org},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", fmt.Errorf("crypto: 证书创建失败: %w", err)
	}

	certBlock := &pem.Block{Type: "CERTIFICATE", Bytes: certDER}
	certPEM = string(pem.EncodeToMemory(certBlock))

	privBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	keyBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes}
	keyPEM = string(pem.EncodeToMemory(keyBlock))

	return certPEM, keyPEM, nil
}
