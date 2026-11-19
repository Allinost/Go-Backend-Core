package settings

import (
	"context"
	"time"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	svcAuth "github.com/Allinost/go-backend-core/internal/services/auth"
	"github.com/Allinost/go-backend-core/internal/services/configcenter"
	"github.com/Allinost/go-backend-core/internal/services/crypto"
	"github.com/Allinost/go-backend-core/zzz/login3rd"
)

// newConfigService 创建配置中心：优先 PostgreSQL 持久化，表未迁移时降级内存。
func newConfigService(cfg *config.Config) *configcenter.Service {
	registry := configcenter.NewRegistry()
	if err := registry.Register(configcenter.DefaultSpecs()...); err != nil {
		logger.Error().Err(err).Msg("settings: 配置 Schema 注册失败")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 默认内存存储：保证 PG/迁移未就绪时配置中心仍可用。
	svc := configcenter.New(registry, configcenter.NewMemoryStore())
	if err := svc.Load(ctx); err != nil {
		logger.Warn().Err(err).Msg("settings: 配置中心内存加载失败")
	}

	if db := database.Main(); db != nil {
		sqlStore := configcenter.NewSQLStore(db)
		if err := sqlStore.Init(ctx); err != nil {
			logger.Warn().Err(err).Msg("settings: system_config 未迁移，配置中心降级为内存存储")
		} else {
			sqlSvc := configcenter.New(registry, sqlStore)
			if err := sqlSvc.Load(ctx); err != nil {
				logger.Warn().Err(err).Msg("settings: 配置中心 PG 加载失败，继续使用内存存储")
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
	if s, ok := stringValue(values["log.level"]); ok {
		cfg.Log.Level = s
	}
	if s, ok := stringValue(values["log.format"]); ok {
		cfg.Log.Format = s
	}
	if s, ok := stringValue(values["cache.default_ttl"]); ok {
		cfg.Cache.DefaultTTL = s
	}
	if n, ok := intValue(values["scheduler.worker_concurrency"]); ok {
		cfg.Scheduler.WorkerConcurrency = n
	}
	if n, ok := intValue(values["integration.rate_limit.requests_per_minute"]); ok {
		cfg.Integration.RateLimit.RequestsPerMinute = n
	}
	if n, ok := intValue(values["integration.retention_days"]); ok {
		cfg.Integration.RetentionDays = n
	}
	if b, ok := boolValue(values["auth.allow_registration"]); ok {
		svcAuth.SetAllowRegistration(b)
	}
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
