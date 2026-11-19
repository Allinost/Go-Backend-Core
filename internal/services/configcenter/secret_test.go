package configcenter

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type prefixCipher struct{}

func (prefixCipher) EncryptString(plain string) (string, error) { return "x" + plain, nil }

func (prefixCipher) DecryptString(encrypted string) (string, error) {
	if !strings.HasPrefix(encrypted, "x") {
		return "", errors.New("bad ciphertext")
	}
	return strings.TrimPrefix(encrypted, "x"), nil
}

func TestServiceEncryptsSensitiveValues(t *testing.T) {
	ctx := context.Background()
	svc, store := testService(t)
	svc.SetSecretCipher(prefixCipher{})

	if err := svc.Update(ctx, map[string]any{"app.token": "jd-pass"}, "tester", "secret"); err != nil {
		t.Fatalf("Update 失败: %v", err)
	}
	if got := svc.Values(false)["app.token"]; got != "jd-pass" {
		t.Errorf("内存应保留明文: %v", got)
	}

	raw, err := store.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := json.Unmarshal(raw["app.token"], &stored); err != nil {
		t.Fatal(err)
	}
	if stored != secretPrefix+"xjd-pass" {
		t.Errorf("落库应为密文: %q", stored)
	}

	reloaded := New(svc.registry, store)
	reloaded.SetSecretCipher(prefixCipher{})
	if err := reloaded.Load(ctx); err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if got := reloaded.Values(false)["app.token"]; got != "jd-pass" {
		t.Errorf("加载应解密: %v", got)
	}
}

func TestServiceIgnoresMaskedSensitiveValue(t *testing.T) {
	ctx := context.Background()
	svc, _ := testService(t)
	svc.SetSecretCipher(prefixCipher{})

	if err := svc.Update(ctx, map[string]any{"app.token": "jd-pass"}, "tester", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.Update(ctx, map[string]any{"app.token": "******pass"}, "tester", ""); err != nil {
		t.Fatalf("脱敏占位符不应报错: %v", err)
	}
	if got := svc.Values(false)["app.token"]; got != "jd-pass" {
		t.Errorf("脱敏占位符不应覆盖原值: %v", got)
	}
}

func TestServiceEncryptedValueRequiresCipher(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	if err := store.Save(ctx, map[string]json.RawMessage{
		"app.token": json.RawMessage(`"` + secretPrefix + `xsecret"`),
	}, "seed", "seed"); err != nil {
		t.Fatal(err)
	}

	svc := New(testRegistry(t), store)
	if err := svc.Load(ctx); err == nil {
		t.Fatal("缺少密钥时应加载失败")
	}
}
