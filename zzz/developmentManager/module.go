// Package developmentmanager 项目开发管理模块：项目台账（基座/模板/模块）、部署资产、
// 数据字典、需求与生成流程的可视化管理后端（规划见 docs/plans/feature-V2-project-dev-management.md）。
package developmentmanager

import (
	"fmt"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/middleware"
	"github.com/Allinost/go-backend-core/internal/modules"
	coreAuth "github.com/Allinost/go-backend-core/internal/modules/auth"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	svcAuth "github.com/Allinost/go-backend-core/internal/services/auth"
	"github.com/gin-gonic/gin"
)

// Module 项目开发管理模块，实现 modules.Module 接口
type Module struct {
	cfg    *config.Config
	repo   *Repository
	svc    *Service
	h      *Handler
	authMW gin.HandlerFunc
	rbac   *svcAuth.RBACService
}

// Name 返回模块名称，用于路由前缀 /api/v1/developmentmanager/*
func (m *Module) Name() string {
	return "developmentmanager"
}

// Init 初始化模块：取 MySQL 连接、装配鉴权、幂等播种权限点、启动台账 seed
func (m *Module) Init(cfg *config.Config) error {
	m.cfg = cfg

	pool := database.DB.MySQL["main"]
	if pool == nil {
		return fmt.Errorf("developmentmanager: MySQL 连接不可用")
	}
	m.repo = NewRepository(pool.DB)

	// 依赖 auth 模块（main.go 注册顺序保证 auth 先于本模块初始化）
	authMod := modules.Get("auth")
	if authMod == nil {
		return fmt.Errorf("developmentmanager: auth 模块未注册")
	}
	am, ok := authMod.(*coreAuth.Module)
	if !ok {
		return fmt.Errorf("developmentmanager: auth 模块类型异常")
	}
	m.authMW = am.AuthMiddleware()
	m.rbac = am.RBAC()

	// 幂等播种权限点与默认角色（失败则启动失败，保证接口不裸奔）
	if err := SeedPermissions(m.rbac); err != nil {
		return err
	}

	m.svc = NewService(m.repo)
	m.h = NewHandler(m.svc)

	// 台账 seed 幂等登记现有项目（迁移未应用时后台重试）
	go m.seedWithRetry()

	logger.Info().Msg("developmentmanager: 模块初始化完成")
	return nil
}

// Close 关闭模块资源
func (m *Module) Close() error {
	return nil
}

// GetService 暴露 Service 实例，供其他模块调用
func (m *Module) GetService() *Service {
	return m.svc
}

// RegisterRoutes 注册模块路由到 /api/v1/developmentmanager/*（全组要求登录，按路由挂载资源×动作权限）
func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	r.Use(m.authMW)
	need := func(resource, action string) gin.HandlerFunc {
		return middleware.RequirePermission(m.rbac, resource, action)
	}

	// 项目台账
	r.GET("/projects", need("dm.project", "read"), m.h.ListProjects)
	r.POST("/projects", need("dm.project", "create"), m.h.CreateProject)
	r.GET("/projects/:id", need("dm.project", "read"), m.h.GetProject)
	r.PUT("/projects/:id", need("dm.project", "update"), m.h.UpdateProject)
	r.DELETE("/projects/:id", need("dm.project", "delete"), m.h.DeleteProject)

	// 项目关联
	r.GET("/projects/:id/relations", need("dm.relation", "read"), m.h.ListRelations)
	r.POST("/projects/:id/relations", need("dm.relation", "create"), m.h.CreateRelation)
	r.DELETE("/projects/:id/relations/:rel_id", need("dm.relation", "delete"), m.h.DeleteRelation)

	// 归类资产
	r.GET("/projects/:id/assets", need("dm.asset", "read"), m.h.ListAssets)
	r.POST("/projects/:id/assets", need("dm.asset", "create"), m.h.CreateAsset)
	r.DELETE("/projects/:id/assets/:asset_id", need("dm.asset", "delete"), m.h.DeleteAsset)

	// 部署资产：容器
	r.GET("/projects/:id/containers", need("dm.deployment", "read"), m.h.ListContainers)
	r.POST("/projects/:id/containers", need("dm.deployment", "create"), m.h.CreateContainer)
	r.GET("/containers/:id", need("dm.deployment", "read"), m.h.GetContainer)
	r.PUT("/containers/:id", need("dm.deployment", "update"), m.h.UpdateContainer)
	r.DELETE("/containers/:id", need("dm.deployment", "delete"), m.h.DeleteContainer)

	// 部署资产：资源分配（容器 1:1）
	r.GET("/containers/:id/resources", need("dm.deployment", "read"), m.h.GetResourceAlloc)
	r.PUT("/containers/:id/resources", need("dm.deployment", "update"), m.h.UpsertResourceAlloc)

	// 部署资产：卷映射
	r.GET("/containers/:id/volumes", need("dm.deployment", "read"), m.h.ListVolumes)
	r.POST("/containers/:id/volumes", need("dm.deployment", "create"), m.h.CreateVolume)
	r.PUT("/volumes/:id", need("dm.deployment", "update"), m.h.UpdateVolume)
	r.DELETE("/volumes/:id", need("dm.deployment", "delete"), m.h.DeleteVolume)

	// 部署资产：网络接入点（端口+反代+地址+白名单引用）
	r.GET("/projects/:id/access-points", need("dm.deployment", "read"), m.h.ListAccessPoints)
	r.POST("/projects/:id/access-points", need("dm.deployment", "create"), m.h.CreateAccessPoint)
	r.GET("/access-points/:id", need("dm.deployment", "read"), m.h.GetAccessPoint)
	r.PUT("/access-points/:id", need("dm.deployment", "update"), m.h.UpdateAccessPoint)
	r.DELETE("/access-points/:id", need("dm.deployment", "delete"), m.h.DeleteAccessPoint)

	// 部署资产：白名单组与成员（组可复用、允许空组）
	r.GET("/projects/:id/whitelists", need("dm.deployment", "read"), m.h.ListWhitelists)
	r.POST("/projects/:id/whitelists", need("dm.deployment", "create"), m.h.CreateWhitelist)
	r.GET("/whitelists/:id", need("dm.deployment", "read"), m.h.GetWhitelist)
	r.PUT("/whitelists/:id", need("dm.deployment", "update"), m.h.UpdateWhitelist)
	r.DELETE("/whitelists/:id", need("dm.deployment", "delete"), m.h.DeleteWhitelist)
	r.GET("/whitelists/:id/members", need("dm.deployment", "read"), m.h.ListWhitelistMembers)
	r.POST("/whitelists/:id/members", need("dm.deployment", "create"), m.h.AddWhitelistMember)
	r.DELETE("/whitelists/:id/members/:member_id", need("dm.deployment", "delete"), m.h.RemoveWhitelistMember)

	// 部署资产：备份
	r.GET("/projects/:id/backups", need("dm.deployment", "read"), m.h.ListBackups)
	r.POST("/projects/:id/backups", need("dm.deployment", "create"), m.h.CreateBackup)
	r.GET("/backups/:id", need("dm.deployment", "read"), m.h.GetBackup)
	r.PUT("/backups/:id", need("dm.deployment", "update"), m.h.UpdateBackup)
	r.DELETE("/backups/:id", need("dm.deployment", "delete"), m.h.DeleteBackup)

	// 统计与权限元数据
	r.GET("/stats", need("dm.project", "read"), m.h.Stats)
	r.GET("/permissions/meta", need("dm.project", "read"), m.h.PermissionMeta)
}
