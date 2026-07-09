package net

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GRPCConfig struct {
	Address    string
	Timeout    time.Duration
	MaxRetries int
	Insecure   bool
}

func DefaultGRPCConfig(addr string) GRPCConfig {
	return GRPCConfig{
		Address:    addr,
		Timeout:    30 * time.Second,
		MaxRetries: 2,
		Insecure:   true,
	}
}

type GRPCClient struct {
	conn   *grpc.ClientConn
	config GRPCConfig
}

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

func (c *GRPCClient) Conn() *grpc.ClientConn {
	return c.conn
}

func (c *GRPCClient) Close() error {
	return c.conn.Close()
}
