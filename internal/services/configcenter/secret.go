package configcenter

import (
	"errors"
	"fmt"
	"strings"
)

// SecretCipher 敏感配置加解密。由 settings 模块用 crypto 密钥注入。
type SecretCipher interface {
	EncryptString(plaintext string) (string, error)
	DecryptString(ciphertext string) (string, error)
}

// secretPrefix 标记已加密的配置值，避免重复加密并兼容历史明文。
const secretPrefix = "enc:v1:"

// maskedPrefix 与 Registry.Mask 保持一致，用于拒绝把脱敏占位符写回。
const maskedPrefix = "******"

// SetSecretCipher 设置敏感项加解密器；未设置时敏感项按明文保存（启动时会告警）。
func (s *Service) SetSecretCipher(cipher SecretCipher) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cipher = cipher
}

// isMasked 判断前端是否回传了脱敏占位符。
func isMasked(value any) bool {
	s, ok := value.(string)
	return ok && strings.HasPrefix(s, maskedPrefix)
}

// sealSecret 加密敏感值；空值和已加密值原样返回。
func (s *Service) sealSecret(spec Spec, value any) (any, error) {
	if !spec.Sensitive {
		return value, nil
	}
	plain, ok := value.(string)
	if !ok || plain == "" || strings.HasPrefix(plain, secretPrefix) {
		return value, nil
	}
	cipher := s.secretCipher()
	if cipher == nil {
		return value, nil
	}
	encrypted, err := cipher.EncryptString(plain)
	if err != nil {
		return nil, fmt.Errorf("configcenter: %s 加密失败: %w", spec.Key, err)
	}
	return secretPrefix + encrypted, nil
}

// openSecret 解密敏感值；兼容历史明文。
func (s *Service) openSecret(spec Spec, value any) (any, error) {
	if !spec.Sensitive {
		return value, nil
	}
	stored, ok := value.(string)
	if !ok || !strings.HasPrefix(stored, secretPrefix) {
		return value, nil
	}
	cipher := s.secretCipher()
	if cipher == nil {
		return nil, errors.New("configcenter: 存在加密配置但未配置 APP_CRYPTO_MASTER_KEY_HEX")
	}
	plain, err := cipher.DecryptString(strings.TrimPrefix(stored, secretPrefix))
	if err != nil {
		return nil, fmt.Errorf("configcenter: %s 解密失败: %w", spec.Key, err)
	}
	return plain, nil
}

func (s *Service) secretCipher() SecretCipher {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cipher
}
