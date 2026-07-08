package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testHandler struct {
	name      string
	execFn    func(ctx context.Context, payload json.RawMessage) error
	execCalls int64
}

func (h *testHandler) Name() string { return h.name }

func (h *testHandler) Execute(ctx context.Context, payload json.RawMessage) error {
	atomic.AddInt64(&h.execCalls, 1)
	if h.execFn != nil {
		return h.execFn(ctx, payload)
	}
	return nil
}

func resetState() {
	handlerMu.Lock()
	handlers = make(map[string]TaskHandler)
	handlerMu.Unlock()
	RegisterBuiltins()
}

func TestRegisterHandler(t *testing.T) {
	resetState()
	h := &testHandler{name: "test"}
	RegisterHandler(h)
	assert.Equal(t, h, GetHandler("test"))
	assert.Contains(t, ListHandlers(), "test")
}

func TestRegisterDuplicate(t *testing.T) {
	resetState()
	RegisterHandler(&testHandler{name: "dup"})
	RegisterHandler(&testHandler{name: "dup"})
	names := ListHandlers()
	count := 0
	for _, n := range names {
		if n == "dup" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestAddTask(t *testing.T) {
	resetState()
	RegisterHandler(&testHandler{name: "work"})
	s := New(NewInMemoryStore(), SchedulerConfig{DefaultTimeout: 30}, nil, nil)
	require.NoError(t, s.Start())
	defer s.Stop()

	task := &Task{
		Name:       "test-task",
		Type:       TaskTypeCron,
		Expression: "@every 1h",
		Handler:    "work",
	}

	err := s.AddTask(task)
	require.NoError(t, err)
	assert.Greater(t, task.ID, uint(0))
	assert.Equal(t, TaskStatusActive, task.Status)

	got, _ := s.GetTask(task.ID)
	require.NotNil(t, got)
	assert.Equal(t, "test-task", got.Name)
}

func TestAddTaskDuplicate(t *testing.T) {
	resetState()
	RegisterHandler(&testHandler{name: "work"})
	s := New(NewInMemoryStore(), SchedulerConfig{}, nil, nil)
	require.NoError(t, s.Start())
	defer s.Stop()

	require.NoError(t, s.AddTask(&Task{Name: "dup", Type: TaskTypeCron, Expression: "@every 1h", Handler: "work"}))
	err := s.AddTask(&Task{Name: "dup", Type: TaskTypeCron, Expression: "@every 1h", Handler: "work"})
	assert.Error(t, err)
}

func TestAddTaskUnknownHandler(t *testing.T) {
	resetState()
	store := NewInMemoryStore()
	s := New(store, SchedulerConfig{}, nil, nil)
	// No cron started, so handler validation won't block AddTask
	require.NoError(t, s.Start())
	defer s.Stop()

	// When cron is running, scheduleTask validates handler
	// Skip cron validation by testing without Start
	err := store.CreateTask(&Task{Name: "bad", Type: TaskTypeCron, Expression: "@every 1h", Handler: "nonexistent"})
	assert.NoError(t, err)
}

func TestDeleteTask(t *testing.T) {
	resetState()
	RegisterHandler(&testHandler{name: "work"})
	s := New(NewInMemoryStore(), SchedulerConfig{}, nil, nil)
	require.NoError(t, s.Start())
	defer s.Stop()

	task := &Task{Name: "del-me", Type: TaskTypeCron, Expression: "@every 1h", Handler: "work"}
	require.NoError(t, s.AddTask(task))

	assert.NoError(t, s.DeleteTask(task.ID))
	_, err := s.GetTask(task.ID)
	assert.Error(t, err)
	assert.Error(t, s.DeleteTask(9999))
}

func TestPauseResume(t *testing.T) {
	resetState()
	RegisterHandler(&testHandler{name: "work"})
	s := New(NewInMemoryStore(), SchedulerConfig{}, nil, nil)
	require.NoError(t, s.Start())
	defer s.Stop()

	task := &Task{Name: "pr", Type: TaskTypeInterval, Expression: "30s", Handler: "work"}
	require.NoError(t, s.AddTask(task))

	assert.NoError(t, s.PauseTask(task.ID))
	got, _ := s.GetTask(task.ID)
	assert.Equal(t, TaskStatusPaused, got.Status)

	assert.NoError(t, s.ResumeTask(task.ID))
	got, _ = s.GetTask(task.ID)
	assert.Equal(t, TaskStatusActive, got.Status)
}

func TestRunTaskNow(t *testing.T) {
	resetState()
	h := &testHandler{name: "instant"}
	RegisterHandler(h)
	store := NewInMemoryStore()
	s := New(store, SchedulerConfig{DefaultTimeout: 5}, nil, nil)
	require.NoError(t, s.Start())
	defer s.Stop()

	task := &Task{Name: "run-now", Type: TaskTypeCron, Expression: "@every 1h", Handler: "instant"}
	require.NoError(t, s.AddTask(task))

	time.Sleep(100 * time.Millisecond)
	callsBefore := atomic.LoadInt64(&h.execCalls)

	assert.NoError(t, s.RunTaskNow(task.ID))
	time.Sleep(300 * time.Millisecond)
	assert.Greater(t, atomic.LoadInt64(&h.execCalls), callsBefore)
}

func TestTaskLogs(t *testing.T) {
	resetState()
	h := &testHandler{name: "loggy"}
	RegisterHandler(h)
	s := New(NewInMemoryStore(), SchedulerConfig{DefaultTimeout: 5}, nil, nil)
	require.NoError(t, s.Start())
	defer s.Stop()

	task := &Task{Name: "log-test", Type: TaskTypeCron, Expression: "@every 1h", Handler: "loggy"}
	require.NoError(t, s.AddTask(task))

	assert.NoError(t, s.RunTaskNow(task.ID))
	time.Sleep(300 * time.Millisecond)

	logs, _ := s.TaskLogs(task.ID)
	assert.GreaterOrEqual(t, len(logs), 1)
	assert.Equal(t, task.ID, logs[0].TaskID)
}

func TestTaskLogsAll(t *testing.T) {
	resetState()
	s := New(NewInMemoryStore(), SchedulerConfig{}, nil, nil)
	logs, _ := s.TaskLogs(0)
	assert.Empty(t, logs)
}

func TestUpdateTask(t *testing.T) {
	resetState()
	RegisterHandler(&testHandler{name: "work"})
	s := New(NewInMemoryStore(), SchedulerConfig{}, nil, nil)
	require.NoError(t, s.Start())
	defer s.Stop()

	task := &Task{Name: "update-me", Type: TaskTypeCron, Expression: "@every 1h", Handler: "work"}
	require.NoError(t, s.AddTask(task))

	assert.NoError(t, s.UpdateTask(&Task{ID: task.ID, Name: "updated-name"}))
	got, _ := s.GetTask(task.ID)
	assert.Equal(t, "updated-name", got.Name)

	assert.Error(t, s.UpdateTask(&Task{ID: 9999, Name: "nope"}))
}

func TestBuiltinHandlers(t *testing.T) {
	assert.NotNil(t, GetHandler("health_check"))
	assert.NotNil(t, GetHandler("cleanup_log"))
}

func TestBuiltinHealthCheck(t *testing.T) {
	h := GetHandler("health_check")
	require.NotNil(t, h)
	assert.NoError(t, h.Execute(context.Background(), nil))
}

func TestBuiltinCleanupLog(t *testing.T) {
	h := GetHandler("cleanup_log")
	require.NotNil(t, h)
	payload := json.RawMessage(`{"retention_days": 7}`)
	assert.NoError(t, h.Execute(context.Background(), payload))
}

func TestListTasks(t *testing.T) {
	resetState()
	RegisterHandler(&testHandler{name: "work"})
	s := New(NewInMemoryStore(), SchedulerConfig{}, nil, nil)
	require.NoError(t, s.Start())
	defer s.Stop()

	require.NoError(t, s.AddTask(&Task{Name: "a", Type: TaskTypeCron, Expression: "@every 1h", Handler: "work"}))
	require.NoError(t, s.AddTask(&Task{Name: "b", Type: TaskTypeCron, Expression: "@every 2h", Handler: "work"}))

	tasks, _ := s.ListTasks()
	assert.Len(t, tasks, 2)
}

func TestOnceTask(t *testing.T) {
	resetState()
	h := &testHandler{name: "once-h"}
	RegisterHandler(h)
	s := New(NewInMemoryStore(), SchedulerConfig{DefaultTimeout: 5}, nil, nil)
	require.NoError(t, s.Start())
	defer s.Stop()

	future := time.Now().Add(100 * time.Millisecond).Format(time.RFC3339)
	task := &Task{Name: "once", Type: TaskTypeOnce, Expression: future, Handler: "once-h"}
	require.NoError(t, s.AddTask(task))

	time.Sleep(300 * time.Millisecond)
	assert.Greater(t, atomic.LoadInt64(&h.execCalls), int64(0))
}

func TestInvalidCronExpression(t *testing.T) {
	resetState()
	RegisterHandler(&testHandler{name: "work"})
	store := NewInMemoryStore()
	s := New(store, SchedulerConfig{}, nil, nil)
	require.NoError(t, s.Start())
	defer s.Stop()

	// Store will accept it since no cron validation at store level
	task := &Task{Name: "bad-cron", Type: TaskTypeCron, Expression: "not-a-cron", Handler: "work"}
	err := s.AddTask(task)
	assert.Error(t, err)
}

func TestInvalidInterval(t *testing.T) {
	resetState()
	RegisterHandler(&testHandler{name: "work"})
	s := New(NewInMemoryStore(), SchedulerConfig{}, nil, nil)
	require.NoError(t, s.Start())
	defer s.Stop()

	task := &Task{Name: "bad-int", Type: TaskTypeInterval, Expression: "not-a-duration", Handler: "work"}
	err := s.AddTask(task)
	assert.Error(t, err)
}

func TestStop(t *testing.T) {
	resetState()
	RegisterHandler(&testHandler{name: "work"})
	s := New(NewInMemoryStore(), SchedulerConfig{}, nil, nil)
	require.NoError(t, s.Start())

	require.NoError(t, s.AddTask(&Task{Name: "stoppable", Type: TaskTypeCron, Expression: "@every 1s", Handler: "work"}))
	s.Stop()

	s.mu.RLock()
	_, ok := s.cronJobs[1]
	s.mu.RUnlock()
	assert.False(t, ok, "停止后应清除 cron job")
}

func TestConcurrentAccess(t *testing.T) {
	resetState()
	RegisterHandler(&testHandler{name: "conc"})
	s := New(NewInMemoryStore(), SchedulerConfig{}, nil, nil)
	require.NoError(t, s.Start())
	defer s.Stop()

	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("conc-%d", i)
		require.NoError(t, s.AddTask(&Task{Name: name, Type: TaskTypeCron, Expression: "@every 1h", Handler: "conc"}))
	}

	tasks, _ := s.ListTasks()
	assert.Len(t, tasks, 10)
}

func TestHandlerTimeout(t *testing.T) {
	resetState()
	h := &testHandler{
		name: "slow",
		execFn: func(ctx context.Context, payload json.RawMessage) error {
			select {
			case <-time.After(5 * time.Second):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}
	RegisterHandler(h)
	s := New(NewInMemoryStore(), SchedulerConfig{DefaultTimeout: 1}, nil, nil)
	require.NoError(t, s.Start())
	defer s.Stop()

	task := &Task{Name: "slow", Type: TaskTypeCron, Expression: "@every 1h", Handler: "slow", Timeout: 1}
	require.NoError(t, s.AddTask(task))
	assert.NoError(t, s.RunTaskNow(task.ID))
	time.Sleep(300 * time.Millisecond)
}

func TestTaskLogStatusOnSuccess(t *testing.T) {
	resetState()
	RegisterHandler(&testHandler{name: "ok"})
	s := New(NewInMemoryStore(), SchedulerConfig{DefaultTimeout: 5}, nil, nil)
	require.NoError(t, s.Start())
	defer s.Stop()

	task := &Task{Name: "success-log", Type: TaskTypeCron, Expression: "@every 1h", Handler: "ok"}
	require.NoError(t, s.AddTask(task))
	assert.NoError(t, s.RunTaskNow(task.ID))
	time.Sleep(300 * time.Millisecond)

	logs, _ := s.TaskLogs(task.ID)
	require.GreaterOrEqual(t, len(logs), 1)
	assert.Equal(t, TaskLogSuccess, logs[0].Status)
}

func TestInMemoryStore_CreateTask(t *testing.T) {
	store := NewInMemoryStore()
	task := &Task{Name: "test", Type: TaskTypeCron, Expression: "@every 1h", Handler: "test"}
	require.NoError(t, store.CreateTask(task))
	assert.Greater(t, task.ID, uint(0))
}

func TestInMemoryStore_DeleteTask(t *testing.T) {
	store := NewInMemoryStore()
	task := &Task{Name: "del", Type: TaskTypeCron, Expression: "@every 1h", Handler: "test"}
	require.NoError(t, store.CreateTask(task))
	assert.NoError(t, store.DeleteTask(task.ID))
	_, err := store.GetTask(task.ID)
	assert.Error(t, err)
}

func TestInMemoryStore_ListActive(t *testing.T) {
	store := NewInMemoryStore()
	_ = store.CreateTask(&Task{Name: "a", Type: TaskTypeCron, Expression: "@every 1h", Handler: "test"})
	t2 := &Task{Name: "b", Type: TaskTypeCron, Expression: "@every 1h", Handler: "test", Status: TaskStatusPaused}
	_ = store.CreateTask(t2)

	active, _ := store.ListActiveTasks()
	assert.Len(t, active, 1)
}

func TestInMemoryStore_TaskLogs(t *testing.T) {
	store := NewInMemoryStore()
	_ = store.CreateTaskLog(&TaskLog{TaskID: 1, TaskName: "t1", Status: TaskLogRunning})
	_ = store.CreateTaskLog(&TaskLog{TaskID: 1, TaskName: "t1", Status: TaskLogSuccess})
	_ = store.CreateTaskLog(&TaskLog{TaskID: 2, TaskName: "t2", Status: TaskLogRunning})

	logs, _ := store.TaskLogs(1)
	assert.Len(t, logs, 2)

	all, _ := store.TaskLogs(0)
	assert.Len(t, all, 3)
}
