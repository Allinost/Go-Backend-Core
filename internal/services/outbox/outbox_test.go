package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestServiceEmitAndList(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store)
	svc.now = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }

	event, err := svc.Emit(ctx, EmitInput{
		Topic:         " goodser.order.created ",
		Source:        "goodser",
		AggregateType: "outbound_order",
		AggregateID:   "order-1",
		Payload:       map[string]any{"order_no": "OUT-1"},
	})
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if event.ID == "" {
		t.Fatal("期望生成事件 ID")
	}
	if event.Topic != "goodser.order.created" {
		t.Fatalf("topic 未 trim: %q", event.Topic)
	}
	if event.Status != StatusPending {
		t.Fatalf("新事件状态 = %s，期望 pending", event.Status)
	}

	var payload map[string]string
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("payload 反序列化: %v", err)
	}
	if payload["order_no"] != "OUT-1" {
		t.Fatalf("payload = %v", payload)
	}

	events, err := svc.List(ctx, Filter{Topic: "goodser.order.created", Status: string(StatusPending)})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(events) != 1 || events[0].ID != event.ID {
		t.Fatalf("List = %+v", events)
	}

	events, err = svc.List(ctx, Filter{AggregateID: "other"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("无关聚合查询应返回空，实际 %d", len(events))
	}
}

func TestServiceEmitValidation(t *testing.T) {
	svc := NewService(NewMemoryStore())
	if _, err := svc.Emit(context.Background(), EmitInput{Topic: "  "}); err == nil {
		t.Fatal("空 topic 应报错")
	}
	if _, err := svc.Emit(context.Background(), EmitInput{Topic: "t", Payload: make(chan int)}); err == nil {
		t.Fatal("不可序列化 payload 应报错")
	}
}

func TestMemoryStoreDeliverAndFail(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store)

	event, err := svc.Emit(ctx, EmitInput{Topic: "t", Payload: map[string]any{"ok": true}})
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}

	if err := svc.MarkFailed(ctx, event.ID, "timeout"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	failed, err := svc.Get(ctx, event.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if failed.Status != StatusFailed || failed.Attempts != 1 || failed.LastError != "timeout" {
		t.Fatalf("失败状态 = %+v", failed)
	}

	if err := svc.MarkFailed(ctx, event.ID, "again"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	failed, _ = svc.Get(ctx, event.ID)
	if failed.Attempts != 2 {
		t.Fatalf("attempts = %d，期望 2", failed.Attempts)
	}

	if err := svc.MarkDelivered(ctx, event.ID); err != nil {
		t.Fatalf("MarkDelivered: %v", err)
	}
	delivered, _ := svc.Get(ctx, event.ID)
	if delivered.Status != StatusDelivered || delivered.DeliveredAt == nil {
		t.Fatalf("投递状态 = %+v", delivered)
	}
	if delivered.LastError != "" {
		t.Fatalf("投递成功应清空错误，实际 %q", delivered.LastError)
	}
}

func TestMemoryStoreNotFound(t *testing.T) {
	svc := NewService(NewMemoryStore())
	ctx := context.Background()
	if _, err := svc.Get(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get 错误 = %v，期望 ErrNotFound", err)
	}
	if err := svc.MarkDelivered(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("MarkDelivered 错误 = %v，期望 ErrNotFound", err)
	}
	if err := svc.MarkFailed(ctx, "missing", "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("MarkFailed 错误 = %v，期望 ErrNotFound", err)
	}
}

func TestMemoryStoreListPaginationAndOrder(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store)
	base := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	for i := 0; i < 3; i++ {
		svc.now = func() time.Time { return base.Add(time.Duration(i) * time.Second) }
		if _, err := svc.Emit(ctx, EmitInput{Topic: "t", Payload: i}); err != nil {
			t.Fatalf("Emit: %v", err)
		}
	}

	all, err := svc.List(ctx, Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("len = %d", len(all))
	}
	for i := 1; i < len(all); i++ {
		if all[i].CreatedAt.Before(all[i-1].CreatedAt) {
			t.Fatalf("未按创建时间升序: %+v", all)
		}
	}

	paged, err := svc.List(ctx, Filter{Offset: 1, Limit: 1})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(paged) != 1 || paged[0].ID != all[1].ID {
		t.Fatalf("分页结果 = %+v", paged)
	}
}

func TestCloneKeepsPayloadIndependent(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store)
	event, err := svc.Emit(ctx, EmitInput{Topic: "t", Payload: map[string]string{"k": "v"}})
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	event.Payload[2] = 'X'
	got, _ := svc.Get(ctx, event.ID)
	if string(got.Payload) == string(event.Payload) {
		t.Fatal("Get 不应暴露内部 payload 切片")
	}
}
