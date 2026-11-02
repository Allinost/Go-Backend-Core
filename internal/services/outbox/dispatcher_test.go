package outbox

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Allinost/go-backend-core/internal/services/connector"
)

type fakeConnector struct {
	id     string
	topics []string
	err    error

	mu   sync.Mutex
	msgs []connector.Message
}

func (f *fakeConnector) ID() string   { return f.id }
func (f *fakeConnector) Type() string { return "fake" }
func (f *fakeConnector) Topics() []string {
	return f.topics
}
func (f *fakeConnector) Send(_ context.Context, msg connector.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.msgs = append(f.msgs, msg)
	return f.err
}
func (f *fakeConnector) messages() []connector.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]connector.Message(nil), f.msgs...)
}

func newTestDispatcher(t *testing.T, cfg DispatcherConfig, items ...connector.Connector) (*Dispatcher, *MemoryStore, *Service) {
	t.Helper()
	store := NewMemoryStore()
	svc := NewService(store)
	registry := connector.NewRegistry()
	for _, item := range items {
		if err := registry.Register(item); err != nil {
			t.Fatal(err)
		}
	}
	return NewDispatcher(svc, registry, cfg), store, svc
}

func TestDispatcherDeliversPendingAndRoutesTopics(t *testing.T) {
	target := &fakeConnector{id: "n8n", topics: []string{"goodser.*"}}
	d, store, svc := newTestDispatcher(t, DispatcherConfig{Enabled: true, BatchSize: 10}, target)
	event, err := svc.Emit(context.Background(), EmitInput{Topic: "goodser.order.created", Payload: map[string]any{"id": 1}})
	if err != nil {
		t.Fatal(err)
	}

	delivered, failed, err := d.RunOnce(context.Background())
	if err != nil || delivered != 1 || failed != 0 {
		t.Fatalf("delivered=%d failed=%d err=%v", delivered, failed, err)
	}
	stored, _ := store.Get(context.Background(), event.ID)
	if stored.Status != StatusDelivered {
		t.Fatalf("status = %s", stored.Status)
	}
	msgs := target.messages()
	if len(msgs) != 1 || msgs[0].Event != "goodser.order.created" {
		t.Fatalf("msgs = %+v", msgs)
	}
	if string(msgs[0].Payload) != `{"id":1}` {
		t.Fatalf("payload = %s", msgs[0].Payload)
	}
}

func TestDispatcherNoMatchingTopicMarksDelivered(t *testing.T) {
	target := &fakeConnector{id: "langbot", topics: []string{"customer.*"}}
	d, store, svc := newTestDispatcher(t, DispatcherConfig{Enabled: true}, target)
	event, _ := svc.Emit(context.Background(), EmitInput{Topic: "goodser.order.created"})

	delivered, failed, err := d.RunOnce(context.Background())
	if err != nil || delivered != 1 || failed != 0 {
		t.Fatalf("delivered=%d failed=%d err=%v", delivered, failed, err)
	}
	stored, _ := store.Get(context.Background(), event.ID)
	if stored.Status != StatusDelivered {
		t.Fatalf("status = %s", stored.Status)
	}
	if len(target.messages()) != 0 {
		t.Fatal("不匹配的连接器不应收到消息")
	}
}

func TestDispatcherMarksFailedAndHonorsBackoff(t *testing.T) {
	target := &fakeConnector{id: "broken", err: errors.New("HTTP 500")}
	d, store, svc := newTestDispatcher(t, DispatcherConfig{
		Enabled: true, BaseBackoff: time.Minute, MaxAttempts: 3,
	}, target)
	event, _ := svc.Emit(context.Background(), EmitInput{Topic: "goodser.note"})

	delivered, failed, err := d.RunOnce(context.Background())
	if err != nil || delivered != 0 || failed != 1 {
		t.Fatalf("delivered=%d failed=%d err=%v", delivered, failed, err)
	}
	stored, _ := store.Get(context.Background(), event.ID)
	if stored.Status != StatusFailed || stored.Attempts != 1 {
		t.Fatalf("status=%s attempts=%d", stored.Status, stored.Attempts)
	}
	if stored.LastError == "" {
		t.Fatal("应记录错误原因")
	}

	// 退避期内不应重试
	delivered, failed, err = d.RunOnce(context.Background())
	if err != nil || delivered != 0 || failed != 0 {
		t.Fatalf("退避期内 delivered=%d failed=%d err=%v", delivered, failed, err)
	}
}

func TestDispatcherRetriesAfterBackoff(t *testing.T) {
	target := &fakeConnector{id: "flaky"}
	d, store, _ := newTestDispatcher(t, DispatcherConfig{
		Enabled: true, BaseBackoff: time.Millisecond, MaxAttempts: 3,
	}, target)
	event := Event{
		ID: "evt-1", Topic: "goodser.note", Payload: []byte(`{}`),
		Status: StatusFailed, Attempts: 1,
		UpdatedAt: time.Now().UTC().Add(-time.Second),
	}
	if err := store.Append(context.Background(), event); err != nil {
		t.Fatal(err)
	}

	delivered, failed, err := d.RunOnce(context.Background())
	if err != nil || delivered != 1 || failed != 0 {
		t.Fatalf("delivered=%d failed=%d err=%v", delivered, failed, err)
	}
	stored, _ := store.Get(context.Background(), event.ID)
	if stored.Status != StatusDelivered {
		t.Fatalf("status = %s", stored.Status)
	}
}

func TestDispatcherStopsAtMaxAttempts(t *testing.T) {
	target := &fakeConnector{id: "dead"}
	d, store, _ := newTestDispatcher(t, DispatcherConfig{
		Enabled: true, BaseBackoff: time.Millisecond, MaxAttempts: 3,
	}, target)
	event := Event{
		ID: "evt-dead", Topic: "goodser.note", Payload: []byte(`{}`),
		Status: StatusFailed, Attempts: 3,
		UpdatedAt: time.Now().UTC().Add(-time.Hour),
	}
	if err := store.Append(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	delivered, failed, err := d.RunOnce(context.Background())
	if err != nil || delivered != 0 || failed != 0 {
		t.Fatalf("超过最大重试后 delivered=%d failed=%d err=%v", delivered, failed, err)
	}
}

func TestMatchesTopic(t *testing.T) {
	cases := []struct {
		topics  []string
		topic   string
		matches bool
	}{
		{nil, "goodser.order.created", true},
		{[]string{"*"}, "goodser.order.created", true},
		{[]string{"goodser.order.created"}, "goodser.order.created", true},
		{[]string{"goodser.*"}, "goodser.order.created", true},
		{[]string{"goodser*"}, "goodser.order.created", true},
		{[]string{"customer.*"}, "goodser.order.created", false},
	}
	for _, tc := range cases {
		item := &fakeConnector{id: "x", topics: tc.topics}
		if got := connector.MatchesTopic(item, tc.topic); got != tc.matches {
			t.Errorf("topics=%v topic=%s got=%v", tc.topics, tc.topic, got)
		}
	}
}
