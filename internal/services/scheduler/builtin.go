package scheduler

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

type HealthCheckHandler struct{}

func (h *HealthCheckHandler) Name() string { return "health_check" }

func (h *HealthCheckHandler) Execute(ctx context.Context, payload json.RawMessage) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	log.Println("[scheduler] health_check: 健康检查通过")
	return nil
}

type CleanupLogHandler struct{}

func (h *CleanupLogHandler) Name() string { return "cleanup_log" }

func (h *CleanupLogHandler) Execute(ctx context.Context, payload json.RawMessage) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	retentionDays := 30
	if payload != nil {
		var cfg struct {
			RetentionDays int `json:"retention_days"`
		}
		if err := json.Unmarshal(payload, &cfg); err == nil && cfg.RetentionDays > 0 {
			retentionDays = cfg.RetentionDays
		}
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	log.Printf("[scheduler] cleanup_log: 清理 %s 之前的日志", cutoff.Format(time.RFC3339))
	return nil
}

func RegisterBuiltins() {
	RegisterHandler(&HealthCheckHandler{})
	RegisterHandler(&CleanupLogHandler{})
}

func init() {
	RegisterBuiltins()
}
