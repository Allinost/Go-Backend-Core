package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Allinost/go-backend-core/internal/pkg/authz"
	"github.com/Allinost/go-backend-core/internal/services/action"
	"github.com/Allinost/go-backend-core/internal/services/connector"
	"github.com/Allinost/go-backend-core/internal/services/outbox"
	"github.com/gin-gonic/gin"
)

func newMCPTestHandler(t *testing.T) *Handler {
	t.Helper()
	registry := action.NewRegistry()
	registry.MustRegister(action.Definition{
		Name:        "goodser.stock.query",
		Description: "库存查询",
		Scopes:      []string{authz.ScopeOwnerRead},
		ParamsSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"keyword":{"type":"string"}},
			"required":["keyword"],
			"additionalProperties":false
		}`),
		Handler: func(_ context.Context, req action.Request) (any, error) {
			var params map[string]any
			_ = json.Unmarshal(req.Params, &params)
			return map[string]any{"params": params, "actor": req.Actor}, nil
		},
	})
	registry.MustRegister(action.Definition{
		Name:        "goodser.order.create",
		Description: "创建订单",
		Scopes:      []string{authz.ScopeOwnerWrite},
		Mutating:    true,
		Handler: func(_ context.Context, req action.Request) (any, error) {
			return map[string]any{"created": true, "request_id": req.RequestID}, nil
		},
	})
	registry.MustRegister(action.Definition{
		Name:        "goodser.customer.order_request",
		Description: "客户下单",
		Scopes:      []string{authz.ScopeCustomerOrderReq},
		Mutating:    true,
		Handler: func(_ context.Context, req action.Request) (any, error) {
			return map[string]any{"order_no": "RSV-1"}, nil
		},
	})
	return NewHandler(action.NewGateway(registry), outbox.NewService(outbox.NewMemoryStore()), connector.NewRegistry(), nil)
}

func newMCPTestRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/mcp", func(c *gin.Context) {
		c.Set("auth_method", "apikey")
		c.Set("api_key_scopes", c.GetHeader("X-Test-Scopes"))
		c.Set("username", "tester")
		h.MCP(c)
	})
	return r
}

type testMCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *mcpError       `json:"error"`
}

func callMCP(t *testing.T, router *gin.Engine, body, scopes string) (*httptest.ResponseRecorder, testMCPResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Scopes", scopes)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp testMCPResponse
	if w.Body.Len() > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("解析响应失败: %v, body=%s", err, w.Body.String())
		}
	}
	return w, resp
}

func TestMCPInitialize(t *testing.T) {
	h := newMCPTestHandler(t)
	router := newMCPTestRouter(h)

	w, resp := callMCP(t, router, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`, authz.ScopeOwnerRead)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if resp.Error != nil {
		t.Fatalf("error = %+v", resp.Error)
	}
	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name string `json:"name"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("解析 result 失败: %v", err)
	}
	if result.ProtocolVersion != "2025-03-26" {
		t.Fatalf("protocolVersion = %s", result.ProtocolVersion)
	}
	if result.ServerInfo.Name != "go-backend-core-integration" {
		t.Fatalf("serverInfo.name = %s", result.ServerInfo.Name)
	}
}

func TestMCPToolsListFiltersByScope(t *testing.T) {
	h := newMCPTestHandler(t)
	router := newMCPTestRouter(h)

	_, resp := callMCP(t, router, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, authz.ScopeOwnerRead)
	if resp.Error != nil {
		t.Fatalf("error = %+v", resp.Error)
	}
	var ownerList struct {
		Tools []mcpTool `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &ownerList); err != nil {
		t.Fatalf("解析 tools 失败: %v", err)
	}
	if len(ownerList.Tools) != 1 || ownerList.Tools[0].Name != "goodser_stock_query" {
		t.Fatalf("owner tools = %+v", ownerList.Tools)
	}
	properties, _ := ownerList.Tools[0].InputSchema["properties"].(map[string]any)
	params, _ := properties["params"].(map[string]any)
	paramProps, _ := params["properties"].(map[string]any)
	if _, ok := paramProps["keyword"]; !ok {
		t.Fatalf("params schema 未生效: %+v", ownerList.Tools[0].InputSchema)
	}
	if _, ok := properties["request_id"]; ok {
		t.Fatalf("只读工具不应包含 request_id: %+v", properties)
	}

	_, resp = callMCP(t, router, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, authz.ScopeCustomerOrderReq)
	if resp.Error != nil {
		t.Fatalf("error = %+v", resp.Error)
	}
	var customerList struct {
		Tools []mcpTool `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &customerList); err != nil {
		t.Fatalf("解析 tools 失败: %v", err)
	}
	if len(customerList.Tools) != 1 || customerList.Tools[0].Name != "goodser_customer_order_request" {
		t.Fatalf("customer tools = %+v", customerList.Tools)
	}
	customerProps, _ := customerList.Tools[0].InputSchema["properties"].(map[string]any)
	if _, ok := customerProps["request_id"]; !ok {
		t.Fatalf("写工具应包含 request_id: %+v", customerProps)
	}
}

func TestMCPToolsCall(t *testing.T) {
	h := newMCPTestHandler(t)
	router := newMCPTestRouter(h)

	_, resp := callMCP(t, router,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"goodser_stock_query","arguments":{"params":{"keyword":"徽章"}}}}`,
		authz.ScopeOwnerRead,
	)
	if resp.Error != nil {
		t.Fatalf("error = %+v", resp.Error)
	}
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("解析 result 失败: %v", err)
	}
	if result.IsError {
		t.Fatalf("isError = true, text=%s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "徽章") || !strings.Contains(result.Content[0].Text, "tester") {
		t.Fatalf("text = %s", result.Content[0].Text)
	}
}

