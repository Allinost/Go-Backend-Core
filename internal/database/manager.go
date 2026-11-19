package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database/minio"
	"github.com/Allinost/go-backend-core/internal/database/mysql"
	"github.com/Allinost/go-backend-core/internal/database/postgres"
	"github.com/Allinost/go-backend-core/internal/database/redis"
	"github.com/Allinost/go-backend-core/internal/database/rustfs"
)

type DBManager struct {
	mu       sync.RWMutex
	MySQL    map[string]*mysql.Pool
	Postgres map[string]*postgres.Pool
	Redis    map[string]*redis.Client
	S3       map[string]*minio.Client
	RustFS   map[string]*rustfs.Client
	breakers map[string]*CircuitBreaker
}

type HealthStatus struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

var DB *DBManager

// InitAll 根据配置初始化所有数据库连接
func InitAll(cfg *config.Config) error {
	skipPing := strings.HasPrefix(cfg.Server.Version, "release")

	DB = &DBManager{
		MySQL:    make(map[string]*mysql.Pool),
		Postgres: make(map[string]*postgres.Pool),
		Redis:    make(map[string]*redis.Client),
		S3:       make(map[string]*minio.Client),
		RustFS:   make(map[string]*rustfs.Client),
		breakers: make(map[string]*CircuitBreaker),
	}

	initMySQL(cfg, skipPing)
	initPostgres(cfg, skipPing)
	initS3(cfg)
	initRedis(cfg, skipPing)

	return nil
}

func initMySQL(cfg *config.Config, skipPing bool) {
	for name, dbCfg := range cfg.Database.MySQL {
		if !dbCfg.Enabled {
			continue
		}
		pool, err := mysql.NewPool(dbCfg, skipPing)
		if err != nil {
			log.Printf("[database] MySQL[%s] 初始化失败: %v", name, err)
			continue
		}
		DB.breakers["mysql:"+name] = NewCircuitBreaker(5, 30*time.Second)
		DB.MySQL[name] = pool
	}
}

func initPostgres(cfg *config.Config, skipPing bool) {
	for name, dbCfg := range cfg.Database.Postgres {
		if !dbCfg.Enabled {
			continue
		}
		pool, err := postgres.NewPool(dbCfg, skipPing)
		if err != nil && dbCfg.FallbackHost != "" {
			fallback := dbCfg
			fallback.Host = dbCfg.FallbackHost
			if dbCfg.FallbackPort > 0 {
				fallback.Port = dbCfg.FallbackPort
			}
			pool, err = postgres.NewPool(fallback, skipPing)
			if err == nil {
				pool.UsingFallback = true
				log.Printf("[database] PostgreSQL[%s] 主库不可用，已切换到 L2 fallback", name)
			}
		}
		if err != nil {
			log.Printf("[database] PostgreSQL[%s] 初始化失败: %v", name, err)
			continue
		}
		DB.breakers["postgres:"+name] = NewCircuitBreaker(5, 30*time.Second)
		DB.Postgres[name] = pool
	}
}

