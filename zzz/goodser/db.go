package goodser

import (
	"context"
	"database/sql"

	"github.com/Allinost/go-backend-core/internal/pkg/dbutil"
)

// rebindDB 包装 *sql.DB，在交给驱动前把 MySQL 风格的 ? 占位符改写为 PostgreSQL 的 $n。
// Goodser 从 V2 MySQL 迁到 V3 PostgreSQL 时，仓库层 SQL 可以继续用 ?，
// 由这里统一改写，避免上百处调用点逐个调整。
type rebindDB struct {
	*sql.DB
}

func (d *rebindDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return d.DB.QueryContext(ctx, dbutil.Rebind(query), args...)
}

func (d *rebindDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return d.DB.QueryRowContext(ctx, dbutil.Rebind(query), args...)
}

func (d *rebindDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return d.DB.ExecContext(ctx, dbutil.Rebind(query), args...)
}

func (d *rebindDB) beginTx(ctx context.Context) (*rebindTx, error) {
	tx, err := d.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &rebindTx{Tx: tx}, nil
}

// rebindTx 包装 *sql.Tx，占位符改写规则与 rebindDB 一致。
type rebindTx struct {
	*sql.Tx
}

func (t *rebindTx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.Tx.QueryContext(ctx, dbutil.Rebind(query), args...)
}

func (t *rebindTx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.Tx.QueryRowContext(ctx, dbutil.Rebind(query), args...)
}

func (t *rebindTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.Tx.ExecContext(ctx, dbutil.Rebind(query), args...)
}
