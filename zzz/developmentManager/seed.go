package developmentmanager

import (
	"context"
	"errors"
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/logger"
)

// seedProjectEntry 台账初始项目（规划 §1.3：基座/模板/模块登记与关联）
type seedProjectEntry struct {
	Key         string
	Name        string
	Type        string
	RepoPath    string
	Description string
	TechStack   []string
}

var seedProjects = []seedProjectEntry{
	{Key: "go-backend-core", Name: "Go-Backend-Core", Type: ProjectTypeBase, RepoPath: ".",
		Description: "Go 后端基座工程（Gin + MySQL/Redis），业务模块以 zzz/ 目录挂载",
		TechStack:   []string{"Go", "Gin", "MySQL", "Redis"}},
	{Key: "nkui", Name: "NKUI", Type: ProjectTypeTemplate, RepoPath: "../NKUI",
		Description: "前端模板（Web/移动/小程序多端）", TechStack: []string{"Vue3", "NKUI"}},
	{Key: "go-frontend-core", Name: "Go-Frontend-Core", Type: ProjectTypeFrontendBase, RepoPath: "../Go-Frontend-Core",
		Description: "前端核心工程（NKUI 之上的基座），关联 NKUI 模板与 Go-Backend-Core 后端",
		TechStack:   []string{"Vue3", "TypeScript", "Vite"}},
	{Key: "local-compose", Name: "本地容器栈", Type: ProjectTypeContainer, RepoPath: "docker-compose.yml",
		Description: "docker-compose 部署栈（app + browserless chrome），容器管理页统一维护",
		TechStack:   []string{"Docker Compose"}},
	{Key: "goodser", Name: "商品服务", Type: ProjectTypeBackendModule, RepoPath: "zzz/goodser",
		Description: "商品/库存/出库业务模块"},
	{Key: "shopchecker", Name: "店铺采集", Type: ProjectTypeBackendModule, RepoPath: "zzz/shopchecker",
		Description: "商品价格监控与采集模块"},
	{Key: "login3rd", Name: "第三方登录", Type: ProjectTypeBackendModule, RepoPath: "zzz/login3rd",
		Description: "第三方账号登录模块"},
	{Key: "ia", Name: "投资分析", Type: ProjectTypeBackendModule, RepoPath: "zzz/ia",
		Description: "投资组合分析与行情模块"},
	{Key: "developmentmanager", Name: "项目开发管理", Type: ProjectTypeBackendModule, RepoPath: "zzz/developmentManager",
		Description: "项目开发管理模块（本模块）：台账/部署资产/数据字典/需求/生成"},
}

// seedRelations 台账初始关联（from → to）
var seedRelations = []struct {
	From, To, Type, Note string
}{
	{From: "goodser", To: "go-backend-core", Type: RelationUsesBase, Note: "业务模块挂载于后端基座"},
	{From: "shopchecker", To: "go-backend-core", Type: RelationUsesBase, Note: "业务模块挂载于后端基座"},
	{From: "login3rd", To: "go-backend-core", Type: RelationUsesBase, Note: "业务模块挂载于后端基座"},
	{From: "ia", To: "go-backend-core", Type: RelationUsesBase, Note: "业务模块挂载于后端基座"},
	{From: "developmentmanager", To: "go-backend-core", Type: RelationUsesBase, Note: "本模块所属基座"},
	{From: "go-frontend-core", To: "nkui", Type: RelationUsesTemplate, Note: "前端工程基于 NKUI 模板"},
	{From: "go-frontend-core", To: "go-backend-core", Type: RelationBindsBackendModule, Note: "前端工程对接后端基座 API"},
}

