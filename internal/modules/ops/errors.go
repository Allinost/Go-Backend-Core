package ops

import appErr "github.com/Allinost/go-backend-core/internal/pkg/errors"

const (
	ErrCodeStorageNotAvailable = 30001
	ErrCodeFileNotFound        = 30002
	ErrCodeScanFailed          = 30003
	ErrCodeCleanupFailed       = 30004
)

var (
	ErrStorageNotAvailable = appErr.New(ErrCodeStorageNotAvailable, "存储服务不可用")
	ErrFileNotFound        = appErr.New(ErrCodeFileNotFound, "文件不存在")
	ErrScanFailed          = appErr.New(ErrCodeScanFailed, "文件扫描失败")
	ErrCleanupFailed       = appErr.New(ErrCodeCleanupFailed, "文件清理失败")
)
