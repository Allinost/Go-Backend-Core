package main

import (
	"context"
	"fmt"
	"github.com/Allinost/go-backend-core/zzz/goodser"
	"net/http"
	"os"
	"time"

	swaggerDocs "github.com/Allinost/go-backend-core/api/swagger"
	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/middleware"
	"github.com/Allinost/go-backend-core/internal/modules"
	"github.com/Allinost/go-backend-core/internal/modules/auth"
	"github.com/Allinost/go-backend-core/internal/modules/migrate"
	"github.com/Allinost/go-backend-core/internal/modules/s0"
	"github.com/Allinost/go-backend-core/internal/modules/s1"
	"github.com/Allinost/go-backend-core/internal/modules/s2"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/services/crypto"
	"github.com/Allinost/go-backend-core/internal/services/eventbus"
	netmodule "github.com/Allinost/go-backend-core/internal/services/net"
	"github.com/Allinost/go-backend-core/internal/services/scheduler"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化结构化日志
	logger.SetGlobal(logger.New(logger.Config{
		Level:      cfg.Log.Level,
		Format:     cfg.Log.Format,
		Output:     cfg.Log.Output,
		LogDir:     cfg.Log.LogDir,
		MaxSizeMB:  cfg.Log.MaxSizeMB,
		MaxAgeDays: cfg.Log.MaxAgeDays,
	}))

	// 初始化审计日志独立文件输出
	if cfg.Log.LogDir != "" {
		auditPath := cfg.Log.LogDir + "/audit.log"
		if err := logger.InitAuditLog(auditPath); err != nil {
			logger.Warn().Err(err).Str("path", auditPath).Msg("审计日志初始化失败")
		}
	}

	// 初始化事件总线
	bus := eventbus.NewLocal()

	// 配置变更时通过 EventBus 发布事件
	config.OnChange(func(cfg *config.Config) {
		if err := bus.Publish(context.Background(), "config.changed", eventbus.Event{
			Source: "config",
			Payload: map[string]any{
				"server.port": cfg.Server.Port,
				"log.level":   cfg.Log.Level,
			},
		}); err != nil {
			logger.Warn().Err(err).Msg("发布配置变更事件失败")
		}
	})

	// 初始化数据库连接（在模块初始化之前）
	if err := database.InitAll(cfg); err != nil {
		logger.Error().Err(err).Msg("初始化数据库连接失败")
		os.Exit(1)
	}

	// 注册数据库热加载（InitAll 之后 DB 不为 nil）
	config.RegisterReloader(database.DB)

	// 注册业务模块
	modules.Register(&s0.Module{Bus: bus})
	modules.Register(&s1.Module{})
	modules.Register(&s2.Module{})
	modules.Register(&scheduler.Module{})
	modules.Register(&auth.Module{})
	modules.Register(netmodule.NewModule())
	modules.Register(crypto.NewModule())
	modules.Register(&migrate.Module{})
	modules.Register(&goodser.Module{})

	// 初始化所有模块（调用 Init，不挂载路由）
	modules.InitAll(cfg)

	// ========== 主引擎 :29090 — 业务 API ==========
	gin.SetMode(cfg.Server.Mode)
	mainR := gin.New()
	mainR.Use(middleware.Logger())
	mainR.Use(middleware.CORS())
	mainR.Use(gin.Recovery())
	mainR.Use(middleware.RateLimiter(middleware.NewRateLimit(100, time.Minute)))
	modules.RegisterAllRoutes(mainR)

	// ========== 调试引擎 :29092 — Swagger UI + s0 调试/监控/运维 ==========
	debugR := gin.New()
	debugR.Use(middleware.Logger())
	debugR.Use(middleware.CORS())
	debugR.Use(gin.Recovery())

	// Swagger UI — 支持 ?backend=IP:PORT 查询参数自定义后端地址（cookie 持久化）
	defaultBackend := "192.168.1.36:29090"
	if cfg.Server.Swagger.BackendAddr != "" {
		defaultBackend = cfg.Server.Swagger.BackendAddr
	}
	swaggerDocs.SwaggerInfo.Host = defaultBackend
	swaggerUI := ginSwagger.WrapHandler(swaggerFiles.Handler)

	debugR.GET("/swagger/*any", func(c *gin.Context) {
		backend := ""

		// 优先使用查询参数
		if q := c.Query("backend"); q != "" {
			backend = q
			c.SetCookie("swagger_backend", backend, 86400*365, "/swagger", "", false, true)
		}

		// 其次使用 cookie
		if backend == "" {
			if cookie, err := c.Cookie("swagger_backend"); err == nil && cookie != "" {
				backend = cookie
			}
		}

		// 最后使用配置文件
		if backend == "" {
			backend = defaultBackend
		}

		swaggerDocs.SwaggerInfo.Host = backend
		swaggerUI(c)
	})
	// 仅注册 s0 调试模块（ping / health / echo / metrics / info / modules）
	modules.RegisterRoutesTo(debugR, "s0")

	// ========== 启动服务 ==========
	mainAddr := fmt.Sprintf(":%d", cfg.Server.Port)
	debugAddr := fmt.Sprintf(":%d", cfg.Server.DebugPort)

	// 调试引擎在 goroutine 中启动
	debugSrv := &http.Server{Addr: debugAddr, Handler: debugR}
	go func() {
		fmt.Printf("\033[36m✓ 调试服务已启动，监听地址 %s (Swagger: http://localhost%s/swagger/index.html)\033[0m\n", debugAddr, debugAddr)
		logger.Info().Str("addr", debugAddr).Msg("调试服务启动")
		if err := debugSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error().Err(err).Msg("调试服务异常退出")
		}
	}()

	fmt.Printf("\033[32m✓ 服务已启动，监听地址 %s\033[0m\n", mainAddr)
	logger.Info().Str("addr", mainAddr).Str("name", cfg.Server.Name).Str("version", cfg.Server.Version).Msg("服务启动")
	if err := mainR.Run(mainAddr); err != nil {
		logger.Error().Err(err).Msg("启动服务失败")
		os.Exit(1)
	}

	// 服务关闭时清理资源
	database.CloseAll()
	modules.CloseAll()
	logger.L.Close()
}
