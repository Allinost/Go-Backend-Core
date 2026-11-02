package integration

import (
	"errors"
	"strings"
	"testing"

	"github.com/Allinost/go-backend-core/internal/pkg/crypto"
	"github.com/Allinost/go-backend-core/internal/services/connector"
)

func testCipher(t *testing.T) *crypto.Cipher {
	t.Helper()
	c, err := crypto.NewCipher(strings.Repeat("ab", 32))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestBuildConfigEncryptsSecret(t *testing.T) {
	c := testCipher(t)
	m := NewConnectorManager(nil, c, connector.NewRegistry())
	secret := "hook-secret"
	cfg, err := m.buildConfig(&ConnectorReq{
		Name: "n8n", URL: "https://example.com/hook", Secret: &secret,
		Topics: []string{"goodser.*", " goodser.* ", ""},
	}, ConnectorConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Secret == "" || cfg.Secret == secret {
		t.Fatalf("secret 应加密保存，实际 %q", cfg.Secret)
	}
	if !strings.HasPrefix(cfg.Secret, "v1:") {
		t.Fatalf("密文前缀异常: %q", cfg.Secret)
	}
	plain, err := c.Decrypt(cfg.Secret)
	if err != nil || plain != secret {
		t.Fatalf("解密失败: %q / %v", plain, err)
	}
	if len(cfg.Topics) != 1 || cfg.Topics[0] != "goodser.*" {
		t.Fatalf("topics 归一化异常: %v", cfg.Topics)
	}
}

func TestBuildConfigSecretRequiresCipher(t *testing.T) {
	m := NewConnectorManager(nil, nil, connector.NewRegistry())
	secret := "hook-secret"
	_, err := m.buildConfig(&ConnectorReq{URL: "https://example.com/hook", Secret: &secret}, ConnectorConfig{})
	if !errors.Is(err, connector.ErrInvalidConfig) {
		t.Fatalf("缺少密钥时应拒绝保存 secret，实际 %v", err)
	}
}

func TestBuildConfigUpdatePreservesSecret(t *testing.T) {
	c := testCipher(t)
	m := NewConnectorManager(nil, c, connector.NewRegistry())
	existing := ConnectorConfig{URL: "https://example.com/hook", Secret: "v1:keep-me", Topics: []string{"a.*"}}
	cfg, err := m.buildConfig(&ConnectorReq{URL: "https://example.com/hook2"}, existing)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Secret != "v1:keep-me" {
		t.Fatalf("secret 应保持不变，实际 %q", cfg.Secret)
	}
	if cfg.URL != "https://example.com/hook2" {
		t.Fatalf("url 未更新: %q", cfg.URL)
	}
	if len(cfg.Topics) != 1 || cfg.Topics[0] != "a.*" {
		t.Fatalf("topics 应保持不变: %v", cfg.Topics)
	}

	empty := ""
	cfg, err = m.buildConfig(&ConnectorReq{URL: "https://example.com/hook2", Secret: &empty}, existing)
	if err != nil || cfg.Secret != "" {
		t.Fatalf("空 secret 应清除，实际 %q / %v", cfg.Secret, err)
	}
}

func TestBuildConfigValidation(t *testing.T) {
	m := NewConnectorManager(nil, testCipher(t), connector.NewRegistry())
	if _, err := m.buildConfig(&ConnectorReq{}, ConnectorConfig{}); err == nil {
		t.Fatal("缺少 url 应报错")
	}
	if _, err := m.buildConfig(&ConnectorReq{URL: "https://x", Type: "feishu"}, ConnectorConfig{}); err == nil {
		t.Fatal("非 webhook 类型应报错")
	}
	if _, err := m.buildConfig(&ConnectorReq{URL: "https://x", TimeoutSeconds: 999}, ConnectorConfig{}); err == nil {
		t.Fatal("超时上限应校验")
	}
	if _, err := normalizeTopics([]string{strings.Repeat("a", 101)}); err == nil {
		t.Fatal("topic 长度应校验")
	}
}

func TestToConnectorDTOHidesSecret(t *testing.T) {
	dto := toConnectorDTO(connectorRecord{ID: "c1", Name: "n8n", Type: "webhook"}, ConnectorConfig{
		URL: "https://example.com/hook", Secret: "v1:xxx",
	})
	if dto.SecretSet != true {
		t.Fatal("secret_set 应为 true")
	}
	if strings.Contains(dto.URL, "v1:") {
		t.Fatal("DTO 不应包含密文")
	}
}
