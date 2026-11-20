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
	"github.com/Allinost/go-backend-core/zzz/developmentManager/generation"
	"github.com/gin-gonic/gin"
)

// Module 项目开发管理模块，实现 modules.Module 接口
type Module struct {
	cfg    *config.Config
	repo   *Repository
	svc    *Service
	pm     *PMService
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

	pg := database.DB.Postgres["main"]
	if pg == nil || pg.SQLDB == nil {
		return fmt.Errorf("developmentmanager: PostgreSQL 连接不可用")
	}
	m.repo = NewRepository(pg.SQLDB)

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
	m.h.SetRBAC(m.rbac)

	// 里程碑/任务与旧台账统一使用 PostgreSQL。
	m.pm = NewPMService(NewPMRepository(pg.SQLDB))
	m.h.SetPM(m.pm)

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

// SetAIWriter 注入可选的 AI 生成器（未注入时 engine=ai 自动降级为 template）
func (m *Module) SetAIWriter(w generation.AIWriter) {
	if m.svc != nil {
		m.svc.SetAIWriter(w)
	}
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
	r.POST("/projects/:id/assets/expand", need("dm.asset", "create"), m.h.ExpandAsset)
	r.POST("/projects/:id/assets/expand-all", need("dm.asset", "create"), m.h.ExpandProjectAssets)
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

	// 数据字典：数据表与字段
	r.GET("/projects/:id/tables", need("dm.dictionary", "read"), m.h.ListTables)
	r.POST("/projects/:id/tables", need("dm.dictionary", "create"), m.h.CreateTable)
	r.GET("/tables/:id", need("dm.dictionary", "read"), m.h.GetTable)
	r.PUT("/tables/:id", need("dm.dictionary", "update"), m.h.UpdateTable)
	r.DELETE("/tables/:id", need("dm.dictionary", "delete"), m.h.DeleteTable)
	r.GET("/tables/:id/columns", need("dm.dictionary", "read"), m.h.ListColumns)
	r.POST("/tables/:id/columns", need("dm.dictionary", "create"), m.h.CreateColumn)
	r.POST("/tables/:id/scan-import", need("dm.dictionary", "scan"), m.h.ScanImportColumns)
	r.PUT("/columns/:id", need("dm.dictionary", "update"), m.h.UpdateColumn)
	r.DELETE("/columns/:id", need("dm.dictionary", "delete"), m.h.DeleteColumn)

	// 需求管理：需求与版本快照
	r.GET("/projects/:id/requirements", need("dm.requirement", "read"), m.h.ListRequirements)
	r.POST("/projects/:id/requirements", need("dm.requirement", "create"), m.h.CreateRequirement)
	r.GET("/requirements/:id", need("dm.requirement", "read"), m.h.GetRequirement)
	r.PUT("/requirements/:id", need("dm.requirement", "update"), m.h.UpdateRequirement)
	r.POST("/requirements/:id/status", need("dm.requirement", "approve"), m.h.ChangeRequirementStatus)
	r.DELETE("/requirements/:id", need("dm.requirement", "delete"), m.h.DeleteRequirement)
	r.GET("/requirements/:id/versions", need("dm.requirement", "read"), m.h.ListRequirementVersions)

	// 生成流程：方案/代码/文档（auto_commit 由 handler 二次校验 autocommit 权限）
	r.POST("/requirements/:id/generations", need("dm.generation", "execute"), m.h.CreateGeneration)
	r.GET("/requirements/:id/generations", need("dm.generation", "read"), m.h.ListGenerations)
	r.GET("/generations/:id", need("dm.generation", "read"), m.h.GetGeneration)
	r.POST("/generations/:id/retry", need("dm.generation", "execute"), m.h.RetryGeneration)

	// 里程碑/任务（V3 PM，PostgreSQL）
	r.GET("/projects/:id/milestones", need("dm.milestone", "read"), m.h.ListMilestones)
	r.POST("/projects/:id/milestones", need("dm.milestone", "create"), m.h.CreateMilestone)
	r.GET("/milestones/:id", need("dm.milestone", "read"), m.h.GetMilestone)
	r.PUT("/milestones/:id", need("dm.milestone", "update"), m.h.UpdateMilestone)
	r.DELETE("/milestones/:id", need("dm.milestone", "delete"), m.h.DeleteMilestone)
	r.GET("/milestones/:id/progress", need("dm.milestone", "read"), m.h.MilestoneProgress)

	r.GET("/projects/:id/tasks", need("dm.task", "read"), m.h.ListTasks)
	r.POST("/projects/:id/tasks", need("dm.task", "create"), m.h.CreateTask)
	r.POST("/projects/:id/tasks/import", need("dm.task", "import"), m.h.ImportTasks)
	r.GET("/tasks/:id", need("dm.task", "read"), m.h.GetTask)
	r.PUT("/tasks/:id", need("dm.task", "update"), m.h.UpdateTask)
	r.DELETE("/tasks/:id", need("dm.task", "delete"), m.h.DeleteTask)
	r.POST("/tasks/:id/status", need("dm.task", "update"), m.h.ChangeTaskStatus)
	r.GET("/tasks/:id/logs", need("dm.task", "read"), m.h.ListTaskLogs)
	r.POST("/tasks/:id/logs", need("dm.task", "update"), m.h.AddTaskLog)
	r.GET("/tasks/:id/dependencies", need("dm.task", "read"), m.h.ListDependencies)
	r.POST("/tasks/:id/dependencies", need("dm.task", "update"), m.h.AddDependency)
	r.DELETE("/tasks/:id/dependencies/:dep_id", need("dm.task", "update"), m.h.RemoveDependency)

	// 迭代/风险/发布/基线
	r.GET("/projects/:id/iterations", need("dm.iteration", "read"), m.h.ListIterations)
	r.POST("/projects/:id/iterations", need("dm.iteration", "create"), m.h.CreateIteration)
	r.GET("/iterations/:id", need("dm.iteration", "read"), m.h.GetIteration)
	r.PUT("/iterations/:id", need("dm.iteration", "update"), m.h.UpdateIteration)
	r.DELETE("/iterations/:id", need("dm.iteration", "delete"), m.h.DeleteIteration)

	r.GET("/projects/:id/risks", need("dm.risk", "read"), m.h.ListRisks)
	r.POST("/projects/:id/risks", need("dm.risk", "create"), m.h.CreateRisk)
	r.GET("/risks/:id", need("dm.risk", "read"), m.h.GetRisk)
	r.PUT("/risks/:id", need("dm.risk", "update"), m.h.UpdateRisk)
	r.DELETE("/risks/:id", need("dm.risk", "delete"), m.h.DeleteRisk)

	r.GET("/projects/:id/releases", need("dm.release", "read"), m.h.ListReleases)
	r.POST("/projects/:id/releases", need("dm.release", "create"), m.h.CreateRelease)
	r.GET("/releases/:id", need("dm.release", "read"), m.h.GetRelease)
	r.PUT("/releases/:id", need("dm.release", "update"), m.h.UpdateRelease)
	r.DELETE("/releases/:id", need("dm.release", "delete"), m.h.DeleteRelease)

	r.GET("/projects/:id/baselines", need("dm.baseline", "read"), m.h.ListBaselines)
	r.POST("/projects/:id/baselines", need("dm.baseline", "create"), m.h.CreateBaseline)
	r.GET("/baselines/:id", need("dm.baseline", "read"), m.h.GetBaseline)
	r.DELETE("/baselines/:id", need("dm.baseline", "delete"), m.h.DeleteBaseline)

	// 评论/检查项/标签/任务资产
	r.GET("/tasks/:id/comments", need("dm.comment", "read"), m.h.ListTaskComments)
	r.POST("/tasks/:id/comments", need("dm.comment", "create"), m.h.AddTaskComment)
	r.DELETE("/comments/:id", need("dm.comment", "delete"), m.h.DeleteComment)

	r.GET("/tasks/:id/checklist", need("dm.task", "read"), m.h.ListChecklist)
	r.POST("/tasks/:id/checklist", need("dm.task", "update"), m.h.CreateChecklistItem)
	r.PUT("/checklist/:id", need("dm.task", "update"), m.h.UpdateChecklistItem)
	r.DELETE("/checklist/:id", need("dm.task", "update"), m.h.DeleteChecklistItem)

	r.GET("/projects/:id/labels", need("dm.label", "read"), m.h.ListLabels)
	r.POST("/projects/:id/labels", need("dm.label", "create"), m.h.CreateLabel)
	r.DELETE("/labels/:id", need("dm.label", "delete"), m.h.DeleteLabel)

	r.GET("/tasks/:id/labels", need("dm.task", "read"), m.h.ListTaskLabels)
	r.POST("/tasks/:id/labels", need("dm.label", "create"), m.h.AddTaskLabel)
	r.DELETE("/tasks/:id/labels/:label_id", need("dm.label", "delete"), m.h.RemoveTaskLabel)

	r.GET("/tasks/:id/assets", need("dm.task", "read"), m.h.ListTaskAssets)
	r.POST("/tasks/:id/assets", need("dm.task", "update"), m.h.AddTaskAsset)
	r.DELETE("/tasks/:id/assets/:asset_id", need("dm.task", "update"), m.h.RemoveTaskAsset)

	// 仪表盘/看板/路线图/周报
	r.GET("/projects/:id/dashboard", need("dm.project", "read"), m.h.GetDashboard)
	r.GET("/projects/:id/board", need("dm.task", "read"), m.h.GetBoard)
	r.GET("/projects/:id/roadmap", need("dm.milestone", "read"), m.h.GetRoadmap)
	r.GET("/projects/:id/report/weekly", need("dm.project", "read"), m.h.GetWeeklyReport)

	// 统计与权限元数据
	r.GET("/stats", need("dm.project", "read"), m.h.Stats)
	r.GET("/permissions/meta", need("dm.project", "read"), m.h.PermissionMeta)
}
