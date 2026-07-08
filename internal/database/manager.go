package database

import (
	"fmt"
	"sync"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database/minio"
	"github.com/Allinost/go-backend-core/internal/database/mysql"
	"github.com/Allinost/go-backend-core/internal/database/postgres"
	"github.com/Allinost/go-backend-core/internal/database/redis"
	"github.com/Allinost/go-backend-core/internal/database/rustfs"
)

// DBManager 统一数据访问管理器
type DBManager struct {
	mu       sync.RWMutex
	MySQL    map[string]*mysql.Pool
	Postgres map[string]*postgres.Pool
	Redis    map[string]*redis.Client
	S3       map[string]*minio.Client
	RustFS   map[string]*rustfs.Client
}

// HealthStatus 健康检查结果
type HealthStatus struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

var DB *DBManager

// InitAll 根据配置初始化所有数据库连接
func InitAll(cfg *config.Config) error {
	DB = &DBManager{
		MySQL:    make(map[string]*mysql.Pool),
		Postgres: make(map[string]*postgres.Pool),
		Redis:    make(map[string]*redis.Client),
		S3:       make(map[string]*minio.Client),
		RustFS:   make(map[string]*rustfs.Client),
	}

	// 初始化 MySQL 连接
	for name, dbCfg := range cfg.Database.MySQL {
		pool, err := mysql.NewPool(dbCfg)
		if err != nil {
			return fmt.Errorf("初始化 MySQL[%s] 失败: %w", name, err)
		}
		DB.MySQL[name] = pool
	}

	// 初始化 PostgreSQL 连接
	for name, dbCfg := range cfg.Database.Postgres {
		pool, err := postgres.NewPool(dbCfg)
		if err != nil {
			return fmt.Errorf("初始化 PostgreSQL[%s] 失败: %w", name, err)
		}
		DB.Postgres[name] = pool
	}

	// 初始化 S3 (MinIO) 连接
	for name, s3Cfg := range cfg.Database.S3 {
		switch name {
		case "minio":
			client, err := minio.NewClient(s3Cfg)
			if err != nil {
				return fmt.Errorf("初始化 MinIO 失败: %w", err)
			}
			DB.S3[name] = client
		case "rustfs":
			client, err := rustfs.NewClient(s3Cfg)
			if err != nil {
				return fmt.Errorf("初始化 RustFS 失败: %w", err)
			}
			DB.RustFS[name] = client
		default:
			return fmt.Errorf("未知的 S3 存储类型: %s", name)
		}
	}

	// 初始化 Redis 连接
	mainCfg := cfg.Redis.Main
	if mainCfg.Addr != "" {
		client, err := redis.NewClient(mainCfg)
		if err != nil {
			return fmt.Errorf("初始化 Redis[main] 失败: %w", err)
		}
		DB.Redis["main"] = client
	}
	for name, rc := range cfg.Redis.Extra {
		client, err := redis.NewClient(rc)
		if err != nil {
			return fmt.Errorf("初始化 Redis[%s] 失败: %w", name, err)
		}
		DB.Redis[name] = client
	}

	return nil
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

	for name, pool := range DB.MySQL {
		key := "mysql:" + name
		if err := pool.Health(); err != nil {
			result[key] = HealthStatus{Status: "error", Error: err.Error()}
		} else {
			result[key] = HealthStatus{Status: "ok"}
		}
	}
	for name, pool := range DB.Postgres {
		key := "postgres:" + name
		if err := pool.Health(); err != nil {
			result[key] = HealthStatus{Status: "error", Error: err.Error()}
		} else {
			result[key] = HealthStatus{Status: "ok"}
		}
	}
	for name, client := range DB.Redis {
		key := "redis:" + name
		if err := client.Health(); err != nil {
			result[key] = HealthStatus{Status: "error", Error: err.Error()}
		} else {
			result[key] = HealthStatus{Status: "ok"}
		}
	}
	for name, client := range DB.S3 {
		key := "s3:" + name
		if err := client.Health(); err != nil {
			result[key] = HealthStatus{Status: "error", Error: err.Error()}
		} else {
			result[key] = HealthStatus{Status: "ok"}
		}
	}
	for name, client := range DB.RustFS {
		key := "rustfs:" + name
		if err := client.Health(); err != nil {
			result[key] = HealthStatus{Status: "error", Error: err.Error()}
		} else {
			result[key] = HealthStatus{Status: "ok"}
		}
	}

	return result
}
