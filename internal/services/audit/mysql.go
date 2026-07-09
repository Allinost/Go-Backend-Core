package audit

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

// MySQLStore 基于 database/sql 的审计日志存储，兼容 MySQL 和 SQLite
type MySQLStore struct {
	db  *sql.DB
	mu  sync.RWMutex
	tbl string
	// 检测是否为 SQLite，用于生成兼容 SQL
	isSQLite bool
}

func NewMySQLStore(db *sql.DB, tableName string) *MySQLStore {
	if tableName == "" {
		tableName = "audit_log"
	}
	isSQLite := false
	if db != nil {
		drv := reflect.TypeOf(db.Driver()).String()
		isSQLite = strings.Contains(strings.ToLower(drv), "sqlite")
	}
	return &MySQLStore{db: db, tbl: tableName, isSQLite: isSQLite}
}

func (s *MySQLStore) Init(ctx context.Context) error {
	autoPK := "BIGINT AUTO_INCREMENT PRIMARY KEY"
	if s.isSQLite {
		autoPK = "INTEGER PRIMARY KEY AUTOINCREMENT"
	}
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id %s,
		action VARCHAR(64) NOT NULL,
		user_id VARCHAR(128) DEFAULT '',
		username VARCHAR(128) DEFAULT '',
		resource VARCHAR(255) DEFAULT '',
		detail TEXT,
		client_ip VARCHAR(64) DEFAULT '',
		status VARCHAR(16) NOT NULL DEFAULT 'success',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`, s.tbl, autoPK)
	if !s.isSQLite {
		q += `;
		CREATE INDEX IF NOT EXISTS idx_audit_action ON ` + s.tbl + `(action);
		CREATE INDEX IF NOT EXISTS idx_audit_user ON ` + s.tbl + `(user_id);
		CREATE INDEX IF NOT EXISTS idx_audit_resource ON ` + s.tbl + `(resource);
		CREATE INDEX IF NOT EXISTS idx_audit_created ON ` + s.tbl + `(created_at);`
	}
	_, err := s.db.ExecContext(ctx, q)
	return err
}

func (s *MySQLStore) Append(ctx context.Context, entry Entry) error {
	q := fmt.Sprintf(`INSERT INTO %s (action, user_id, username, resource, detail, client_ip, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, s.tbl)
	_, err := s.db.ExecContext(ctx, q,
		entry.Action, entry.UserID, entry.Username,
		entry.Resource, entry.Detail, entry.ClientIP,
		entry.Status, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("audit: append failed: %w", err)
	}
	return nil
}

func (s *MySQLStore) Query(ctx context.Context, filter Filter) ([]Entry, error) {
	where := "1=1"
	args := []any{}
	if filter.Action != "" {
		where += " AND action=?"
		args = append(args, filter.Action)
	}
	if filter.UserID != "" {
		where += " AND user_id=?"
		args = append(args, filter.UserID)
	}
	if filter.Resource != "" {
		where += " AND resource=?"
		args = append(args, filter.Resource)
	}
	if filter.Status != "" {
		where += " AND status=?"
		args = append(args, filter.Status)
	}
	if !filter.StartTime.IsZero() {
		where += " AND created_at>=?"
		args = append(args, filter.StartTime)
	}
	if !filter.EndTime.IsZero() {
		where += " AND created_at<=?"
		args = append(args, filter.EndTime)
	}

	q := fmt.Sprintf("SELECT id, action, user_id, username, resource, COALESCE(detail,''), client_ip, status, created_at FROM %s WHERE %s ORDER BY id DESC", s.tbl, where)
	if filter.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		q += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("audit: query failed: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.Action, &e.UserID, &e.Username, &e.Resource, &e.Detail, &e.ClientIP, &e.Status, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("audit: scan failed: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []Entry{}
	}
	return entries, nil
}

func (s *MySQLStore) Close() error {
	return s.db.Close()
}

var _ = time.Second
