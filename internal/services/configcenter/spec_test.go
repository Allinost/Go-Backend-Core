package configcenter

import (
	"encoding/json"
	"testing"
)

func testRegistry(t *testing.T) *Registry {
	t.Helper()
	r := NewRegistry()
	min := 1.0
	max := 10.0
	if err := r.Register(
		Spec{Key: "app.name", Module: "app", Type: TypeString, Default: "core", Group: "app"},
		Spec{Key: "app.workers", Module: "app", Type: TypeInt, Default: 2, Min: &min, Max: &max, Group: "app"},
		Spec{Key: "app.enabled", Module: "app", Type: TypeBool, Default: true, Group: "app"},
		Spec{Key: "app.timeout", Module: "app", Type: TypeDuration, Default: "5s", Group: "app"},
		Spec{Key: "app.token", Module: "app", Type: TypeSecret, Default: "", Sensitive: true, Group: "app"},
	); err != nil {
		t.Fatalf("Register 失败: %v", err)
	}
	return r
}

func TestRegistryRegisterAndDefaults(t *testing.T) {
	r := testRegistry(t)
	if len(r.Specs()) != 5 {
		t.Fatalf("specs 数量异常: %d", len(r.Specs()))
	}
	defaults := r.Defaults()
	if defaults["app.name"] != "core" {
		t.Errorf("默认值异常: %+v", defaults)
	}
	if defaults["app.workers"] != 2 {
		t.Errorf("int 默认值异常: %+v", defaults["app.workers"])
	}
}

func TestRegistryRegisterRejectsDuplicate(t *testing.T) {
	r := testRegistry(t)
	err := r.Register(Spec{Key: "app.name", Module: "app", Type: TypeString, Default: "x"})
	if err == nil {
		t.Fatal("重复 key 应报错")
	}
}

func TestRegistryValidate(t *testing.T) {
	r := testRegistry(t)

	if err := r.Validate(map[string]any{
		"app.name":    "core",
		"app.workers": float64(5),
		"app.enabled": true,
		"app.timeout": "10s",
	}); err != nil {
		t.Fatalf("合法配置校验失败: %v", err)
	}

	cases := []map[string]any{
		{"app.workers": float64(0)},
		{"app.workers": float64(11)},
		{"app.workers": "not-int"},
		{"app.enabled": "true"},
		{"app.timeout": "invalid"},
		{"unknown.key": "x"},
	}
	for i, values := range cases {
		if err := r.Validate(values); err == nil {
			t.Errorf("用例 %d 应校验失败: %+v", i, values)
		}
	}
}

func TestRegistryValidateEnum(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(Spec{
		Key: "log.level", Module: "log", Type: TypeString, Default: "info",
		Enum: []string{"debug", "info", "warn", "error"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.Validate(map[string]any{"log.level": "warn"}); err != nil {
		t.Errorf("合法枚举失败: %v", err)
	}
	if err := r.Validate(map[string]any{"log.level": "trace"}); err == nil {
		t.Error("非法枚举应失败")
	}
}

func TestRegistryMask(t *testing.T) {
	r := testRegistry(t)
	masked := r.Mask(map[string]any{
		"app.token": "super-secret-token",
		"app.name":  "core",
	})
	if masked["app.name"] != "core" {
		t.Errorf("非敏感项不应脱敏: %+v", masked["app.name"])
	}
	if masked["app.token"] == "super-secret-token" {
		t.Error("敏感项未脱敏")
	}
	if masked["app.token"] != "******oken" {
		t.Errorf("脱敏格式异常: %+v", masked["app.token"])
	}
}

func TestNormalizeValueJSON(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(Spec{Key: "app.meta", Module: "app", Type: TypeJSON, Default: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	normalized, ok := r.Get("app.meta")
	if !ok {
		t.Fatal("app.meta 未注册")
	}
	if _, ok := normalized.Default.(json.RawMessage); !ok {
		t.Errorf("JSON 默认值类型异常: %T", normalized.Default)
	}
}
