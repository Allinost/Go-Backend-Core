package feature

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// Store 特性开关持久化存储接口
type Store interface {
	Load(ctx context.Context) ([]Flag, error)      // 加载所有开关
	Save(ctx context.Context, flag Flag) error     // 保存单个开关
	Delete(ctx context.Context, name string) error // 删除开关
	Close() error                                  // 关闭存储
}

// MySQLStore 基于 MySQL 的特性开关持久化存储，需要先调用 Init 建表
type MySQLStore struct {
	db  *sql.DB      // 数据库连接
	mu  sync.RWMutex // 读写锁
	tbl string       // 表名
}

// NewMySQLStore 创建 MySQL 存储，默认表名为 feature_flags
func NewMySQLStore(db *sql.DB, tableName string) *MySQLStore {
	if tableName == "" {
		tableName = "feature_flags"
	}
	return &MySQLStore{db: db, tbl: tableName}
}

// Init 创建特性开关表（如果不存在）
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

// Load 从 MySQL 查询所有开关
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

// Save 写入或替换开关到 MySQL
func (s *MySQLStore) Save(ctx context.Context, flag Flag) error {
	q := fmt.Sprintf("REPLACE INTO %s (name, enabled, description) VALUES (?,?,?)", s.tbl)
	_, err := s.db.ExecContext(ctx, q, flag.Name, flag.Enabled, flag.Description)
	if err != nil {
		return fmt.Errorf("feature: save flag %s failed: %w", flag.Name, err)
	}
	return nil
}

// Delete 从 MySQL 删除指定开关
func (s *MySQLStore) Delete(ctx context.Context, name string) error {
	q := fmt.Sprintf("DELETE FROM %s WHERE name=?", s.tbl)
	_, err := s.db.ExecContext(ctx, q, name)
	if err != nil {
		return fmt.Errorf("feature: delete flag %s failed: %w", name, err)
	}
	return nil
}

// Close 关闭 MySQL 连接
func (s *MySQLStore) Close() error {
	return s.db.Close()
}

// syncManager 包装 Manager，所有变更操作自动持久化到 Store
// syncManager 自动持久化的特性管理器，所有变更操作同步写入 Store
type syncManager struct {
	Manager
	store Store           // 持久化存储
	ctx   context.Context // 全局上下文
	done  chan struct{}   // 停止信号通道
}

// NewSyncManager 创建自动持久化的特性管理器
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

// Stop 停止管理器，关闭存储连接
func (s *syncManager) Stop() {
	close(s.done)
	_ = s.store.Close()
}

var _ = time.Second
