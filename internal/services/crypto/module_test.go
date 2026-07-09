package crypto

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Allinost/go-backend-core/internal/config"
)

func setupCryptoModule(keyHex string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	m := NewModule()
	m.Init(&config.Config{
		Crypto: config.CryptoConfig{
			Enabled:      true,
			MasterKeyHex: keyHex,
		},
	})
	rg := r.Group("/api/v1/crypto")
	m.RegisterRoutes(rg)
	return r
}

func TestCryptoEncryptDecrypt(t *testing.T) {
	r := setupCryptoModule("0123456789abcdef0123456789abcdef")

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"data":"hello-world"}`)
	req, _ := http.NewRequest("POST", "/api/v1/crypto/encrypt", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var encResp cryptoResp
	err := json.Unmarshal(w.Body.Bytes(), &encResp)
	require.NoError(t, err)
	assert.Contains(t, encResp.Result, "v1:")
	assert.Equal(t, "aes-gcm", encResp.Algo)

	w = httptest.NewRecorder()
	body = strings.NewReader(`{"data":"` + encResp.Result + `"}`)
	req, _ = http.NewRequest("POST", "/api/v1/crypto/decrypt", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var decResp cryptoResp
	err = json.Unmarshal(w.Body.Bytes(), &decResp)
	require.NoError(t, err)
	assert.Equal(t, "hello-world", decResp.Result)
}

func TestCryptoEncryptWithKeyHex(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	m := NewModule()
	m.Init(&config.Config{Crypto: config.CryptoConfig{Enabled: false}})
	rg := r.Group("/api/v1/crypto")
	m.RegisterRoutes(rg)

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"data":"test","key_hex":"0123456789abcdef0123456789abcdef"}`)
	req, _ := http.NewRequest("POST", "/api/v1/crypto/encrypt", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var encResp cryptoResp
	err := json.Unmarshal(w.Body.Bytes(), &encResp)
	require.NoError(t, err)
	assert.NotEmpty(t, encResp.Result)

	w = httptest.NewRecorder()
	body = strings.NewReader(`{"data":"` + encResp.Result + `","key_hex":"0123456789abcdef0123456789abcdef"}`)
	req, _ = http.NewRequest("POST", "/api/v1/crypto/decrypt", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	var decResp cryptoResp
	err = json.Unmarshal(w.Body.Bytes(), &decResp)
	require.NoError(t, err)
	assert.Equal(t, "test", decResp.Result)
}

func TestCryptoEncryptNoKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	m := NewModule()
	m.Init(&config.Config{Crypto: config.CryptoConfig{Enabled: false}})
	rg := r.Group("/api/v1/crypto")
	m.RegisterRoutes(rg)

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"data":"test"}`)
	req, _ := http.NewRequest("POST", "/api/v1/crypto/encrypt", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCryptoCompressDecompress(t *testing.T) {
	r := setupCryptoModule("0123456789abcdef0123456789abcdef")

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"data":"compressible data compressible data"}`)
	req, _ := http.NewRequest("POST", "/api/v1/crypto/compress", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var compResp compressResp
	err := json.Unmarshal(w.Body.Bytes(), &compResp)
	require.NoError(t, err)
	assert.NotEmpty(t, compResp.Result)
	assert.Equal(t, "gzip", compResp.Method)

	w = httptest.NewRecorder()
	body = strings.NewReader(`{"data":"` + compResp.Result + `"}`)
	req, _ = http.NewRequest("POST", "/api/v1/crypto/decompress", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var decompResp compressResp
	err = json.Unmarshal(w.Body.Bytes(), &decompResp)
	require.NoError(t, err)
	assert.Equal(t, "compressible data compressible data", decompResp.Result)
}

func TestCryptoHash(t *testing.T) {
	r := setupCryptoModule("")

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"data":"hello"}`)
	req, _ := http.NewRequest("POST", "/api/v1/crypto/hash", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp cryptoResp
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 64, len(resp.Result))
	assert.Equal(t, "sha256", resp.Algo)
}

func TestCryptoHashBLAKE3(t *testing.T) {
	r := setupCryptoModule("")

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"data":"hello","algo":"blake3"}`)
	req, _ := http.NewRequest("POST", "/api/v1/crypto/hash", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp cryptoResp
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 64, len(resp.Result))
	assert.Equal(t, "blake3", resp.Algo)
}

func TestCryptoHashInvalidAlgo(t *testing.T) {
	r := setupCryptoModule("")

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"data":"hello","algo":"md5"}`)
	req, _ := http.NewRequest("POST", "/api/v1/crypto/hash", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCryptoChaCha20(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	m := NewModule()
	m.Init(&config.Config{Crypto: config.CryptoConfig{Enabled: false}})
	rg := r.Group("/api/v1/crypto")
	m.RegisterRoutes(rg)

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"data":"secret","algo":"chacha20-poly1305","key_hex":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}`)
	req, _ := http.NewRequest("POST", "/api/v1/crypto/encrypt", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var encResp cryptoResp
	err := json.Unmarshal(w.Body.Bytes(), &encResp)
	require.NoError(t, err)
	assert.NotEmpty(t, encResp.Result)

	w = httptest.NewRecorder()
	body = strings.NewReader(`{"data":"` + encResp.Result + `","algo":"chacha20-poly1305","key_hex":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}`)
	req, _ = http.NewRequest("POST", "/api/v1/crypto/decrypt", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	var decResp cryptoResp
	err = json.Unmarshal(w.Body.Bytes(), &decResp)
	require.NoError(t, err)
	assert.Equal(t, "secret", decResp.Result)
}
