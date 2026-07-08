package scheduler

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

type TaskType string

const (
	TaskTypeCron     TaskType = "cron"
	TaskTypeOnce     TaskType = "once"
	TaskTypeInterval TaskType = "interval"
)

type TaskStatus string

const (
	TaskStatusActive   TaskStatus = "active"
	TaskStatusPaused   TaskStatus = "paused"
	TaskStatusFinished TaskStatus = "finished"
)

type Task struct {
	ID         uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	Name       string          `json:"name" gorm:"uniqueIndex;size:255"`
	Type       TaskType        `json:"type" gorm:"size:20"`
	Expression string          `json:"expression" gorm:"size:255"`
	Handler    string          `json:"handler" gorm:"size:255"`
	Payload    json.RawMessage `json:"payload,omitempty" gorm:"type:json"`
	Status     TaskStatus      `json:"status" gorm:"size:20;default:active"`
	MaxRetries int             `json:"max_retries" gorm:"default:3"`
	Timeout    int             `json:"timeout" gorm:"default:300"`
	LastRunAt  *time.Time      `json:"last_run_at,omitempty"`
	NextRunAt  *time.Time      `json:"next_run_at,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

type TaskLogStatus string

const (
	TaskLogRunning TaskLogStatus = "running"
	TaskLogSuccess TaskLogStatus = "success"
	TaskLogFailed  TaskLogStatus = "failed"
	TaskLogTimeout TaskLogStatus = "timeout"
)

type TaskLog struct {
	ID        uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskID    uint            `json:"task_id" gorm:"index"`
	TaskName  string          `json:"task_name" gorm:"size:255"`
	Status    TaskLogStatus   `json:"status" gorm:"size:20"`
	StartedAt time.Time       `json:"started_at"`
	EndedAt   *time.Time      `json:"ended_at,omitempty"`
	Result    json.RawMessage `json:"result,omitempty" gorm:"type:json"`
	Error     string          `json:"error,omitempty" gorm:"type:text"`
	CreatedAt time.Time       `json:"created_at"`
}

type TaskHandler interface {
	Name() string
	Execute(ctx context.Context, payload json.RawMessage) error
}

var (
	handlerMu sync.RWMutex
	handlers  = make(map[string]TaskHandler)
)

func RegisterHandler(h TaskHandler) {
	handlerMu.Lock()
	defer handlerMu.Unlock()
	handlers[h.Name()] = h
}

func GetHandler(name string) TaskHandler {
	handlerMu.RLock()
	defer handlerMu.RUnlock()
	return handlers[name]
}

func ListHandlers() []string {
	handlerMu.RLock()
	defer handlerMu.RUnlock()
	names := make([]string, 0, len(handlers))
	for n := range handlers {
		names = append(names, n)
	}
	return names
}
