package goodsercatalog

import (
	"fmt"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/middleware"
	"github.com/Allinost/go-backend-core/internal/modules"
	coreAuth "github.com/Allinost/go-backend-core/internal/modules/auth"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/services/storage"
	"github.com/gin-gonic/gin"
)

// Module Goodser 谷子图鉴模块（PG-only，独立于 V2 MySQL Goodser）
type Module struct {
	h      *Handler
	svc    *Service
	store  storage.Storage
	authMW gin.HandlerFunc
}

func (m *Module) Name() string { return "goodser-catalog" }

func (m *Module) Init(cfg *config.Config) error {
	pg := database.DB.Postgres["main"]
	if pg == nil || pg.SQLDB == nil {
		return fmt.Errorf("goodser-catalog: PostgreSQL 连接不可用")
	}

	if client, ok := database.DB.RustFS["rustfs"]; ok {
		m.store = storage.NewS3StoreFromClient(client.Client, client.Bucket)
		logger.Info().Msg("goodser-catalog: 使用 RustFS 作为图鉴图片存储")
	} else {
		logger.Warn().Msg("goodser-catalog: RustFS 不可用，图鉴上传将不可用（可先用 image_url）")
	}

	repo := NewRepository(pg.SQLDB)
	m.svc = NewService(repo, m.store)
	m.h = NewHandler(m.svc)

	authMod := modules.Get("auth")
	if authMod == nil {
		return fmt.Errorf("goodser-catalog: auth 模块未注册")
	}
	am, ok := authMod.(*coreAuth.Module)
	if !ok {
		return fmt.Errorf("goodser-catalog: auth 模块类型异常")
	}
	m.authMW = am.APIKeyOrJWTMiddleware()
	return nil
}

func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	r.Use(m.authMW)
	read := middleware.RequireScope(middleware.ScopeOwnerRead)
	write := middleware.RequireScope(middleware.ScopeOwnerWrite)

	r.GET("/catalogs", read, m.h.ListCatalogs)
	r.POST("/catalogs", write, m.h.CreateCatalog)
	r.GET("/catalogs/:id", read, m.h.GetCatalog)
	r.PUT("/catalogs/:id", write, m.h.UpdateCatalog)
	r.DELETE("/catalogs/:id", write, m.h.DeleteCatalog)
	r.POST("/catalogs/:id/images", write, m.h.AddImage)
	r.POST("/catalogs/:id/items", write, m.h.CreateItem)
	r.POST("/catalog-images/:id/segment", write, m.h.SegmentImage)
	r.PUT("/catalog-items/:id", write, m.h.UpdateItem)
	r.DELETE("/catalog-items/:id", write, m.h.DeleteItem)
}

func (m *Module) Close() error { return nil }

var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)