// seedAssets 台账初始资产索引（历史资料统一登记，幂等）
var seedAssets = []struct {
	Key      string
	Category string
	Title    string
	URI      string
	Format   string
	Note     string
}{
	// 项目开发管理
	{Key: "developmentmanager", Category: AssetCategoryDoc, Title: "V2 项目开发管理规划",
		URI: "docs/plans/feature-V2-project-dev-management.md", Format: "md", Note: "seed 登记"},
	{Key: "developmentmanager", Category: AssetCategoryDoc, Title: "项目开发管理模块文档",
		URI: "docs/modules/developmentManager/", Format: "dir", Note: "架构/数据模型/API/模块结构四件套"},

	// 后端基座：架构与规范
	{Key: "go-backend-core", Category: AssetCategoryDoc, Title: "数据模型与表前缀规范",
		URI: "docs/data-model.md", Format: "md", Note: "seed 登记"},
	{Key: "go-backend-core", Category: AssetCategoryDoc, Title: "文档索引",
		URI: "docs/README.md", Format: "md", Note: "seed 登记"},
	{Key: "go-backend-core", Category: AssetCategoryDoc, Title: "整体架构说明",
		URI: "docs/architecture.md", Format: "md"},
	{Key: "go-backend-core", Category: AssetCategoryDoc, Title: "技术选型与依赖",
		URI: "docs/tech-stack.md", Format: "md"},
	{Key: "go-backend-core", Category: AssetCategoryDoc, Title: "模块设计（含各模块设计）",
		URI: "docs/module-design.md", Format: "md"},
	{Key: "go-backend-core", Category: AssetCategoryDoc, Title: "部署方案",
		URI: "docs/deployment.md", Format: "md"},
	{Key: "go-backend-core", Category: AssetCategoryDoc, Title: "测试规范",
		URI: "docs/test.md", Format: "md"},
	{Key: "go-backend-core", Category: AssetCategoryDoc, Title: "开发规划与阶段安排",
		URI: "docs/development-plan.md", Format: "md"},
	{Key: "go-backend-core", Category: AssetCategoryDoc, Title: "第三方 OAuth 登录方案",
		URI: "docs/social-login.md", Format: "md"},
	{Key: "go-backend-core", Category: AssetCategoryDoc, Title: "第三方登录实施计划",
		URI: "docs/login3rd-plan.md", Format: "md"},
	{Key: "go-backend-core", Category: AssetCategoryDoc, Title: "API 文档索引（通用/s0/s1/s2）",
		URI: "docs/API/", Format: "dir"},
	{Key: "go-backend-core", Category: AssetCategoryDoc, Title: "发布记录（P0–P17 生产就绪清单）",
		URI: "docs/release/", Format: "dir"},
	{Key: "go-backend-core", Category: AssetCategoryDoc, Title: "周报与迭代总结",
		URI: "docs/weekly/", Format: "dir"},
	{Key: "go-backend-core", Category: AssetCategoryDoc, Title: "Jekyll 文档导航站",
		URI: "docs/docs-page/", Format: "dir"},
	{Key: "go-backend-core", Category: AssetCategoryDoc, Title: "服务导航模块文档",
		URI: "docs/modules/servicenav/", Format: "dir"},
	{Key: "go-backend-core", Category: AssetCategoryDoc, Title: "Webhook 模块文档",
		URI: "docs/modules/webhook/", Format: "dir"},

	// 业务模块文档
	{Key: "goodser", Category: AssetCategoryDoc, Title: "商品服务模块文档（含 Rust→Go 迁移）",
		URI: "docs/modules/goodser/dev/", Format: "dir"},
	{Key: "shopchecker", Category: AssetCategoryDoc, Title: "店铺采集模块文档",
		URI: "docs/modules/shopChecker/", Format: "dir"},
	{Key: "login3rd", Category: AssetCategoryDoc, Title: "第三方登录模块文档",
		URI: "docs/modules/login3rd/", Format: "dir"},
	{Key: "ia", Category: AssetCategoryDoc, Title: "投资分析模块文档",
		URI: "docs/modules/ia/", Format: "dir"},

	// 前端基座 / NKUI 模板 / 容器栈
	{Key: "go-frontend-core", Category: AssetCategoryDoc, Title: "前端工程说明与模块登记",
		URI: "AGENTS.md", Format: "md"},
	{Key: "nkui", Category: AssetCategoryDoc, Title: "NKUI 设计语言与组件规范",
		URI: "AllDesign/", Format: "dir", Note: "48 篇设计规范"},
	{Key: "nkui", Category: AssetCategoryDoc, Title: "NKUI 组件使用文档",
		URI: "AllDocs/", Format: "dir", Note: "204 篇组件文档"},
	{Key: "local-compose", Category: AssetCategoryCode, Title: "本地容器栈编排文件",
		URI: "docker-compose.yml", Format: "yml"},
}

// seedRequirement 历史需求（统一导入台账，状态为已完成/进行中）
type seedRequirement struct {
	Key      string
	Title    string
	Content  string
	Type     string
	Priority string
	Status   string
	Owner    string
}

