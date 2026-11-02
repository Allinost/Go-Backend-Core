package outbox

import (
	"context"
	"testing"
	"time"
)

func TestRetentionDeletesOnlyOldDelivered(t *testing.T) {
	store := NewMemoryStore()
	svc := NewService(store)
	ctx := context.Background()
	now := time.Now().UTC()

	oldDelivered := Event{
		ID: "old", Topic: "goodser.old", Payload: []byte(`{}`),
		Status: StatusDelivered, DeliveredAt: timePtr(now.AddDate(0, 0, -40)),
	}
	newDelivered := Event{
		ID: "new", Topic: "goodser.new", Payload: []byte(`{}`),
		Status: StatusDelivered, DeliveredAt: timePtr(now.AddDate(0, 0, -2)),
	}
	failed := Event{
		ID: "failed", Topic: "goodser.failed", Payload: []byte(`{}`),
		Status: StatusFailed,
	}
	for _, event := range []Event{oldDelivered, newDelivered, failed} {
		if err := store.Append(ctx, event); err != nil {
			t.Fatal(err)
		}
	}

	retention := NewRetention(svc, RetentionConfig{Days: 30})
	deleted, err := retention.RunOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("应删除 1 条，实际 %d", deleted)
	}
	if _, err := store.Get(ctx, "old"); err == nil {
		t.Fatal("过期已投递事件应被删除")
	}
	if _, err := store.Get(ctx, "new"); err != nil {
		t.Fatal("未过期事件不应删除")
	}
	if _, err := store.Get(ctx, "failed"); err != nil {
		t.Fatal("失败事件不应删除")
	}
}

func TestRetentionDisabled(t *testing.T) {
	retention := NewRetention(NewService(NewMemoryStore()), RetentionConfig{Days: 0})
	deleted, err := retention.RunOnce(context.Background())
	if err != nil || deleted != 0 {
		t.Fatalf("禁用时不应清理，deleted=%d err=%v", deleted, err)
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}
