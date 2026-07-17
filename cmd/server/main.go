package main

import (
	"context"
	"fmt"
	"github.com/Allinost/go-backend-core/internal/modules/ops"
	"github.com/Allinost/go-backend-core/zzz/goodser"
	"html/template"
	"net/http"
	"os"
	"strings"
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
	modules.Register(&ops.Module{})

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

	// Swagger UI — 环境预设 + 手动输入 + ?backend=IP:PORT（cookie 持久化）
	defaultBackend := "192.168.1.36:29090"
	if cfg.Server.Swagger.BackendAddr != "" {
		defaultBackend = cfg.Server.Swagger.BackendAddr
	}

	// 构建环境预设映射，注入 __custom__ 占位
	presets := cfg.Server.Swagger.Presets
	if presets == nil {
		presets = make(map[string]string)
	}
	if _, ok := presets["自定义"]; !ok {
		presets["自定义"] = "__custom__"
	}

	swaggerDocs.SwaggerInfo.Host = defaultBackend
	swaggerUI := ginSwagger.WrapHandler(swaggerFiles.Handler)

	debugR.GET("/swagger/*any", func(c *gin.Context) {
		path := c.Param("any")

		// 自定义 Swagger UI 首页（含环境选择 + 手动输入）
		if path == "/" || path == "/index.html" {
			backend := resolveSwaggerBackend(c, defaultBackend)
			serveSwaggerHTML(c, backend, presets)
			return
		}

		// doc.json — 动态生成规范时注入 backend
		if strings.HasSuffix(path, "doc.json") {
			backend := resolveSwaggerBackend(c, defaultBackend)
			swaggerDocs.SwaggerInfo.Host = backend
		}

		// 静态资源（swagger-ui JS/CSS/图片）由 swaggerFiles 原生提供
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

// resolveSwaggerBackend 解析后端地址：查询参数 > cookie > 配置文件默认值
func resolveSwaggerBackend(c *gin.Context, defaultBackend string) string {
	backend := ""

	if q := c.Query("backend"); q != "" {
		backend = q
		c.SetCookie("swagger_backend", backend, 86400*365, "/swagger", "", false, true)
	}

	if backend == "" {
		if cookie, err := c.Cookie("swagger_backend"); err == nil && cookie != "" {
			backend = cookie
		}
	}

	if backend == "" {
		backend = defaultBackend
	}

	return backend
}

// serveSwaggerHTML 渲染自定义 Swagger UI 首页（环境选择 + 手动输入）
func serveSwaggerHTML(c *gin.Context, backend string, presets map[string]string) {
	tmpl := template.Must(template.New("swagger").Parse(swaggerUITemplate))
	data := map[string]any{
		"Backend": backend,
		"Presets": presets,
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.Execute(c.Writer, data)
}

const swaggerUITemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>API 文档 - Go Backend Core</title>
    <link rel="stylesheet" href="/swagger/swagger-ui.css">
    <style>
        body { margin: 0; padding: 0; font-family: -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,sans-serif; }
        .swagger-toolbar {
            background: #1b1b1b; color: #fff; padding: 10px 20px;
            display: flex; align-items: center; gap: 10px; flex-wrap: wrap;
            border-bottom: 2px solid #4990e2;
        }
        .swagger-toolbar label { font-size: 14px; font-weight: 600; white-space: nowrap; }
        .swagger-toolbar select, .swagger-toolbar input {
            padding: 6px 10px; border-radius: 4px; border: 1px solid #555;
            background: #2d2d2d; color: #fff; font-size: 13px;
        }
        .swagger-toolbar select option { background: #2d2d2d; }
        .swagger-toolbar input { width: 200px; }
        .swagger-toolbar button {
            padding: 6px 16px; border: none; border-radius: 4px;
            background: #4990e2; color: #fff; cursor: pointer; font-size: 13px; font-weight: 600;
        }
        .swagger-toolbar button:hover { background: #357abd; }
        .swagger-toolbar .current {
            font-size: 12px; color: #999; margin-left: auto;
        }
    </style>
</head>
<body>
    <div class="swagger-toolbar">
        <label>后端地址：</label>
        <select id="envSelect" onchange="onEnvChange()">
            {{range $name, $addr := .Presets}}
            <option value="{{$addr}}" {{if eq $addr $.Backend}}selected{{end}}>{{$name}}</option>
            {{end}}
        </select>
        <input type="text" id="customAddr" placeholder="IP:PORT" style="display:none">
        <button onclick="applyBackend()">切换</button>
        <span class="current">当前：{{.Backend}}</span>
    </div>
    <div id="swagger-ui"></div>
    <script src="/swagger/swagger-ui-bundle.js"></script>
    <script src="/swagger/swagger-ui-standalone-preset.js"></script>
    <script>
        (function () {
            var backend = '{{.Backend}}';
            var select = document.getElementById('envSelect');
            var customInput = document.getElementById('customAddr');

            // 检查当前后端是否匹配某个预设
            var matched = false;
            for (var i = 0; i < select.options.length; i++) {
                if (select.options[i].value === backend && select.options[i].value !== '__custom__') {
                    select.selectedIndex = i;
                    matched = true;
                    break;
                }
            }
            if (!matched) {
                select.value = '__custom__';
                customInput.value = backend;
                customInput.style.display = 'inline-block';
            }

            window.ui = SwaggerUIBundle({
                url: '/swagger/doc.json?backend=' + encodeURIComponent(backend),
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout"
            });
        })();

        function onEnvChange() {
            var select = document.getElementById('envSelect');
            var customInput = document.getElementById('customAddr');
            if (select.value === '__custom__') {
                customInput.style.display = 'inline-block';
                customInput.focus();
            } else {
                customInput.style.display = 'none';
            }
        }

        function applyBackend() {
            var select = document.getElementById('envSelect');
            var backend = select.value;
            if (backend === '__custom__') {
                backend = document.getElementById('customAddr').value.trim();
                if (!backend) { alert('请输入 IP:PORT'); return; }
            }
            if (backend === '{{.Backend}}') return;
            window.location.href = '/swagger/?backend=' + encodeURIComponent(backend);
        }
    </script>
</body>
</html>`
