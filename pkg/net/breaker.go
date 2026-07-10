package net

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// State 断路器状态类型
type State int32

const (
	StateClosed   State = 0 // 关闭状态（正常）
	StateOpen     State = 1 // 打开状态（熔断）
	StateHalfOpen State = 2 // 半开状态（尝试恢复）
)

// ErrCircuitOpen 断路器打开时返回的错误
var ErrCircuitOpen = errors.New("net: 断路器已打开，请求熔断")

// CircuitBreaker 断路器，用于防止级联故障
type CircuitBreaker struct {
	state            atomic.Int32
	failureCount     atomic.Int32
	successCount     atomic.Int32
	failureThreshold int32
	successThreshold int32
	timeout          time.Duration
	lastFailure      atomic.Int64
	mu               sync.Mutex
	halfOpenDone     chan struct{}
}

// CircuitBreakerConfig 断路器配置
type CircuitBreakerConfig struct {
	FailureThreshold int
	SuccessThreshold int
	Timeout          time.Duration
}

// DefaultCircuitBreakerConfig 返回默认断路器配置（失败阈值=5，成功阈值=2，超时=30s）
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
	}
}

// NewCircuitBreaker 根据配置创建断路器，初始状态为关闭
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	cb := &CircuitBreaker{
		failureThreshold: int32(cfg.FailureThreshold),
		successThreshold: int32(cfg.SuccessThreshold),
		timeout:          cfg.Timeout,
	}
	cb.state.Store(int32(StateClosed))
	return cb
}

// State 返回当前断路器状态
func (cb *CircuitBreaker) State() State {
	return State(cb.state.Load())
}

// Allow 判断请求是否允许通过，半开状态下会尝试放行
func (cb *CircuitBreaker) Allow() bool {
	state := State(cb.state.Load())
	switch state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(time.Unix(0, cb.lastFailure.Load())) > cb.timeout {
			cb.mu.Lock()
			if cb.state.CompareAndSwap(int32(StateOpen), int32(StateHalfOpen)) {
				cb.successCount.Store(0)
				cb.halfOpenDone = make(chan struct{})
			}
			cb.mu.Unlock()
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return true
	}
}

// Success 记录一次成功调用，半开状态下达到阈值则关闭断路器
func (cb *CircuitBreaker) Success() {
	state := State(cb.state.Load())
	switch state {
	case StateHalfOpen:
		count := cb.successCount.Add(1)
		if count >= cb.successThreshold {
			cb.mu.Lock()
			cb.state.Store(int32(StateClosed))
			cb.failureCount.Store(0)
			cb.successCount.Store(0)
			if cb.halfOpenDone != nil {
				close(cb.halfOpenDone)
				cb.halfOpenDone = nil
			}
			cb.mu.Unlock()
		}
	case StateClosed:
		cb.failureCount.Store(0)
	}
}

// Failure 记录一次失败调用，达到阈值时打开断路器
func (cb *CircuitBreaker) Failure() {
	count := cb.failureCount.Add(1)
	state := State(cb.state.Load())
	if state == StateHalfOpen {
		cb.mu.Lock()
		cb.state.Store(int32(StateOpen))
		cb.lastFailure.Store(time.Now().UnixNano())
		cb.failureCount.Store(0)
		if cb.halfOpenDone != nil {
			close(cb.halfOpenDone)
			cb.halfOpenDone = nil
		}
		cb.mu.Unlock()
		return
	}
	if count >= cb.failureThreshold && cb.state.CompareAndSwap(int32(StateClosed), int32(StateOpen)) {
		cb.lastFailure.Store(time.Now().UnixNano())
	}
}

// Reset 手动重置断路器到关闭状态，清零计数器
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state.Store(int32(StateClosed))
	cb.failureCount.Store(0)
	cb.successCount.Store(0)
}
