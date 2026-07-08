package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Allinost/go-backend-core/internal/config"
)

// Client 封装 *redis.Client，提供健康检查
type Client struct {
	*redis.Client
}

// NewClient 根据配置创建 Redis 客户端
func NewClient(cfg config.RedisInstance) (*Client, error) {
	opts := &redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}

	if cfg.PoolSize > 0 {
		opts.PoolSize = cfg.PoolSize
	} else {
		opts.PoolSize = 10
	}

	rdb := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		return nil, fmt.Errorf("redis ping 失败 [%s]: %w", cfg.Addr, err)
	}

	return &Client{rdb}, nil
}

// Close 关闭 Redis 连接
func (c *Client) Close() error {
	if c.Client != nil {
		return c.Client.Close()
	}
	return nil
}

// Health 健康检查
func (c *Client) Health() error {
	if c.Client == nil {
		return fmt.Errorf("redis 连接未初始化")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return c.Ping(ctx).Err()
}
