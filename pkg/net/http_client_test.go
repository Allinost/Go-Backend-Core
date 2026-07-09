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