func TestMCPToolsCallDirectArguments(t *testing.T) {
	h := newMCPTestHandler(t)
	router := newMCPTestRouter(h)

	_, resp := callMCP(t, router,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"goodser_stock_query","arguments":{"keyword":"拍立得"}}}`,
		authz.ScopeOwnerRead,
	)
	if resp.Error != nil {
		t.Fatalf("error = %+v", resp.Error)
	}
	if !strings.Contains(string(resp.Result), "拍立得") {
		t.Fatalf("result = %s", string(resp.Result))
	}
}

func TestMCPToolsCallPassesRequestID(t *testing.T) {
	h := newMCPTestHandler(t)
	router := newMCPTestRouter(h)

	_, resp := callMCP(t, router,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"goodser_order_create","arguments":{"request_id":"mcp-req-1","params":{}}}}`,
		authz.ScopeOwnerWrite,
	)
	if resp.Error != nil {
		t.Fatalf("error = %+v", resp.Error)
	}
	if !strings.Contains(string(resp.Result), "mcp-req-1") {
		t.Fatalf("request_id 未传递到动作: %s", string(resp.Result))
	}
}

func TestMCPToolsCallForbidden(t *testing.T) {
	h := newMCPTestHandler(t)
	router := newMCPTestRouter(h)

	_, resp := callMCP(t, router,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"goodser_stock_query","arguments":{}}}`,
		authz.ScopeCustomerRead,
	)
	if resp.Error != nil {
		t.Fatalf("error = %+v", resp.Error)
	}
	var result struct {
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("解析 result 失败: %v", err)
	}
	if !result.IsError {
		t.Fatalf("缺少作用域应返回 isError=true, result=%s", string(resp.Result))
	}
}

func TestMCPUnknownTool(t *testing.T) {
	h := newMCPTestHandler(t)
	router := newMCPTestRouter(h)

	w, resp := callMCP(t, router,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"missing","arguments":{}}}`,
		authz.ScopeOwnerRead,
	)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if resp.Error == nil || resp.Error.Code != -32602 {
		t.Fatalf("error = %+v", resp.Error)
	}
}

func TestMCPNotificationAndInvalidJSON(t *testing.T) {
	h := newMCPTestHandler(t)
	router := newMCPTestRouter(h)

	w, _ := callMCP(t, router, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, authz.ScopeOwnerRead)
	if w.Code != http.StatusAccepted {
		t.Fatalf("notification status = %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("notification body = %s", w.Body.String())
	}

	w, resp := callMCP(t, router, `{`, authz.ScopeOwnerRead)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid json status = %d", w.Code)
	}
	if resp.Error == nil || resp.Error.Code != -32700 {
		t.Fatalf("error = %+v", resp.Error)
	}
}

func TestMCPPing(t *testing.T) {
	h := newMCPTestHandler(t)
	router := newMCPTestRouter(h)

	_, resp := callMCP(t, router, `{"jsonrpc":"2.0","id":"p1","method":"ping"}`, authz.ScopeOwnerRead)
	if resp.Error != nil {
		t.Fatalf("error = %+v", resp.Error)
	}
	if string(resp.Result) != "{}" {
		t.Fatalf("result = %s", string(resp.Result))
	}
}

func TestMCPAcceptsSSEOnlyClient(t *testing.T) {
	h := newMCPTestHandler(t)
	router := newMCPTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("X-Test-Scopes", authz.ScopeOwnerRead)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if !strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("content-type = %s", w.Header().Get("Content-Type"))
	}
	if !strings.Contains(w.Body.String(), "event: message") || !strings.Contains(w.Body.String(), `"jsonrpc":"2.0"`) {
		t.Fatalf("sse body = %s", w.Body.String())
	}
}
