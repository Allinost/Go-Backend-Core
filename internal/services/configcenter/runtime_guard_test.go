package configcenter

import (
	"context"
	"encoding/json"
	"testing"
)

func TestServiceDegradedRejectsWrites(t *testing.T) {
	svc, _ := testService(t)
	svc.SetDegraded("测试降级")
	if err := svc.Update(context.Background(), map[string]any{"app.name": "x"}, "t", ""); err == nil {
		t.Fatal("降级状态下 Update 应被拒绝")
	}
	if err := svc.Reset(context.Background(), []string{"app.name"}, "t", ""); err == nil {
		t.Fatal("降级状态下 Reset 应被拒绝")
	}
}

func TestServiceLoadSkipsUnknownKeys(t *testing.T) {
	r := testRegistry(t)
	store := NewMemoryStore()
	store.values["retired.key"] = json.RawMessage(`"old"`)
	svc := New(r, store)
	if err := svc.Load(context.Background()); err != nil {
		t.Fatalf("未知配置项不应导致加载失败: %v", err)
	}
	if _, ok := svc.Values(false)["retired.key"]; ok {
		t.Fatal("未知配置项不应进入运行值")
	}
}

func TestServiceResetRejectsUnknownKey(t *testing.T) {
	svc, _ := testService(t)
	if err := svc.Reset(context.Background(), []string{"no.such.key"}, "t", ""); err == nil {
		t.Fatal("未知 key 的 Reset 应被拒绝")
	}
}

func TestServiceEnvLockedRejected(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(Spec{Key: "sys.locked", Type: TypeString, Default: "a", EnvLocked: true}); err != nil {
		t.Fatal(err)
	}
	svc := New(r, NewMemoryStore())
	if _, err := svc.ValidatePatch(map[string]any{"sys.locked": "b"}); err == nil {
		t.Fatal("EnvLocked 项不应允许在线修改")
	}
}

func TestServiceMaskedPlaceholderOnlyIgnoredForSensitive(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(Spec{Key: "app.note", Type: TypeString, Default: ""}); err != nil {
		t.Fatal(err)
	}
	svc := New(r, NewMemoryStore())
	if err := svc.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := svc.Update(context.Background(), map[string]any{"app.note": "******abc"}, "t", ""); err != nil {
		t.Fatal(err)
	}
	if svc.Values(false)["app.note"] != "******abc" {
		t.Fatal("非敏感项应按原值保存，不得被当作脱敏占位符忽略")
	}
}
