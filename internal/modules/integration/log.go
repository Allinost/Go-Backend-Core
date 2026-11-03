package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
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

// MySQLActionLogger 基于 MySQL 的动作日志写入器
type MySQLActionLogger struct {
	db *sql.DB
}

// NewMySQLActionLogger 创建动作日志写入器
func NewMySQLActionLogger(db *sql.DB) *MySQLActionLogger {
	return &MySQLActionLogger{db: db}
}

// Write 写入动作日志，失败不影响主流程
func (l *MySQLActionLogger) Write(ctx context.Context, entry ActionLog) error {
	var params, result any
	if len(entry.Params) > 0 {
		params = string(entry.Params)
	}
	if len(entry.Result) > 0 {
		result = string(entry.Result)
	}
	_, err := l.db.ExecContext(ctx, `INSERT INTO zzz_integration_action_logs
		(action, actor, auth_method, api_key_id, status, params, request_id, replayed, result, error, duration_ms, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.Action, entry.Actor, entry.AuthMethod, entry.APIKeyID, entry.Status,
		params, entry.RequestID, entry.Replayed, result, entry.Error, entry.DurationMS, time.Now().UTC(),
	)
	return err
}
