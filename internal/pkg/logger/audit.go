package logger

import (
	"io"
	"os"
	"path/filepath"
	"time"
)

type AuditEvent struct {
	Action    string      `json:"action"`
	UserID    int64       `json:"user_id,omitempty"`
	Resource  string      `json:"resource,omitempty"`
	Detail    interface{} `json:"detail,omitempty"`
	ClientIP  string      `json:"client_ip,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Success   bool        `json:"success"`
}

var auditLogger *Logger

func InitAuditLog(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	auditLogger = NewWithWriter(Config{Level: "info", Format: "json"}, file)
	return nil
}

func Audit(ev AuditEvent) {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now()
	}
	if auditLogger != nil {
		auditLogger.Logger.Info().Bool("audit", true).Interface("event", ev).Msg("audit_log")
	}
	L.Logger.Info().Bool("audit", true).Interface("event", ev).Msg("audit_log")
}

func (l *Logger) AuditWriter() io.Writer {
	return l.Logger
}
