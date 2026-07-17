package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"
)

// FileInfo 文件元信息
type FileInfo struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	ContentType  string    `json:"content_type,omitempty"`
	LastModified time.Time `json:"last_modified"`
	ETag         string    `json:"etag,omitempty"`
}

// ListOptions 文件列表查询选项
type ListOptions struct {
	Prefix    string
	Offset    int
	Limit     int
	Recursive bool
}

// Storage 文件存储抽象接口，支持上传、下载、删除、列表、签名 URL
type Storage interface {
	Upload(ctx context.Context, filePath string, reader io.Reader, opts ...UploadOption) (*FileInfo, error)
	Download(ctx context.Context, filePath string) (io.ReadCloser, error)
	Delete(ctx context.Context, filePath string) error
	List(ctx context.Context, prefix string, opts ...ListOption) ([]FileInfo, error)
	Exists(ctx context.Context, filePath string) (bool, error)
	SignedURL(ctx context.Context, filePath string, expire time.Duration) (string, error)
	Close() error
}

// UploadOptions 上传选项
type UploadOptions struct {
	ContentType string
	Metadata    map[string]string
}

type UploadOption func(*UploadOptions)

func WithContentType(ct string) UploadOption {
	return func(o *UploadOptions) { o.ContentType = ct }
}

func WithMetadata(md map[string]string) UploadOption {
	return func(o *UploadOptions) { o.Metadata = md }
}

type ListOption func(*ListOptions)

func WithOffset(offset int) ListOption {
	return func(o *ListOptions) { o.Offset = offset }
}

func WithLimit(limit int) ListOption {
	return func(o *ListOptions) { o.Limit = limit }
}

func WithRecursive(recursive bool) ListOption {
	return func(o *ListOptions) { o.Recursive = recursive }
}

// MemoryStore 内存文件存储，适用于开发和测试
type MemoryStore struct {
	files map[string]*memFile
}

type memFile struct {
	data []byte
	info FileInfo
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		files: make(map[string]*memFile),
	}
}

// Upload 上传文件到内存
func (m *MemoryStore) Upload(ctx context.Context, filePath string, reader io.Reader, opts ...UploadOption) (*FileInfo, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("storage: read failed: %w", err)
	}
	var o UploadOptions
	for _, opt := range opts {
		opt(&o)
	}
	info := &FileInfo{
		Name:         path.Base(filePath),
		Path:         filePath,
		Size:         int64(len(data)),
		ContentType:  o.ContentType,
		LastModified: time.Now(),
	}
	m.files[filePath] = &memFile{data: data, info: *info}
	return info, nil
}

// Download 从内存下载文件
func (m *MemoryStore) Download(ctx context.Context, filePath string) (io.ReadCloser, error) {
	f, ok := m.files[filePath]
	if !ok {
		return nil, fmt.Errorf("storage: file %s not found", filePath)
	}
	return io.NopCloser(bytes.NewReader(f.data)), nil
}

// Delete 从内存删除文件
func (m *MemoryStore) Delete(ctx context.Context, filePath string) error {
	if _, ok := m.files[filePath]; !ok {
		return fmt.Errorf("storage: file %s not found", filePath)
	}
	delete(m.files, filePath)
	return nil
}

// List 按前缀列出文件，支持分页
func (m *MemoryStore) List(ctx context.Context, prefix string, opts ...ListOption) ([]FileInfo, error) {
	var o ListOptions
	for _, opt := range opts {
		opt(&o)
	}
	var result []FileInfo
	for _, f := range m.files {
		if strings.HasPrefix(f.info.Path, prefix) {
			result = append(result, f.info)
		}
	}
	if o.Offset > 0 && o.Offset < len(result) {
		result = result[o.Offset:]
	}
	if o.Limit > 0 && o.Limit < len(result) {
		result = result[:o.Limit]
	}
	if result == nil {
		result = []FileInfo{}
	}
	return result, nil
}

func (m *MemoryStore) Exists(ctx context.Context, filePath string) (bool, error) {
	_, ok := m.files[filePath]
	return ok, nil
}

// SignedURL 内存存储不支持预签名 URL
func (m *MemoryStore) SignedURL(ctx context.Context, filePath string, expire time.Duration) (string, error) {
	return "", fmt.Errorf("storage: memory store does not support signed URLs")
}

func (m *MemoryStore) Close() error {
	m.files = make(map[string]*memFile)
	return nil
}
