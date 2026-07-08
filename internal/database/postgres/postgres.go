package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allinost/go-backend-core/internal/config"
)

type Pool struct {
	*pgxpool.Pool
}

func NewPool(cfg config.PGConfig) (*Pool, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres 解析配置失败 [%s:%d/%s]: %w", cfg.Host, cfg.Port, cfg.DBName, err)
	}

	if cfg.MaxOpen > 0 {
		poolCfg.MaxConns = int32(cfg.MaxOpen)
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

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping 失败 [%s:%d/%s]: %w", cfg.Host, cfg.Port, cfg.DBName, err)
	}

	return &Pool{pool}, nil
}

func (p *Pool) Close() error {
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
