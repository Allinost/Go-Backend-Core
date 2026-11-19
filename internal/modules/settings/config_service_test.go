package settings

import (
	"testing"

	"github.com/Allinost/go-backend-core/internal/config"
)

func TestApplyConfigValues(t *testing.T) {
	cfg := &config.Config{}
	applyConfigValues(cfg, map[string]any{
		"server.public_base_url":       "https://example.com",
		"server.mode":                  "release",
		"log.level":                    "warn",
		"log.format":                   "text",
		"cache.default_ttl":            "10m",
		"scheduler.worker_concurrency": 8,
		"integration.rate_limit.requests_per_minute": 1200,
		"integration.retention_days":                 60,
	})

	if cfg.Server.PublicBaseURL != "https://example.com" {
		t.Errorf("public_base_url 未应用: %q", cfg.Server.PublicBaseURL)
	}
	if cfg.Server.Mode != "release" {
		t.Errorf("server.mode 未应用: %q", cfg.Server.Mode)
	}
	if cfg.Log.Level != "warn" || cfg.Log.Format != "text" {
		t.Errorf("log 配置未应用: %+v", cfg.Log)
	}
	if cfg.Cache.DefaultTTL != "10m" {
		t.Errorf("cache.default_ttl 未应用: %q", cfg.Cache.DefaultTTL)
	}
	if cfg.Scheduler.WorkerConcurrency != 8 {
		t.Errorf("scheduler.worker_concurrency 未应用: %d", cfg.Scheduler.WorkerConcurrency)
	}
	if cfg.Integration.RateLimit.RequestsPerMinute != 1200 {
		t.Errorf("integration rate limit 未应用: %d", cfg.Integration.RateLimit.RequestsPerMinute)
	}
	if cfg.Integration.RetentionDays != 60 {
		t.Errorf("integration retention 未应用: %d", cfg.Integration.RetentionDays)
	}
}

func TestNewConfigService_FallbackMemory(t *testing.T) {
	cfg := &config.Config{}
	svc := newConfigService(cfg)
	if svc == nil {
		t.Fatal("配置中心不应为 nil")
	}
	if len(svc.Specs()) == 0 {
		t.Fatal("配置 Schema 不应为空")
	}
	values := svc.Values(true)
	if _, ok := values["log.level"]; !ok {
		t.Fatalf("默认配置缺失: %+v", values)
	}
}
