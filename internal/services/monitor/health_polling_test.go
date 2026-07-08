package monitor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHealthChecker_PollingEnabled(t *testing.T) {
	h := NewHealthChecker()
	h.Register("db", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusUp}
	})

	h.StartPolling(context.Background(), PollingConfig{Enabled: true, Interval: 50 * time.Millisecond})
	defer h.Stop()

	time.Sleep(100 * time.Millisecond)
	result := h.LastResult()
	assert.NotEmpty(t, result)
	assert.Equal(t, StatusUp, result["db"].Status)
}

func TestHealthChecker_PollingDisabled(t *testing.T) {
	h := NewHealthChecker()
	h.Register("db", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusUp}
	})

	h.StartPolling(context.Background(), PollingConfig{Enabled: false})
	assert.Nil(t, h.LastResult())
}

func TestHealthChecker_StopPolling(t *testing.T) {
	h := NewHealthChecker()
	h.Register("db", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusUp}
	})

	h.StartPolling(context.Background(), PollingConfig{Enabled: true})
	h.Stop()
	time.Sleep(50 * time.Millisecond)
	assert.NotPanics(t, func() { h.Stop() })
}

func TestHealthChecker_LastResult(t *testing.T) {
	h := NewHealthChecker()
	assert.Nil(t, h.LastResult())

	h.Register("db", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusUp}
	})
	h.pollOnce(context.Background())

	result := h.LastResult()
	assert.Len(t, result, 1)
	assert.Equal(t, StatusUp, result["db"].Status)
}

func TestHealthChecker_PollingDefaultInterval(t *testing.T) {
	h := NewHealthChecker()
	h.Register("db", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusUp}
	})

	h.StartPolling(context.Background(), PollingConfig{Enabled: true, Interval: 0})
	defer h.Stop()
	time.Sleep(100 * time.Millisecond)
	assert.NotEmpty(t, h.LastResult())
}

func TestHealthChecker_RemoveWithPolling(t *testing.T) {
	h := NewHealthChecker()
	h.Register("db", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusUp}
	})
	h.pollOnce(context.Background())
	assert.Len(t, h.LastResult(), 1)

	h.Remove("db")
	assert.Empty(t, h.LastResult())
}
