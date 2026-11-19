package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/dbutil"
)

// ActionLog 动作调用日志
type ActionLog struct {
	Action     string
	Actor      string
	AuthMethod string
	APIKeyID   uint
	Status     string
	Params     json.RawMessage
	RequestID  string
	Replayed   bool
	Result     json.RawMessage
	Error      string
	DurationMS int
}

// ActionLogger 动作调用日志写入接口
type ActionLogger interface {
	Write(ctx context.Context, entry ActionLog) error
}

// PostgresActionLogger 基于 PostgreSQL 的动作日志写入器
type PostgresActionLogger struct {
	db *sql.DB
}

// NewPostgresActionLogger 创建动作日志写入器
func NewPostgresActionLogger(db *sql.DB) *PostgresActionLogger {
	return &PostgresActionLogger{db: db}
}

// Write 写入动作日志，失败不影响主流程
func (l *PostgresActionLogger) Write(ctx context.Context, entry ActionLog) error {
	var params, result any
	if len(entry.Params) > 0 {
		params = string(entry.Params)
	}
	if len(entry.Result) > 0 {
		result = string(entry.Result)
	}
	_, err := l.db.ExecContext(ctx, dbutil.Rebind(`INSERT INTO zzz_integration_action_logs
		(action, actor, auth_method, api_key_id, status, params, request_id, replayed, result, error, duration_ms, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		entry.Action, entry.Actor, entry.AuthMethod, entry.APIKeyID, entry.Status,
		params, entry.RequestID, entry.Replayed, result, entry.Error, entry.DurationMS, time.Now().UTC(),
	)
	return err
}
