package monitor

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRuntimeCollector_Collect(t *testing.T) {
	reg := NewMetricsRegistry("test", "runtime")
	NewRuntimeCollector(reg)

	handler := reg.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body, _ := io.ReadAll(w.Body)
	output := string(body)
	assert.Contains(t, output, "test_runtime_goroutines")
	assert.Contains(t, output, "test_runtime_mem_alloc_bytes")
	assert.Contains(t, output, "test_runtime_mem_sys_bytes")
	assert.Contains(t, output, "test_runtime_gc_count")
}
