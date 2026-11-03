package goodser

import (
	"encoding/json"
	"testing"

	"github.com/Allinost/go-backend-core/internal/services/action"
)

func TestActionParamsSchemasValid(t *testing.T) {
	for name, raw := range actionParamsSchemas {
		if !json.Valid(raw) {
			t.Fatalf("%s 的 params schema 不是合法 JSON", name)
		}
		var schema map[string]any
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatalf("%s 解析失败: %v", name, err)
		}
		if schema["type"] != "object" {
			t.Fatalf("%s schema type = %v, want object", name, schema["type"])
		}
	}
}

func TestActionParamsSchemasAttached(t *testing.T) {
	registry := action.NewRegistry()
	if err := RegisterActions(registry, nil); err != nil {
		t.Fatalf("RegisterActions: %v", err)
	}
	def, ok := registry.Get("goodser.stock.query")
	if !ok {
		t.Fatalf("goodser.stock.query 未注册")
	}
	if len(def.ParamsSchema) == 0 {
		t.Fatalf("goodser.stock.query 未挂载 params schema")
	}
	if string(def.ParamsSchema) == "" || !json.Valid(def.ParamsSchema) {
		t.Fatalf("params schema 无效: %s", def.ParamsSchema)
	}
}
