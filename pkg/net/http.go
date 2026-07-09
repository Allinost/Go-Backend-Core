package net

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"time"
)

type HTTPConfig struct {
	Timeout           time.Duration
	RetryMax          int
	RetryWaitMin      time.Duration
	RetryWaitMax      time.Duration
	MaxIdleConns      int
	IdleConnTimeout   time.Duration
	DisableKeepAlives bool
	ProxyURL          string
	TLSConfig         *tls.Config
	Breaker           *CircuitBreaker
}

func DefaultHTTPConfig() HTTPConfig {
	return HTTPConfig{
		Timeout:         30 * time.Second,
		RetryMax:        3,
		RetryWaitMin:    500 * time.Millisecond,
		RetryWaitMax:    5 * time.Second,
		MaxIdleConns:    100,
		IdleConnTimeout: 90 * time.Second,
	}
}

func DefaultHTTPConfigWithBreaker() HTTPConfig {
	cfg := DefaultHTTPConfig()
	cfg.Breaker = NewCircuitBreaker(DefaultCircuitBreakerConfig())
	return cfg
}

type HTTPClient struct {
	client *http.Client
	config HTTPConfig
}

func NewHTTPClient(cfg HTTPConfig) *HTTPClient {
	transport := &http.Transport{
		MaxIdleConns:      cfg.MaxIdleConns,
		IdleConnTimeout:   cfg.IdleConnTimeout,
		DisableKeepAlives: cfg.DisableKeepAlives,
		TLSClientConfig:   cfg.TLSConfig,
	}

	if cfg.ProxyURL != "" {
		if proxyURL, err := url.Parse(cfg.ProxyURL); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	wrappedTransport := newMetricsRoundTripper(transport, cfg.Breaker)

	return &HTTPClient{
		client: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: wrappedTransport,
		},
		config: cfg,
	}
}

type HTTPResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

func (c *HTTPClient) Get(ctx context.Context, url string) (*HTTPResponse, error) {
	return c.doRequest(ctx, http.MethodGet, url, nil)
}

func (c *HTTPClient) Post(ctx context.Context, url string, body []byte, contentType string) (*HTTPResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("net: 创建请求失败: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return c.do(req)
}

func (c *HTTPClient) Put(ctx context.Context, url string, body []byte, contentType string) (*HTTPResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("net: 创建请求失败: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return c.do(req)
}

func (c *HTTPClient) Delete(ctx context.Context, url string) (*HTTPResponse, error) {
	return c.doRequest(ctx, http.MethodDelete, url, nil)
}

func (c *HTTPClient) Do(ctx context.Context, method, url string, body []byte, headers map[string]string) (*HTTPResponse, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("net: 创建请求失败: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.do(req)
}

func (c *HTTPClient) doRequest(ctx context.Context, method, urlStr string, body []byte) (*HTTPResponse, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("net: 创建请求失败: %w", err)
	}
	return c.do(req)
}

func (c *HTTPClient) do(req *http.Request) (*HTTPResponse, error) {
	if c.config.Breaker != nil {
		if !c.config.Breaker.Allow() {
			return nil, ErrCircuitOpen
		}
	}

	var lastErr error
	for i := 0; i <= c.config.RetryMax; i++ {
		if i > 0 {
			waitTime := c.calcBackoff(i)
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(waitTime):
			}
		}

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			if c.config.Breaker != nil {
				c.config.Breaker.Failure()
			}
			recordRetry(req.Method, req.URL.Host)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("net: 读取响应失败: %w", err)
			if c.config.Breaker != nil {
				c.config.Breaker.Failure()
			}
			recordRetry(req.Method, req.URL.Host)
			continue
		}

		result := &HTTPResponse{
			StatusCode: resp.StatusCode,
			Body:       body,
			Headers:    resp.Header,
		}

		if resp.StatusCode >= 500 && i < c.config.RetryMax {
			lastErr = fmt.Errorf("net: 服务端错误 %d", resp.StatusCode)
			if c.config.Breaker != nil {
				c.config.Breaker.Failure()
			}
			recordRetry(req.Method, req.URL.Host)
			continue
		}

		if c.config.Breaker != nil {
			c.config.Breaker.Success()
		}

		return result, nil
	}

	return nil, fmt.Errorf("net: 请求失败（已重试 %d 次）: %w", c.config.RetryMax, lastErr)
}

func (c *HTTPClient) calcBackoff(attempt int) time.Duration {
	min := float64(c.config.RetryWaitMin)
	max := float64(c.config.RetryWaitMax)
	mul := 1 << uint(attempt-1)
	wait := min * float64(mul)
	if wait > max {
		wait = max
	}
	jitter := rand.Float64() * (wait * 0.3)
	return time.Duration(wait + jitter)
}

func CheckHTTP(ctx context.Context, url string) (int, error) {
	client := NewHTTPClient(HTTPConfig{
		Timeout:  5 * time.Second,
		RetryMax: 0,
	})
	resp, err := client.Get(ctx, url)
	if err != nil {
		return 0, fmt.Errorf("net: HTTP 检查失败: %w", err)
	}
	return resp.StatusCode, nil
}
