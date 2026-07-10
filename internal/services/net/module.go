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

// Module 网络模块，封装 HTTP/TCP/DNS 检测和代理管理
type Module struct {
	name   string
	cfg    config.NetConfig   // 网络配置
	client *pkgnet.HTTPClient // HTTP 客户端
}

// NewModule 创建网络模块实例
func NewModule() *Module {
	return &Module{name: "net"}
}

// Name 返回模块名称
func (m *Module) Name() string { return m.name }

// Init 初始化网络模块，配置 HTTP 客户端参数
func (m *Module) Init(cfg *config.Config) error {
	m.cfg = cfg.Net
	m.client = pkgnet.NewHTTPClient(pkgnet.HTTPConfig{
		Timeout:         m.cfg.HTTP.DefaultTimeout,
		RetryMax:        m.cfg.HTTP.MaxRetries,
		RetryWaitMin:    m.cfg.HTTP.RetryWaitMin,
		RetryWaitMax:    m.cfg.HTTP.RetryWaitMax,
		MaxIdleConns:    m.cfg.HTTP.MaxIdleConns,
		IdleConnTimeout: m.cfg.HTTP.IdleConnTimeout,
		Breaker:         pkgnet.NewCircuitBreaker(pkgnet.DefaultCircuitBreakerConfig()),
	})
	return nil
}

// Close 关闭网络模块（空操作）
func (m *Module) Close() error { return nil }

// RegisterRoutes 注册网络检测相关的 HTTP 路由
func (m *Module) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/http/request", m.handleHTTPRequest)
	rg.POST("/http/check", m.handleHTTPCheck)
	rg.POST("/tcp/check", m.handleTCPCheck)
	rg.POST("/dns/lookup", m.handleDNSLookup)
	rg.GET("/proxy", m.handleGetProxy)
	rg.PUT("/proxy", m.handleUpdateProxy)
}

// httpRequestReq HTTP 请求检测请求体
type httpRequestReq struct {
	URL     string            `json:"url" binding:"required"` // 目标 URL
	Method  string            `json:"method"`                 // HTTP 方法（默认 GET）
	Body    string            `json:"body,omitempty"`         // 请求体
	Headers map[string]string `json:"headers,omitempty"`      // 请求头
	Timeout int               `json:"timeout"`                // 超时秒数
}

// httpRequestResp HTTP 请求检测响应体
type httpRequestResp struct {
	StatusCode int         `json:"status_code"` // HTTP 状态码
	Body       string      `json:"body"`        // 响应体
	Headers    http.Header `json:"headers"`     // 响应头
}

// handleHTTPRequest 发送 HTTP 请求
// @Summary      发送 HTTP 请求
// @Description  向指定 URL 发送 HTTP 请求并返回响应状态码、头部和正文
// @Tags         net-HTTP
// @Accept       json
// @Produce      json
// @Param        body  body  httpRequestReq  true  "HTTP 请求参数"
// @Success      200   {object}  httpRequestResp
// @Failure      400   {object}  object{error=string}
// @Failure      502   {object}  object{error=string}
// @Router       /net/http/request [post]
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

// httpCheckReq HTTP 可达性检测请求体
type httpCheckReq struct {
	URL     string `json:"url" binding:"required"` // 目标 URL
	Timeout int    `json:"timeout"`                // 超时秒数
}

// httpCheckResp HTTP 可达性检测响应体
type httpCheckResp struct {
	StatusCode int    `json:"status_code"` // HTTP 状态码
	Latency    string `json:"latency"`     // 响应延迟
	Reachable  bool   `json:"reachable"`   // 是否可达
}

// handleHTTPCheck HTTP 可达性检查
// @Summary      HTTP 可达性检查
// @Description  检查指定 URL 的 HTTP 可达性，返回状态码和延迟
// @Tags         net-HTTP
// @Accept       json
// @Produce      json
// @Param        body  body  httpCheckReq  true  "检查参数"
// @Success      200   {object}  httpCheckResp
// @Failure      400   {object}  object{error=string}
// @Router       /net/http/check [post]
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

// tcpCheckReq TCP 可达性检测请求体
type tcpCheckReq struct {
	Host    string `json:"host" binding:"required"` // 目标主机
	Port    int    `json:"port" binding:"required"` // 目标端口
	Timeout int    `json:"timeout"`                 // 超时秒数
}

// tcpCheckResp TCP 可达性检测响应体
type tcpCheckResp struct {
	Reachable bool   `json:"reachable"` // 是否可达
	Latency   string `json:"latency"`   // 连接延迟
}

// handleTCPCheck TCP 可达性检查
// @Summary      TCP 端口可达性检查
// @Description  检查指定主机和端口的 TCP 连接可达性
// @Tags         net-TCP
// @Accept       json
// @Produce      json
// @Param        body  body  tcpCheckReq  true  "检查参数"
// @Success      200   {object}  tcpCheckResp
// @Failure      400   {object}  object{error=string}
// @Router       /net/tcp/check [post]
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

// dnsLookupReq DNS 查询请求体
type dnsLookupReq struct {
	Hostname string `json:"hostname" binding:"required"` // 目标主机名
}

// dnsLookupResp DNS 查询响应体
type dnsLookupResp struct {
	Hostname  string   `json:"hostname"`  // 原始主机名
	Addresses []string `json:"addresses"` // 解析出的 IP 地址列表
}

// handleDNSLookup DNS 解析
// @Summary      DNS 域名解析
// @Description  查询指定主机名的 DNS 解析结果
// @Tags         net-DNS
// @Accept       json
// @Produce      json
// @Param        body  body  dnsLookupReq  true  "DNS 查询参数"
// @Success      200   {object}  dnsLookupResp
// @Failure      400   {object}  object{error=string}
// @Failure      502   {object}  object{error=string}
// @Router       /net/dns/lookup [post]
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

// proxyResp 代理配置响应体
type proxyResp struct {
	HTTP   string `json:"http"`   // HTTP 代理地址
	HTTPS  string `json:"https"`  // HTTPS 代理地址
	SOCKS5 string `json:"socks5"` // SOCKS5 代理地址
}

// handleGetProxy 获取代理配置
// @Summary      获取当前代理配置
// @Description  返回当前 HTTP/HTTPS/SOCKS5 代理配置
// @Tags         net-代理
// @Produce      json
// @Success      200  {object}  proxyResp
// @Router       /net/proxy [get]
func (m *Module) handleGetProxy(c *gin.Context) {
	proxyMu.RLock()
	defer proxyMu.RUnlock()
	c.JSON(http.StatusOK, proxyResp{
		HTTP:   proxyCfg.HTTP,
		HTTPS:  proxyCfg.HTTPS,
		SOCKS5: proxyCfg.SOCKS5,
	})
}

// proxyUpdateReq 代理配置更新请求体
type proxyUpdateReq struct {
	HTTP   string `json:"http,omitempty"`   // HTTP 代理地址
	HTTPS  string `json:"https,omitempty"`  // HTTPS 代理地址
	SOCKS5 string `json:"socks5,omitempty"` // SOCKS5 代理地址
}

// handleUpdateProxy 更新代理配置
// @Summary      更新代理配置
// @Description  更新 HTTP/HTTPS/SOCKS5 代理设置
// @Tags         net-代理
// @Accept       json
// @Produce      json
// @Param        body  body  proxyUpdateReq  true  "代理配置"
// @Success      200   {object}  proxyResp
// @Failure      400   {object}  object{error=string}
// @Router       /net/proxy [put]
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
