package database

import (
	"errors"
	"sync/atomic"
	"time"
)

const (
	stateClosed   int32 = 0
	stateOpen     int32 = 1
	stateHalfOpen int32 = 2
)

type CircuitBreaker struct {
	state       int32
	failures    int64
	threshold   int64
	recoveryTTL time.Duration
	lastFailure time.Time
}

func NewCircuitBreaker(threshold int64, recoveryTTL time.Duration) *CircuitBreaker {
	if threshold <= 0 {
		threshold = 5
	}
	if recoveryTTL <= 0 {
		recoveryTTL = 30 * time.Second
	}
	return &CircuitBreaker{threshold: threshold, recoveryTTL: recoveryTTL}
}

func (cb *CircuitBreaker) Allow() bool {
	state := atomic.LoadInt32(&cb.state)
	if state == stateOpen {
		if time.Since(cb.lastFailure) > cb.recoveryTTL {
			atomic.StoreInt32(&cb.state, stateHalfOpen)
			return true
		}
		return false
	}
	return true
}

func (cb *CircuitBreaker) Success() {
	atomic.StoreInt32(&cb.state, stateClosed)
	atomic.StoreInt64(&cb.failures, 0)
}

func (cb *CircuitBreaker) Failure() {
	fails := atomic.AddInt64(&cb.failures, 1)
	cb.lastFailure = time.Now()
	if fails >= cb.threshold {
		atomic.StoreInt32(&cb.state, stateOpen)
	}
}

func (cb *CircuitBreaker) Run(fn func() error) error {
	if !cb.Allow() {
		return errors.New("circuit breaker: open, request rejected")
	}
	err := fn()
	if err != nil {
		cb.Failure()
		return err
	}
	cb.Success()
	return nil
}
