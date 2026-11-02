package action

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/authz"
)

func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	reg := NewRegistry()
	if err := reg.Register(Definition{
		Name:        "stock.read",
		Description: "读取库存",
		Scopes:      []string{authz.ScopeOwnerRead, authz.ScopeCustomerRead},
		Handler: func(ctx context.Context, req Request) (any, error) {
			return map[string]string{"product": "p1"}, nil
		},
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := reg.Register(Definition{
		Name:     "order.request",
		Scopes:   []string{authz.ScopeCustomerOrderReq},
		Mutating: true,
		Handler: func(ctx context.Context, req Request) (any, error) {
			var params map[string]any
			if len(req.Params) > 0 {
				if err := json.Unmarshal(req.Params, &params); err != nil {
					return nil, err
				}
			}
			return params, nil
		},
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return reg
}

func TestRegistryRegisterValidation(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(Definition{Name: " ", Handler: func(context.Context, Request) (any, error) { return nil, nil }}); !errors.Is(err, ErrInvalidDefinition) {
		t.Fatalf("空名称错误 = %v", err)
	}
	if err := reg.Register(Definition{Name: "x"}); !errors.Is(err, ErrInvalidDefinition) {
		t.Fatalf("空 Handler 错误 = %v", err)
	}
	def := Definition{Name: "x", Handler: func(context.Context, Request) (any, error) { return nil, nil }}
	if err := reg.Register(def); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := reg.Register(def); !errors.Is(err, ErrAlreadyRegistered) {
		t.Fatalf("重复注册错误 = %v", err)
	}
}

func TestRegistryListSorted(t *testing.T) {
	reg := newTestRegistry(t)
	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("len = %d", len(list))
	}
	if list[0].Name != "order.request" || list[1].Name != "stock.read" {
		t.Fatalf("排序错误: %s, %s", list[0].Name, list[1].Name)
	}
}

func TestGatewayInvokeScopes(t *testing.T) {
	gw := NewGateway(newTestRegistry(t))
	ctx := context.Background()

	result, err := gw.Invoke(ctx, Request{Action: "stock.read", Scopes: authz.ScopeCustomerRead})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.Action != "stock.read" {
		t.Fatalf("result = %+v", result)
	}
	data, ok := result.Data.(map[string]string)
	if !ok || data["product"] != "p1" {
		t.Fatalf("data = %+v", result.Data)
	}

	if _, err := gw.Invoke(ctx, Request{Action: "stock.read", Scopes: authz.ScopeOwnerWrite}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("缺少作用域错误 = %v", err)
	}

	if _, err := gw.Invoke(ctx, Request{Action: "unknown", Scopes: "*"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("未知动作错误 = %v", err)
	}
}

func TestGatewayInvokePassesParams(t *testing.T) {
	gw := NewGateway(newTestRegistry(t))
	result, err := gw.Invoke(context.Background(), Request{
		Action: "order.request",
		Scopes: authz.ScopeCustomerOrderReq,
		Params: json.RawMessage(`{"product_id":"p1","quantity":2}`),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	params := result.Data.(map[string]any)
	if params["product_id"] != "p1" {
		t.Fatalf("params = %+v", params)
	}
}

func TestGatewayInvokeHandlerError(t *testing.T) {
	reg := NewRegistry()
	expected := errors.New("boom")
	reg.MustRegister(Definition{
		Name:   "fail",
		Scopes: []string{authz.ScopeOwnerWrite},
		Handler: func(context.Context, Request) (any, error) {
			return nil, expected
		},
	})
	gw := NewGateway(reg)
	gw.now = func() time.Time { return time.Unix(100, 0) }
	_, err := gw.Invoke(context.Background(), Request{Action: "fail", Scopes: authz.ScopeOwnerWrite})
	if !errors.Is(err, expected) {
		t.Fatalf("错误 = %v，期望 %v", err, expected)
	}
}

func TestGatewayWildcardScope(t *testing.T) {
	gw := NewGateway(newTestRegistry(t))
	if _, err := gw.Invoke(context.Background(), Request{Action: "order.request", Scopes: "*:*"}); err != nil {
		t.Fatalf("通配作用域应放行: %v", err)
	}
}
