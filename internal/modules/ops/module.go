package ops

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/services/fileref"
	"github.com/Allinost/go-backend-core/internal/services/storage"
	"github.com/gin-gonic/gin"
)

// Module 运维管理模块
type Module struct {
	svc    *Service
	h      *Handler
	refSvc *fileref.Service
}

func (m *Module) Name() string {
	return "ops"
}

func (m *Module) Init(cfg *config.Config) error {
	// 获取 RustFS 连接（优先）
	rustfsClient, ok := database.DB.RustFS["rustfs"]
	if !ok {
		logger.Warn().Msg("ops: RustFS 不可用，尝试使用 MinIO")
		// 回退到 MinIO
		if client, ok2 := database.DB.S3["minio"]; ok2 {
			s3Cfg, ok3 := cfg.Database.S3["minio"]
			if !ok3 {
				return fmt.Errorf("ops: MinIO 配置不存在")
			}
			endpoint := s3Cfg.Endpoint
			useSSL := s3Cfg.UseSSL
			store := storage.NewS3StoreFromClient(client.Client, client.Bucket)
			m.svc = NewService(nil, client.Client, client.Bucket, store, endpoint, useSSL, nil)
			logger.Info().Msg("ops: 使用 MinIO 作为存储后端")
			m.h = NewHandler(m.svc)
			return nil
		}
		return fmt.Errorf("ops: 没有可用的 S3 存储")
	}

	s3Cfg, ok := cfg.Database.S3["rustfs"]
	if !ok {
		return fmt.Errorf("ops: RustFS 配置不存在")
	}

	// 获取 MySQL 连接（用于扫描）
	var pool = database.DB.MySQL["main"]
	if pool == nil {
		pool = database.DB.MySQL["goodser"]
	}

	var sqlDB *sql.DB
	if pool != nil {
		sqlDB = pool.DB
	}

	store := storage.NewS3StoreFromClient(rustfsClient.Client, rustfsClient.Bucket)

	// 初始化文件引用服务
	if sqlDB != nil {
		refStore := fileref.NewMySQLStore(sqlDB)
		m.refSvc = fileref.NewService(refStore)
		if err := m.refSvc.Init(context.Background()); err != nil {
			logger.Warn().Err(err).Msg("ops: 文件引用服务初始化失败，降级运行")
			m.refSvc = nil
		} else {
			logger.Info().Msg("ops: 文件引用服务初始化完成")
		}
	}

	m.svc = NewService(sqlDB, rustfsClient.Client, rustfsClient.Bucket, store, s3Cfg.Endpoint, s3Cfg.UseSSL, m.refSvc)
	m.h = NewHandler(m.svc)

	logger.Info().Str("endpoint", s3Cfg.Endpoint).Str("bucket", rustfsClient.Bucket).Msg("ops: 模块初始化完成")
	return nil
}

func (m *Module) Close() error {
	return nil
}

func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	// RustFS 文件操作
	rustfs := r.Group("/rustfs")
	rustfs.GET("/files", m.h.ListFiles)
	rustfs.GET("/info", m.h.GetFileInfo)
	rustfs.GET("/url", m.h.GetSignedURL)
	rustfs.GET("/download", m.h.DownloadFile)
	rustfs.POST("/upload", m.h.UploadFile)
	rustfs.DELETE("/delete", m.h.DeleteFile)
	rustfs.POST("/batch-delete", m.h.BatchDelete)

	// RustFS 运维管理
	rustfs.GET("/scan", m.h.ScanFileUsage)
	rustfs.POST("/cleanup", m.h.CleanupOrphans)
	rustfs.GET("/stats", m.h.BucketStats)

	// 文件引用管理（跨模块，全局限用）
	ref := r.Group("/references")
	ref.POST("", m.h.RegisterReferences)
	ref.DELETE("/:id", m.h.RemoveReference)
	ref.GET("", m.h.ListReferences)
	ref.GET("/stats", m.h.ReferenceStats)
	ref.POST("/sync", m.h.SyncReferences)

	// 扫描目标管理
	st := r.Group("/scan-targets")
	st.GET("", m.h.ListScanTargets)
	st.POST("", m.h.CreateScanTarget)
	st.PUT("/:id", m.h.UpdateScanTarget)
	st.DELETE("/:id", m.h.DeleteScanTarget)
}

var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)
