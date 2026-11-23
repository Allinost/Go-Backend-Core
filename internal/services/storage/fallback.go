package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrUnavailable 表示对象存储和暂存都不可用。
	ErrUnavailable = errors.New("storage: 存储服务暂不可用")
	// ErrStaged 表示对象仍在 Redis 暂存区，尚未提交到对象存储。
	ErrStaged = errors.New("storage: 文件暂存在 Redis，尚未同步到对象存储")
)

var errStageNotFound = errors.New("storage: 暂存文件不存在")

const (
	defaultStageTTL      = 24 * time.Hour
	defaultMaxStageBytes = 64 << 20 // 64MB
)

// Stage 是对象暂存区。Redis 实现用于在对象存储故障时承接上传。
type Stage interface {
	Upload(ctx context.Context, filePath string, data []byte, info FileInfo, ttl time.Duration) error
	Download(ctx context.Context, filePath string) (io.ReadCloser, *FileInfo, error)
	Stat(ctx context.Context, filePath string) (*FileInfo, error)
	Exists(ctx context.Context, filePath string) (bool, error)
	Delete(ctx context.Context, filePath string) error
	List(ctx context.Context) ([]FileInfo, error)
	Count(ctx context.Context) (int, error)
	Close() error
}

// RedisStage 基于 Redis 的临时对象暂存区。
type RedisStage struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

func NewRedisStage(client *redis.Client, ttl time.Duration) *RedisStage {
	if ttl <= 0 {
		ttl = defaultStageTTL
	}
	return &RedisStage{client: client, prefix: "v3:storage:stage", ttl: ttl}
}

func (s *RedisStage) objectKey(filePath string) string {
	sum := sha256.Sum256([]byte(filePath))
	return s.prefix + ":obj:" + hex.EncodeToString(sum[:])
}

func (s *RedisStage) indexKey() string { return s.prefix + ":index" }

func (s *RedisStage) Upload(ctx context.Context, filePath string, data []byte, info FileInfo, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = s.ttl
	}
	if info.ContentType == "" {
		info.ContentType = "application/octet-stream"
	}
	if info.LastModified.IsZero() {
		info.LastModified = time.Now()
	}
	metadata, err := json.Marshal(info.Metadata)
	if err != nil {
		return fmt.Errorf("storage: 序列化暂存元信息失败: %w", err)
	}
	key := s.objectKey(filePath)
	pipe := s.client.TxPipeline()
	pipe.HSet(ctx, key, map[string]any{
		"path":          filePath,
		"size":          info.Size,
		"content_type":  info.ContentType,
		"last_modified": info.LastModified.Format(time.RFC3339Nano),
		"etag":          info.ETag,
		"metadata":      string(metadata),
		"data":          data,
	})
	pipe.Expire(ctx, key, ttl)
	pipe.SAdd(ctx, s.indexKey(), filePath)
	pipe.Expire(ctx, s.indexKey(), ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("storage: Redis 暂存失败: %w", err)
	}
	return nil
}

func (s *RedisStage) read(ctx context.Context, filePath string) (map[string]string, error) {
	values, err := s.client.HGetAll(ctx, s.objectKey(filePath)).Result()
	if err != nil {
		return nil, fmt.Errorf("storage: 读取 Redis 暂存失败: %w", err)
	}
	if len(values) == 0 {
		return nil, errStageNotFound
	}
	return values, nil
}

func (s *RedisStage) fileInfo(values map[string]string) *FileInfo {
	size, _ := strconv.ParseInt(values["size"], 10, 64)
	modified, _ := time.Parse(time.RFC3339Nano, values["last_modified"])
	var metadata map[string]string
	if raw := values["metadata"]; raw != "" && raw != "null" {
		_ = json.Unmarshal([]byte(raw), &metadata)
	}
	return &FileInfo{
		Name:         path.Base(values["path"]),
		Path:         values["path"],
		Size:         size,
		ContentType:  values["content_type"],
		LastModified: modified,
		ETag:         values["etag"],
		Metadata:     metadata,
		Staged:       true,
	}
}

func (s *RedisStage) Download(ctx context.Context, filePath string) (io.ReadCloser, *FileInfo, error) {
	values, err := s.read(ctx, filePath)
	if err != nil {
		return nil, nil, err
	}
	return io.NopCloser(bytes.NewReader([]byte(values["data"]))), s.fileInfo(values), nil
}

func (s *RedisStage) Stat(ctx context.Context, filePath string) (*FileInfo, error) {
	values, err := s.read(ctx, filePath)
	if err != nil {
		return nil, err
	}
	return s.fileInfo(values), nil
}

