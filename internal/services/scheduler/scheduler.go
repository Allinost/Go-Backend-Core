package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	mu         sync.RWMutex
	store      Store
	cron       *cron.Cron
	cronJobs   map[uint]cron.EntryID
	close      chan struct{}
	closeOnce  sync.Once
	timeout    int
	maxRetries int
	metrics    Metrics

	asynqClient *asynq.Client
	asynqServer *asynq.Server
}

func New(store Store, cfg SchedulerConfig, asynqClient *asynq.Client, asynqServer *asynq.Server) *Scheduler {
	timeout := cfg.DefaultTimeout
	if timeout <= 0 {
		timeout = 300
	}
	maxRetries := cfg.DefaultMaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	return &Scheduler{
		store:       store,
		cronJobs:    make(map[uint]cron.EntryID),
		close:       make(chan struct{}),
		timeout:     timeout,
		maxRetries:  maxRetries,
		metrics:     nopMetrics{},
		asynqClient: asynqClient,
		asynqServer: asynqServer,
	}
}

func (s *Scheduler) SetMetrics(m Metrics) {
	if m != nil {
		s.metrics = m
	}
}

type SchedulerConfig struct {
	Enabled           bool
	WorkerConcurrency int
	DefaultTimeout    int
	DefaultMaxRetries int
	LogRetentionDays  int
}

func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.asynqServer != nil {
		handler := NewAsynqHandler(s.store, s.metrics)
		mux := SetupAsynqMux(handler)
		if err := s.asynqServer.Start(mux); err != nil {
			return fmt.Errorf("scheduler: asynq 服务启动失败: %w", err)
		}
	}

	s.cron = cron.New(cron.WithSeconds())
	s.cron.Start()

	activeTasks, err := s.store.ListActiveTasks()
	if err != nil {
		return fmt.Errorf("scheduler: 加载任务失败: %w", err)
	}

	for i := range activeTasks {
		t := activeTasks[i]
		if err := s.scheduleTask(&t); err != nil {
			log.Printf("scheduler: 调度任务 %s 失败: %v", t.Name, err)
		}
	}

	return nil
}

func (s *Scheduler) Stop() {
	s.closeOnce.Do(func() {
		close(s.close)
	})
	if s.cron != nil {
		ctx := s.cron.Stop()
		<-ctx.Done()
	}
	if s.asynqServer != nil {
		s.asynqServer.Shutdown()
	}
	if s.asynqClient != nil {
		s.asynqClient.Close()
	}
	s.mu.Lock()
	s.cronJobs = make(map[uint]cron.EntryID)
	s.mu.Unlock()
}

func (s *Scheduler) AddTask(t *Task) error {
	if err := s.store.CreateTask(t); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cron != nil && t.Status == TaskStatusActive {
		if err := s.scheduleTask(t); err != nil {
			return err
		}
	}

	return nil
}

func (s *Scheduler) UpdateTask(t *Task) error {
	existing, err := s.store.GetTask(t.ID)
	if err != nil {
		return fmt.Errorf("scheduler: 任务 %d 不存在", t.ID)
	}

	s.mu.Lock()
	if s.cron != nil {
		if eid, ok := s.cronJobs[t.ID]; ok {
			s.cron.Remove(eid)
			delete(s.cronJobs, t.ID)
		}
	}
	s.mu.Unlock()

	if t.Name != "" {
		existing.Name = t.Name
	}
	if t.Type != "" {
		existing.Type = t.Type
	}
	if t.Expression != "" {
		existing.Expression = t.Expression
	}
	if t.Handler != "" {
		existing.Handler = t.Handler
	}
	if t.Payload != nil {
		existing.Payload = t.Payload
	}
	if t.Status != "" {
		existing.Status = t.Status
	}
	if t.MaxRetries > 0 {
		existing.MaxRetries = t.MaxRetries
	}
	if t.Timeout > 0 {
		existing.Timeout = t.Timeout
	}

	if err := s.store.UpdateTask(existing); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cron != nil && existing.Status == TaskStatusActive {
		if err := s.scheduleTask(existing); err != nil {
			return err
		}
	}

	return nil
}

func (s *Scheduler) DeleteTask(id uint) error {
	if err := s.store.DeleteTask(id); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cron != nil {
		if eid, ok := s.cronJobs[id]; ok {
			s.cron.Remove(eid)
			delete(s.cronJobs, id)
		}
	}

	return nil
}

func (s *Scheduler) GetTask(id uint) (*Task, error) {
	return s.store.GetTask(id)
}

func (s *Scheduler) ListTasks() ([]Task, error) {
	return s.store.ListTasks()
}

