package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/Allinost/go-backend-core/internal/config"
)

type Pool struct {
	*pgxpool.Pool
	// SQLDB 是 pgx 的 database/sql 兼容层，供现有 *sql.DB 仓库复用。
	// 它由同一个 pgxpool 提供连接，Close 时需先关 SQLDB 再关 pgxpool。
	SQLDB         *sql.DB
	Endpoint      string // 当前实际连接地址
	UsingFallback bool   // 是否已切换到 L2
}

func NewPool(cfg config.PGConfig, skipPing bool) (*Pool, error) {
	dsn, err := buildDSN(cfg)
	if err != nil {
		return nil, err
	}

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres 解析配置失败 [%s:%d/%s]: %w", cfg.Host, cfg.Port, cfg.DBName, err)
	}

	if cfg.MaxOpen > 0 {
		poolCfg.MaxConns = int32(cfg.MaxOpen)
	}
	if cfg.MaxIdle > 0 {
		poolCfg.MinConns = int32(cfg.MaxIdle)
	}
	if cfg.ConnMaxLifetime != "" {
		d, err := time.ParseDuration(cfg.ConnMaxLifetime)
		if err == nil {
			poolCfg.MaxConnLifetime = d
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("postgres 创建连接池失败 [%s:%d/%s]: %w", cfg.Host, cfg.Port, cfg.DBName, err)
	}

	if !skipPing {
		if err := pool.Ping(ctx); err != nil {
			pool.Close()
			return nil, fmt.Errorf("postgres ping 失败 [%s:%d/%s]: %w", cfg.Host, cfg.Port, cfg.DBName, err)
		}
	}

	// 与 pgxpool 共享连接，避免现有 database/sql 仓库再建一套连接池。
	sqlDB := stdlib.OpenDBFromPool(pool)

	return &Pool{
		Pool:     pool,
		SQLDB:    sqlDB,
		Endpoint: net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
	}, nil
}

// buildDSN 统一构造 PostgreSQL DSN，补齐 V3 默认参数并转义用户/密码。
func buildDSN(cfg config.PGConfig) (string, error) {
	if strings.TrimSpace(cfg.Host) == "" {
		return "", fmt.Errorf("postgres host 不能为空")
	}
	if cfg.Port <= 0 {
		return "", fmt.Errorf("postgres port 必须大于 0")
	}
	if strings.TrimSpace(cfg.DBName) == "" {
		return "", fmt.Errorf("postgres dbname 不能为空")
	}

	u := &url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Path:   "/" + cfg.DBName,
	}
	if cfg.User != "" {
		u.User = url.UserPassword(cfg.User, cfg.Password)
	}

	q := u.Query()
	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	q.Set("sslmode", sslMode)

	searchPath := cfg.SearchPath
	if searchPath == "" {
		searchPath = "public"
	}
	q.Set("search_path", searchPath)

	tz := cfg.TimeZone
	if tz == "" {
		tz = "UTC"
	}
	q.Set("TimeZone", tz)

	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (p *Pool) Close() error {
	if p.SQLDB != nil {
		_ = p.SQLDB.Close()
	}
	if p.Pool != nil {
		p.Pool.Close()
	}
	return nil
}

func (p *Pool) Health() error {
	if p.Pool == nil {
		return fmt.Errorf("postgres 连接未初始化")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return p.Ping(ctx)
}
