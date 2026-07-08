package monitor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidMetricName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"http_requests_total", true},
		{"http_requests_5xx", true},
		{"_leading_underscore", true},
		{"go_backend_core_s0_request_duration_ms", true},
		{"metric:with:colons", true},
		{"abc123", true},
		{"ABC_DEF", true},
		{"", false},
		{"123_starts_with_digits", false},
		{"has spaces", false},
		{"has-dash", false},
		{"has.period", false},
		{"has/slash", false},
	}
	for _, tt := range tests {
		err := validMetricName(tt.name)
		if tt.valid {
			assert.NoError(t, err, "指标名 %q 应合法", tt.name)
		} else {
			assert.Error(t, err, "指标名 %q 应非法", tt.name)
		}
	}
}

func TestNewCounter_PanicsOnInvalidName(t *testing.T) {
	reg := NewMetricsRegistry("test", "ns")
	assert.NotPanics(t, func() {
		reg.NewCounter("valid_name", "help")
	})
	assert.Panics(t, func() {
		reg.NewCounter("invalid name!", "help")
	})
}

func TestNewGauge_PanicsOnInvalidName(t *testing.T) {
	reg := NewMetricsRegistry("test", "ns")
	assert.NotPanics(t, func() {
		reg.NewGauge("valid_name", "help")
	})
	assert.Panics(t, func() {
		reg.NewGauge("invalid name!", "help")
	})
}

func TestNewHistogram_PanicsOnInvalidName(t *testing.T) {
	reg := NewMetricsRegistry("test", "ns")
	assert.NotPanics(t, func() {
		reg.NewHistogram("valid_name", "help", nil)
	})
	assert.Panics(t, func() {
		reg.NewHistogram("invalid name!", "help", nil)
	})
}
