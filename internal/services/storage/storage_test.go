package storage

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryStore_UploadDownload(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	info, err := s.Upload(ctx, "/test/hello.txt", strings.NewReader("world"))
	require.NoError(t, err)
	assert.Equal(t, "hello.txt", info.Name)
	assert.Equal(t, int64(5), info.Size)

	rc, err := s.Download(ctx, "/test/hello.txt")
	require.NoError(t, err)
	defer rc.Close()

	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "world", string(data))
}

func TestMemoryStore_UploadWithOptions(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	info, err := s.Upload(ctx, "/test/file.bin", bytes.NewReader([]byte{1, 2, 3}),
		WithContentType("application/octet-stream"),
		WithMetadata(map[string]string{"key": "val"}))
	require.NoError(t, err)
	assert.Equal(t, "application/octet-stream", info.ContentType)
}

func TestMemoryStore_DownloadNotFound(t *testing.T) {
	s := NewMemoryStore()
	_, err := s.Download(context.Background(), "/not/found")
	assert.Error(t, err)
}

func TestMemoryStore_Delete(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	_, err := s.Upload(ctx, "/tmp/test.txt", strings.NewReader("data"))
	require.NoError(t, err)

	err = s.Delete(ctx, "/tmp/test.txt")
	require.NoError(t, err)

	exists, err := s.Exists(ctx, "/tmp/test.txt")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestMemoryStore_DeleteNotFound(t *testing.T) {
	s := NewMemoryStore()
	err := s.Delete(context.Background(), "/not/found")
	assert.Error(t, err)
}

func TestMemoryStore_Exists(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	exists, err := s.Exists(ctx, "/missing")
	require.NoError(t, err)
	assert.False(t, exists)

	_, err = s.Upload(ctx, "/exists.txt", strings.NewReader("x"))
	require.NoError(t, err)

	exists, err = s.Exists(ctx, "/exists.txt")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestMemoryStore_List(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	s.Upload(ctx, "/a/1.txt", strings.NewReader("1"))
	s.Upload(ctx, "/a/2.txt", strings.NewReader("2"))
	s.Upload(ctx, "/b/3.txt", strings.NewReader("3"))

	files, err := s.List(ctx, "/a")
	require.NoError(t, err)
	assert.Len(t, files, 2)

	files, err = s.List(ctx, "/")
	require.NoError(t, err)
	assert.Len(t, files, 3)
}

func TestMemoryStore_ListEmpty(t *testing.T) {
	s := NewMemoryStore()
	files, err := s.List(context.Background(), "/")
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestMemoryStore_SignedURL(t *testing.T) {
	s := NewMemoryStore()
	_, err := s.SignedURL(context.Background(), "/test", 0)
	assert.Error(t, err)
}

func TestMemoryStore_Close(t *testing.T) {
	s := NewMemoryStore()
	s.Upload(context.Background(), "/x", strings.NewReader("data"))
	require.NoError(t, s.Close())

	exists, err := s.Exists(context.Background(), "/x")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestWithOffsetLimit(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		s.Upload(ctx, "/list/file", strings.NewReader("x"))
	}
	s.Upload(ctx, "/list/file0", strings.NewReader("a"))
	s.Upload(ctx, "/list/file1", strings.NewReader("b"))
	s.Upload(ctx, "/list/file2", strings.NewReader("c"))

	files, err := s.List(ctx, "/list", WithOffset(1), WithLimit(1))
	require.NoError(t, err)
	assert.Len(t, files, 1)
}
