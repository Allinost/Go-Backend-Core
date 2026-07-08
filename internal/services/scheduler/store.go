package scheduler

import (
	"database/sql"
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Store interface {
	CreateTask(t *Task) error
	UpdateTask(t *Task) error
	DeleteTask(id uint) error
	GetTask(id uint) (*Task, error)
	ListTasks() ([]Task, error)
	ListActiveTasks() ([]Task, error)

	CreateTaskLog(l *TaskLog) error
	UpdateTaskLog(l *TaskLog) error
	TaskLogs(taskID uint) ([]TaskLog, error)
}

type MySQLStore struct {
	db *gorm.DB
}

func NewMySQLStoreFromDB(sqlDB *sql.DB) (*MySQLStore, error) {
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("scheduler: GORM 初始化失败: %w", err)
	}
	return &MySQLStore{db: db}, nil
}

func (s *MySQLStore) AutoMigrate() error {
	return s.db.AutoMigrate(&Task{}, &TaskLog{})
}

func (s *MySQLStore) CreateTask(t *Task) error {
	return s.db.Create(t).Error
}

func (s *MySQLStore) UpdateTask(t *Task) error {
	updates := map[string]interface{}{
		"name":        t.Name,
		"type":        t.Type,
		"expression":  t.Expression,
		"handler":     t.Handler,
		"payload":     t.Payload,
		"status":      t.Status,
		"max_retries": t.MaxRetries,
		"timeout":     t.Timeout,
		"last_run_at": t.LastRunAt,
		"next_run_at": t.NextRunAt,
		"updated_at":  gorm.Expr("NOW()"),
	}
	return s.db.Model(&Task{}).Where("id = ?", t.ID).Updates(updates).Error
}

func (s *MySQLStore) DeleteTask(id uint) error {
	return s.db.Delete(&Task{}, id).Error
}

func (s *MySQLStore) GetTask(id uint) (*Task, error) {
	var t Task
	err := s.db.First(&t, id).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *MySQLStore) ListTasks() ([]Task, error) {
	var tasks []Task
	err := s.db.Order("created_at DESC").Find(&tasks).Error
	return tasks, err
}

func (s *MySQLStore) ListActiveTasks() ([]Task, error) {
	var tasks []Task
	err := s.db.Where("status = ?", TaskStatusActive).Find(&tasks).Error
	return tasks, err
}

func (s *MySQLStore) CreateTaskLog(l *TaskLog) error {
	return s.db.Create(l).Error
}

func (s *MySQLStore) UpdateTaskLog(l *TaskLog) error {
	return s.db.Save(l).Error
}

func (s *MySQLStore) TaskLogs(taskID uint) ([]TaskLog, error) {
	var logs []TaskLog
	q := s.db.Order("created_at DESC")
	if taskID > 0 {
		q = q.Where("task_id = ?", taskID)
	}
	err := q.Find(&logs).Error
	return logs, err
}

var _ Store = (*MySQLStore)(nil)

type InMemoryStore struct {
	tasks    []Task
	taskLogs []TaskLog
	taskSeq  uint
	logSeq   uint
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{}
}

func (s *InMemoryStore) CreateTask(t *Task) error {
	for _, existing := range s.tasks {
		if existing.Name == t.Name {
			return fmt.Errorf("scheduler: 任务 %s 已存在", t.Name)
		}
	}
	s.taskSeq++
	t.ID = s.taskSeq
	now := time.Now()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	if t.Status == "" {
		t.Status = TaskStatusActive
	}
	if t.MaxRetries <= 0 {
		t.MaxRetries = 3
	}
	if t.Timeout <= 0 {
		t.Timeout = 300
	}
	s.tasks = append(s.tasks, *t)
	*t = s.tasks[len(s.tasks)-1]
	return nil
}

func (s *InMemoryStore) UpdateTask(t *Task) error {
	for i, existing := range s.tasks {
		if existing.ID == t.ID {
			if t.Name != "" {
				s.tasks[i].Name = t.Name
			}
			if t.Type != "" {
				s.tasks[i].Type = t.Type
			}
			if t.Expression != "" {
				s.tasks[i].Expression = t.Expression
			}
			if t.Handler != "" {
				s.tasks[i].Handler = t.Handler
			}
			if t.Payload != nil {
				s.tasks[i].Payload = t.Payload
			}
			if t.Status != "" {
				s.tasks[i].Status = t.Status
			}
			if t.MaxRetries > 0 {
				s.tasks[i].MaxRetries = t.MaxRetries
			}
			if t.Timeout > 0 {
				s.tasks[i].Timeout = t.Timeout
			}
			if t.LastRunAt != nil {
				s.tasks[i].LastRunAt = t.LastRunAt
			}
			if t.NextRunAt != nil {
				s.tasks[i].NextRunAt = t.NextRunAt
			}
			s.tasks[i].UpdatedAt = time.Now()
			*t = s.tasks[i]
			return nil
		}
	}
	return fmt.Errorf("scheduler: 任务 %d 不存在", t.ID)
}

func (s *InMemoryStore) DeleteTask(id uint) error {
	for i, t := range s.tasks {
		if t.ID == id {
			s.tasks = append(s.tasks[:i], s.tasks[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("scheduler: 任务 %d 不存在", id)
}

func (s *InMemoryStore) GetTask(id uint) (*Task, error) {
	for _, t := range s.tasks {
		if t.ID == id {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("scheduler: 任务 %d 不存在", id)
}

func (s *InMemoryStore) ListTasks() ([]Task, error) {
	result := make([]Task, len(s.tasks))
	copy(result, s.tasks)
	return result, nil
}

func (s *InMemoryStore) ListActiveTasks() ([]Task, error) {
	result := make([]Task, 0)
	for _, t := range s.tasks {
		if t.Status == TaskStatusActive {
			result = append(result, t)
		}
	}
	return result, nil
}

func (s *InMemoryStore) CreateTaskLog(l *TaskLog) error {
	s.logSeq++
	l.ID = s.logSeq
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now()
	}
	s.taskLogs = append(s.taskLogs, *l)
	*l = s.taskLogs[len(s.taskLogs)-1]
	return nil
}

func (s *InMemoryStore) UpdateTaskLog(l *TaskLog) error {
	for i, existing := range s.taskLogs {
		if existing.ID == l.ID {
			s.taskLogs[i] = *l
			return nil
		}
	}
	return nil
}

func (s *InMemoryStore) TaskLogs(taskID uint) ([]TaskLog, error) {
	result := make([]TaskLog, 0)
	for _, l := range s.taskLogs {
		if taskID == 0 || l.TaskID == taskID {
			result = append(result, l)
		}
	}
	return result, nil
}

var _ Store = (*InMemoryStore)(nil)