func (s *Scheduler) PauseTask(id uint) error {
	t, err := s.store.GetTask(id)
	if err != nil {
		return err
	}
	t.Status = TaskStatusPaused
	return s.store.UpdateTask(t)
}

func (s *Scheduler) ResumeTask(id uint) error {
	t, err := s.store.GetTask(id)
	if err != nil {
		return err
	}
	t.Status = TaskStatusActive
	if err := s.store.UpdateTask(t); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cron != nil {
		if err := s.scheduleTask(t); err != nil {
			return err
		}
	}

	return nil
}

func (s *Scheduler) RunTaskNow(id uint) error {
	t, err := s.store.GetTask(id)
	if err != nil {
		return err
	}

	if s.asynqClient != nil {
		return enqueueTask(s.asynqClient, t, t.Timeout)
	}

	handler := GetHandler(t.Handler)
	if handler == nil {
		return fmt.Errorf("scheduler: 处理器 %s 未注册", t.Handler)
	}

	go func() {
		logEntry := &TaskLog{
			TaskID:    t.ID,
			TaskName:  t.Name,
			Status:    TaskLogRunning,
			StartedAt: time.Now(),
		}
		_ = s.store.CreateTaskLog(logEntry)

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(t.Timeout)*time.Second)
		defer cancel()

		err := handler.Execute(ctx, t.Payload)
		now := time.Now()
		logEntry.EndedAt = &now

		if err != nil {
			logEntry.Status = TaskLogFailed
			logEntry.Error = err.Error()
		} else {
			logEntry.Status = TaskLogSuccess
		}
		_ = s.store.UpdateTaskLog(logEntry)
	}()

	return nil
}

func (s *Scheduler) TaskLogs(taskID uint) ([]TaskLog, error) {
	return s.store.TaskLogs(taskID)
}

func (s *Scheduler) scheduleTask(t *Task) error {
	handler := GetHandler(t.Handler)
	if handler == nil {
		return fmt.Errorf("scheduler: 处理器 %s 未注册", t.Handler)
	}

	var entryID cron.EntryID
	var err error

	switch t.Type {
	case TaskTypeCron:
		entryID, err = s.cron.AddFunc(t.Expression, func() {
			s.executeTask(t)
		})
		if err != nil {
			return fmt.Errorf("scheduler: cron %q 无效: %w", t.Expression, err)
		}
		t.NextRunAt = nextTime(t.Expression)

	case TaskTypeInterval:
		dur, err := time.ParseDuration(t.Expression)
		if err != nil {
			return fmt.Errorf("scheduler: 间隔 %q 无效: %w", t.Expression, err)
		}
		entryID = s.cron.Schedule(cron.Every(dur), cron.FuncJob(func() {
			s.executeTask(t)
		}))
		now := time.Now()
		t.NextRunAt = &now

	case TaskTypeOnce:
		go func() {
			delay := parseOnceDelay(t.Expression)
			select {
			case <-time.After(delay):
				s.executeTask(t)
			case <-s.close:
			}
		}()
		now := time.Now()
		t.NextRunAt = &now
		return nil

	default:
		return fmt.Errorf("scheduler: 未知任务类型 %s", t.Type)
	}

	s.cronJobs[t.ID] = entryID
	return nil
}

func (s *Scheduler) executeTask(t *Task) {
	if s.asynqClient != nil {
		if err := enqueueTask(s.asynqClient, t, t.Timeout); err != nil {
			log.Printf("scheduler: 入队任务 %s 失败: %v", t.Name, err)
		}
		return
	}

	handler := GetHandler(t.Handler)
	if handler == nil {
		log.Printf("scheduler: 任务 %s 处理器 %s 未注册", t.Name, t.Handler)
		return
	}

	logEntry := &TaskLog{
		TaskID:    t.ID,
		TaskName:  t.Name,
		Status:    TaskLogRunning,
		StartedAt: time.Now(),
	}
	_ = s.store.CreateTaskLog(logEntry)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(t.Timeout)*time.Second)
	defer cancel()

	err := handler.Execute(ctx, t.Payload)
	now := time.Now()
	logEntry.EndedAt = &now

	if err != nil {
		logEntry.Status = TaskLogFailed
		logEntry.Error = err.Error()
	} else {
		logEntry.Status = TaskLogSuccess
	}
	_ = s.store.UpdateTaskLog(logEntry)
}

func nextTime(expr string) *time.Time {
	t := time.Now().Add(time.Minute)
	return &t
}

func parseOnceDelay(expr string) time.Duration {
	t, err := time.Parse(time.RFC3339, expr)
	if err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		return d
	}
	return 0
}
