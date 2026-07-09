package net

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCircuitBreaker_ClosedByDefault(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())
	assert.Equal(t, StateClosed, cb.State())
	assert.True(t, cb.Allow())
}

func TestCircuitBreaker_OpenOnFailures(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 1,
		Timeout:          10 * time.Minute,
	})

	assert.True(t, cb.Allow())
	cb.Failure()
	assert.True(t, cb.Allow())
	cb.Failure()
	assert.True(t, cb.Allow())
	cb.Failure()
	assert.Equal(t, StateOpen, cb.State())
	assert.False(t, cb.Allow())
}

func TestCircuitBreaker_HalfOpenOnTimeout(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          1 * time.Nanosecond,
	})
	cb.Failure()
	assert.False(t, cb.Allow())
	time.Sleep(10 * time.Millisecond)
	assert.True(t, cb.Allow())
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          10 * time.Minute,
	})
	cb.Failure()
	assert.Equal(t, StateOpen, cb.State())
	cb.Reset()
	assert.Equal(t, StateClosed, cb.State())
	assert.True(t, cb.Allow())
}

func TestCircuitBreaker_HalfOpenSuccessCloses(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 2,
		Timeout:          1 * time.Nanosecond,
	})
	cb.Failure()
	time.Sleep(10 * time.Millisecond)
	assert.True(t, cb.Allow())
	cb.Success()
	assert.Equal(t, StateHalfOpen, cb.State())
	cb.Success()
	assert.Equal(t, StateClosed, cb.State())
}

func TestCircuitBreaker_HalfOpenFailureReopens(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          1 * time.Nanosecond,
	})
	cb.Failure()
	time.Sleep(10 * time.Millisecond)
	assert.True(t, cb.Allow())
	cb.Failure()
	assert.Equal(t, StateOpen, cb.State())
}

func TestCircuitBreaker_DefaultConfig(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	assert.Equal(t, 5, cfg.FailureThreshold)
	assert.Equal(t, 2, cfg.SuccessThreshold)
	assert.Equal(t, 30*time.Second, cfg.Timeout)
}

func TestCircuitBreaker_ConcurrentSafe(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 100,
		SuccessThreshold: 1,
		Timeout:          10 * time.Minute,
	})
	for i := 0; i < 50; i++ {
		go cb.Success()
		go cb.Failure()
	}
}
