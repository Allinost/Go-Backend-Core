package auth_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Allinost/go-backend-core/internal/services/auth"
	"github.com/Allinost/go-backend-core/internal/testutil"
)

func TestPostgresRBACSeed(t *testing.T) {
	db := testutil.OpenPG(t)
	require.NoError(t, testutil.ApplyMigrations(t, db))

	store, err := auth.NewPostgresRBACStore(db)
	require.NoError(t, err)
	require.NoError(t, store.EnsureDefaultPermissions())

	roles, err := store.ListRoles()
	require.NoError(t, err)
	require.NotEmpty(t, roles)

	perms, err := store.GetRolePermissions("admin")
	require.NoError(t, err)
	require.NotEmpty(t, perms)
}
