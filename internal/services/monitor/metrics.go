package monitor

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var metricNameRe = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)

func validMetricName(name string) error {
	if !metricNameRe.MatchString(name) {
		return fmt.Errorf("指标名 %q 不符合规范（仅允许 [a-zA-Z_:][a-zA-Z0-9_:]*）", name)
	}
	return nil
}

type MetricsRegistry struct {
	namespace string
	subsystem string
	reg       *prometheus.Registry
}

func NewMetricsRegistry(namespace, subsystem string) *MetricsRegistry {
	return &MetricsRegistry{
		namespace: namespace,
		subsystem: subsystem,
		reg:       prometheus.NewRegistry(),
	}
}

func (m *MetricsRegistry) Register(collector prometheus.Collector) error {
	return m.reg.Register(collector)
}

func (m *MetricsRegistry) RegisterOrReuse(collector prometheus.Collector) {
	err := m.Register(collector)
	if err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			_ = are
		}
	}
}

func (m *MetricsRegistry) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}

func (m *MetricsRegistry) NewCounter(name, help string, labels ...string) *prometheus.CounterVec {
	if err := validMetricName(name); err != nil {
		panic(err)
	}
	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.namespace,
			Subsystem: m.subsystem,
			Name:      name,
			Help:      help,
		},
		labels,
	)
	m.RegisterOrReuse(counter)
	return counter
}

func (m *MetricsRegistry) NewGauge(name, help string, labels ...string) *prometheus.GaugeVec {
	if err := validMetricName(name); err != nil {
		panic(err)
	}
	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: m.namespace,
			Subsystem: m.subsystem,
			Name:      name,
			Help:      help,
		},
		labels,
	)
	m.RegisterOrReuse(gauge)
	return gauge
}

func (m *MetricsRegistry) NewHistogram(name, help string, buckets []float64, labels ...string) *prometheus.HistogramVec {
	if err := validMetricName(name); err != nil {
		panic(err)
	}
	if len(buckets) == 0 {
		buckets = prometheus.DefBuckets
	}
	histogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.namespace,
			Subsystem: m.subsystem,
			Name:      name,
			Help:      help,
			Buckets:   buckets,
		},
		labels,
	)
	m.RegisterOrReuse(histogram)
	return histogram
}

func (m *MetricsRegistry) NewGaugeFunc(opts prometheus.GaugeOpts, fn func() float64) {
	if err := validMetricName(opts.Name); err != nil {
		panic(err)
	}
	m.RegisterOrReuse(prometheus.NewGaugeFunc(opts, fn))
}
