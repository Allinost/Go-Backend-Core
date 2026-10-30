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

// seedAssets 台账初始资产索引
var seedAssets = []struct {
	Key      string
	Category string
	Title    string
	URI      string
}{
	{Key: "developmentmanager", Category: AssetCategoryDoc, Title: "V2 项目开发管理规划",
		URI: "docs/plans/feature-V2-project-dev-management.md"},
	{Key: "go-backend-core", Category: AssetCategoryDoc, Title: "数据模型与表前缀规范",
		URI: "docs/data-model.md"},
	{Key: "go-backend-core", Category: AssetCategoryDoc, Title: "文档索引",
		URI: "docs/README.md"},
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
		if _, err := repo.CreateAsset(ctx, &ProjectAsset{
			ProjectID: id,
			Category:  sa.Category,
			Title:     sa.Title,
			URI:       sa.URI,
			Format:    "md",
			Note:      "seed 登记",
		}); err != nil {
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
		logger.Info().Int("projects", len(seedProjects)).Msg("developmentmanager: 项目台账 seed 完成")
		return
	}
	logger.Error().Err(lastErr).Msg("developmentmanager: 项目台账 seed 失败（已达最大重试次数）")
}
