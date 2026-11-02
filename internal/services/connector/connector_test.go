package connector

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRegistry(t *testing.T) {
	reg := NewRegistry()
	reg.Register(nil)
	if err := reg.Register(nil); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("nil 连接器错误 = %v", err)
	}

	h, err := NewWebhook(WebhookConfig{ID: "hook-1", URL: "https://example.com/hook"})
	if err != nil {
		t.Fatalf("NewWebhook: %v", err)
	}
	if err := reg.Register(h); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := reg.Register(h); !errors.Is(err, ErrAlreadyRegistered) {
		t.Fatalf("重复注册错误 = %v", err)
	}

	got, err := reg.Get("hook-1")
	if err != nil || got.ID() != "hook-1" || got.Type() != "webhook" {
		t.Fatalf("Get = %v, %v", got, err)
	}
	if _, err := reg.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("不存在错误 = %v", err)
	}
	list := reg.List()
	if len(list) != 1 || list[0].ID != "hook-1" {
		t.Fatalf("List = %+v", list)
	}
	reg.Remove("hook-1")
	if len(reg.List()) != 0 {
		t.Fatal("Remove 后仍存在")
	}
}

func TestNewWebhookValidation(t *testing.T) {
	cases := []WebhookConfig{
		{ID: "", URL: "https://example.com"},
		{ID: "x", URL: ""},
		{ID: "x", URL: "ftp://example.com"},
		{ID: "x", URL: "not-a-url"},
	}
	for _, cfg := range cases {
		if _, err := NewWebhook(cfg); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("cfg=%+v 错误 = %v", cfg, err)
		}
	}
}

func TestWebhookSendSignedJSON(t *testing.T) {
	var (
		gotBody      []byte
		gotSignature string
		gotTimestamp string
		gotEvent     string
		gotEventID   string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotSignature = r.Header.Get("X-Integration-Signature")
		gotTimestamp = r.Header.Get("X-Integration-Timestamp")
		gotEvent = r.Header.Get("X-Integration-Event")
		gotEventID = r.Header.Get("X-Integration-Event-ID")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	hook, err := NewWebhook(WebhookConfig{ID: "hook", URL: srv.URL, Secret: "s3cret"})
	if err != nil {
		t.Fatalf("NewWebhook: %v", err)
	}
	fixed := time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)
	hook.now = func() time.Time { return fixed }

	msg := Message{
		EventID:    "evt-1",
		Event:      "goodser.order.created",
		OccurredAt: fixed,
		Payload:    json.RawMessage(`{"order_no":"OUT-1"}`),
		Metadata:   map[string]string{"source": "test"},
	}
	if err := hook.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if gotEvent != msg.Event || gotEventID != msg.EventID {
		t.Fatalf("事件头 = %q / %q", gotEvent, gotEventID)
	}
	timestamp, err := strconv.ParseInt(gotTimestamp, 10, 64)
	if err != nil {
		t.Fatalf("时间戳头 = %q", gotTimestamp)
	}
	if !VerifySignature("s3cret", timestamp, gotSignature, gotBody) {
		t.Fatalf("签名校验失败: %s", gotSignature)
	}
	if VerifySignature("wrong", timestamp, gotSignature, gotBody) {
		t.Fatal("错误密钥不应通过签名校验")
	}

	var envelope struct {
		EventID  string            `json:"event_id"`
		Event    string            `json:"event"`
		Payload  json.RawMessage   `json:"payload"`
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(gotBody, &envelope); err != nil {
		t.Fatalf("请求体解析: %v", err)
	}
	if envelope.EventID != "evt-1" || envelope.Event != msg.Event {
		t.Fatalf("信封 = %+v", envelope)
	}
	if string(envelope.Payload) != `{"order_no":"OUT-1"}` {
		t.Fatalf("payload = %s", envelope.Payload)
	}
	if envelope.Metadata["source"] != "test" {
		t.Fatalf("metadata = %+v", envelope.Metadata)
	}
}

func TestWebhookSendHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer srv.Close()

	hook, err := NewWebhook(WebhookConfig{ID: "hook", URL: srv.URL})
	if err != nil {
		t.Fatalf("NewWebhook: %v", err)
	}
	err = hook.Send(context.Background(), Message{Event: "t", Payload: json.RawMessage(`{}`)})
	if err == nil {
		t.Fatal("非 2xx 应返回错误")
	}
	if !strings.Contains(err.Error(), "502") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("错误信息 = %v", err)
	}
}

func TestWebhookSendContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	hook, _ := NewWebhook(WebhookConfig{ID: "hook", URL: srv.URL, Timeout: time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := hook.Send(ctx, Message{Event: "t", Payload: json.RawMessage(`{}`)}); err == nil {
		t.Fatal("已取消的 context 应返回错误")
	}
}
