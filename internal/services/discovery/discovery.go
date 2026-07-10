package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Instance 服务实例，包含实例的唯一标识、名称、地址端口和状态信息
type Instance struct {
	ID       string            `json:"id"`                 // 实例唯一标识
	Name     string            `json:"name"`               // 服务名称
	Address  string            `json:"address"`            // 实例地址
	Port     int               `json:"port"`               // 实例端口
	Tags     []string          `json:"tags,omitempty"`     // 标签列表
	Metadata map[string]string `json:"metadata,omitempty"` // 附加元数据
	Status   string            `json:"status"`             // 运行状态：up / down / unknown
	LastSeen time.Time         `json:"last_seen"`          // 最后心跳时间
}

// Registry 服务注册中心接口，定义实例注册、发现、监听等基本操作
type Registry interface {
	Register(ctx context.Context, inst Instance, ttl time.Duration) error
	Deregister(ctx context.Context, id string) error
	Renew(ctx context.Context, id string) error
	Discover(ctx context.Context, name string) ([]Instance, error)
	List(ctx context.Context) ([]Instance, error)
	Watch(ctx context.Context, name string) (<-chan []Instance, error)
	Close() error
}

// MemoryRegistry 基于内存的服务注册中心，协程安全，支持监听机制
type MemoryRegistry struct {
	mu        sync.RWMutex
	instances map[string]Instance          // 实例 ID 到实例的映射
	watchers  map[string][]chan []Instance // 服务名到监听器通道列表的映射
}

// NewMemoryRegistry 创建内存注册中心实例
func NewMemoryRegistry() *MemoryRegistry {
	return &MemoryRegistry{
		instances: make(map[string]Instance),
		watchers:  make(map[string][]chan []Instance),
	}
}

// Register 注册服务实例，设置状态为 up 并更新最后时间，通知监听者
func (r *MemoryRegistry) Register(ctx context.Context, inst Instance, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	inst.Status = "up"
	inst.LastSeen = time.Now()
	r.instances[inst.ID] = inst
	r.notifyWatchers(inst.Name)
	return nil
}

// Deregister 注销指定 ID 的服务实例，通知监听者，实例不存在时返回错误
func (r *MemoryRegistry) Deregister(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	inst, ok := r.instances[id]
	if !ok {
		return fmt.Errorf("discovery: instance %s not found", id)
	}
	delete(r.instances, id)
	r.notifyWatchers(inst.Name)
	return nil
}

// Renew 续约实例心跳，更新最后时间并保持状态为 up
func (r *MemoryRegistry) Renew(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	inst, ok := r.instances[id]
	if !ok {
		return fmt.Errorf("discovery: instance %s not found", id)
	}
	inst.LastSeen = time.Now()
	inst.Status = "up"
	r.instances[id] = inst
	return nil
}

// Discover 发现指定服务名下所有状态为 up 的实例
func (r *MemoryRegistry) Discover(ctx context.Context, name string) ([]Instance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []Instance
	for _, inst := range r.instances {
		if inst.Name == name && inst.Status == "up" {
			result = append(result, inst)
		}
	}
	if result == nil {
		result = []Instance{}
	}
	return result, nil
}

// List 返回所有注册的服务实例
func (r *MemoryRegistry) List(ctx context.Context) ([]Instance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Instance, 0, len(r.instances))
	for _, inst := range r.instances {
		result = append(result, inst)
	}
	if result == nil {
		result = []Instance{}
	}
	return result, nil
}

// Watch 监听指定服务的实例变化，context 取消时自动清理监听器
func (r *MemoryRegistry) Watch(ctx context.Context, name string) (<-chan []Instance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch := make(chan []Instance, 16)
	r.watchers[name] = append(r.watchers[name], ch)
	go func() {
		<-ctx.Done()
		r.mu.Lock()
		defer r.mu.Unlock()
		watchers := r.watchers[name]
		for i, w := range watchers {
			if w == ch {
				r.watchers[name] = append(watchers[:i], watchers[i+1:]...)
				break
			}
		}
		close(ch)
	}()
	return ch, nil
}

// Close 关闭注册中心，关闭所有监听通道并清空数据
func (r *MemoryRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, watchers := range r.watchers {
		for _, ch := range watchers {
			close(ch)
		}
	}
	r.instances = make(map[string]Instance)
	r.watchers = make(map[string][]chan []Instance)
	return nil
}

// notifyWatchers 向指定服务的所有监听器发送当前快照（非阻塞）
func (r *MemoryRegistry) notifyWatchers(name string) {
	watchers := r.watchers[name]
	for _, ch := range watchers {
		select {
		case ch <- r.snapshot(name):
		default:
		}
	}
}

// snapshot 获取指定服务的所有实例快照
func (r *MemoryRegistry) snapshot(name string) []Instance {
	var result []Instance
	for _, inst := range r.instances {
		if inst.Name == name {
			result = append(result, inst)
		}
	}
	if result == nil {
		result = []Instance{}
	}
	return result
}
