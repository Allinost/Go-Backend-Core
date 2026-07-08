package monitor

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegisterDBPoolMetrics(t *testing.T) {
	reg := NewMetricsRegistry("test", "db")
	RegisterDBPoolMetrics(reg, func() map[string]DBPoolStats {
		return map[string]DBPoolStats{
			"mysql:main": {Open: 5, InUse: 3, Idle: 2},
			"redis:main": {Open: 2, InUse: 1, Idle: 1},
		}
	})

	handler := reg.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body, _ := io.ReadAll(w.Body)
	output := string(body)
	assert.Contains(t, output, `test_db_db_connections{instance="mysql:main",state="open"} 5`)
	assert.Contains(t, output, `test_db_db_connections{instance="mysql:main",state="in_use"} 3`)
	assert.Contains(t, output, `test_db_db_connections{instance="redis:main",state="idle"} 1`)
}
