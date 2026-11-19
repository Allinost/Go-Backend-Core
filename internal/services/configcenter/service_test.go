package configcenter

import (
	"context"
	"encoding/json"
	"testing"
)

func testService(t *testing.T) (*Service, *MemoryStore) {
	t.Helper()
	r := testRegistry(t)
	store := NewMemoryStore()
	svc := New(r, store)
	if err := svc.Load(context.Background()); err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	return svc, store
}

func TestServiceUpdateAndMask(t *testing.T) {
	svc, _ := testService(t)
	var applied map[string]any
	svc.SetApply(func(values map[string]any) error {
		applied = values
		return nil
	})

	if err := svc.Update(context.Background(), map[string]any{
		"app.name":    "next",
		"app.workers": float64(4),
	}, "tester", "test"); err != nil {
		t.Fatalf("Update 失败: %v", err)
	}

	values := svc.Values(false)
	if values["app.name"] != "next" {
		t.Errorf("更新未生效: %+v", values)
	}
	if values["app.workers"] != 4 {
		t.Errorf("int 归一化异常: %#v", values["app.workers"])
	}
	if applied == nil || applied["app.name"] != "next" {
		t.Errorf("apply 未调用或值异常: %+v", applied)
	}

	masked := svc.Values(true)
	if masked["app.name"] != "next" {
		t.Errorf("非敏感项不应脱敏: %+v", masked["app.name"])
	}
}

func TestServiceUpdateRejectsInvalid(t *testing.T) {
	svc, _ := testService(t)
	if err := svc.Update(context.Background(), map[string]any{"app.workers": float64(0)}, "tester", ""); err == nil {
		t.Fatal("越界值应失败")
	}
	if err := svc.Update(context.Background(), map[string]any{"unknown.key": "x"}, "tester", ""); err == nil {
		t.Fatal("未知 key 应失败")
	}
}

func TestServiceResetAndHistory(t *testing.T) {
	svc, _ := testService(t)
	ctx := context.Background()

	if err := svc.Update(ctx, map[string]any{"app.name": "next"}, "tester", "update"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Reset(ctx, []string{"app.name"}, "tester", "reset"); err != nil {
		t.Fatal(err)
	}
	if got := svc.Values(false)["app.name"]; got != "core" {
		t.Errorf("重置未恢复默认值: %v", got)
	}

	history, err := svc.History(ctx, "app.name", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("历史条数异常: %d", len(history))
	}
	if history[0].Note != "reset" {
		t.Errorf("最新历史应为 reset: %+v", history[0])
	}
}

func TestServiceLoadFromStore(t *testing.T) {
	r := testRegistry(t)
	store := NewMemoryStore()
	if err := store.Save(context.Background(), map[string]json.RawMessage{
		"app.name": json.RawMessage(`"persisted"`),
	}, "seed", "seed"); err != nil {
		t.Fatal(err)
	}

	svc := New(r, store)
	if err := svc.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := svc.Values(false)["app.name"]; got != "persisted" {
		t.Errorf("持久化值未加载: %v", got)
	}
}
