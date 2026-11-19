package feature

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Allinost/go-backend-core/internal/pkg/dbutil"
)

// PostgresStore 基于 PostgreSQL 的特性开关存储。表由 migrations 管理。
type PostgresStore struct {
	db  *sql.DB
	tbl string
}

func NewPostgresStore(db *sql.DB, tableName string) *PostgresStore {
	if tableName == "" {
		tableName = "feature_flags"
	}
	return &PostgresStore{db: db, tbl: tableName}
}

// Init 检查特性开关表是否存在。
func (s *PostgresStore) Init(ctx context.Context) error {
	if s.db == nil {
		return fmt.Errorf("feature: db 为空")
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, s.tbl).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("feature: %s 表不存在，请先执行迁移", s.tbl)
	}
	return nil
}

func (s *PostgresStore) Load(ctx context.Context) ([]Flag, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, enabled, COALESCE(description,'') FROM `+s.tbl)
	if err != nil {
		return nil, fmt.Errorf("feature: load flags failed: %w", err)
	}
	defer rows.Close()

	var flags []Flag
	for rows.Next() {
		var f Flag
		if err := rows.Scan(&f.Name, &f.Enabled, &f.Description); err != nil {
			return nil, fmt.Errorf("feature: scan flag failed: %w", err)
		}
		flags = append(flags, f)
	}
	return flags, rows.Err()
}

func (s *PostgresStore) Save(ctx context.Context, flag Flag) error {
	_, err := s.db.ExecContext(ctx, dbutil.Rebind(`INSERT INTO `+s.tbl+` (name, enabled, description)
		VALUES (?, ?, ?)
		ON CONFLICT (name) DO UPDATE SET enabled = EXCLUDED.enabled,
			description = EXCLUDED.description, updated_at = now()`),
		flag.Name, flag.Enabled, flag.Description)
	if err != nil {
		return fmt.Errorf("feature: save flag %s failed: %w", flag.Name, err)
	}
	return nil
}

func (s *PostgresStore) Delete(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx, dbutil.Rebind(`DELETE FROM `+s.tbl+` WHERE name = ?`), name)
	if err != nil {
		return fmt.Errorf("feature: delete flag %s failed: %w", name, err)
	}
	return nil
}

// Close 不关闭共享连接池。
func (s *PostgresStore) Close() error { return nil }
