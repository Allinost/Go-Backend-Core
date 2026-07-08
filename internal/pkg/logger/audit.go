package logger

import (
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

func Audit(ev AuditEvent) {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now()
	}
	L.Logger.Info().Bool("audit", true).Interface("event", ev).Msg("audit_log")
}
