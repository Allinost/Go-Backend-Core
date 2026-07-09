package net

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Allinost/go-backend-core/internal/config"
)

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	m := NewModule()
	m.Init(&config.Config{
		Net: config.NetConfig{
			HTTP: config.NetHTTPConfig{
				DefaultTimeout: 30 * time.Second,
				MaxRetries:     3,
			},
		},
	})
	rg := r.Group("/api/v1/net")
	m.RegisterRoutes(rg)
	return r
}

func TestHTTPCheck(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	r := setupRouter()
	w := httptest.NewRecorder()
	body := strings.NewReader(`{"url":"` + ts.URL + `"}`)
	req, _ := http.NewRequest("POST", "/api/v1/net/http/check", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp httpCheckResp
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Reachable)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestHTTPCheck_InvalidURL(t *testing.T) {
	r := setupRouter()
	w := httptest.NewRecorder()
	body := strings.NewReader(`{"url":"http://127.0.0.1:1"}`)
	req, _ := http.NewRequest("POST", "/api/v1/net/http/check", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp httpCheckResp
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Reachable)
}

func TestHTTPCheck_MissingURL(t *testing.T) {
	r := setupRouter()
	w := httptest.NewRecorder()
	body := strings.NewReader(`{}`)
	req, _ := http.NewRequest("POST", "/api/v1/net/http/check", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTCPCheck(t *testing.T) {
	r := setupRouter()
	w := httptest.NewRecorder()
	body := strings.NewReader(`{"host":"127.0.0.1","port":1}`)
	req, _ := http.NewRequest("POST", "/api/v1/net/tcp/check", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp tcpCheckResp
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Reachable)
}

func TestTCPCheck_MissingFields(t *testing.T) {
	r := setupRouter()
	w := httptest.NewRecorder()
	body := strings.NewReader(`{"host":"127.0.0.1"}`)
	req, _ := http.NewRequest("POST", "/api/v1/net/tcp/check", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDNSLookup(t *testing.T) {
	r := setupRouter()
	w := httptest.NewRecorder()
	body := strings.NewReader(`{"hostname":"localhost"}`)
	req, _ := http.NewRequest("POST", "/api/v1/net/dns/lookup", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp dnsLookupResp
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "localhost", resp.Hostname)
	assert.NotEmpty(t, resp.Addresses)
}

func TestDNSLookup_Invalid(t *testing.T) {
	r := setupRouter()
	w := httptest.NewRecorder()
	body := strings.NewReader(`{"hostname":"invalid-host-xxxxx.local"}`)
	req, _ := http.NewRequest("POST", "/api/v1/net/dns/lookup", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestDNSLookup_MissingHostname(t *testing.T) {
	r := setupRouter()
	w := httptest.NewRecorder()
	body := strings.NewReader(`{}`)
	req, _ := http.NewRequest("POST", "/api/v1/net/dns/lookup", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetProxy_Default(t *testing.T) {
	r := setupRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/net/proxy", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp proxyResp
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Empty(t, resp.HTTP)
	assert.Empty(t, resp.HTTPS)
	assert.Empty(t, resp.SOCKS5)
}

func TestUpdateProxy(t *testing.T) {
	r := setupRouter()

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"http":"http://proxy:8080","https":"http://proxy:8080"}`)
	req, _ := http.NewRequest("PUT", "/api/v1/net/proxy", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp proxyResp
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "http://proxy:8080", resp.HTTP)
	assert.Equal(t, "http://proxy:8080", resp.HTTPS)
	assert.Empty(t, resp.SOCKS5)
}

func TestGetProxy_AfterUpdate(t *testing.T) {
	r := setupRouter()

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"socks5":"socks5://proxy:1080"}`)
	req, _ := http.NewRequest("PUT", "/api/v1/net/proxy", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/net/proxy", nil)
	r.ServeHTTP(w, req)

	var resp proxyResp
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "socks5://proxy:1080", resp.SOCKS5)
}

func TestHTTPRequest_Basic(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ts.Close()

	r := setupRouter()
	w := httptest.NewRecorder()
	body := strings.NewReader(`{"url":"` + ts.URL + `","method":"GET"}`)
	req, _ := http.NewRequest("POST", "/api/v1/net/http/request", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp httpRequestResp
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, `{"status":"ok"}`, resp.Body)
}
