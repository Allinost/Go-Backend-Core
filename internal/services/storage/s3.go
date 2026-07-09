package storage

import (
	"context"
	"fmt"
	"io"
	"path"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Store 基于 MinIO S3 的文件存储实现
type S3Store struct {
	client *minio.Client
	bucket string
}

// NewS3Store 创建 S3 存储客户端
func NewS3Store(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*S3Store, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: s3 client failed: %w", err)
	}
	return &S3Store{client: client, bucket: bucket}, nil
}

// Upload 上传文件到 S3，返回文件元信息
func (s *S3Store) Upload(ctx context.Context, filePath string, reader io.Reader, opts ...UploadOption) (*FileInfo, error) {
	var o UploadOptions
	for _, opt := range opts {
		opt(&o)
	}
	ct := o.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	_, err := s.client.PutObject(ctx, s.bucket, filePath, reader, -1, minio.PutObjectOptions{
		ContentType:  ct,
		UserMetadata: o.Metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: s3 upload %s failed: %w", filePath, err)
	}
	obj, err := s.client.StatObject(ctx, s.bucket, filePath, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("storage: s3 stat %s failed: %w", filePath, err)
	}
	return &FileInfo{
		Name:         path.Base(filePath),
		Path:         filePath,
		Size:         obj.Size,
		ContentType:  obj.ContentType,
		LastModified: obj.LastModified,
		ETag:         obj.ETag,
	}, nil
}

// Download 从 S3 下载文件
func (s *S3Store) Download(ctx context.Context, filePath string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, filePath, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("storage: s3 download %s failed: %w", filePath, err)
	}
	return obj, nil
}

// Delete 从 S3 删除文件
func (s *S3Store) Delete(ctx context.Context, filePath string) error {
	err := s.client.RemoveObject(ctx, s.bucket, filePath, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("storage: s3 delete %s failed: %w", filePath, err)
	}
	return nil
}

// List 按前缀列出 S3 文件
func (s *S3Store) List(ctx context.Context, prefix string, opts ...ListOption) ([]FileInfo, error) {
	var o ListOptions
	for _, opt := range opts {
		opt(&o)
	}
	if o.Offset > 0 {
		_ = o.Offset
	}

	ctxDone, cancel := context.WithCancel(ctx)
	defer cancel()

	var result []FileInfo
	objCh := s.client.ListObjects(ctxDone, s.bucket, minio.ListObjectsOptions{
		Prefix: prefix,
	})
	for obj := range objCh {
		if obj.Err != nil {
			return nil, fmt.Errorf("storage: s3 list failed: %w", obj.Err)
		}
		info := FileInfo{
			Name:         path.Base(obj.Key),
			Path:         obj.Key,
			Size:         obj.Size,
			ContentType:  obj.ContentType,
			LastModified: obj.LastModified,
			ETag:         obj.ETag,
		}
		if o.Offset > 0 && len(result) < o.Offset {
			_ = info
			continue
		}
		if o.Limit > 0 && len(result) >= o.Limit {
			break
		}
		result = append(result, info)
	}
	if result == nil {
		result = []FileInfo{}
	}
	return result, nil
}

// Exists 检查 S3 文件是否存在
func (s *S3Store) Exists(ctx context.Context, filePath string) (bool, error) {
	_, err := s.client.StatObject(ctx, s.bucket, filePath, minio.StatObjectOptions{})
	if err != nil {
		errResponse := minio.ToErrorResponse(err)
		if errResponse.Code == "NoSuchKey" {
			return false, nil
		}
		return false, fmt.Errorf("storage: s3 stat %s failed: %w", filePath, err)
	}
	return true, nil
}

// SignedURL 生成 S3 预签名下载 URL
func (s *S3Store) SignedURL(ctx context.Context, filePath string, expire time.Duration) (string, error) {
	url, err := s.client.PresignedGetObject(ctx, s.bucket, filePath, expire, nil)
	if err != nil {
		return "", fmt.Errorf("storage: s3 signed url %s failed: %w", filePath, err)
	}
	return url.String(), nil
}

func (s *S3Store) Close() error {
	return nil
}
