package scheduler

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

const taskQueue = "scheduler:tasks"

type asynqPayload struct {
	TaskID      uint
	HandlerName string
	Payload     json.RawMessage
}

func newAsynqClient(rdb *redis.Client) *asynq.Client {
	return asynq.NewClient(asynq.RedisClientOpt{
		Addr:     rdb.Options().Addr,
		Username: rdb.Options().Username,
		Password: rdb.Options().Password,
		DB:       rdb.Options().DB,
	})
}

func newAsynqServer(rdb *redis.Client, concurrency, timeout int) *asynq.Server {
	return asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     rdb.Options().Addr,
			Username: rdb.Options().Username,
			Password: rdb.Options().Password,
			DB:       rdb.Options().DB,
		},
		asynq.Config{
			Concurrency: concurrency,
			IsFailure: func(err error) bool {
				return true
			},
			RetryDelayFunc: func(n int, err error, t *asynq.Task) time.Duration {
				return time.Duration(1<<uint(n)) * 100 * time.Millisecond
			},
		},
	)
}

func enqueueTask(client *asynq.Client, t *Task, timeout int) error {
	payload := asynqPayload{
		TaskID:      t.ID,
		HandlerName: t.Handler,
		Payload:     t.Payload,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	opts := []asynq.Option{
		asynq.MaxRetry(t.MaxRetries),
		asynq.Timeout(time.Duration(timeout) * time.Second),
		asynq.Queue(taskQueue),
	}

	_, err = client.Enqueue(
		asynq.NewTask("scheduler:execute", data, opts...),
	)
	return err
}

type AsynqHandler struct {
	store   Store
	metrics Metrics
}

func NewAsynqHandler(store Store, metrics Metrics) *AsynqHandler {
	return &AsynqHandler{store: store, metrics: metrics}
}

func (h *AsynqHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var p asynqPayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		return err
	}

	handler := GetHandler(p.HandlerName)
	if handler == nil {
		return asynq.SkipRetry
	}

	logEntry := &TaskLog{
		TaskID:    p.TaskID,
		TaskName:  p.HandlerName,
		Status:    TaskLogRunning,
		StartedAt: time.Now(),
	}

	if err := h.store.CreateTaskLog(logEntry); err != nil {
		log.Printf("scheduler: 写入 task_log 失败: %v", err)
	}

	if h.metrics != nil {
		h.metrics.TaskStarted(p.HandlerName)
	}

	err := handler.Execute(ctx, p.Payload)

	now := time.Now()
	logEntry.EndedAt = &now

	if err != nil {
		logEntry.Status = TaskLogFailed
		logEntry.Error = err.Error()
		if h.metrics != nil {
			h.metrics.TaskFailed(p.HandlerName)
		}
	} else {
		logEntry.Status = TaskLogSuccess
		if h.metrics != nil {
			h.metrics.TaskCompleted(p.HandlerName)
		}
	}

	if updateErr := h.store.UpdateTaskLog(logEntry); updateErr != nil {
		log.Printf("scheduler: 更新 task_log 失败: %v", updateErr)
	}

	return err
}

func SetupAsynqMux(handler *AsynqHandler) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.HandleFunc("scheduler:execute", handler.ProcessTask)
	return mux
}