func initS3(cfg *config.Config) {
	for name, s3Cfg := range cfg.Database.S3 {
		if !s3Cfg.Enabled {
			continue
		}
		switch name {
		case "minio":
			client, err := minio.NewClient(s3Cfg)
			if err != nil {
				log.Printf("[database] MinIO 初始化失败: %v", err)
				continue
			}
			if s3Cfg.FallbackEndpoint != "" {
				if healthErr := client.Health(); healthErr != nil {
					fallback := s3Cfg
					fallback.Endpoint = s3Cfg.FallbackEndpoint
					fbClient, fbErr := minio.NewClient(fallback)
					if fbErr == nil && fbClient.Health() == nil {
						_ = client.Close()
						client = fbClient
						client.UsingFallback = true
						log.Printf("[database] MinIO 主存储不可用，已切换到 L2 fallback")
					} else {
						log.Printf("[database] MinIO L2 fallback 不可用: %v", fbErr)
					}
				}
			}
			DB.breakers["s3:"+name] = NewCircuitBreaker(5, 30*time.Second)
			DB.S3[name] = client
		case "rustfs":
			client, err := rustfs.NewClient(s3Cfg)
			if err != nil {
				log.Printf("[database] RustFS 初始化失败: %v", err)
				continue
			}
			if s3Cfg.FallbackEndpoint != "" {
				if healthErr := client.Health(); healthErr != nil {
					fallback := s3Cfg
					fallback.Endpoint = s3Cfg.FallbackEndpoint
					fbClient, fbErr := rustfs.NewClient(fallback)
					if fbErr == nil && fbClient.Health() == nil {
						_ = client.Close()
						client = fbClient
						client.UsingFallback = true
						log.Printf("[database] RustFS 主存储不可用，已切换到 L2 fallback")
					} else {
						log.Printf("[database] RustFS L2 fallback 不可用: %v", fbErr)
					}
				}
			}
			DB.breakers["rustfs:"+name] = NewCircuitBreaker(5, 30*time.Second)
			DB.RustFS[name] = client
		default:
			log.Printf("[database] 未知的 S3 存储类型: %s", name)
		}
	}
}

func initRedis(cfg *config.Config, skipPing bool) {
	mainCfg := cfg.Redis.Main
	if mainCfg.Enabled && mainCfg.Addr != "" {
		client, err := redis.NewClient(mainCfg, skipPing)
		if err != nil && mainCfg.FallbackAddr != "" {
			fallback := mainCfg
			fallback.Addr = mainCfg.FallbackAddr
			client, err = redis.NewClient(fallback, skipPing)
			if err == nil {
				client.UsingFallback = true
				log.Printf("[database] Redis[main] 主库不可用，已切换到 L2 fallback")
			}
		}
		if err != nil {
			log.Printf("[database] Redis[main] 初始化失败: %v", err)
		} else {
			DB.breakers["redis:main"] = NewCircuitBreaker(5, 30*time.Second)
			DB.Redis["main"] = client
		}
	}
	for name, rc := range cfg.Redis.Extra {
		if !rc.Enabled {
			continue
		}
		client, err := redis.NewClient(rc, skipPing)
		if err != nil && rc.FallbackAddr != "" {
			fallback := rc
			fallback.Addr = rc.FallbackAddr
			client, err = redis.NewClient(fallback, skipPing)
			if err == nil {
				client.UsingFallback = true
				log.Printf("[database] Redis[%s] 主库不可用，已切换到 L2 fallback", name)
			}
		}
		if err != nil {
			log.Printf("[database] Redis[%s] 初始化失败: %v", name, err)
			continue
		}
		DB.breakers["redis:"+name] = NewCircuitBreaker(5, 30*time.Second)
		DB.Redis[name] = client
	}
}

// Reload 重建所有连接池（零停机：先建新连接，再原子替换）
func (m *DBManager) Reload(cfg *config.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldMySQL := m.MySQL
	oldPostgres := m.Postgres
	oldRedis := m.Redis
	oldS3 := m.S3
	oldRustFS := m.RustFS

	m.MySQL = make(map[string]*mysql.Pool)
	m.Postgres = make(map[string]*postgres.Pool)
	m.Redis = make(map[string]*redis.Client)
	m.S3 = make(map[string]*minio.Client)
	m.RustFS = make(map[string]*rustfs.Client)

	skipPing := strings.HasPrefix(cfg.Server.Version, "release")
	initMySQL(cfg, skipPing)
	initPostgres(cfg, skipPing)
	initS3(cfg)
	initRedis(cfg, skipPing)

	for _, pool := range oldMySQL {
		pool.Close()
	}
	for _, pool := range oldPostgres {
		pool.Close()
	}
	for _, client := range oldRedis {
		client.Close()
	}
	for _, client := range oldS3 {
		client.Close()
	}
	for _, client := range oldRustFS {
		client.Close()
	}

	return nil
}

