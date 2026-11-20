package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type flakyStore struct {
	*MemoryStore
	fail bool
}

var errPrimaryDown = errors.New("primary down")

func (f *flakyStore) Upload(ctx context.Context, filePath string, reader io.Reader, opts ...UploadOption) (*FileInfo, error) {
	if f.fail {
		return nil, errPrimaryDown
	}
	return f.MemoryStore.Upload(ctx, filePath, reader, opts...)
}

func (f *flakyStore) Download(ctx context.Context, filePath string) (io.ReadCloser, error) {
	if f.fail {
		return nil, errPrimaryDown
	}
	return f.MemoryStore.Download(ctx, filePath)
}

func (f *flakyStore) Stat(ctx context.Context, filePath string) (*FileInfo, error) {
	if f.fail {
		return nil, errPrimaryDown
	}
	return f.MemoryStore.Stat(ctx, filePath)
}

func (f *flakyStore) Delete(ctx context.Context, filePath string) error {
	if f.fail {
		return errPrimaryDown
	}
	return f.MemoryStore.Delete(ctx, filePath)
}

func (f *flakyStore) List(ctx context.Context, prefix string, opts ...ListOption) ([]FileInfo, error) {
	if f.fail {
		return nil, errPrimaryDown
	}
	return f.MemoryStore.List(ctx, prefix, opts...)
}

func (f *flakyStore) Exists(ctx context.Context, filePath string) (bool, error) {
	if f.fail {
		return false, errPrimaryDown
	}
	return f.MemoryStore.Exists(ctx, filePath)
}

func (f *flakyStore) SignedURL(ctx context.Context, filePath string, expire time.Duration) (string, error) {
	if f.fail {
		return "", errPrimaryDown
	}
	return f.MemoryStore.SignedURL(ctx, filePath, expire)
}

func newTestStage(t *testing.T) (*RedisStage, func()) {
	t.Helper()
	server, err := miniredis.Run()
	require.NoError(t, err)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	stage := NewRedisStage(client, time.Minute)
	return stage, func() {
		_ = client.Close()
		server.Close()
	}
}

func TestFallbackStore_StagesAndFlushes(t *testing.T) {
	stage, cleanup := newTestStage(t)
	defer cleanup()

	primary := &flakyStore{MemoryStore: NewMemoryStore(), fail: true}
	store := NewFallbackStore(primary, stage, 1<<20)
	ctx := context.Background()

	info, err := store.Upload(ctx, "images/a.png", strings.NewReader("hello"), WithContentType("image/png"))
	require.NoError(t, err)
	assert.True(t, info.Staged)
	assert.Equal(t, int64(5), info.Size)

	stagedInfo, err := store.Stat(ctx, "images/a.png")
	require.NoError(t, err)
	assert.True(t, stagedInfo.Staged)
	assert.Equal(t, "image/png", stagedInfo.ContentType)

	reader, err := store.Download(ctx, "images/a.png")
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	assert.Equal(t, "hello", string(data))

	files, err := store.List(ctx, "images/")
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.True(t, files[0].Staged)

	count, err := store.StagedCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// 对象存储恢复后提交。
	primary.fail = false
	flushed, err := store.Flush(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, flushed)

	count, err = store.StagedCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	remote, err := primary.Stat(ctx, "images/a.png")
	require.NoError(t, err)
	assert.False(t, remote.Staged)

	reader, err = store.Download(ctx, "images/a.png")
	require.NoError(t, err)
	data, err = io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	assert.Equal(t, "hello", string(data))
}

func TestFallbackStore_NoBackends(t *testing.T) {
	store := NewFallbackStore(nil, nil, 1<<20)
	ctx := context.Background()

	_, err := store.Upload(ctx, "images/a.png", strings.NewReader("hello"))
	assert.ErrorIs(t, err, ErrUnavailable)

	_, err = store.Download(ctx, "images/a.png")
	assert.ErrorIs(t, err, ErrUnavailable)

	_, err = store.Stat(ctx, "images/a.png")
	assert.ErrorIs(t, err, ErrUnavailable)

	_, err = store.Exists(ctx, "images/a.png")
	assert.ErrorIs(t, err, ErrUnavailable)

	_, err = store.List(ctx, "images/")
	assert.ErrorIs(t, err, ErrUnavailable)
}

func TestFallbackStore_OnlyStage(t *testing.T) {
	stage, cleanup := newTestStage(t)
	defer cleanup()

	store := NewFallbackStore(nil, stage, 1<<20)
	ctx := context.Background()

	info, err := store.Upload(ctx, "images/b.png", strings.NewReader("only-redis"))
	require.NoError(t, err)
	assert.True(t, info.Staged)

	_, err = store.SignedURL(ctx, "images/b.png", time.Minute)
	assert.ErrorIs(t, err, ErrStaged)

	_, err = store.Flush(ctx)
	assert.ErrorIs(t, err, ErrUnavailable)
}

func TestFallbackStore_StageLimit(t *testing.T) {
	stage, cleanup := newTestStage(t)
	defer cleanup()

	primary := &flakyStore{MemoryStore: NewMemoryStore(), fail: true}
	store := NewFallbackStore(primary, stage, 4)
	ctx := context.Background()

	_, err := store.Upload(ctx, "images/big.png", bytes.NewReader([]byte("12345")))
	assert.ErrorIs(t, err, ErrUnavailable)

	count, err := store.StagedCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestFallbackStore_NotFound(t *testing.T) {
	stage, cleanup := newTestStage(t)
	defer cleanup()

	store := NewFallbackStore(NewMemoryStore(), stage, 1<<20)
	_, err := store.Stat(context.Background(), "images/missing.png")
	assert.ErrorIs(t, err, ErrNotFound)
}
