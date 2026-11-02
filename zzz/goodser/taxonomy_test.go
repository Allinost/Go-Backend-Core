package goodser

import (
	"encoding/json"
	"testing"

	"github.com/Allinost/go-backend-core/internal/pkg/authz"
	"github.com/Allinost/go-backend-core/internal/services/action"
)

func TestComboAvailability(t *testing.T) {
	cases := []struct {
		name  string
		items []ComboItem
		want  int
	}{
		{"空组合", nil, 0},
		{"单组件", []ComboItem{{Quantity: 2, Available: 5}}, 2},
		{"多组件取最小", []ComboItem{{Quantity: 2, Available: 5}, {Quantity: 3, Available: 4}}, 1},
		{"组件不足", []ComboItem{{Quantity: 2, Available: 1}}, 0},
		{"非法组件数量", []ComboItem{{Quantity: 0, Available: 10}}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := comboAvailability(c.items); got != c.want {
				t.Fatalf("comboAvailability = %d，期望 %d", got, c.want)
			}
		})
	}
}

func TestValidDictionaryKind(t *testing.T) {
	for _, kind := range []string{"series", "character", "set_type", "product_type"} {
		if !ValidDictionaryKind(kind) {
			t.Fatalf("%s 应为合法类型", kind)
		}
	}
	for _, kind := range []string{"", "tag", "SERIES"} {
		if ValidDictionaryKind(kind) {
			t.Fatalf("%s 应为非法类型", kind)
		}
	}
}

func TestCustomerStockQueryReqNormalize(t *testing.T) {
	req := &CustomerStockQueryReq{}
	req.Normalize()
	if req.Page != 1 || req.PageSize != 20 {
		t.Fatalf("默认分页 = %d/%d", req.Page, req.PageSize)
	}

	req = &CustomerStockQueryReq{Page: 2, PageSize: 100}
	req.Normalize()
	if req.Page != 2 || req.PageSize != 20 {
		t.Fatalf("超限分页 = %d/%d", req.Page, req.PageSize)
	}
}

func TestDecodeParams(t *testing.T) {
	var target struct {
		InventoryID string `json:"inventory_id"`
	}
	if err := decodeParams(action.Request{Params: json.RawMessage(`{"inventory_id":"inv-1"}`)}, &target); err != nil {
		t.Fatalf("decodeParams: %v", err)
	}
	if target.InventoryID != "inv-1" {
		t.Fatalf("inventory_id = %q", target.InventoryID)
	}

	if err := decodeParams(action.Request{Params: json.RawMessage(`{`)}, &target); err == nil {
		t.Fatal("非法 JSON 应返回错误")
	}
}

func TestRegisterActions(t *testing.T) {
	registry := action.NewRegistry()
	if err := RegisterActions(registry, &Service{}); err != nil {
		t.Fatalf("RegisterActions: %v", err)
	}

	defs := registry.List()
	if len(defs) != 19 {
		t.Fatalf("动作数量 = %d，期望 19", len(defs))
	}

	required := map[string]string{
		"goodser.stock.query":            authz.ScopeOwnerRead,
		"goodser.order.create":           authz.ScopeOwnerWrite,
		"goodser.customer.stock_query":   authz.ScopeCustomerRead,
		"goodser.customer.order_request": authz.ScopeCustomerOrderReq,
	}
	for name, scope := range required {
		def, ok := registry.Get(name)
		if !ok {
			t.Fatalf("缺少动作 %s", name)
		}
		found := false
		for _, item := range def.Scopes {
			if item == scope {
				found = true
			}
		}
		if !found {
			t.Fatalf("动作 %s 作用域 = %v，缺少 %s", name, def.Scopes, scope)
		}
	}

	gateway := action.NewGateway(registry)
	if _, err := gateway.Invoke(nil, action.Request{Action: "goodser.customer.order_request", Scopes: authz.ScopeCustomerRead}); err == nil {
		t.Fatal("客户只读作用域不应能提交订单")
	}
}