// Main 返回 V3 主库的 database/sql 句柄（PostgreSQL main）。
// 供现有 *sql.DB 仓库平滑迁移；未配置/未连接时返回 nil。
func Main() *sql.DB {
	if DB == nil {
		return nil
	}
	DB.mu.RLock()
	defer DB.mu.RUnlock()
	pool := DB.Postgres["main"]
	if pool == nil {
		return nil
	}
	return pool.SQLDB
}

// RequireMain 校验 V3 主库已配置且可用；启动阶段调用，失败即快速退出。
// V3 不再回落到 MySQL 主库。
func RequireMain() error {
	if DB == nil {
		return fmt.Errorf("database: DB 未初始化")
	}
	DB.mu.RLock()
	pool := DB.Postgres["main"]
	DB.mu.RUnlock()
	if pool == nil || pool.Pool == nil || pool.SQLDB == nil {
		return fmt.Errorf("database: PostgreSQL main 未配置或连接失败")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.Pool.Ping(ctx); err != nil {
		return fmt.Errorf("database: PostgreSQL main 健康检查失败: %w", err)
	}
	return nil
}

// PG 返回 V3 主库的 pgxpool 句柄（PostgreSQL main）。
// 新代码优先使用 pgx；未配置/未连接时返回 nil。
func PG() *pgxpool.Pool {
	if DB == nil {
		return nil
	}
	DB.mu.RLock()
	defer DB.mu.RUnlock()
	pool := DB.Postgres["main"]
	if pool == nil {
		return nil
	}
	return pool.Pool
}

// GetRedis 按名称获取 Redis 客户端
func GetRedis(name string) *goredis.Client {
	if DB == nil {
		return nil
	}
	DB.mu.RLock()
	defer DB.mu.RUnlock()
	client, ok := DB.Redis[name]
	if !ok {
		return nil
	}
	return client.Client
}

// CloseAll 优雅关闭所有连接
func CloseAll() {
	DB.mu.Lock()
	defer DB.mu.Unlock()

	for _, pool := range DB.MySQL {
		pool.Close()
	}
	for _, pool := range DB.Postgres {
		pool.Close()
	}
	for _, client := range DB.Redis {
		client.Close()
	}
	for _, client := range DB.S3 {
		client.Close()
	}
	for _, client := range DB.RustFS {
		client.Close()
	}
}

// Health 聚合所有连接的健康状态
func Health() map[string]HealthStatus {
	result := make(map[string]HealthStatus)

	if DB == nil {
		result["database"] = HealthStatus{Status: "error", Error: "数据库未初始化"}
		return result
	}

	DB.mu.RLock()
	defer DB.mu.RUnlock()

	check := func(key string, healthy bool, err error) {
		if !healthy || err != nil {
			result[key] = HealthStatus{Status: "error", Error: err.Error()}
		} else {
			result[key] = HealthStatus{Status: "ok"}
		}
	}

	for name, pool := range DB.MySQL {
		check("mysql:"+name, true, pool.Health())
	}
	for name, pool := range DB.Postgres {
		check("postgres:"+name, true, pool.Health())
	}
	for name, client := range DB.Redis {
		check("redis:"+name, true, client.Health())
	}
	for name, client := range DB.S3 {
		check("s3:"+name, true, client.Health())
	}
	for name, client := range DB.RustFS {
		check("rustfs:"+name, true, client.Health())
	}

	return result
}

// RunWithBreaker 通过熔断器执行数据库操作
func RunWithBreaker(name string, fn func() error) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}
	DB.mu.RLock()
	cb, ok := DB.breakers[name]
	DB.mu.RUnlock()
	if !ok {
		return fn()
	}
	return cb.Run(fn)
}
