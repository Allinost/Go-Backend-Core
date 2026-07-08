package logger

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAudit_Event(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter(Config{Level: "info", Format: "json"}, &buf)
	SetGlobal(l)

	Audit(AuditEvent{
		Action:   "user.login",
		UserID:   1,
		Resource: "user",
		Success:  true,
	})

	assert.Contains(t, buf.String(), "audit_log")
	assert.Contains(t, buf.String(), "user.login")
	assert.Contains(t, buf.String(), "true")
	assert.Contains(t, buf.String(), `"audit":true`)
}

func TestAudit_DefaultTimestamp(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter(Config{Level: "info", Format: "json"}, &buf)
	SetGlobal(l)

	Audit(AuditEvent{Action: "test"})
	assert.Contains(t, buf.String(), "test")
}

func TestAudit_FailedEvent(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter(Config{Level: "info", Format: "json"}, &buf)
	SetGlobal(l)

	Audit(AuditEvent{
		Action:  "user.delete",
		UserID:  2,
		Success: false,
	})

	assert.Contains(t, buf.String(), "user.delete")
	assert.Contains(t, buf.String(), "false")
}
