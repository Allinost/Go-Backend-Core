package discovery

import (
	"context"
	"fmt"
	"time"
)

// EtcdRegistry Etcd 服务注册中心实现
type EtcdRegistry struct {
	inner *MemoryRegistry // 内部委托的内存注册中心
}

// NewEtcdRegistry 创建 Etcd 注册中心
func NewEtcdRegistry() *EtcdRegistry {
	return &EtcdRegistry{inner: NewMemoryRegistry()}
}

// Register 注册服务实例
func (e *EtcdRegistry) Register(ctx context.Context, inst Instance, ttl time.Duration) error {
	fmt.Printf("discovery: etcd register %s/%s (ttl=%v)\n", inst.Name, inst.ID, ttl)
	return e.inner.Register(ctx, inst, ttl)
}

// Deregister 注销服务实例
func (e *EtcdRegistry) Deregister(ctx context.Context, id string) error {
	fmt.Printf("discovery: etcd deregister %s\n", id)
	return e.inner.Deregister(ctx, id)
}

// Renew 续约服务实例心跳
func (e *EtcdRegistry) Renew(ctx context.Context, id string) error {
	fmt.Printf("discovery: etcd renew %s\n", id)
	return e.inner.Renew(ctx, id)
}

// Discover 发现指定服务的可用实例
func (e *EtcdRegistry) Discover(ctx context.Context, name string) ([]Instance, error) {
	return e.inner.Discover(ctx, name)
}

// List 列出所有服务实例
func (e *EtcdRegistry) List(ctx context.Context) ([]Instance, error) {
	return e.inner.List(ctx)
}

// Watch 监听指定服务的实例变化
func (e *EtcdRegistry) Watch(ctx context.Context, name string) (<-chan []Instance, error) {
	return e.inner.Watch(ctx, name)
}

// Close 关闭 Etcd 注册中心
func (e *EtcdRegistry) Close() error {
	fmt.Println("discovery: etcd close")
	return e.inner.Close()
}

var _ Registry = (*EtcdRegistry)(nil)