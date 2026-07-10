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

// Scheduler 任务调度器，基于 cron 表达式和 asynq 队列管理定时任务
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

// New 创建调度器实例，配置默认超时和重试次数
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

// SetMetrics 设置监控指标收集器
func (s *Scheduler) SetMetrics(m Metrics) {
	if m != nil {
		s.metrics = m
	}
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	Enabled           bool // 是否启用调度器
	WorkerConcurrency int  // 工作协程并发数
	DefaultTimeout    int  // 任务默认超时秒数
	DefaultMaxRetries int  // 任务默认最大重试次数
	LogRetentionDays  int  // 任务日志保留天数
}

// Start 启动调度器，启动 asynq 服务器和 cron 调度，加载所有活跃任务
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

// Stop 停止调度器，关闭 cron 调度器、asynq 服务器和客户端
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

// AddTask 添加新任务到存储，并立即调度（如果 cron 运行中且任务为活跃状态）
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

// UpdateTask 更新已有任务，移除旧 cron 条目后按新配置重新调度
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

// DeleteTask 删除指定任务并从 cron 中移除对应条目
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

// GetTask 根据 ID 获取单个任务
func (s *Scheduler) GetTask(id uint) (*Task, error) {
	return s.store.GetTask(id)
}

// ListTasks 获取所有任务列表
func (s *Scheduler) ListTasks() ([]Task, error) {
	return s.store.ListTasks()
}

// PauseTask 暂停任务，将状态设置为已暂停
func (s *Scheduler) PauseTask(id uint) error {
	t, err := s.store.GetTask(id)
	if err != nil {
		return err
	}
	t.Status = TaskStatusPaused
	return s.store.UpdateTask(t)
}

// ResumeTask 恢复已暂停的任务，将其重新加入 cron 调度
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

// RunTaskNow 立即执行指定任务，支持 asynq 队列或直接在 goroutine 中运行
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

// TaskLogs 查询指定任务的执行日志
func (s *Scheduler) TaskLogs(taskID uint) ([]TaskLog, error) {
	return s.store.TaskLogs(taskID)
}

// scheduleTask 根据任务类型（cron/interval/once）将任务加入调度队列
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

// executeTask 执行任务逻辑：通过 asynq 入队或直接调用处理器，并记录任务日志
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

// nextTime 计算 cron 表达式的下次执行时间（当前简化实现为 1 分钟后）
func nextTime(expr string) *time.Time {
	t := time.Now().Add(time.Minute)
	return &t
}

// parseOnceDelay 解析一次性任务的延迟时间（支持 RFC3339 格式）
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
