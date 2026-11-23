package webhook_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Allinost/go-backend-core/internal/modules/webhook"
	"github.com/Allinost/go-backend-core/internal/testutil"
)

func TestRepositoryCreateGetDeleteConfig(t *testing.T) {
	db := testutil.OpenPG(t)
	require.NoError(t, testutil.ApplyMigrations(t, db))

	repo := webhook.NewRepository(db, nil)
	ctx := context.Background()

	created, err := repo.CreateConfig(ctx, &webhook.CreateConfigReq{
		Name:   "integration-webhook",
		Type:   "generic",
		URL:    "https://example.invalid/hook",
		Source: "test",
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)

	got, err := repo.GetConfig(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "integration-webhook", got.Name)
	require.Equal(t, "generic", got.Type)

	require.NoError(t, repo.DeleteConfig(ctx, created.ID))
	_, err = repo.GetConfig(ctx, created.ID)
	require.Error(t, err)
}
