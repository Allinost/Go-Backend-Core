package feature

import (
	"fmt"
	"sync"
)

// Flag 表示一个特性开关
type Flag struct {
	Name        string `json:"name"`                  // 开关名称
	Enabled     bool   `json:"enabled"`               // 是否启用
	Description string `json:"description,omitempty"` // 开关描述
}

// Manager 管理内存中的特性开关，所有操作都是协程安全的
type Manager struct {
	mu    sync.RWMutex
	flags map[string]Flag // 开关名称到 Flag 的映射
}

// NewManager 创建特性管理器实例
func NewManager() *Manager {
	return &Manager{flags: make(map[string]Flag)}
}

// Register 注册一个新特性开关，如果已存在则返回错误
func (m *Manager) Register(flag Flag) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.flags[flag.Name]; ok {
		return fmt.Errorf("feature: flag %s already registered", flag.Name)
	}
	m.flags[flag.Name] = flag
	return nil
}

// RegisterOrUpdate 注册或更新一个特性开关
func (m *Manager) RegisterOrUpdate(flag Flag) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.flags[flag.Name] = flag
}

// IsEnabled 检查指定开关是否启用，不存在的开关返回 false
func (m *Manager) IsEnabled(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	f, ok := m.flags[name]
	return ok && f.Enabled
}

// Enable 启用指定特性开关
func (m *Manager) Enable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.flags[name]
	if !ok {
		return fmt.Errorf("feature: flag %s not found", name)
	}
	f.Enabled = true
	m.flags[name] = f
	return nil
}

// Disable 禁用指定特性开关
func (m *Manager) Disable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.flags[name]
	if !ok {
		return fmt.Errorf("feature: flag %s not found", name)
	}
	f.Enabled = false
	m.flags[name] = f
	return nil
}

// Delete 删除指定特性开关，开关不存在时返回错误
func (m *Manager) Delete(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.flags[name]; !ok {
		return fmt.Errorf("feature: flag %s not found", name)
	}
	delete(m.flags, name)
	return nil
}

// List 返回所有已注册的特性开关
func (m *Manager) List() []Flag {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]Flag, 0, len(m.flags))
	for _, f := range m.flags {
		list = append(list, f)
	}
	return list
}

// Get 查询指定开关，第二个返回值表示是否存在
func (m *Manager) Get(name string) (Flag, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	f, ok := m.flags[name]
	return f, ok
}
