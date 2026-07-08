package minio

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/Allinost/go-backend-core/internal/config"
)

type Client struct {
	*minio.Client
	Bucket string
	Region string
}

func NewClient(cfg config.S3Config) (*Client, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("minio 创建客户端失败 [%s]: %w", cfg.Endpoint, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("minio 检查 bucket 失败 [%s/%s]: %w", cfg.Endpoint, cfg.Bucket, err)
	}
	if !exists {
		return nil, fmt.Errorf("minio bucket 不存在 [%s/%s]", cfg.Endpoint, cfg.Bucket)
	}

	return &Client{
		Client: client,
		Bucket: cfg.Bucket,
		Region: cfg.Region,
	}, nil
}

func (c *Client) Close() error {
	return nil
}

func (c *Client) Health() error {
	if c.Client == nil {
		return fmt.Errorf("minio 连接未初始化")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.BucketExists(ctx, c.Bucket)
	return err
}
