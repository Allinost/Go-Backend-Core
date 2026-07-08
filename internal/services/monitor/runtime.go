package monitor

import (
	"runtime"

	"github.com/prometheus/client_golang/prometheus"
)

func NewRuntimeCollector(reg *MetricsRegistry) {
	reg.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: reg.namespace,
		Subsystem: reg.subsystem,
		Name:      "goroutines",
		Help:      "当前 goroutine 数量",
	}, func() float64 { return float64(runtime.NumGoroutine()) })

	reg.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: reg.namespace,
		Subsystem: reg.subsystem,
		Name:      "mem_alloc_bytes",
		Help:      "堆分配字节数",
	}, func() float64 {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return float64(m.Alloc)
	})

	reg.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: reg.namespace,
		Subsystem: reg.subsystem,
		Name:      "mem_sys_bytes",
		Help:      "从系统获取的内存字节数",
	}, func() float64 {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return float64(m.Sys)
	})

	reg.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: reg.namespace,
		Subsystem: reg.subsystem,
		Name:      "gc_count",
		Help:      "GC 总次数",
	}, func() float64 {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return float64(m.NumGC)
	})
}
