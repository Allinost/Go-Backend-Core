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

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	registry := action.NewRegistry()
	registry.MustRegister(action.Definition{
		Name:   "stock.read",
		Scopes: []string{authz.ScopeOwnerRead, authz.ScopeCustomerRead},
		Handler: func(_ context.Context, req action.Request) (any, error) {
			return map[string]string{"actor": req.Actor}, nil
		},
	})
	gateway := action.NewGateway(registry)
	return NewHandler(gateway, outbox.NewService(outbox.NewMemoryStore()), connector.NewRegistry(), nil)
}

func newTestRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/actions", func(c *gin.Context) {
		c.Set("auth_method", "apikey")
		c.Set("api_key_scopes", authz.ScopeOwnerRead)
		h.ListActions(c)
	})
	r.POST("/actions/:name", func(c *gin.Context) {
		c.Set("auth_method", "apikey")
		c.Set("api_key_scopes", c.GetHeader("X-Test-Scopes"))
		c.Set("username", "tester")
		h.InvokeAction(c)
	})
	r.GET("/events", func(c *gin.Context) {
		c.Set("auth_method", "apikey")
		h.ListEvents(c)
	})
	r.POST("/events", func(c *gin.Context) {
		c.Set("auth_method", "apikey")
		h.EmitEvent(c)
	})
	r.GET("/connectors", func(c *gin.Context) {
		h.ListConnectors(c)
	})
	r.POST("/connectors/:id/test", func(c *gin.Context) {
		h.TestConnector(c)
	})
	return r
}

type captureConnector struct {
	id  string
	err error
	got connector.Message
}

func (c *captureConnector) ID() string   { return c.id }
func (c *captureConnector) Type() string { return "fake" }
func (c *captureConnector) Send(_ context.Context, msg connector.Message) error {
	c.got = msg
	return c.err
}

func TestHandlerListActions(t *testing.T) {
	h := newTestHandler(t)
	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/actions", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "stock.read") {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestHandlerInvokeAction(t *testing.T) {
	h := newTestHandler(t)
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/actions/stock.read", strings.NewReader(`{"params":{"product_id":"p1"}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Scopes", authz.ScopeCustomerRead)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "tester") {
		t.Fatalf("未传递调用者: %s", w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/actions/stock.read", nil)
	req.Header.Set("X-Test-Scopes", authz.ScopeOwnerWrite)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("缺少作用域 status = %d, body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/actions/missing", nil)
	req.Header.Set("X-Test-Scopes", "*")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("未知动作 status = %d, body=%s", w.Code, w.Body.String())
	}
}

func TestHandlerEmitAndListEvents(t *testing.T) {
	h := newTestHandler(t)
	router := newTestRouter(h)

	body := `{"topic":"goodser.order.created","source":"goodser","payload":{"order_no":"OUT-1"}}`
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("emit status = %d, body=%s", w.Code, w.Body.String())
	}

	var emitted struct {
		Data outbox.Event `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &emitted); err != nil {
		t.Fatalf("解析响应: %v", err)
	}
	if emitted.Data.ID == "" || emitted.Data.Status != outbox.StatusPending {
		t.Fatalf("事件 = %+v", emitted.Data)
	}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/events?topic=goodser.order.created", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "OUT-1") {
		t.Fatalf("list body = %s", w.Body.String())
	}
}

func TestHandlerEmitInvalidJSON(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(`{"topic":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("空 topic status = %d, body=%s", w.Code, w.Body.String())
	}
}

type captureLogger struct {
	entries []ActionLog
}

func (l *captureLogger) Write(_ context.Context, entry ActionLog) error {
	l.entries = append(l.entries, entry)
	return nil
}

func TestHandlerRecordsActionLog(t *testing.T) {
	h := newTestHandler(t)
	logger := &captureLogger{}
	h.logger = logger
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/actions/stock.read", strings.NewReader(`{"params":{}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Scopes", authz.ScopeOwnerRead)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if len(logger.entries) != 1 {
		t.Fatalf("日志条数 = %d", len(logger.entries))
	}
	entry := logger.entries[0]
	if entry.Action != "stock.read" || entry.Actor != "tester" || entry.Status != "success" {
		t.Fatalf("日志 = %+v", entry)
	}

	req = httptest.NewRequest(http.MethodPost, "/actions/stock.read", nil)
	req.Header.Set("X-Test-Scopes", authz.ScopeOwnerWrite)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if len(logger.entries) != 2 || logger.entries[1].Status != "error" {
		t.Fatalf("失败日志 = %+v", logger.entries)
	}
}

func TestHandlerTestConnector(t *testing.T) {
	h := newTestHandler(t)
	item := &captureConnector{id: "hook-1"}
	if err := h.connectors.Register(item); err != nil {
		t.Fatal(err)
	}
	router := newTestRouter(h)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/connectors/hook-1/test", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if item.got.Event != "integration.test" || item.got.EventID == "" {
		t.Fatalf("测试消息异常: %+v", item.got)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/connectors/missing/test", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在连接器应返回 404，实际 %d", rec.Code)
	}
}

func TestHandlerListConnectors(t *testing.T) {
	h := newTestHandler(t)
	hook, err := connector.NewWebhook(connector.WebhookConfig{ID: "hook-1", URL: "https://example.com/hook"})
	if err != nil {
		t.Fatalf("NewWebhook: %v", err)
	}
	if err := h.connectors.Register(hook); err != nil {
		t.Fatalf("Register: %v", err)
	}

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/connectors", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "hook-1") {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
}
