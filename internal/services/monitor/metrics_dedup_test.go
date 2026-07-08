package monitor

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestRegisterOrReuse_Duplicate(t *testing.T) {
	reg := NewMetricsRegistry("test", "dup")
	c1 := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "test", Subsystem: "dup", Name: "dup_total", Help: "test",
	}, []string{})
	c2 := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "test", Subsystem: "dup", Name: "dup_total", Help: "test",
	}, []string{})

	reg.RegisterOrReuse(c1)
	assert.NotPanics(t, func() { reg.RegisterOrReuse(c2) })
}

func TestNewCounter_DuplicateSafe(t *testing.T) {
	reg := NewMetricsRegistry("test", "safe")
	c1 := reg.NewCounter("safe_total", "test")
	c2 := reg.NewCounter("safe_total", "test")
	assert.NotNil(t, c1)
	assert.NotNil(t, c2)
}

func TestNewGauge_DuplicateSafe(t *testing.T) {
	reg := NewMetricsRegistry("test", "safe")
	g1 := reg.NewGauge("safe_gauge", "test")
	g2 := reg.NewGauge("safe_gauge", "test")
	assert.NotNil(t, g1)
	assert.NotNil(t, g2)
}

func TestNewHistogram_DuplicateSafe(t *testing.T) {
	reg := NewMetricsRegistry("test", "safe")
	h1 := reg.NewHistogram("safe_hist", "test", nil)
	h2 := reg.NewHistogram("safe_hist", "test", nil)
	assert.NotNil(t, h1)
	assert.NotNil(t, h2)
}

func TestNewGaugeFunc_DuplicateSafe(t *testing.T) {
	reg := NewMetricsRegistry("test", "safe")
	reg.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "test", Subsystem: "safe", Name: "safe_gf", Help: "test",
	}, func() float64 { return 1 })
	assert.NotPanics(t, func() {
		reg.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "test", Subsystem: "safe", Name: "safe_gf", Help: "test",
		}, func() float64 { return 1 })
	})
}
