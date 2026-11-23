package servicenav_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Allinost/go-backend-core/internal/modules/servicenav"
	"github.com/Allinost/go-backend-core/internal/testutil"
)

func TestRepositoryCreateGetDeleteType(t *testing.T) {
	db := testutil.OpenPG(t)
	require.NoError(t, testutil.ApplyMigrations(t, db))

	repo := servicenav.NewRepository(db)
	ctx := context.Background()

	created, err := repo.CreateType(ctx, &servicenav.CreateTypeReq{Name: "integration-type", Remark: "test"})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)

	got, err := repo.GetType(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "integration-type", got.Name)

	require.NoError(t, repo.DeleteType(ctx, created.ID))
	_, err = repo.GetType(ctx, created.ID)
	require.Error(t, err)
}
