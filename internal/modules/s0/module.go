package s0

import (
	"context"
	"runtime"
	"time"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/modules"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/Allinost/go-backend-core/internal/services/eventbus"
	"github.com/Allinost/go-backend-core/internal/services/monitor"
	"github.com/gin-gonic/gin"
)

type Module struct {
	Bus       eventbus.EventBus
	cfg       *config.Config
	checker   *monitor.HealthChecker
	metrics   *monitor.MetricsRegistry
	startTime time.Time
	subs      []eventbus.Subscription
}

func (m *Module) Name() string {
	return "s0"
}

func (m *Module) Init(cfg *config.Config) error {
	m.cfg = cfg
	m.startTime = time.Now()

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

	if m.Bus != nil {
		sub, err := m.Bus.Subscribe("config.changed", m.onConfigChanged)
		if err != nil {
			logger.Warn().Err(err).Msg("s0: 订阅 config.changed 事件失败")
		} else {
			m.subs = append(m.subs, sub)
		}
	}

	return nil
}

func (m *Module) onConfigChanged(ctx context.Context, event eventbus.Event) error {
	logger.Info().Str("source", event.Source).Msg("s0: 配置已变更")

	if m.cfg.Config.Watch {
		m.checker.Stop()
		m.checker.StartPolling(ctx, monitor.PollingConfig{
			Enabled:  true,
			Interval: 30 * time.Second,
		})
	}

	return nil
}

func (m *Module) Close() error {
	if m.checker != nil {
		m.checker.Stop()
	}
	for _, sub := range m.subs {
		if m.Bus != nil {
			_ = m.Bus.Unsubscribe(sub)
		}
	}
	return nil
}

func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/ping", m.Ping)
	r.GET("/health", m.Health)
	r.GET("/echo", m.Echo)
	r.GET("/metrics", m.Metrics)
	r.GET("/info", m.Info)
	r.GET("/modules", m.Modules)
}

// Ping 存活检查
// @Summary      存活检查
// @Description  返回 pong 表示服务运行正常
// @Tags         s0-调试
// @Success      200  {object}  response.Response{data=object{message=string}}
// @Router       /s0/ping [get]
func (m *Module) Ping(c *gin.Context) {
	response.Success(c, gin.H{"message": "pong"})
}

// Health 健康检查
// @Summary      全服务健康状态
// @Description  返回所有依赖服务的健康检查结果
// @Tags         s0-调试
// @Success      200  {object}  response.Response{data=object{status=string,version=string,uptime=string,checks=object}}
// @Router       /s0/health [get]
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
		"status":   overall,
		"version":  m.cfg.Server.Version,
		"uptime":   time.Since(m.startTime).String(),
		"checks":   results,
	})
}

// Metrics Prometheus 指标
// @Summary      Prometheus 指标暴露
// @Description  返回 Prometheus 格式的监控指标
// @Tags         s0-调试
// @Produce      plain
// @Success      200  {string}  string
// @Router       /s0/metrics [get]
func (m *Module) Metrics(c *gin.Context) {
	m.metrics.Handler().ServeHTTP(c.Writer, c.Request)
}

// Echo 请求回显
// @Summary      请求回显（调试用）
// @Description  返回请求的详细信息，包括方法、路径、查询参数、请求头和客户端 IP
// @Tags         s0-调试
// @Success      200  {object}  response.Response{data=object{method=string,path=string,query=string,headers=object,client_ip=string}}
// @Router       /s0/echo [get]
func (m *Module) Echo(c *gin.Context) {
	response.Success(c, gin.H{
		"method":    c.Request.Method,
		"path":      c.Request.URL.Path,
		"query":     c.Request.URL.RawQuery,
		"headers":   c.Request.Header,
		"client_ip": c.ClientIP(),
	})
}

// Info 服务信息
// @Summary      服务信息
// @Description  返回服务名称、版本、运行时长、Go 运行时信息等
// @Tags         s0-调试
// @Success      200  {object}  response.Response{data=object{name=string,version=string,mode=string,uptime=string,start_time=string,go_version=string,go_os=string,go_arch=string,goroutines=int,modules=[]string,config_watch=bool}}
// @Router       /s0/info [get]
func (m *Module) Info(c *gin.Context) {
	response.Success(c, gin.H{
		"name":        m.cfg.Server.Name,
		"version":     m.cfg.Server.Version,
		"mode":        m.cfg.Server.Mode,
		"uptime":      time.Since(m.startTime).String(),
		"start_time":  m.startTime.Format(time.RFC3339),
		"go_version":  runtime.Version(),
		"go_os":       runtime.GOOS,
		"go_arch":     runtime.GOARCH,
		"goroutines":  runtime.NumGoroutine(),
		"modules":     modules.List(),
		"config_watch": m.cfg.Config.Watch,
	})
}

// Modules 模块列表
// @Summary      已注册模块列表
// @Description  返回所有已注册的业务模块名称
// @Tags         s0-调试
// @Success      200  {object}  response.Response{data=object{count=int,modules=[]object}}
// @Router       /s0/modules [get]
func (m *Module) Modules(c *gin.Context) {
	all := modules.List()
	details := make([]gin.H, 0, len(all))
	for _, name := range all {
		mod := modules.Get(name)
		if mod != nil {
			details = append(details, gin.H{
				"name": name,
			})
		}
	}

	response.Success(c, gin.H{
		"count":   len(details),
		"modules": details,
	})
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

var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)
