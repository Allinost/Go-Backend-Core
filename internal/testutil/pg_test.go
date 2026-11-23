package testutil_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Allinost/go-backend-core/internal/testutil"
)

func TestApplyMigrations(t *testing.T) {
	db := testutil.OpenPG(t)
	require.NoError(t, testutil.ApplyMigrations(t, db))

	var tables int
	require.NoError(t, db.QueryRowContext(context.Background(), `
		SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_type = 'BASE TABLE'
	`).Scan(&tables))
	require.Greater(t, tables, 40)

	// 迁移可重复执行。
	require.NoError(t, testutil.ApplyMigrations(t, db))
	var tablesAgain int
	require.NoError(t, db.QueryRowContext(context.Background(), `
		SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_type = 'BASE TABLE'
	`).Scan(&tablesAgain))
	require.Equal(t, tables, tablesAgain)
}
