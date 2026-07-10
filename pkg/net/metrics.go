package net

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus 指标定义
var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "net_http_requests_total",
			Help: "HTTP 客户端请求总数",
		},
		[]string{"method", "host", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "net_http_request_duration_seconds",
			Help:    "HTTP 请求耗时分布",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "host"},
	)

	httpRetriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "net_http_retries_total",
			Help: "HTTP 重试次数",
		},
		[]string{"method", "host"},
	)

	httpActiveRequests = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "net_http_active_requests",
			Help: "当前正在处理的 HTTP 请求数",
		},
	)

	httpConnectionsActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "net_http_connections_active",
			Help: "当前活跃的 HTTP 连接数",
		},
		[]string{"host"},
	)

	httpCircuitBreakerState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "net_http_circuit_breaker_state",
			Help: "断路器状态 (0=closed, 1=open, 2=half-open)",
		},
		[]string{"name"},
	)
)

// init 注册所有 Prometheus 指标
func init() {
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(httpRetriesTotal)
	prometheus.MustRegister(httpActiveRequests)
	prometheus.MustRegister(httpConnectionsActive)
	prometheus.MustRegister(httpCircuitBreakerState)
}

// metricsRoundTripper 包装 http.RoundTripper，自动收集指标
type metricsRoundTripper struct {
	next    http.RoundTripper
	metrics *clientMetrics
}

// clientMetrics 客户端内部指标状态
type clientMetrics struct {
	breaker *CircuitBreaker // 关联的断路器
}

// newMetricsRoundTripper 创建带指标收集的 RoundTripper 包装
func newMetricsRoundTripper(next http.RoundTripper, breaker *CircuitBreaker) http.RoundTripper {
	return &metricsRoundTripper{
		next: next,
		metrics: &clientMetrics{
			breaker: breaker,
		},
	}
}

// RoundTrip 执行 HTTP 请求并记录请求数、持续时间、活跃连接数和断路器状态
func (rt *metricsRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	httpActiveRequests.Inc()
	defer httpActiveRequests.Dec()

	host := req.URL.Host
	httpConnectionsActive.WithLabelValues(host).Inc()
	defer httpConnectionsActive.WithLabelValues(host).Dec()

	if rt.metrics.breaker != nil {
		httpCircuitBreakerState.WithLabelValues("http_client").Set(float64(rt.metrics.breaker.State()))
		defer httpCircuitBreakerState.WithLabelValues("http_client").Set(float64(rt.metrics.breaker.State()))
	}

	start := time.Now()
	resp, err := rt.next.RoundTrip(req)
	duration := time.Since(start)

	status := "error"
	if err == nil {
		status = strconv.Itoa(resp.StatusCode)
	}

	httpRequestsTotal.WithLabelValues(req.Method, host, status).Inc()
	httpRequestDuration.WithLabelValues(req.Method, host).Observe(duration.Seconds())

	return resp, err
}

// recordRetry 记录重试次数指标
func recordRetry(method, host string) {
	httpRetriesTotal.WithLabelValues(method, host).Inc()
}

// MetricsHandler 返回 Prometheus HTTP Handler，用于暴露指标
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
