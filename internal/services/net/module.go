package net

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Allinost/go-backend-core/internal/config"
	pkgnet "github.com/Allinost/go-backend-core/pkg/net"
)

var (
	proxyCfg struct {
		HTTP   string `json:"http"`
		HTTPS  string `json:"https"`
		SOCKS5 string `json:"socks5"`
	}
	proxyMu sync.RWMutex
)

type Module struct {
	name   string
	cfg    config.NetConfig
	client *pkgnet.HTTPClient
}

func NewModule() *Module {
	return &Module{name: "net"}
}

func (m *Module) Name() string { return m.name }

func (m *Module) Init(cfg *config.Config) error {
	m.cfg = cfg.Net
	m.client = pkgnet.NewHTTPClient(pkgnet.HTTPConfig{
		Timeout:          m.cfg.HTTP.DefaultTimeout,
		RetryMax:         m.cfg.HTTP.MaxRetries,
		RetryWaitMin:     m.cfg.HTTP.RetryWaitMin,
		RetryWaitMax:     m.cfg.HTTP.RetryWaitMax,
		MaxIdleConns:     m.cfg.HTTP.MaxIdleConns,
		IdleConnTimeout:  m.cfg.HTTP.IdleConnTimeout,
		Breaker:          pkgnet.NewCircuitBreaker(pkgnet.DefaultCircuitBreakerConfig()),
	})
	return nil
}

func (m *Module) Close() error { return nil }

func (m *Module) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/http/request", m.handleHTTPRequest)
	rg.POST("/http/check", m.handleHTTPCheck)
	rg.POST("/tcp/check", m.handleTCPCheck)
	rg.POST("/dns/lookup", m.handleDNSLookup)
	rg.GET("/proxy", m.handleGetProxy)
	rg.PUT("/proxy", m.handleUpdateProxy)
}

type httpRequestReq struct {
	URL     string            `json:"url" binding:"required"`
	Method  string            `json:"method"`
	Body    string            `json:"body,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Timeout int               `json:"timeout"`
}

type httpRequestResp struct {
	StatusCode int         `json:"status_code"`
	Body       string      `json:"body"`
	Headers    http.Header `json:"headers"`
}

func (m *Module) handleHTTPRequest(c *gin.Context) {
	var req httpRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Method == "" {
		req.Method = "GET"
	}
	if req.Timeout <= 0 {
		req.Timeout = 30
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(req.Timeout)*time.Second)
	defer cancel()

	resp, err := m.client.Do(ctx, req.Method, req.URL, []byte(req.Body), req.Headers)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, httpRequestResp{
		StatusCode: resp.StatusCode,
		Body:       string(resp.Body),
		Headers:    resp.Headers,
	})
}

type httpCheckReq struct {
	URL     string `json:"url" binding:"required"`
	Timeout int    `json:"timeout"`
}

type httpCheckResp struct {
	StatusCode int    `json:"status_code"`
	Latency    string `json:"latency"`
	Reachable  bool   `json:"reachable"`
}

func (m *Module) handleHTTPCheck(c *gin.Context) {
	var req httpCheckReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Timeout <= 0 {
		req.Timeout = 10
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(req.Timeout)*time.Second)
	defer cancel()

	start := time.Now()
	code, err := pkgnet.CheckHTTP(ctx, req.URL)
	latency := time.Since(start)

	resp := httpCheckResp{
		StatusCode: code,
		Latency:    latency.String(),
		Reachable:  err == nil,
	}
	c.JSON(http.StatusOK, resp)
}

type tcpCheckReq struct {
	Host    string `json:"host" binding:"required"`
	Port    int    `json:"port" binding:"required"`
	Timeout int    `json:"timeout"`
}

type tcpCheckResp struct {
	Reachable bool   `json:"reachable"`
	Latency   string `json:"latency"`
}

func (m *Module) handleTCPCheck(c *gin.Context) {
	var req tcpCheckReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Timeout <= 0 {
		req.Timeout = 5
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(req.Timeout)*time.Second)
	defer cancel()

	start := time.Now()
	addr := fmt.Sprintf("%s:%d", req.Host, req.Port)
	err := pkgnet.CheckTCP(ctx, addr)
	latency := time.Since(start)

	c.JSON(http.StatusOK, tcpCheckResp{
		Reachable: err == nil,
		Latency:   latency.String(),
	})
}

type dnsLookupReq struct {
	Hostname string `json:"hostname" binding:"required"`
}

type dnsLookupResp struct {
	Hostname  string   `json:"hostname"`
	Addresses []string `json:"addresses"`
}

func (m *Module) handleDNSLookup(c *gin.Context) {
	var req dnsLookupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := pkgnet.DefaultResolveHost(c.Request.Context(), req.Hostname)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dnsLookupResp{
		Hostname:  result.Hostname,
		Addresses: result.Addresses,
	})
}

type proxyResp struct {
	HTTP   string `json:"http"`
	HTTPS  string `json:"https"`
	SOCKS5 string `json:"socks5"`
}

func (m *Module) handleGetProxy(c *gin.Context) {
	proxyMu.RLock()
	defer proxyMu.RUnlock()
	c.JSON(http.StatusOK, proxyResp{
		HTTP:   proxyCfg.HTTP,
		HTTPS:  proxyCfg.HTTPS,
		SOCKS5: proxyCfg.SOCKS5,
	})
}

type proxyUpdateReq struct {
	HTTP   string `json:"http,omitempty"`
	HTTPS  string `json:"https,omitempty"`
	SOCKS5 string `json:"socks5,omitempty"`
}

func (m *Module) handleUpdateProxy(c *gin.Context) {
	var req proxyUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	proxyMu.Lock()
	if req.HTTP != "" {
		proxyCfg.HTTP = req.HTTP
	}
	if req.HTTPS != "" {
		proxyCfg.HTTPS = req.HTTPS
	}
	if req.SOCKS5 != "" {
		proxyCfg.SOCKS5 = req.SOCKS5
	}
	proxyMu.Unlock()

	proxyMu.RLock()
	defer proxyMu.RUnlock()
	c.JSON(http.StatusOK, proxyResp{
		HTTP:   proxyCfg.HTTP,
		HTTPS:  proxyCfg.HTTPS,
		SOCKS5: proxyCfg.SOCKS5,
	})
}
