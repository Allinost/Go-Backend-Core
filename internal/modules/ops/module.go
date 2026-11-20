package ops

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/minio/minio-go/v7"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/middleware"
	"github.com/Allinost/go-backend-core/internal/modules"
	coreAuth "github.com/Allinost/go-backend-core/internal/modules/auth"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	svcAuth "github.com/Allinost/go-backend-core/internal/services/auth"
	cryptoSvc "github.com/Allinost/go-backend-core/internal/services/crypto"
	"github.com/Allinost/go-backend-core/internal/services/fileref"
	"github.com/Allinost/go-backend-core/internal/services/storage"
	"github.com/gin-gonic/gin"
)

// Module 运维管理模块
type Module struct {
	svc     *Service
	h       *Handler
	refSvc  *fileref.Service
	maint   *MaintenanceService
	maintH  *MaintenanceHandler
	hostSvc *HostService
	hostH   *HostHandler
	authMW  gin.HandlerFunc
	rbac    *svcAuth.RBACService
}

func (m *Module) Name() string {
	return "ops"
}

func (m *Module) Init(cfg *config.Config) error {
	// 获取任意可用 S3 客户端
	var s3Client *minio.Client
	var s3Bucket string
	var s3Endpoint string
	var s3UseSSL bool
	var store storage.Storage

	if c, ok := database.DB.RustFS["rustfs"]; ok {
		s3Cfg := cfg.Database.S3["rustfs"]
		s3Client = c.Client
		s3Bucket = c.Bucket
		s3Endpoint = s3Cfg.Endpoint
		s3UseSSL = s3Cfg.UseSSL
		store = storage.NewS3StoreFromClient(c.Client, c.Bucket)
		logger.Info().Str("endpoint", s3Endpoint).Msg("ops: 使用 RustFS 作为存储后端")
	} else if c, ok := database.DB.S3["minio"]; ok {
		s3Cfg := cfg.Database.S3["minio"]
		s3Client = c.Client
		s3Bucket = c.Bucket
		s3Endpoint = s3Cfg.Endpoint
		s3UseSSL = s3Cfg.UseSSL
		store = storage.NewS3StoreFromClient(c.Client, c.Bucket)
		logger.Warn().Str("endpoint", s3Endpoint).Msg("ops: RustFS 不可用，使用 MinIO 作为存储后端")
	} else {
		return fmt.Errorf("ops: 没有可用的 S3 存储")
	}

	// V3 使用 PostgreSQL 主库
	var sqlDB *sql.DB
	if pg := database.DB.Postgres["main"]; pg != nil && pg.SQLDB != nil {
		sqlDB = pg.SQLDB
		logger.Info().Msg("ops: 使用 PostgreSQL 连接池")
	}
	if sqlDB == nil {
		logger.Warn().Msg("ops: PostgreSQL 连接不可用，扫描/引用功能将降级")
	} else {
		// 初始化文件引用服务
		refStore := fileref.NewPostgresStore(sqlDB)
		m.refSvc = fileref.NewService(refStore)
		if err := m.refSvc.Init(context.Background()); err != nil {
			logger.Warn().Err(err).Msg("ops: 文件引用服务初始化失败，降级运行")
			m.refSvc = nil
		} else {
			logger.Info().Msg("ops: 文件引用服务初始化完成")
		}
	}

	if sqlDB != nil {
		m.maint = NewMaintenanceService(NewMaintenanceRepository(sqlDB))
		m.maintH = NewMaintenanceHandler(m.maint)
		var aes *cryptoSvc.AESGCM
		if cfg.Crypto.MasterKeyHex != "" {
			if cipher, err := cryptoSvc.NewAESGCMFromHex(cfg.Crypto.MasterKeyHex); err != nil {
				logger.Warn().Err(err).Msg("ops: 主机凭据加密器初始化失败")
			} else {
				aes = cipher
			}
		}
		m.hostSvc = NewHostService(NewHostRepository(sqlDB), aes, cfg.Ops.SSHKnownHostsEnabled, cfg.Ops.SSHKnownHostsPath)
		m.hostH = NewHostHandler(m.hostSvc)
	}

	if authMod := modules.Get("auth"); authMod != nil {
		if am, ok := authMod.(*coreAuth.Module); ok {
			m.authMW = am.AuthMiddleware()
			m.rbac = am.RBAC()
			if err := SeedMaintenancePermissions(m.rbac); err != nil {
				logger.Warn().Err(err).Msg("ops: 维护权限初始化失败")
			}
			if err := SeedHostPermissions(m.rbac); err != nil {
				logger.Warn().Err(err).Msg("ops: 主机权限初始化失败")
			}
		}
	}
	if m.authMW == nil {
		logger.Warn().Msg("ops: auth 模块不可用，维护 API 未启用")
	}

	m.svc = NewService(sqlDB, s3Client, s3Bucket, store, s3Endpoint, s3UseSSL, m.refSvc)
	m.h = NewHandler(m.svc)

	logger.Info().Str("endpoint", s3Endpoint).Str("bucket", s3Bucket).Msg("ops: 模块初始化完成")
	return nil
}

func (m *Module) Close() error {
	return nil
}

func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	// 服务诊断
	r.GET("/status", m.h.Status)
	r.POST("/reinit", m.h.ReinitTables)

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

	// L1/L2 手动维护
	if m.maintH != nil && m.authMW != nil {
		need := func(action string) gin.HandlerFunc {
			return middleware.RequirePermission(m.rbac, "ops.maintenance", action)
		}
		maint := r.Group("/maintenance")
		maint.Use(m.authMW)
		maint.GET("", need("read"), m.maintH.Status)
		maint.POST("/start", need("start"), m.maintH.Start)
		maint.POST("/end", need("end"), m.maintH.End)
		maint.GET("/history", need("read"), m.maintH.History)
	}

	// 主机台账 / SSH 连通
	if m.hostH != nil && m.authMW != nil {
		need := func(action string) gin.HandlerFunc {
			return middleware.RequirePermission(m.rbac, "ops.host", action)
		}
		hosts := r.Group("/hosts")
		hosts.Use(m.authMW)
		hosts.GET("", need("read"), m.hostH.List)
		hosts.POST("", need("manage"), m.hostH.Create)
		hosts.GET("/:id", need("read"), m.hostH.Get)
		hosts.PUT("/:id", need("manage"), m.hostH.Update)
		hosts.DELETE("/:id", need("manage"), m.hostH.Delete)
		hosts.POST("/:id/test", need("connect"), m.hostH.Test)
		hosts.GET("/:id/status", need("connect"), m.hostH.Status)
	}
}

var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)
