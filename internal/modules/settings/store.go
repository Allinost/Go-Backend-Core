package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Data 可持久化的应用设置（不含任何密码/密钥）
type Data struct {
	PublicBaseURL   string `json:"public_base_url"`
	FrontendTitle   string `json:"frontend_title"`
	FrontendAPIBase string `json:"frontend_api_base_url"`
	BrowserURL      string `json:"browser_url"`
	N8nURL          string `json:"n8n_url"`
	OpenClawURL     string `json:"openclaw_url"`
	LangBotURL      string `json:"langbot_url"`
}

// Store 设置文件存储（JSON，原子写入）
type Store struct {
	path string
	mu   sync.RWMutex
}

// NewStore 创建设置存储
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Path 返回设置文件路径
func (s *Store) Path() string {
	return s.path
}

// Exists 设置文件是否已存在
func (s *Store) Exists() bool {
	_, err := os.Stat(s.path)
	return err == nil
}

// Load 读取设置；文件不存在时返回 defaults
func (s *Store) Load(defaults Data) (Data, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaults, nil
		}
		return defaults, fmt.Errorf("读取设置文件失败: %w", err)
	}
	data := defaults
	if err := json.Unmarshal(raw, &data); err != nil {
		return defaults, fmt.Errorf("解析设置文件失败: %w", err)
	}
	return data, nil
}

// Save 原子写入设置文件（0600）
func (s *Store) Save(data Data) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("创建设置目录失败: %w", err)
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化设置失败: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("写入临时设置文件失败: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("替换设置文件失败: %w", err)
	}
	return nil
}
