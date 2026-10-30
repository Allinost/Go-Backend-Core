package developmentmanager

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/Allinost/go-backend-core/zzz/developmentManager/generation"
)

// projectKeyRe 项目唯一键格式：小写字母/数字/中划线
var projectKeyRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,63}$`)

// Service 项目开发管理业务逻辑
type Service struct {
	repo     *Repository
	aiWriter generation.AIWriter // 可选的 AI 生成器，未注入时 engine=ai 自动降级为 template
}

// NewService 创建 Service
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// SetAIWriter 注入 AI 生成器（不注入或调用失败时自动降级为模板引擎）
func (s *Service) SetAIWriter(w generation.AIWriter) {
	s.aiWriter = w
}

// validateProjectReq 校验项目请求（键格式 / 名称 / 类型），返回归一化后的项目
func validateProjectReq(req *CreateProjectReq, actor string, now time.Time) (*Project, error) {
	key := strings.TrimSpace(req.ProjectKey)
	if !projectKeyRe.MatchString(key) {
		return nil, NewDevelopmentManagerError(ErrInvalidProjectKey)
	}
	name := strings.TrimSpace(req.Name)
	if l := len([]rune(name)); l < 2 || l > 128 {
		return nil, NewDevelopmentManagerError(ErrInvalidProjectName)
	}
	if !ValidProjectTypes[req.Type] {
		return nil, NewDevelopmentManagerError(ErrInvalidProjectType)
	}
	status := req.Status
	if status == "" {
		status = ProjectStatusActive
	}
	if status != ProjectStatusActive && status != ProjectStatusArchived {
		return nil, NewDevelopmentManagerError(ErrInvalidProjectType)
	}
	branch := strings.TrimSpace(req.DefaultBranch)
	if branch == "" {
		branch = "main"
	}
	return &Project{
		ProjectKey:    key,
		Name:          name,
		Type:          req.Type,
		Description:   strings.TrimSpace(req.Description),
		RepoURL:       strings.TrimSpace(req.RepoURL),
		RepoPath:      strings.TrimSpace(req.RepoPath),
		DefaultBranch: branch,
		TechStack:     req.TechStack,
		Status:        status,
		CreatedBy:     actor,
		UpdatedBy:     actor,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// ListProjects 分页查询项目
func (s *Service) ListProjects(ctx context.Context, req *ListProjectsReq) ([]Project, bool, int, error) {
	return s.repo.ListProjects(ctx, req)
}

// GetProject 查询项目详情
func (s *Service) GetProject(ctx context.Context, id int64) (*Project, error) {
	return s.repo.GetProject(ctx, id)
}

// CreateProject 创建项目（校验 + 唯一键查重）
func (s *Service) CreateProject(ctx context.Context, req *CreateProjectReq, actor string) (*Project, error) {
	p, err := validateProjectReq(req, actor, time.Now())
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.GetProjectByKey(ctx, p.ProjectKey); err == nil {
		return nil, NewDevelopmentManagerError(ErrDuplicateProjectKey)
	}
	return s.repo.CreateProject(ctx, p)
}

// UpdateProject 全量更新项目
func (s *Service) UpdateProject(ctx context.Context, id int64, req *UpdateProjectReq, actor string) (*Project, error) {
	p, err := validateProjectReq(req, actor, time.Now())
	if err != nil {
		return nil, err
	}
	// 唯一键未变时跳过查重
	if existing, err := s.repo.GetProjectByKey(ctx, p.ProjectKey); err == nil && existing.ID != id {
		return nil, NewDevelopmentManagerError(ErrDuplicateProjectKey)
	}
	p.ID = id
	return s.repo.UpdateProject(ctx, p)
}

// DeleteProject 软删除项目：默认存在启用中的被依赖关联时拒绝（需 force），删除时级联标记关联与资产
func (s *Service) DeleteProject(ctx context.Context, id int64, force bool) error {
	// 先确认存在
	if _, err := s.repo.GetProject(ctx, id); err != nil {
		return err
	}
	if !force {
		dependents, err := s.repo.ListDependents(ctx, id)
		if err != nil {
			return err
		}
		if len(dependents) > 0 {
			var names []string
			for _, d := range dependents {
				names = append(names, d.FromProjectKey+"("+d.RelationType+")")
			}
			return NewDevelopmentManagerError(ErrProjectHasDependents).WithDetail(
				"被依赖关联: " + strings.Join(names, ", ") + "，确认无影响可加 force=1 强制删除")
		}
	}
	return s.repo.DeleteProjectTx(ctx, id, time.Now())
}

// ListRelations 查询项目关联
func (s *Service) ListRelations(ctx context.Context, projectID int64, req *ListRelationsReq) ([]ProjectRelation, bool, int, error) {
	if _, err := s.repo.GetProject(ctx, projectID); err != nil {
		return nil, false, 0, err
	}
	return s.repo.ListRelations(ctx, projectID, req)
}

// CreateRelation 创建项目关联（校验类型/目标/自关联/重复）
func (s *Service) CreateRelation(ctx context.Context, fromID int64, req *CreateRelationReq, actor string) (*ProjectRelation, error) {
	if _, err := s.repo.GetProject(ctx, fromID); err != nil {
		return nil, err
	}
	if !ValidRelationTypes[req.RelationType] {
		return nil, NewDevelopmentManagerError(ErrInvalidRelationType)
	}
	if req.ToProjectID == fromID {
		return nil, NewDevelopmentManagerError(ErrSelfRelation)
	}
	target, err := s.repo.GetProject(ctx, req.ToProjectID)
	if err != nil {
		var dmErr *DevelopmentManagerError
		if errors.As(err, &dmErr) && dmErr.Code == ErrProjectNotFound {
			return nil, NewDevelopmentManagerError(ErrTargetProjectMissing)
		}
		return nil, err
	}
	if target.Deleted {
		return nil, NewDevelopmentManagerError(ErrTargetProjectMissing)
	}
	exists, err := s.repo.RelationExists(ctx, fromID, req.ToProjectID, req.RelationType)
	if err != nil {
		return nil, err
	}
	if exists {
		// 重复关联按"已存在"语义返回冲突
		return nil, NewDevelopmentManagerError(ErrDuplicateProjectKey)
	}
	return s.repo.CreateRelation(ctx, &ProjectRelation{
		FromProjectID: fromID,
		ToProjectID:   req.ToProjectID,
		RelationType:  req.RelationType,
		Note:          strings.TrimSpace(req.Note),
		CreatedBy:     actor,
	})
}

// DeleteRelation 软删除关联
func (s *Service) DeleteRelation(ctx context.Context, id int64) error {
	if _, err := s.repo.GetRelation(ctx, id); err != nil {
		return err
	}
	return s.repo.SoftDeleteRelation(ctx, id, time.Now())
}

// ListAssets 查询项目资产索引
func (s *Service) ListAssets(ctx context.Context, projectID int64, req *ListAssetsReq) ([]ProjectAsset, bool, int, error) {
	if _, err := s.repo.GetProject(ctx, projectID); err != nil {
		return nil, false, 0, err
	}
	return s.repo.ListAssets(ctx, projectID, req)
}

// CreateAsset 创建资产索引
func (s *Service) CreateAsset(ctx context.Context, projectID int64, req *CreateAssetReq) (*ProjectAsset, error) {
	if _, err := s.repo.GetProject(ctx, projectID); err != nil {
		return nil, err
	}
	if !ValidAssetCategories[req.Category] {
		return nil, NewDevelopmentManagerError(ErrInvalidAssetCategory)
	}
	if strings.TrimSpace(req.URI) == "" || strings.TrimSpace(req.Title) == "" {
		return nil, NewDevelopmentManagerError(ErrInvalidProjectName)
	}
	return s.repo.CreateAsset(ctx, &ProjectAsset{
		ProjectID: projectID,
		Category:  req.Category,
		Title:     strings.TrimSpace(req.Title),
		URI:       strings.TrimSpace(req.URI),
		Format:    req.Format,
		Version:   req.Version,
		Size:      req.Size,
		Checksum:  req.Checksum,
		Note:      req.Note,
	})
}

// DeleteAsset 软删除资产索引
func (s *Service) DeleteAsset(ctx context.Context, id int64) error {
	if _, err := s.repo.GetAsset(ctx, id); err != nil {
		return err
	}
	return s.repo.SoftDeleteAsset(ctx, id, time.Now())
}

// Stats 仪表盘统计
func (s *Service) Stats(ctx context.Context) (*StatsResp, error) {
	return s.repo.Stats(ctx)
}
