package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRateLimit_Allowed(t *testing.T) {
	rl := NewRateLimit(5, time.Minute)
	router := gin.New()
	router.Use(RateLimiter(rl))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestRateLimit_Exceeded(t *testing.T) {
	rl := NewRateLimit(2, time.Minute)
	router := gin.New()
	router.Use(RateLimiter(rl))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, 429, w.Code)
}

func TestRateLimit_DifferentIP(t *testing.T) {
	rl := NewRateLimit(1, time.Minute)
	router := gin.New()
	router.Use(RateLimiter(rl))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "10.0.0.1:12345"
	router.ServeHTTP(w1, req1)
	assert.Equal(t, 200, w1.Code)

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "10.0.0.2:12345"
	router.ServeHTTP(w2, req2)
	assert.Equal(t, 200, w2.Code)
}

func TestRateLimit_WindowReset(t *testing.T) {
	rl := NewRateLimit(1, 50*time.Millisecond)
	router := gin.New()
	router.Use(RateLimiter(rl))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, 429, w.Code)

	time.Sleep(60 * time.Millisecond)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

func TestRedisRateLimit_Allow(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Skip("miniredis not available:", err)
	}
	defer s.Close()

	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer client.Close()

	rl := NewRedisRateLimit(client, 3, time.Minute)
	assert.True(t, rl.Allow("10.0.0.1"))
	assert.True(t, rl.Allow("10.0.0.1"))
	assert.True(t, rl.Allow("10.0.0.1"))
	assert.False(t, rl.Allow("10.0.0.1"))
	assert.False(t, rl.Allow("10.0.0.1"))
}

func TestRedisRateLimit_DifferentIP(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Skip("miniredis not available:", err)
	}
	defer s.Close()

	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer client.Close()

	rl := NewRedisRateLimit(client, 1, time.Minute)
	assert.True(t, rl.Allow("10.0.0.1"))
	assert.False(t, rl.Allow("10.0.0.1"))
	assert.True(t, rl.Allow("10.0.0.2"))
	assert.True(t, rl.Allow("10.0.0.3"))
}

func TestRedisRateLimit_SequentialWindows(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Skip("miniredis not available:", err)
	}
	defer s.Close()

	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer client.Close()

	rl := NewRedisRateLimit(client, 2, time.Minute)
	assert.True(t, rl.Allow("10.0.0.1"))
	assert.True(t, rl.Allow("10.0.0.1"))
	assert.False(t, rl.Allow("10.0.0.1"))
}
