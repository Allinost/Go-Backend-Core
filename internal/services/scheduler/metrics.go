package scheduler

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics interface {
	TaskStarted(handler string)
	TaskCompleted(handler string)
	TaskFailed(handler string)
}

type nopMetrics struct{}

func (nopMetrics) TaskStarted(string)   {}
func (nopMetrics) TaskCompleted(string) {}
func (nopMetrics) TaskFailed(string)    {}

type promMetrics struct {
	started   *prometheus.CounterVec
	completed *prometheus.CounterVec
	failed    *prometheus.CounterVec
}

func newPromMetrics(reg interface {
	NewCounter(name, help string, labels ...string) *prometheus.CounterVec
}) *promMetrics {
	return &promMetrics{
		started:   reg.NewCounter("task_started_total", "任务启动次数", "handler"),
		completed: reg.NewCounter("task_completed_total", "任务成功次数", "handler"),
		failed:    reg.NewCounter("task_failed_total", "任务失败次数", "handler"),
	}
}

func (p *promMetrics) TaskStarted(handler string) {
	if p.started != nil {
		p.started.WithLabelValues(handler).Inc()
	}
}

func (p *promMetrics) TaskCompleted(handler string) {
	if p.completed != nil {
		p.completed.WithLabelValues(handler).Inc()
	}
}

func (p *promMetrics) TaskFailed(handler string) {
	if p.failed != nil {
		p.failed.WithLabelValues(handler).Inc()
	}
}
