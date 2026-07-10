package scheduler

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// TaskType 任务类型
type TaskType string

const (
	TaskTypeCron     TaskType = "cron"     // cron 表达式定时任务
	TaskTypeOnce     TaskType = "once"     // 一次性执行任务
	TaskTypeInterval TaskType = "interval" // 固定间隔执行任务
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusActive   TaskStatus = "active"   // 活跃状态
	TaskStatusPaused   TaskStatus = "paused"   // 已暂停
	TaskStatusFinished TaskStatus = "finished" // 已完成
)

// Task 定时任务实体，映射数据库表
type Task struct {
	ID         uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	Name       string          `json:"name" gorm:"uniqueIndex;size:255"`
	Type       TaskType        `json:"type" gorm:"size:20"`
	Expression string          `json:"expression" gorm:"size:255"`         // cron 表达式或时间间隔
	Handler    string          `json:"handler" gorm:"size:255"`            // 注册的处理器名称
	Payload    json.RawMessage `json:"payload,omitempty" gorm:"type:json"` // 任务自定义参数
	Status     TaskStatus      `json:"status" gorm:"size:20;default:active"`
	MaxRetries int             `json:"max_retries" gorm:"default:3"`
	Timeout    int             `json:"timeout" gorm:"default:300"`
	LastRunAt  *time.Time      `json:"last_run_at,omitempty"` // 上次执行时间
	NextRunAt  *time.Time      `json:"next_run_at,omitempty"` // 下次执行时间
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// TaskLogStatus 任务日志状态
type TaskLogStatus string

const (
	TaskLogRunning TaskLogStatus = "running" // 运行中
	TaskLogSuccess TaskLogStatus = "success" // 执行成功
	TaskLogFailed  TaskLogStatus = "failed"  // 执行失败
	TaskLogTimeout TaskLogStatus = "timeout" // 执行超时
)

// TaskLog 任务执行日志实体
type TaskLog struct {
	ID        uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskID    uint            `json:"task_id" gorm:"index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Task      *Task           `json:"-" gorm:"foreignKey:TaskID"`
	TaskName  string          `json:"task_name" gorm:"size:255"`
	Status    TaskLogStatus   `json:"status" gorm:"size:20"`
	StartedAt time.Time       `json:"started_at"`
	EndedAt   *time.Time      `json:"ended_at,omitempty"`
	Result    json.RawMessage `json:"result,omitempty" gorm:"type:json"`
	Error     string          `json:"error,omitempty" gorm:"type:text"`
	CreatedAt time.Time       `json:"created_at"`
}

// TaskHandler 任务处理器接口，所有注册的处理器需实现此接口
type TaskHandler interface {
	Name() string                                               // 处理器唯一名称
	Execute(ctx context.Context, payload json.RawMessage) error // 执行任务逻辑
}

var (
	handlerMu sync.RWMutex
	handlers  = make(map[string]TaskHandler)
)

// RegisterHandler 注册任务处理器到全局处理器映射表
func RegisterHandler(h TaskHandler) {
	handlerMu.Lock()
	defer handlerMu.Unlock()
	handlers[h.Name()] = h
}

// GetHandler 根据名称获取已注册的任务处理器
func GetHandler(name string) TaskHandler {
	handlerMu.RLock()
	defer handlerMu.RUnlock()
	return handlers[name]
}

// ListHandlers 列出所有已注册的处理器名称
func ListHandlers() []string {
	handlerMu.RLock()
	defer handlerMu.RUnlock()
	names := make([]string, 0, len(handlers))
	for n := range handlers {
		names = append(names, n)
	}
	return names
}
