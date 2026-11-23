package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Allinost/go-backend-core/internal/services/audit"
	"github.com/Allinost/go-backend-core/internal/testutil"
)

func TestPostgresStoreAppendAndQuery(t *testing.T) {
	db := testutil.OpenPG(t)
	require.NoError(t, testutil.ApplyMigrations(t, db))

	store := audit.NewPostgresStore(db)
	ctx := context.Background()
	require.NoError(t, store.Init(ctx))

	entry := audit.Entry{
		Action:    "test.integration",
		UserID:    "1",
		Username:  "tester",
		Resource:  "audit_log",
		Detail:    `{"ok":true}`,
		ClientIP:  "127.0.0.1",
		Status:    "success",
		CreatedAt: time.Now().UTC(),
	}
	require.NoError(t, store.Append(ctx, entry))

	items, err := store.Query(ctx, audit.Filter{Action: "test.integration", Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "tester", items[0].Username)
	require.Equal(t, "success", items[0].Status)
}
