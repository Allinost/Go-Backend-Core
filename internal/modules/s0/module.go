package s0

import (
	"context"
	"time"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/Allinost/go-backend-core/internal/services/eventbus"
	"github.com/Allinost/go-backend-core/internal/services/monitor"
	"github.com/gin-gonic/gin"
)

type Module struct {
	Bus     eventbus.EventBus
	cfg     *config.Config
	checker *monitor.HealthChecker
	metrics *monitor.MetricsRegistry
}

func (m *Module) Name() string {
	return "s0"
}

func (m *Module) Init(cfg *config.Config) error {
	m.cfg = cfg

	m.checker = monitor.NewHealthChecker()
	m.metrics = monitor.NewMetricsRegistry("go_backend_core", "s0")

	dbHealth := database.Health()
	for name := range dbHealth {
		name := name
		m.checker.Register("database:"+name, func(ctx context.Context) monitor.CheckResult {
			health := database.Health()
			h, ok := health[name]
			if !ok {
				return monitor.CheckResult{Status: monitor.StatusDown, Error: "no health data"}
			}
			if h.Status == "error" {
				return monitor.CheckResult{Status: monitor.StatusDown, Error: h.Error}
			}
			return monitor.CheckResult{Status: monitor.StatusUp}
		})
	}

	if cfg.Config.Watch {
		m.checker.StartPolling(context.Background(), monitor.PollingConfig{
			Enabled:  true,
			Interval: 30 * time.Second,
		})
	}

	monitor.NewRuntimeCollector(m.metrics)

	monitor.RegisterDBPoolMetrics(m.metrics, func() map[string]monitor.DBPoolStats {
		return collectDBPoolStats()
	})

	return nil
}

func collectDBPoolStats() map[string]monitor.DBPoolStats {
	stats := make(map[string]monitor.DBPoolStats)
	if database.DB == nil {
		return stats
	}
	for name, pool := range database.DB.MySQL {
		if pool.DB != nil {
			s := pool.DB.Stats()
			stats["mysql:"+name] = monitor.DBPoolStats{
				Open:  s.OpenConnections,
				InUse: s.InUse,
				Idle:  s.Idle,
			}
		}
	}
	for name, pool := range database.DB.Postgres {
		if pool.Pool != nil {
			s := pool.Stat()
			stats["postgres:"+name] = monitor.DBPoolStats{
				Open:  int(s.TotalConns()),
				InUse: int(s.AcquiredConns()),
				Idle:  int(s.IdleConns()),
			}
		}
	}
	for name, client := range database.DB.Redis {
		if client.Client != nil {
			s := client.PoolStats()
			stats["redis:"+name] = monitor.DBPoolStats{
				Open:  int(s.TotalConns),
				InUse: int(s.TotalConns - s.IdleConns),
				Idle:  int(s.IdleConns),
			}
		}
	}
	return stats
}

func (m *Module) Close() error {
	if m.checker != nil {
		m.checker.Stop()
	}
	return nil
}

func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/ping", m.Ping)
	r.GET("/health", m.Health)
	r.GET("/echo", m.Echo)
	r.GET("/metrics", m.Metrics)
}

func (m *Module) Ping(c *gin.Context) {
	response.Success(c, gin.H{"message": "pong"})
}

func (m *Module) Health(c *gin.Context) {
	results := m.checker.Check(c.Request.Context())

	overall := "ok"
	for _, r := range results {
		if r.Status == monitor.StatusDown {
			overall = "degraded"
			break
		}
	}

	response.Success(c, gin.H{
		"status":  overall,
		"version": m.cfg.Server.Version,
		"checks":  results,
	})
}

func (m *Module) Metrics(c *gin.Context) {
	m.metrics.Handler().ServeHTTP(c.Writer, c.Request)
}

func (m *Module) Echo(c *gin.Context) {
	response.Success(c, gin.H{
		"method":    c.Request.Method,
		"path":      c.Request.URL.Path,
		"query":     c.Request.URL.RawQuery,
		"headers":   c.Request.Header,
		"client_ip": c.ClientIP(),
	})
}

var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)
