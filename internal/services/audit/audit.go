package audit

import (
	"context"
	"sync"
	"time"
)

// Entry 审计日志条目
type Entry struct {
	ID        uint64    `json:"id"`
	Action    string    `json:"action"`
	UserID    string    `json:"user_id,omitempty"`
	Username  string    `json:"username,omitempty"`
	Resource  string    `json:"resource,omitempty"`
	Detail    string    `json:"detail,omitempty"`
	ClientIP  string    `json:"client_ip,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Store 审计日志存储接口
type Store interface {
	Append(ctx context.Context, entry Entry) error
	Query(ctx context.Context, filter Filter) ([]Entry, error)
	Close() error
}

// Filter 审计日志查询过滤器，所有字段可选，空字段不参与过滤
type Filter struct {
	Action    string    `json:"action,omitempty"`
	UserID    string    `json:"user_id,omitempty"`
	Resource  string    `json:"resource,omitempty"`
	Status    string    `json:"status,omitempty"`
	StartTime time.Time `json:"start_time,omitempty"`
	EndTime   time.Time `json:"end_time,omitempty"`
	Offset    int       `json:"offset"`
	Limit     int       `json:"limit"`
}

// Service 审计日志服务，提供 Record 记录和 Query 查询功能
type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

// Record 写入一条审计日志（ID 由 Store 分配）
func (s *Service) Record(ctx context.Context, action, userID, username, resource, detail, clientIP, status string) error {
	entry := Entry{
		Action:    action,
		UserID:    userID,
		Username:  username,
		Resource:  resource,
		Detail:    detail,
		ClientIP:  clientIP,
		Status:    status,
		CreatedAt: time.Now(),
	}
	return s.store.Append(ctx, entry)
}

// Query 按条件查询审计日志
func (s *Service) Query(ctx context.Context, filter Filter) ([]Entry, error) {
	return s.store.Query(ctx, filter)
}

func (s *Service) Close() error {
	return s.store.Close()
}

// MemoryStore 基于内存的审计日志存储，自带二级索引（action/user/resource/status）加速查询
type MemoryStore struct {
	mu     sync.RWMutex
	log    []Entry
	nextID uint64

	// 倒排索引：字段值 -> 条目 ID 集合
	idxAction   map[string]map[uint64]struct{}
	idxUserID   map[string]map[uint64]struct{}
	idxResource map[string]map[uint64]struct{}
	idxStatus   map[string]map[uint64]struct{}
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		log:         make([]Entry, 0),
		idxAction:   make(map[string]map[uint64]struct{}),
		idxUserID:   make(map[string]map[uint64]struct{}),
		idxResource: make(map[string]map[uint64]struct{}),
		idxStatus:   make(map[string]map[uint64]struct{}),
	}
}

// Append 追加审计日志并更新倒排索引
func (m *MemoryStore) Append(ctx context.Context, entry Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	entry.ID = m.nextID
	m.log = append(m.log, entry)

	// 更新倒排索引
	m.indexAdd(m.idxAction, entry.Action, entry.ID)
	m.indexAdd(m.idxUserID, entry.UserID, entry.ID)
	m.indexAdd(m.idxResource, entry.Resource, entry.ID)
	m.indexAdd(m.idxStatus, entry.Status, entry.ID)

	return nil
}

// indexAdd 向倒排索引中添加一条记录
func (m *MemoryStore) indexAdd(idx map[string]map[uint64]struct{}, key string, id uint64) {
	if key == "" {
		return
	}
	set, ok := idx[key]
	if !ok {
		set = make(map[uint64]struct{})
		idx[key] = set
	}
	set[id] = struct{}{}
}

// Query 按条件过滤，优先利用倒排索引缩小扫描范围
func (m *MemoryStore) Query(ctx context.Context, filter Filter) ([]Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 收集所有非空过滤条件的匹配 ID 集合
	var sets []map[uint64]struct{}
	if filter.Action != "" {
		if s := m.idxAction[filter.Action]; len(s) > 0 {
			sets = append(sets, s)
		} else {
			return []Entry{}, nil
		}
	}
	if filter.UserID != "" {
		if s := m.idxUserID[filter.UserID]; len(s) > 0 {
			sets = append(sets, s)
		} else {
			return []Entry{}, nil
		}
	}
	if filter.Resource != "" {
		if s := m.idxResource[filter.Resource]; len(s) > 0 {
			sets = append(sets, s)
		} else {
			return []Entry{}, nil
		}
	}
	if filter.Status != "" {
		if s := m.idxStatus[filter.Status]; len(s) > 0 {
			sets = append(sets, s)
		} else {
			return []Entry{}, nil
		}
	}

	var result []Entry

	if len(sets) == 0 {
		// 无条件过滤，遍历全量数据
		result = make([]Entry, 0, len(m.log))
		for _, e := range m.log {
			if m.matchTime(&e, filter) {
				result = append(result, e)
			}
		}
	} else {
		// 取所有索引集合的交集，只扫描命中的条目
		candidates := intersectSets(sets)
		result = make([]Entry, 0, len(candidates))
		for _, e := range m.log {
			if _, ok := candidates[e.ID]; ok {
				if m.matchTime(&e, filter) {
					result = append(result, e)
				}
			}
		}
	}

	// 分页
	if filter.Offset > 0 && filter.Offset < len(result) {
		result = result[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}
	if result == nil {
		result = []Entry{}
	}
	return result, nil
}

// matchTime 检查条目的 CreatedAt 是否在时间范围内
func (m *MemoryStore) matchTime(e *Entry, f Filter) bool {
	if !f.StartTime.IsZero() && e.CreatedAt.Before(f.StartTime) {
		return false
	}
	if !f.EndTime.IsZero() && e.CreatedAt.After(f.EndTime) {
		return false
	}
	return true
}

// intersectSets 计算多个 ID 集合的交集
func intersectSets(sets []map[uint64]struct{}) map[uint64]struct{} {
	if len(sets) == 0 {
		return nil
	}
	result := make(map[uint64]struct{})
	for id := range sets[0] {
		inAll := true
		for _, s := range sets[1:] {
			if _, ok := s[id]; !ok {
				inAll = false
				break
			}
		}
		if inAll {
			result[id] = struct{}{}
		}
	}
	return result
}

func (m *MemoryStore) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.log = nil
	m.idxAction = nil
	m.idxUserID = nil
	m.idxResource = nil
	m.idxStatus = nil
	return nil
}

// RecordLogin 记录登录事件
func RecordLogin(ctx context.Context, userID, username, clientIP, status string) error {
	return getDefaultService().Record(ctx, "login", userID, username, "auth", "", clientIP, status)
}

// RecordLogout 记录登出事件
func RecordLogout(ctx context.Context, userID, username, clientIP string) error {
	return getDefaultService().Record(ctx, "logout", userID, username, "auth", "", clientIP, "success")
}

// RecordCreate 记录资源创建事件
func RecordCreate(ctx context.Context, userID, username, resource, detail string) error {
	return getDefaultService().Record(ctx, "create", userID, username, resource, detail, "", "success")
}

// RecordUpdate 记录资源更新事件
func RecordUpdate(ctx context.Context, userID, username, resource, detail string) error {
	return getDefaultService().Record(ctx, "update", userID, username, resource, detail, "", "success")
}

// RecordDelete 记录资源删除事件
func RecordDelete(ctx context.Context, userID, username, resource, detail string) error {
	return getDefaultService().Record(ctx, "delete", userID, username, resource, detail, "", "success")
}

// RecordFailure 记录操作失败事件
func RecordFailure(ctx context.Context, action, userID, resource, detail string) error {
	return getDefaultService().Record(ctx, action, userID, "", resource, detail, "", "failure")
}

var (
	defaultServiceMu sync.RWMutex
	defaultService   = NewService(NewMemoryStore())
)

// InitDefaultService 设置全局默认审计服务（协程安全）
func InitDefaultService(s *Service) {
	defaultServiceMu.Lock()
	defer defaultServiceMu.Unlock()
	defaultService = s
}

func getDefaultService() *Service {
	defaultServiceMu.RLock()
	defer defaultServiceMu.RUnlock()
	return defaultService
}
