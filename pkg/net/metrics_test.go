package net

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetricsHandler(t *testing.T) {
	handler := MetricsHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestMetricsRoundTripper_Compiles(t *testing.T) {
	cfg := DefaultHTTPConfigWithBreaker()
	client := NewHTTPClient(cfg)
	assert.NotNil(t, client)
}
