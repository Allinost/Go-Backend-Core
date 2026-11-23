package feature_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Allinost/go-backend-core/internal/services/feature"
	"github.com/Allinost/go-backend-core/internal/testutil"
)

func TestPostgresStoreSaveLoadDelete(t *testing.T) {
	db := testutil.OpenPG(t)
	require.NoError(t, testutil.ApplyMigrations(t, db))

	store := feature.NewPostgresStore(db, "feature_flags")
	ctx := context.Background()
	require.NoError(t, store.Init(ctx))

	require.NoError(t, store.Save(ctx, feature.Flag{
		Name:        "demo.flag",
		Enabled:     true,
		Description: "integration test",
	}))

	flags, err := store.Load(ctx)
	require.NoError(t, err)
	require.Len(t, flags, 1)
	require.Equal(t, "demo.flag", flags[0].Name)
	require.True(t, flags[0].Enabled)

	require.NoError(t, store.Delete(ctx, "demo.flag"))
	flags, err = store.Load(ctx)
	require.NoError(t, err)
	require.Empty(t, flags)
}
