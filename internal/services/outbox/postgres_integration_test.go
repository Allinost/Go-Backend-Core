package outbox_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Allinost/go-backend-core/internal/services/outbox"
	"github.com/Allinost/go-backend-core/internal/testutil"
)

func TestPostgresStoreAppendGet(t *testing.T) {
	db := testutil.OpenPG(t)
	require.NoError(t, testutil.ApplyMigrations(t, db))

	store := outbox.NewPostgresStore(db)
	ctx := context.Background()
	now := time.Now().UTC()

	event := outbox.Event{
		ID:            "evt-integration-1",
		Topic:         "test.topic",
		Source:        "integration-test",
		AggregateType: "test",
		AggregateID:   "1",
		Payload:       json.RawMessage(`{"ok":true}`),
		Status:        outbox.StatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	require.NoError(t, store.Append(ctx, event))

	got, err := store.Get(ctx, event.ID)
	require.NoError(t, err)
	require.Equal(t, "test.topic", got.Topic)
	require.JSONEq(t, `{"ok":true}`, string(got.Payload))
}
