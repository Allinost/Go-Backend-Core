package minio

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/Allinost/go-backend-core/internal/config"
)

type Client struct {
	*minio.Client
	Bucket        string
	Region        string
	Endpoint      string
	UsingFallback bool
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

	// 对象存储只是图片/文件能力，bucket 探测放到后台，避免拖慢后端启动。
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		exists, err := client.BucketExists(ctx, cfg.Bucket)
		if err != nil {
			log.Printf("[minio] 检查 bucket 失败 [%s/%s]（允许为空，后续可恢复）: %v", cfg.Endpoint, cfg.Bucket, err)
		} else if !exists {
			log.Printf("[minio] bucket 不存在 [%s/%s]（允许为空，后续可恢复）", cfg.Endpoint, cfg.Bucket)
		}
	}()

	return &Client{
		Client:   client,
		Bucket:   cfg.Bucket,
		Region:   cfg.Region,
		Endpoint: cfg.Endpoint,
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