func (s *RedisStage) Exists(ctx context.Context, filePath string) (bool, error) {
	n, err := s.client.Exists(ctx, s.objectKey(filePath)).Result()
	if err != nil {
		return false, fmt.Errorf("storage: 检查 Redis 暂存失败: %w", err)
	}
	return n > 0, nil
}

func (s *RedisStage) Delete(ctx context.Context, filePath string) error {
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, s.objectKey(filePath))
	pipe.SRem(ctx, s.indexKey(), filePath)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("storage: 删除 Redis 暂存失败: %w", err)
	}
	return nil
}

func (s *RedisStage) List(ctx context.Context) ([]FileInfo, error) {
	paths, err := s.client.SMembers(ctx, s.indexKey()).Result()
	if err != nil {
		return nil, fmt.Errorf("storage: 列出 Redis 暂存失败: %w", err)
	}
	items := make([]FileInfo, 0, len(paths))
	stale := make([]any, 0)
	for _, filePath := range paths {
		info, err := s.Stat(ctx, filePath)
		if errors.Is(err, errStageNotFound) {
			stale = append(stale, filePath)
			continue
		}
		if err != nil {
			return nil, err
		}
		items = append(items, *info)
	}
	if len(stale) > 0 {
		_ = s.client.SRem(ctx, s.indexKey(), stale...).Err()
	}
	return items, nil
}

func (s *RedisStage) Count(ctx context.Context) (int, error) {
	n, err := s.client.SCard(ctx, s.indexKey()).Result()
	if err != nil {
		return 0, fmt.Errorf("storage: 统计 Redis 暂存失败: %w", err)
	}
	return int(n), nil
}

func (s *RedisStage) Close() error { return nil }

// FallbackStore 优先写对象存储；对象存储不可用时把对象暂存 Redis，
// 恢复后由 Flush 或后台任务提交。两者都不可用时只让文件接口报错。
type FallbackStore struct {
	primary        Storage
	stage          Stage
	maxStageBytes  int64
	primaryTimeout time.Duration

	flushMu sync.Mutex
}

func NewFallbackStore(primary Storage, stage Stage, maxStageBytes int64) *FallbackStore {
	if maxStageBytes <= 0 {
		maxStageBytes = defaultMaxStageBytes
	}
	return &FallbackStore{
		primary:        primary,
		stage:          stage,
		maxStageBytes:  maxStageBytes,
		primaryTimeout: 10 * time.Second,
	}
}

// SetPrimaryTimeout 设置单次对象存储操作超时，避免故障时请求长时间挂起。
func (f *FallbackStore) SetPrimaryTimeout(timeout time.Duration) {
	if timeout > 0 {
		f.primaryTimeout = timeout
	}
}

func (f *FallbackStore) primaryCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, f.primaryTimeout)
}

func (f *FallbackStore) HasPrimary() bool { return f.primary != nil }
func (f *FallbackStore) HasStage() bool   { return f.stage != nil }

// PrimaryHealth 检查对象存储当前是否可访问。
func (f *FallbackStore) PrimaryHealth(ctx context.Context) error {
	if f.primary == nil {
		return ErrUnavailable
	}
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if checker, ok := f.primary.(interface {
		Health(context.Context) error
	}); ok {
		return checker.Health(checkCtx)
	}
	_, err := f.primary.List(checkCtx, "", WithLimit(1))
	return err
}

// Mode 返回当前存储组合，便于状态接口展示。
func (f *FallbackStore) Mode() string {
	switch {
	case f.primary != nil && f.stage != nil:
		return "s3+redis"
	case f.primary != nil:
		return "s3"
	case f.stage != nil:
		return "redis"
	default:
		return "unavailable"
	}
}

