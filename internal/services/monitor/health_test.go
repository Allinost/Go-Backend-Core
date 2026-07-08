package monitor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHealthChecker_AllUp(t *testing.T) {
	h := NewHealthChecker()
	h.Register("db", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusUp}
	})
	h.Register("redis", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusUp}
	})

	results := h.Check(context.Background())
	assert.Len(t, results, 2)
	assert.Equal(t, StatusUp, results["db"].Status)
	assert.Equal(t, StatusUp, results["redis"].Status)
	assert.True(t, h.IsHealthy(context.Background()))
}

func TestHealthChecker_PartialDown(t *testing.T) {
	h := NewHealthChecker()
	h.Register("db", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusUp}
	})
	h.Register("redis", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusDown, Error: "connection refused"}
	})

	results := h.Check(context.Background())
	assert.Equal(t, StatusUp, results["db"].Status)
	assert.Equal(t, StatusDown, results["redis"].Status)
	assert.Contains(t, results["redis"].Error, "connection refused")
	assert.False(t, h.IsHealthy(context.Background()))
}

func TestHealthChecker_AllDown(t *testing.T) {
	h := NewHealthChecker()
	h.Register("db", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusDown, Error: "timeout"}
	})
	h.Register("redis", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusDown, Error: "no reachable"}
	})

	assert.False(t, h.IsHealthy(context.Background()))
}

func TestHealthChecker_Remove(t *testing.T) {
	h := NewHealthChecker()
	h.Register("db", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusUp}
	})
	h.Remove("db")

	results := h.Check(context.Background())
	assert.Empty(t, results)
}

func TestHealthChecker_Latency(t *testing.T) {
	h := NewHealthChecker()
	h.Register("slow", func(ctx context.Context) CheckResult {
		time.Sleep(10 * time.Millisecond)
		return CheckResult{Status: StatusUp}
	})

	result := h.Check(context.Background())
	assert.GreaterOrEqual(t, result["slow"].Latency, 10*time.Millisecond)
}

func TestHealthChecker_Concurrent(t *testing.T) {
	h := NewHealthChecker()
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		h.Register(name, func(ctx context.Context) CheckResult {
			time.Sleep(5 * time.Millisecond)
			return CheckResult{Status: StatusUp}
		})
	}

	start := time.Now()
	results := h.Check(context.Background())
	duration := time.Since(start)

	assert.Len(t, results, 10)
	assert.Less(t, duration, 50*time.Millisecond)
}

func TestHealthChecker_Timeout(t *testing.T) {
	h := NewHealthChecker()
	h.Register("slow", func(ctx context.Context) CheckResult {
		<-ctx.Done()
		return CheckResult{Status: StatusDown, Error: "context deadline exceeded"}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	results := h.Check(ctx)
	assert.Equal(t, StatusDown, results["slow"].Status)
}

func TestHealthChecker_OnStatusChange(t *testing.T) {
	h := NewHealthChecker()
	var changes []struct {
		name string
		from Status
		to   Status
	}

	h.OnStatusChange(func(name string, from, to Status) {
		changes = append(changes, struct {
			name string
			from Status
			to   Status
		}{name, from, to})
	})

	status := StatusUp
	h.Register("db", func(ctx context.Context) CheckResult {
		return CheckResult{Status: status}
	})

	// 第一次 pollOnce: previous 为空，不触发回调
	h.pollOnce(context.Background())
	assert.Empty(t, changes)

	// 第二次 pollOnce: status 未变，不触发回调
	h.pollOnce(context.Background())
	assert.Empty(t, changes)

	// 第三次 pollOnce: up → down，触发回调
	status = StatusDown
	h.pollOnce(context.Background())
	assert.Len(t, changes, 1)
	assert.Equal(t, "db", changes[0].name)
	assert.Equal(t, StatusUp, changes[0].from)
	assert.Equal(t, StatusDown, changes[0].to)

	// 第四次 pollOnce: down → up，再次触发
	status = StatusUp
	h.pollOnce(context.Background())
	assert.Len(t, changes, 2)
	assert.Equal(t, StatusDown, changes[1].from)
	assert.Equal(t, StatusUp, changes[1].to)

	// 第五次 pollOnce: up → up 无变化
	h.pollOnce(context.Background())
	assert.Len(t, changes, 2)
}
