package net

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClient_Get(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ts.Close()

	client := NewHTTPClient(DefaultHTTPConfig())
	resp, err := client.Get(context.Background(), ts.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, `{"status":"ok"}`, string(resp.Body))
}

func TestHTTPClient_Post(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":1}`))
	}))
	defer ts.Close()

	client := NewHTTPClient(DefaultHTTPConfig())
	resp, err := client.Post(context.Background(), ts.URL, []byte(`{"name":"test"}`), "application/json")
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestHTTPClient_Put(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := NewHTTPClient(DefaultHTTPConfig())
	resp, err := client.Put(context.Background(), ts.URL, []byte(`{"name":"updated"}`), "application/json")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHTTPClient_Delete(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	client := NewHTTPClient(DefaultHTTPConfig())
	resp, err := client.Delete(context.Background(), ts.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestHTTPClient_Do_CustomMethod(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "custom-value", r.Header.Get("X-Custom"))
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := NewHTTPClient(DefaultHTTPConfig())
	resp, err := client.Do(context.Background(), "PATCH", ts.URL, []byte(`{}`), map[string]string{
		"X-Custom": "custom-value",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHTTPClient_RetryOn5xx(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := NewHTTPClient(HTTPConfig{
		Timeout:      5 * time.Second,
		RetryMax:     3,
		RetryWaitMin: 10 * time.Millisecond,
		RetryWaitMax: 50 * time.Millisecond,
	})
	resp, err := client.Get(context.Background(), ts.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 3, attempts)
}

func TestHTTPClient_NoRetryOn4xx(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	client := NewHTTPClient(HTTPConfig{
		Timeout:      5 * time.Second,
		RetryMax:     3,
		RetryWaitMin: 10 * time.Millisecond,
		RetryWaitMax: 50 * time.Millisecond,
	})
	resp, err := client.Get(context.Background(), ts.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, 1, attempts, "4xx 不应重试")
}

func TestHTTPClient_ContextTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	client := NewHTTPClient(DefaultHTTPConfig())
	_, err := client.Get(ctx, ts.URL)
	assert.Error(t, err)
}

func TestHTTPClient_InvalidURL(t *testing.T) {
	client := NewHTTPClient(DefaultHTTPConfig())
	_, err := client.Get(context.Background(), "://invalid")
	assert.Error(t, err)
}

func TestCheckHTTP(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	code, err := CheckHTTP(context.Background(), ts.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)
}

func TestHTTPClient_ProxyConfig(t *testing.T) {
	cfg := HTTPConfig{
		ProxyURL: "http://proxy.example.com:8080",
	}
	client := NewHTTPClient(cfg)
	assert.NotNil(t, client)
}

func TestHTTPClient_BreakerRetryDecoupling(t *testing.T) {
	// 验证重试与熔断解耦：retry=3 的连续失败请求只计 1 次 Failure，
	// 而非之前的每次重试都计数（导致阈值 5 被 1-2 次请求就触发）。
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 模拟网络错误：关闭连接不写任何响应
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
		}
	}))
	defer ts.Close()

	breaker := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3, // 阈值=3
		SuccessThreshold: 1,
		Timeout:          1 * time.Hour, // 不会自动恢复
	})

	client := NewHTTPClient(HTTPConfig{
		Timeout:      2 * time.Second,
		RetryMax:     3,
		RetryWaitMin: 10 * time.Millisecond,
		RetryWaitMax: 20 * time.Millisecond,
		Breaker:      breaker,
	})

	// 第一次请求：3 次重试全部失败，应只计 1 次 Failure
	_, err := client.Get(context.Background(), ts.URL)
	assert.Error(t, err)
	assert.Equal(t, StateClosed, breaker.State(), "1 次 Failure < 阈值 3，breaker 应仍关闭")

	// 第二次请求：再 1 次 Failure（累计 2 次），仍未达阈值
	_, err = client.Get(context.Background(), ts.URL)
	assert.Error(t, err)
	assert.Equal(t, StateClosed, breaker.State(), "累计 2 次 Failure < 阈值 3，breaker 应仍关闭")

	// 第三次请求：再 1 次 Failure（累计 3 次），达到阈值 → 打开
	_, err = client.Get(context.Background(), ts.URL)
	assert.Error(t, err)
	assert.Equal(t, StateOpen, breaker.State(), "累计 3 次 Failure = 阈值 3，breaker 应打开")

	// 第四次请求：breaker 已打开，应立即返回 ErrCircuitOpen（不发 HTTP 请求）
	attempts := 0
	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
	}))
	defer ts2.Close()

	_, err = client.Get(context.Background(), ts2.URL)
	assert.ErrorIs(t, err, ErrCircuitOpen)
	assert.Equal(t, 0, attempts, "breaker 打开后不应发送 HTTP 请求")
}
