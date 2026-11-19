package audit

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Allinost/go-backend-core/internal/pkg/dbutil"
)

// PostgresStore 基于 PostgreSQL 的审计日志存储。表由 migrations 管理。
type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

// Init 检查 audit_log 表是否存在。
func (s *PostgresStore) Init(ctx context.Context) error {
	if s.db == nil {
		return fmt.Errorf("audit: db 为空")
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT to_regclass('audit_log') IS NOT NULL`).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("audit: audit_log 表不存在，请先执行迁移")
	}
	return nil
}

func (s *PostgresStore) Append(ctx context.Context, entry Entry) error {
	_, err := s.db.ExecContext(ctx, dbutil.Rebind(`INSERT INTO audit_log
		(action, user_id, username, resource, detail, client_ip, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`),
		entry.Action, entry.UserID, entry.Username, entry.Resource, entry.Detail, entry.ClientIP, entry.Status, entry.CreatedAt)
	if err != nil {
		return fmt.Errorf("audit: append failed: %w", err)
	}
	return nil
}

func (s *PostgresStore) Query(ctx context.Context, filter Filter) ([]Entry, error) {
	where := "1=1"
	args := []any{}
	if filter.Action != "" {
		where += " AND action = ?"
		args = append(args, filter.Action)
	}
	if filter.UserID != "" {
		where += " AND user_id = ?"
		args = append(args, filter.UserID)
	}
	if filter.Resource != "" {
		where += " AND resource = ?"
		args = append(args, filter.Resource)
	}
	if filter.Status != "" {
		where += " AND status = ?"
		args = append(args, filter.Status)
	}
	if !filter.StartTime.IsZero() {
		where += " AND created_at >= ?"
		args = append(args, filter.StartTime)
	}
	if !filter.EndTime.IsZero() {
		where += " AND created_at <= ?"
		args = append(args, filter.EndTime)
	}

	query := `SELECT id, action, user_id, username, resource, COALESCE(detail,''), client_ip, status, created_at
		FROM audit_log WHERE ` + where + ` ORDER BY id DESC`
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, dbutil.Rebind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("audit: query failed: %w", err)
	}
	defer rows.Close()

	entries := make([]Entry, 0)
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.Action, &e.UserID, &e.Username, &e.Resource, &e.Detail, &e.ClientIP, &e.Status, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("audit: scan failed: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// Close 不关闭共享连接池。
func (s *PostgresStore) Close() error { return nil }