// seedRequirements 历史需求清单（幂等：按项目 + 标题去重）
var seedRequirements = []seedRequirement{
	// 后端基座里程碑
	{Key: "go-backend-core", Title: "P0 发布前整改与生产就绪", Type: RequirementTypeOps, Priority: PriorityP1,
		Content: "日志、配置、缓存、事件总线、统一数据访问、密钥与 Secrets、监控、公开工具库等生产就绪整改。\n\n文档：`docs/release/P0-*`"},
	{Key: "go-backend-core", Title: "P1 基础设施：Docker、构建、缓存与发布流水线", Type: RequirementTypeOps, Priority: PriorityP2,
		Content: "Docker 部署配置、构建与自动化、缓存管理、发布流水线。\n\n文档：`docs/release/P1-*`"},
	{Key: "go-backend-core", Title: "P8 任务调度模块", Type: RequirementTypeFeature, Priority: PriorityP2,
		Content: "任务调度模块与执行日志。\n\n文档：`docs/release/P8-任务调度模块.md`"},
	{Key: "go-backend-core", Title: "P9 JWT 认证模块", Type: RequirementTypeFeature, Priority: PriorityP1,
		Content: "JWT 登录、刷新、黑名单与鉴权中间件。\n\n文档：`docs/release/P9-JWT认证模块.md`"},
	{Key: "go-backend-core", Title: "P10 用户管理", Type: RequirementTypeFeature, Priority: PriorityP2,
		Content: "用户 CRUD、状态与资料维护。\n\n文档：`docs/release/P10-用户管理.md`"},
	{Key: "go-backend-core", Title: "P11 API 密钥管理", Type: RequirementTypeFeature, Priority: PriorityP2,
		Content: "API Key 创建、吊销与鉴权。\n\n文档：`docs/release/P11-API密钥管理.md`"},
	{Key: "go-backend-core", Title: "P12 数据库迁移、导入导出与备份", Type: RequirementTypeFeature, Priority: PriorityP1,
		Content: "schema 迁移、表数据导入导出、备份与恢复。\n\n文档：`docs/release/P12-数据库迁移导入导出与备份.md`"},
	{Key: "go-backend-core", Title: "P13 加解密与压缩", Type: RequirementTypeFeature, Priority: PriorityP2,
		Content: "AES 加解密、压缩工具与前端加密联调。\n\n文档：`docs/release/P13-加解密与压缩.md`"},
	{Key: "go-backend-core", Title: "P14 网络通信", Type: RequirementTypeFeature, Priority: PriorityP2,
		Content: "HTTP/TCP/DNS 检测、代理与网络工具。\n\n文档：`docs/release/P14-网络通信.md`"},
	{Key: "go-backend-core", Title: "P16 s0 模块增强与集成收尾", Type: RequirementTypeOps, Priority: PriorityP2,
		Content: "s0 运维接口增强与模块集成收尾。\n\n文档：`docs/release/P16-s0模块增强与集成收尾.md`"},
	{Key: "go-backend-core", Title: "P17 公共服务基础设施", Type: RequirementTypeFeature, Priority: PriorityP2,
		Content: "公共服务与基础设施能力沉淀。\n\n文档：`docs/release/P17-公共服务基础设施.md`"},
	{Key: "go-backend-core", Title: "服务导航模块", Type: RequirementTypeFeature, Priority: PriorityP2,
		Content: "服务条目与类型管理、项目/容器关联、从部署接入点快速导入。\n\n文档：`docs/modules/servicenav/`"},
	{Key: "go-backend-core", Title: "Webhook 模块", Type: RequirementTypeFeature, Priority: PriorityP2,
		Content: "Webhook 配置、投递与日志。\n\n文档：`docs/modules/webhook/`"},

	// 业务模块
	{Key: "goodser", Title: "商品服务模块（Rust→Go 迁移）", Type: RequirementTypeFeature, Priority: PriorityP2,
		Content: "商品/库存/出入库/标签/状态码管理，并完成 Rust 版本到 Go 的迁移。\n\n文档：`docs/modules/goodser/dev/`"},
	{Key: "shopchecker", Title: "店铺价格采集与监控", Type: RequirementTypeFeature, Priority: PriorityP2,
		Content: "多平台商品价格采集、降价监控与通知。\n\n文档：`docs/modules/shopChecker/`"},
	{Key: "login3rd", Title: "第三方账号登录", Type: RequirementTypeFeature, Priority: PriorityP2,
		Content: "京东等平台第三方登录、会话保存与自动续期。\n\n文档：`docs/modules/login3rd/`"},
	{Key: "ia", Title: "投资组合分析与行情", Type: RequirementTypeFeature, Priority: PriorityP2,
		Content: "账户/标的/交易/持仓/定投/告警/收益分析。\n\n文档：`docs/modules/ia/`"},

	// 前端与模板
	{Key: "go-frontend-core", Title: "前端基座与管理端", Type: RequirementTypeFeature, Priority: PriorityP2,
		Content: "基于 NKUI 的前端基座：登录、用户/角色、存储、迁移、网络、调度、服务导航、各业务模块管理端（含 developmentManager）。\n\n文档：`AGENTS.md`"},
	{Key: "nkui", Title: "NKUI 组件库与设计系统", Type: RequirementTypeFeature, Priority: PriorityP2,
		Content: "Vue3 组件库与设计语言：48 篇设计规范、204 篇组件使用文档。\n\n文档：`AllDesign/`、`AllDocs/`"},
	{Key: "local-compose", Title: "本地容器栈（app + browserless chrome）", Type: RequirementTypeOps, Priority: PriorityP2,
		Content: "docker-compose 编排：业务 app 与 browserless chrome，接入 1panel-network。\n\n文件：`docker-compose.yml`"},

	// 当前模块
	{Key: "developmentmanager", Title: "V2 项目开发管理", Type: RequirementTypeFeature, Priority: PriorityP1,
		Status:  RequirementStatusDeveloping,
		Content: "项目台账、部署资产、数据字典、需求与生成、前端管理端；历史资料统一纳入。\n\n规划：`docs/plans/feature-V2-project-dev-management.md`"},
}

