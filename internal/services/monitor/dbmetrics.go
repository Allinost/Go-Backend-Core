package monitor

import "github.com/prometheus/client_golang/prometheus"

func RegisterDBPoolMetrics(reg *MetricsRegistry, statsFunc func() map[string]DBPoolStats) {
	reg.RegisterOrReuse(&dbPoolCollector{
		desc: prometheus.NewDesc(
			prometheus.BuildFQName(reg.namespace, reg.subsystem, "db_connections"),
			"数据库连接数",
			[]string{"instance", "state"},
			nil,
		),
		statsFunc: statsFunc,
	})
}

type dbPoolCollector struct {
	desc      *prometheus.Desc
	statsFunc func() map[string]DBPoolStats
}

func (c *dbPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *dbPoolCollector) Collect(ch chan<- prometheus.Metric) {
	for name, stats := range c.statsFunc() {
		ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, float64(stats.Open), name, "open")
		ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, float64(stats.InUse), name, "in_use")
		ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, float64(stats.Idle), name, "idle")
	}
}

type DBPoolStats struct {
	Open  int
	InUse int
	Idle  int
}