func (f *FallbackStore) Upload(ctx context.Context, filePath string, reader io.Reader, opts ...UploadOption) (*FileInfo, error) {
	if f.primary == nil && f.stage == nil {
		return nil, ErrUnavailable
	}
	var o UploadOptions
	for _, opt := range opts {
		opt(&o)
	}

	data, err := io.ReadAll(io.LimitReader(reader, f.maxStageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("storage: 读取上传内容失败: %w", err)
	}
	tooLarge := int64(len(data)) > f.maxStageBytes

	var primaryErr error
	if f.primary != nil {
		var src io.Reader = bytes.NewReader(data)
		if tooLarge {
			src = io.MultiReader(bytes.NewReader(data), reader)
		}
		pctx, cancel := f.primaryCtx(ctx)
		info, err := f.primary.Upload(pctx, filePath, src, opts...)
		cancel()
		if err == nil {
			return info, nil
		}
		primaryErr = err
	}
	if f.stage == nil {
		if primaryErr != nil {
			return nil, fmt.Errorf("%w: 对象存储失败(%v)，Redis 暂存不可用", ErrUnavailable, primaryErr)
		}
		return nil, ErrUnavailable
	}
	if tooLarge {
		return nil, fmt.Errorf("%w: 文件超过 Redis 暂存上限 %dMB，且对象存储不可用", ErrUnavailable, f.maxStageBytes>>20)
	}

	info := FileInfo{
		Name:         path.Base(filePath),
		Path:         filePath,
		Size:         int64(len(data)),
		ContentType:  o.ContentType,
		LastModified: time.Now(),
		Metadata:     o.Metadata,
		Staged:       true,
	}
	if err := f.stage.Upload(ctx, filePath, data, info, 0); err != nil {
		if primaryErr != nil {
			return nil, fmt.Errorf("%w: 对象存储失败(%v)，Redis 暂存失败(%v)", ErrUnavailable, primaryErr, err)
		}
		return nil, fmt.Errorf("%w: Redis 暂存失败(%v)", ErrUnavailable, err)
	}
	return &info, nil
}

func (f *FallbackStore) Download(ctx context.Context, filePath string) (io.ReadCloser, error) {
	var primaryErr error
	if f.primary != nil {
		reader, err := f.primary.Download(ctx, filePath)
		if err == nil {
			return reader, nil
		}
		primaryErr = err
	}
	if f.stage != nil {
		reader, _, err := f.stage.Download(ctx, filePath)
		if err == nil {
			return reader, nil
		}
		if !errors.Is(err, errStageNotFound) && primaryErr == nil {
			primaryErr = err
		}
	}
	if primaryErr != nil {
		return nil, primaryErr
	}
	return nil, ErrUnavailable
}

func (f *FallbackStore) Stat(ctx context.Context, filePath string) (*FileInfo, error) {
	var primaryErr error
	if f.primary != nil {
		pctx, cancel := f.primaryCtx(ctx)
		info, err := f.primary.Stat(pctx, filePath)
		cancel()
		if err == nil {
			return info, nil
		}
		primaryErr = err
	}
	if f.stage != nil {
		info, err := f.stage.Stat(ctx, filePath)
		if err == nil {
			return info, nil
		}
		if errors.Is(err, errStageNotFound) && errors.Is(primaryErr, ErrNotFound) {
			return nil, ErrNotFound
		}
		if !errors.Is(err, errStageNotFound) && primaryErr == nil {
			primaryErr = err
		}
	}
	if errors.Is(primaryErr, ErrNotFound) {
		return nil, ErrNotFound
	}
	if primaryErr != nil {
		return nil, primaryErr
	}
	return nil, ErrUnavailable
}

func (f *FallbackStore) Exists(ctx context.Context, filePath string) (bool, error) {
	if f.primary == nil && f.stage == nil {
		return false, ErrUnavailable
	}
	var primaryErr error
	if f.primary != nil {
		pctx, cancel := f.primaryCtx(ctx)
		exists, err := f.primary.Exists(pctx, filePath)
		cancel()
		if err == nil && exists {
			return true, nil
		}
		if err != nil {
			primaryErr = err
		}
	}
	if f.stage != nil {
		exists, err := f.stage.Exists(ctx, filePath)
		if err == nil && exists {
			return true, nil
		}
		if err != nil && primaryErr == nil {
			primaryErr = err
		}
	}
	if primaryErr != nil {
		return false, primaryErr
	}
	return false, nil
}

func (f *FallbackStore) Delete(ctx context.Context, filePath string) error {
	var primaryErr error
	if f.primary != nil {
		pctx, cancel := f.primaryCtx(ctx)
		err := f.primary.Delete(pctx, filePath)
		cancel()
		if err == nil {
			if f.stage != nil {
				_ = f.stage.Delete(ctx, filePath)
			}
			return nil
		} else if !errors.Is(err, ErrNotFound) {
			primaryErr = err
		}
	}
	if f.stage != nil {
		if err := f.stage.Delete(ctx, filePath); err == nil {
			return nil
		} else if primaryErr == nil {
			primaryErr = err
		}
	}
	if errors.Is(primaryErr, ErrNotFound) {
		return ErrNotFound
	}
	if primaryErr != nil {
		return primaryErr
	}
	return ErrUnavailable
}

func (f *FallbackStore) List(ctx context.Context, prefix string, opts ...ListOption) ([]FileInfo, error) {
	var primaryItems []FileInfo
	var primaryErr error
	if f.primary != nil {
		pctx, cancel := f.primaryCtx(ctx)
		primaryItems, primaryErr = f.primary.List(pctx, prefix, opts...)
		cancel()
	}
	if f.stage == nil {
		if primaryErr != nil {
			return nil, primaryErr
		}
		if f.primary == nil {
			return nil, ErrUnavailable
		}
		return primaryItems, nil
	}

	staged, err := f.stage.List(ctx)
	if err != nil {
		if primaryErr != nil {
			return nil, primaryErr
		}
		if f.primary == nil {
			return nil, err
		}
		return primaryItems, nil
	}
	seen := make(map[string]int, len(primaryItems))
	for i, item := range primaryItems {
		seen[item.Path] = i
	}
	for _, item := range staged {
		if prefix != "" && !strings.HasPrefix(item.Path, prefix) {
			continue
		}
		if idx, ok := seen[item.Path]; ok {
			primaryItems[idx].Staged = true
			continue
		}
		primaryItems = append(primaryItems, item)
	}
	// 对象存储不可用时仍返回暂存对象，让文件页可见。
	return primaryItems, nil
}

func (f *FallbackStore) SignedURL(ctx context.Context, filePath string, expire time.Duration) (string, error) {
	if f.stage != nil {
		if exists, err := f.stage.Exists(ctx, filePath); err == nil && exists {
			return "", ErrStaged
		}
	}
	if f.primary == nil {
		return "", ErrUnavailable
	}
	pctx, cancel := f.primaryCtx(ctx)
	defer cancel()
	return f.primary.SignedURL(pctx, filePath, expire)
}

func (f *FallbackStore) Close() error {
	var firstErr error
	if f.primary != nil {
		firstErr = f.primary.Close()
	}
	if f.stage != nil {
		if err := f.stage.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// StagedCount 返回暂存对象数量。
func (f *FallbackStore) StagedCount(ctx context.Context) (int, error) {
	if f.stage == nil {
		return 0, nil
	}
	return f.stage.Count(ctx)
}

// ListStaged 返回暂存对象列表。
func (f *FallbackStore) ListStaged(ctx context.Context) ([]FileInfo, error) {
	if f.stage == nil {
		return []FileInfo{}, nil
	}
	return f.stage.List(ctx)
}

// Flush 把 Redis 暂存对象提交到对象存储；失败的对象继续保留在 Redis。
func (f *FallbackStore) Flush(ctx context.Context) (int, error) {
	if f.primary == nil {
		return 0, ErrUnavailable
	}
	if f.stage == nil {
		return 0, nil
	}
	f.flushMu.Lock()
	defer f.flushMu.Unlock()

	items, err := f.stage.List(ctx)
	if err != nil {
		return 0, err
	}
	flushed := 0
	for _, item := range items {
		reader, info, err := f.stage.Download(ctx, item.Path)
		if err != nil {
			if errors.Is(err, errStageNotFound) {
				continue
			}
			return flushed, err
		}
		var opts []UploadOption
		if info.ContentType != "" {
			opts = append(opts, WithContentType(info.ContentType))
		}
		if len(info.Metadata) > 0 {
			opts = append(opts, WithMetadata(info.Metadata))
		}
		pctx, cancel := f.primaryCtx(ctx)
		_, uploadErr := f.primary.Upload(pctx, item.Path, reader, opts...)
		cancel()
		_ = reader.Close()
		if uploadErr != nil {
			return flushed, uploadErr
		}
		if err := f.stage.Delete(ctx, item.Path); err != nil {
			return flushed, err
		}
		flushed++
	}
	return flushed, nil
}

// StartFlushLoop 启动后台提交任务；onError 用于记录失败而不阻塞启动。
func (f *FallbackStore) StartFlushLoop(ctx context.Context, interval time.Duration, onError func(error)) {
	if f.primary == nil || f.stage == nil {
		return
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				count, err := f.stagedCountSafe(ctx)
				if err != nil || count == 0 {
					continue
				}
				if _, err := f.Flush(ctx); err != nil && onError != nil {
					onError(err)
				}
			}
		}
	}()
}

func (f *FallbackStore) stagedCountSafe(ctx context.Context) (int, error) {
	if f.stage == nil {
		return 0, nil
	}
	return f.stage.Count(ctx)
}