// seedAll 幂等登记项目/关联/资产（表未迁移时报错交由上层重试）
func seedAll(ctx context.Context, repo *Repository) error {
	ids := make(map[string]int64, len(seedProjects))

	for _, sp := range seedProjects {
		p, err := repo.GetProjectByKey(ctx, sp.Key)
		if err != nil {
			var dmErr *DevelopmentManagerError
			if errors.As(err, &dmErr) && dmErr.Code == ErrProjectNotFound {
				created, cerr := repo.CreateProject(ctx, &Project{
					ProjectKey:    sp.Key,
					Name:          sp.Name,
					Type:          sp.Type,
					Description:   sp.Description,
					RepoPath:      sp.RepoPath,
					DefaultBranch: "main",
					TechStack:     sp.TechStack,
					Status:        ProjectStatusActive,
					CreatedBy:     "seed",
					UpdatedBy:     "seed",
				})
				if cerr != nil {
					return cerr
				}
				ids[sp.Key] = created.ID
				continue
			}
			return err
		}
		ids[sp.Key] = p.ID
	}

	for _, sr := range seedRelations {
		fromID, okFrom := ids[sr.From]
		toID, okTo := ids[sr.To]
		if !okFrom || !okTo {
			continue
		}
		exists, err := repo.RelationExists(ctx, fromID, toID, sr.Type)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if _, err := repo.CreateRelation(ctx, &ProjectRelation{
			FromProjectID: fromID,
			ToProjectID:   toID,
			RelationType:  sr.Type,
			Note:          sr.Note,
			CreatedBy:     "seed",
		}); err != nil {
			return err
		}
	}

	for _, sa := range seedAssets {
		id, ok := ids[sa.Key]
		if !ok {
			continue
		}
		exists, err := repo.AssetExists(ctx, id, sa.URI)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		format := sa.Format
		if format == "" {
			format = "md"
		}
		note := sa.Note
		if note == "" {
			note = "seed 登记"
		}
		if _, err := repo.CreateAsset(ctx, &ProjectAsset{
			ProjectID: id,
			Category:  sa.Category,
			Title:     sa.Title,
			URI:       sa.URI,
			Format:    format,
			Note:      note,
		}); err != nil {
			return err
		}
	}

	// 历史需求统一导入（按项目 + 标题幂等）
	for _, sr := range seedRequirements {
		projectID, ok := ids[sr.Key]
		if !ok {
			continue
		}
		exists, err := repo.RequirementTitleExists(ctx, projectID, sr.Title)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		no, err := repo.NextRequirementNo(ctx, projectID)
		if err != nil {
			return err
		}
		status := sr.Status
		if status == "" {
			status = RequirementStatusDone
		}
		owner := sr.Owner
		if owner == "" {
			owner = "legacy"
		}
		if _, err := repo.CreateRequirementTx(ctx, &Requirement{
			ProjectID: projectID,
			Code:      formatRequirementCode(no),
			Title:     sr.Title,
			Content:   sr.Content,
			Type:      sr.Type,
			Priority:  sr.Priority,
			Status:    status,
			Source:    RequirementSourceManual,
			Owner:     owner,
			CreatedBy: "seed",
			UpdatedBy: "seed",
		}, "历史资料统一导入"); err != nil {
			return err
		}
	}
	return nil
}

// seedWithRetry 后台重试执行 seed（迁移未应用时表不存在，应用后自动补齐）
func (m *Module) seedWithRetry() {
	ctx := context.Background()
	const maxAttempts = 60
	const interval = 5 * time.Second

	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if err := seedAll(ctx, m.repo); err != nil {
			lastErr = err
			if i == 0 {
				logger.Warn().Err(err).Msg(
					"developmentmanager: 项目台账 seed 未就绪（如提示表不存在，请先执行 POST /api/v1/migrate/schema/migrate 应用迁移），5 秒后重试")
			}
			time.Sleep(interval)
			continue
		}
		logger.Info().
			Int("projects", len(seedProjects)).
			Int("assets", len(seedAssets)).
			Int("requirements", len(seedRequirements)).
			Msg("developmentmanager: 项目台账 seed 完成")
		return
	}
	logger.Error().Err(lastErr).Msg("developmentmanager: 项目台账 seed 失败（已达最大重试次数）")
}
