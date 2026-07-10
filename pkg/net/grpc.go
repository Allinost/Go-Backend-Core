package net

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCConfig gRPC 客户端配置
type GRPCConfig struct {
	Address    string
	Timeout    time.Duration
	MaxRetries int
	Insecure   bool
}

// DefaultGRPCConfig 返回默认 gRPC 配置（30s 超时，2 次重试，不安全连接）
func DefaultGRPCConfig(addr string) GRPCConfig {
	return GRPCConfig{
		Address:    addr,
		Timeout:    30 * time.Second,
		MaxRetries: 2,
		Insecure:   true,
	}
}

// GRPCClient gRPC 客户端封装
type GRPCClient struct {
	conn   *grpc.ClientConn
	config GRPCConfig
}

// NewGRPCClient 根据配置创建 gRPC 连接
func NewGRPCClient(cfg GRPCConfig) (*GRPCClient, error) {
	opts := []grpc.DialOption{
		grpc.WithBlock(),
	}
	if cfg.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, cfg.Address, opts...)
	if err != nil {
		return nil, fmt.Errorf("net: gRPC 连接 %s 失败: %w", cfg.Address, err)
	}

	return &GRPCClient{conn: conn, config: cfg}, nil
}

// Conn 返回底层的 gRPC 连接
func (c *GRPCClient) Conn() *grpc.ClientConn {
	return c.conn
}

// Close 关闭 gRPC 连接
func (c *GRPCClient) Close() error {
	return c.conn.Close()
}
