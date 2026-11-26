package settings

import (
	"context"
	"time"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/middleware"
	"github.com/Allinost/go-backend-core/internal/modules/integration"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	svcAuth "github.com/Allinost/go-backend-core/internal/services/auth"
	"github.com/Allinost/go-backend-core/internal/services/configcenter"
	"github.com/Allinost/go-backend-core/internal/services/crypto"
	"github.com/Allinost/go-backend-core/zzz/login3rd"
)

// preparedConfigSvc 在 modules.InitAll 之前由 PrepareConfigCenter 构建，
// 保证 scheduler/integration 等先初始化的模块也能读到数据库中保存的配置。
var preparedConfigSvc *configcenter.Service

// PrepareConfigCenter 在数据库连接就绪后、模块初始化前调用：加载配置中心并把生效值写入 cfg。
func PrepareConfigCenter(cfg *config.Config) {
	preparedConfigSvc = newConfigService(cfg)
}

// newConfigService 创建配置中心：优先 PostgreSQL 持久化；不可用时降级为内存存储并标记 degraded（拒绝写入）。
func newConfigService(cfg *config.Config) *configcenter.Service {
	registry := configcenter.NewRegistry()
	if err := registry.Register(configcenter.DefaultSpecs()...); err != nil {
		logger.Error().Err(err).Msg("settings: 配置 Schema 注册失败")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 默认内存存储：保证 PG/迁移未就绪时配置中心仍可用（只读降级）。
	newSvc := func(store configcenter.Store) *configcenter.Service {
		s := configcenter.New(registry, store)
		s.SetBaseline(configBaseline(cfg))
		return s
	}
	svc := newSvc(configcenter.NewMemoryStore())
	if err := svc.Load(ctx); err != nil {
		logger.Warn().Err(err).Msg("settings: 配置中心内存加载失败")
	}

	db := database.Main()
	if db == nil {
		svc.SetDegraded("PostgreSQL 主库未连接")
		logger.Warn().Msg("settings: PostgreSQL 主库未连接，配置中心降级为只读内存模式")
	} else {
		sqlStore := configcenter.NewSQLStore(db)
		if err := sqlStore.Init(ctx); err != nil {
			svc.SetDegraded("system_config 未迁移")
			logger.Warn().Err(err).Msg("settings: system_config 未迁移，配置中心降级为只读内存模式")
		} else {
			sqlSvc := newSvc(sqlStore)
			if err := sqlSvc.Load(ctx); err != nil {
				svc.SetDegraded("PostgreSQL 配置加载失败: " + err.Error())
				logger.Error().Err(err).Msg("settings: 配置中心 PG 加载失败，降级为只读内存模式")
			} else {
				svc = sqlSvc
				logger.Info().Msg("settings: 配置中心已使用 PostgreSQL 持久化")
			}
		}
	}

	svc.SetApply(func(values map[string]any) error {
		applyConfigValues(cfg, values)
		return nil
	})
	if cfg.Crypto.MasterKeyHex == "" {
		logger.Warn().Msg("settings: 未配置 crypto.master_key_hex，敏感配置将以明文写入数据库")
	} else if cipher, err := crypto.NewAESGCMFromHex(cfg.Crypto.MasterKeyHex); err != nil {
		logger.Warn().Err(err).Msg("settings: 加密密钥不可用，敏感配置将以明文写入数据库")
	} else {
		svc.SetSecretCipher(cipher)
	}
	if err := svc.ApplyCurrent(); err != nil {
		logger.Warn().Err(err).Msg("settings: 配置中心启动应用失败")
	}
	return svc
}

// configBaseline 把 env/config.yaml 中的当前值作为配置中心启动基线；
// DB 中已保存的值仍会覆盖基线。
func configBaseline(cfg *config.Config) map[string]any {
	out := map[string]any{}
	if cfg == nil {
		return out
	}
	if cfg.Server.PublicBaseURL != "" {
		out["server.public_base_url"] = cfg.Server.PublicBaseURL
	}
	if cfg.Server.Mode != "" {
		out["server.mode"] = cfg.Server.Mode
	}
	if cfg.Log.Level != "" {
		out["log.level"] = cfg.Log.Level
	}
	if cfg.Log.Format != "" {
		out["log.format"] = cfg.Log.Format
	}
	if cfg.Cache.DefaultTTL != "" {
		out["cache.default_ttl"] = cfg.Cache.DefaultTTL
	}
	if cfg.Scheduler.WorkerConcurrency > 0 {
		out["scheduler.worker_concurrency"] = cfg.Scheduler.WorkerConcurrency
	}
	if cfg.Integration.RateLimit.RequestsPerMinute > 0 {
		out["integration.rate_limit.requests_per_minute"] = cfg.Integration.RateLimit.RequestsPerMinute
	}
	if cfg.Integration.RetentionDays > 0 {
		out["integration.retention_days"] = cfg.Integration.RetentionDays
	}
	return out
}

// applyConfigValues 将配置中心的值映射到运行中的 Config。
func applyConfigValues(cfg *config.Config, values map[string]any) {
	if cfg == nil {
		return
	}
	if s, ok := stringValue(values["server.public_base_url"]); ok {
		cfg.Server.PublicBaseURL = s
		login3rd.SetPublicBaseURL(s)
	}
	if s, ok := stringValue(values["server.mode"]); ok {
		cfg.Server.Mode = s
	}
	levelChanged := false
	formatChanged := false
	if s, ok := stringValue(values["log.level"]); ok {
		if s != cfg.Log.Level {
			levelChanged = true
		}
		cfg.Log.Level = s
	}
	if s, ok := stringValue(values["log.format"]); ok {
		if s != cfg.Log.Format {
			formatChanged = true
		}
		cfg.Log.Format = s
	}
	if levelChanged || formatChanged {
		logger.Reconfigure(logger.Config{
			Level: cfg.Log.Level, Format: cfg.Log.Format, Output: cfg.Log.Output,
			LogDir: cfg.Log.LogDir, MaxSizeMB: cfg.Log.MaxSizeMB, MaxAgeDays: cfg.Log.MaxAgeDays,
		})
	}
	if s, ok := stringValue(values["cache.default_ttl"]); ok {
		cfg.Cache.DefaultTTL = s
	}
	if n, ok := intValue(values["scheduler.worker_concurrency"]); ok {
		cfg.Scheduler.WorkerConcurrency = n
	}
	if n, ok := intValue(values["integration.rate_limit.requests_per_minute"]); ok {
		cfg.Integration.RateLimit.RequestsPerMinute = n
		integration.SetRateLimitPerMinute(n)
	}
	if n, ok := intValue(values["integration.retention_days"]); ok {
		cfg.Integration.RetentionDays = n
		integration.SetRetentionDays(n)
	}
	if n, ok := intValue(values["security.upload_max_mb"]); ok {
		middleware.SetUploadMaxMB(n)
	}
	if b, ok := boolValue(values["auth.allow_registration"]); ok {
		svcAuth.SetAllowRegistration(b)
	}
	maxFailures, lockMinutes := 5, 15
	if n, ok := intValue(values["auth.login_max_failures"]); ok {
		maxFailures = n
	}
	if n, ok := intValue(values["auth.login_lock_minutes"]); ok {
		lockMinutes = n
	}
	svcAuth.ConfigureLoginPolicy(maxFailures, lockMinutes)
}

func stringValue(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

func intValue(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

func boolValue(v any) (bool, bool) {
	b, ok := v.(bool)
	return b, ok
}
