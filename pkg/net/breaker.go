package net

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

type State int32

const (
	StateClosed   State = 0
	StateOpen     State = 1
	StateHalfOpen State = 2
)

var ErrCircuitOpen = errors.New("net: 断路器已打开，请求熔断")

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

type CircuitBreakerConfig struct {
	FailureThreshold int
	SuccessThreshold int
	Timeout          time.Duration
}

func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
	}
}

func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	cb := &CircuitBreaker{
		failureThreshold: int32(cfg.FailureThreshold),
		successThreshold: int32(cfg.SuccessThreshold),
		timeout:          cfg.Timeout,
	}
	cb.state.Store(int32(StateClosed))
	return cb
}

func (cb *CircuitBreaker) State() State {
	return State(cb.state.Load())
}

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

func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state.Store(int32(StateClosed))
	cb.failureCount.Store(0)
	cb.successCount.Store(0)
}
