package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Instance struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Address   string            `json:"address"`
	Port      int               `json:"port"`
	Tags      []string          `json:"tags,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Status    string            `json:"status"` // up / down / unknown
	LastSeen  time.Time         `json:"last_seen"`
}

type Registry interface {
	Register(ctx context.Context, inst Instance, ttl time.Duration) error
	Deregister(ctx context.Context, id string) error
	Renew(ctx context.Context, id string) error
	Discover(ctx context.Context, name string) ([]Instance, error)
	List(ctx context.Context) ([]Instance, error)
	Watch(ctx context.Context, name string) (<-chan []Instance, error)
	Close() error
}

type MemoryRegistry struct {
	mu        sync.RWMutex
	instances map[string]Instance
	watchers  map[string][]chan []Instance
}

func NewMemoryRegistry() *MemoryRegistry {
	return &MemoryRegistry{
		instances: make(map[string]Instance),
		watchers:  make(map[string][]chan []Instance),
	}
}

func (r *MemoryRegistry) Register(ctx context.Context, inst Instance, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	inst.Status = "up"
	inst.LastSeen = time.Now()
	r.instances[inst.ID] = inst
	r.notifyWatchers(inst.Name)
	return nil
}

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

func (r *MemoryRegistry) notifyWatchers(name string) {
	watchers := r.watchers[name]
	for _, ch := range watchers {
		select {
		case ch <- r.snapshot(name):
		default:
		}
	}
}

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
