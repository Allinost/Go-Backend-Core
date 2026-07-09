package feature

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

type Store interface {
	Load(ctx context.Context) ([]Flag, error)
	Save(ctx context.Context, flag Flag) error
	Delete(ctx context.Context, name string) error
	Close() error
}

// MySQLStore 基于 MySQL 的特性开关持久化存储，需要先调用 Init 建表
type MySQLStore struct {
	db  *sql.DB
	mu  sync.RWMutex
	tbl string
}

func NewMySQLStore(db *sql.DB, tableName string) *MySQLStore {
	if tableName == "" {
		tableName = "feature_flags"
	}
	return &MySQLStore{db: db, tbl: tableName}
}

func (s *MySQLStore) Init(ctx context.Context) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		name VARCHAR(255) PRIMARY KEY,
		enabled BOOLEAN NOT NULL DEFAULT FALSE,
		description TEXT,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`, s.tbl)
	_, err := s.db.ExecContext(ctx, q)
	return err
}

func (s *MySQLStore) Load(ctx context.Context) ([]Flag, error) {
	q := fmt.Sprintf("SELECT name, enabled, COALESCE(description,'') FROM %s", s.tbl)
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("feature: load flags failed: %w", err)
	}
	defer rows.Close()

	var flags []Flag
	for rows.Next() {
		var f Flag
		if err := rows.Scan(&f.Name, &f.Enabled, &f.Description); err != nil {
			return nil, fmt.Errorf("feature: scan flag failed: %w", err)
		}
		flags = append(flags, f)
	}
	return flags, rows.Err()
}

func (s *MySQLStore) Save(ctx context.Context, flag Flag) error {
	q := fmt.Sprintf("REPLACE INTO %s (name, enabled, description) VALUES (?,?,?)", s.tbl)
	_, err := s.db.ExecContext(ctx, q, flag.Name, flag.Enabled, flag.Description)
	if err != nil {
		return fmt.Errorf("feature: save flag %s failed: %w", flag.Name, err)
	}
	return nil
}

func (s *MySQLStore) Delete(ctx context.Context, name string) error {
	q := fmt.Sprintf("DELETE FROM %s WHERE name=?", s.tbl)
	_, err := s.db.ExecContext(ctx, q, name)
	if err != nil {
		return fmt.Errorf("feature: delete flag %s failed: %w", name, err)
	}
	return nil
}

func (s *MySQLStore) Close() error {
	return s.db.Close()
}

// syncManager 包装 Manager，所有变更操作自动持久化到 Store
type syncManager struct {
	Manager
	store Store
	ctx   context.Context
	done  chan struct{}
}

func NewSyncManager(store Store) *syncManager {
	return &syncManager{
		Manager: *NewManager(),
		store:   store,
		ctx:     context.Background(),
		done:    make(chan struct{}),
	}
}

// Start 从持久化存储加载所有开关到内存
func (s *syncManager) Start() error {
	flags, err := s.store.Load(s.ctx)
	if err != nil {
		return fmt.Errorf("feature: initial load failed: %w", err)
	}
	for _, f := range flags {
		s.RegisterOrUpdate(f)
	}
	return nil
}

// Enable 启用开关并持久化
func (s *syncManager) Enable(name string) error {
	if err := s.Manager.Enable(name); err != nil {
		return err
	}
	f, _ := s.Manager.Get(name)
	return s.store.Save(s.ctx, f)
}

// Disable 禁用开关并持久化
func (s *syncManager) Disable(name string) error {
	if err := s.Manager.Disable(name); err != nil {
		return err
	}
	f, _ := s.Manager.Get(name)
	return s.store.Save(s.ctx, f)
}

// Register 注册开关并持久化
func (s *syncManager) Register(flag Flag) error {
	if err := s.Manager.Register(flag); err != nil {
		return err
	}
	return s.store.Save(s.ctx, flag)
}

// RegisterOrUpdate 注册或更新开关并持久化
func (s *syncManager) RegisterOrUpdate(flag Flag) {
	s.Manager.RegisterOrUpdate(flag)
	_ = s.store.Save(s.ctx, flag)
}

// Delete 删除内存中的开关并从持久化存储中移除
func (s *syncManager) Delete(name string) error {
	if err := s.Manager.Delete(name); err != nil {
		return err
	}
	return s.store.Delete(s.ctx, name)
}

func (s *syncManager) Stop() {
	close(s.done)
	_ = s.store.Close()
}

var _ = time.Second