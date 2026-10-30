package developmentmanager

import "time"

// 项目类型
const (
	ProjectTypeBase           = "base"              // 后端基座
	ProjectTypeTemplate       = "template"          // 前端模板
	ProjectTypeBackendModule  = "backend_module"    // 后端模块
	ProjectTypeFrontendModule = "frontend_module"   // 前端模块
	ProjectTypeFrontendBase   = "frontend_base"     // 前端基座（NKUI 之上的前端工程，如 Go-Frontend-Core）
	ProjectTypeContainer      = "container_project" // 容器项目（docker compose 部署栈）
)

// 项目状态
const (
	ProjectStatusActive   = "active"
	ProjectStatusArchived = "archived"
)

// 关联类型
const (
	RelationUsesBase           = "uses_base"            // 模块 → 后端基座
	RelationUsesTemplate       = "uses_template"        // 前端模块 → 前端模板
	RelationBindsBackendModule = "binds_backend_module" // 前端模块 → 后端模块
	RelationRelated            = "related"              // 其他横向关联
)

// 资产类别
const (
	AssetCategoryRequirement = "requirement"
	AssetCategoryCode        = "code"
	AssetCategoryDoc         = "doc"
	AssetCategoryAsset       = "asset"
)

// ValidProjectTypes 项目类型合法集合
var ValidProjectTypes = map[string]bool{
	ProjectTypeBase:           true,
	ProjectTypeTemplate:       true,
	ProjectTypeBackendModule:  true,
	ProjectTypeFrontendModule: true,
	ProjectTypeFrontendBase:   true,
	ProjectTypeContainer:      true,
}

// ValidRelationTypes 关联类型合法集合
var ValidRelationTypes = map[string]bool{
	RelationUsesBase:           true,
	RelationUsesTemplate:       true,
	RelationBindsBackendModule: true,
	RelationRelated:            true,
}

// ValidAssetCategories 资产类别合法集合
var ValidAssetCategories = map[string]bool{
	AssetCategoryRequirement: true,
	AssetCategoryCode:        true,
	AssetCategoryDoc:         true,
	AssetCategoryAsset:       true,
}

// Project 项目台账实体
type Project struct {
	ID            int64      `json:"id" db:"id"`
	ProjectKey    string     `json:"project_key" db:"project_key"`
	Name          string     `json:"name" db:"name"`
	Type          string     `json:"type" db:"type"`
	Description   string     `json:"description" db:"description"`
	RepoURL       string     `json:"repo_url" db:"repo_url"`
	RepoPath      string     `json:"repo_path" db:"repo_path"`
	DefaultBranch string     `json:"default_branch" db:"default_branch"`
	TechStack     []string   `json:"tech_stack,omitempty"` // JSON 列，扫描后解析
	Status        string     `json:"status" db:"status"`
	Deleted       bool       `json:"deleted" db:"deleted"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
	CreatedBy     string     `json:"created_by" db:"created_by"`
	UpdatedBy     string     `json:"updated_by" db:"updated_by"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

// ProjectRelation 项目关联实体（含两端项目信息，便于列表展示）
type ProjectRelation struct {
	ID              int64     `json:"id" db:"id"`
	FromProjectID   int64     `json:"from_project_id" db:"from_project_id"`
	ToProjectID     int64     `json:"to_project_id" db:"to_project_id"`
	RelationType    string    `json:"relation_type" db:"relation_type"`
	Note            string    `json:"note" db:"note"`
	Deleted         bool      `json:"deleted" db:"deleted"`
	CreatedBy       string    `json:"created_by" db:"created_by"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
	FromProjectKey  string    `json:"from_project_key,omitempty" db:"from_project_key"`
	FromProjectName string    `json:"from_project_name,omitempty" db:"from_project_name"`
	ToProjectKey    string    `json:"to_project_key,omitempty" db:"to_project_key"`
	ToProjectName   string    `json:"to_project_name,omitempty" db:"to_project_name"`
}

// ProjectAsset 归类资产索引实体
type ProjectAsset struct {
	ID        int64     `json:"id" db:"id"`
	ProjectID int64     `json:"project_id" db:"project_id"`
	Category  string    `json:"category" db:"category"`
	Title     string    `json:"title" db:"title"`
	URI       string    `json:"uri" db:"uri"`
	Format    string    `json:"format" db:"format"`
	Version   string    `json:"version" db:"version"`
	Size      int64     `json:"size" db:"size"`
	Checksum  string    `json:"checksum" db:"checksum"`
	Note      string    `json:"note" db:"note"`
	Deleted   bool      `json:"deleted" db:"deleted"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// PaginatedResp 分页响应（与现有 zzz 模块保持一致）
type PaginatedResp[T any] struct {
	Items   []T  `json:"items"`
	HasMore bool `json:"has_more"`
	Total   int  `json:"total"`
}

// ---------- 请求结构 ----------

// ListProjectsReq 项目列表查询参数
type ListProjectsReq struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Type     string `form:"type"`
	Status   string `form:"status"`
	Keyword  string `form:"keyword"`
	Deleted  *bool  `form:"deleted"`
}

// Normalize 分页参数归一化（page>=1，page_size 1-100 默认 20）
func (r *ListProjectsReq) Normalize() {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.PageSize < 1 || r.PageSize > 100 {
		r.PageSize = 20
	}
}

// Offset 计算 SQL OFFSET
func (r *ListProjectsReq) Offset() int { return (r.Page - 1) * r.PageSize }

// CreateProjectReq 创建项目请求
type CreateProjectReq struct {
	ProjectKey    string   `json:"project_key" binding:"required"`
	Name          string   `json:"name" binding:"required"`
	Type          string   `json:"type" binding:"required"`
	Description   string   `json:"description"`
	RepoURL       string   `json:"repo_url"`
	RepoPath      string   `json:"repo_path"`
	DefaultBranch string   `json:"default_branch"`
	TechStack     []string `json:"tech_stack"`
	Status        string   `json:"status"`
}

// UpdateProjectReq 更新项目请求（PUT 全量更新）
type UpdateProjectReq = CreateProjectReq

// ListRelationsReq 关联列表查询参数
type ListRelationsReq struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Type     string `form:"type"`
	Deleted  *bool  `form:"deleted"`
}

// Normalize 分页参数归一化
func (r *ListRelationsReq) Normalize() {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.PageSize < 1 || r.PageSize > 100 {
		r.PageSize = 20
	}
}

// Offset 计算 SQL OFFSET
func (r *ListRelationsReq) Offset() int { return (r.Page - 1) * r.PageSize }

// CreateRelationReq 创建项目关联请求（POST /projects/:id/relations）
type CreateRelationReq struct {
	ToProjectID  int64  `json:"to_project_id" binding:"required"`
	RelationType string `json:"relation_type" binding:"required"`
	Note         string `json:"note"`
}

// ListAssetsReq 资产列表查询参数
type ListAssetsReq struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Category string `form:"category"`
	Deleted  *bool  `form:"deleted"`
}

// Normalize 分页参数归一化
func (r *ListAssetsReq) Normalize() {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.PageSize < 1 || r.PageSize > 100 {
		r.PageSize = 20
	}
}

// Offset 计算 SQL OFFSET
func (r *ListAssetsReq) Offset() int { return (r.Page - 1) * r.PageSize }

// CreateAssetReq 创建资产索引请求
type CreateAssetReq struct {
	Category string `json:"category" binding:"required"`
	Title    string `json:"title" binding:"required"`
	URI      string `json:"uri" binding:"required"`
	Format   string `json:"format"`
	Version  string `json:"version"`
	Size     int64  `json:"size"`
	Checksum string `json:"checksum"`
	Note     string `json:"note"`
}

// StatsResp 仪表盘统计
type StatsResp struct {
	ProjectsTotal    int            `json:"projects_total"`
	ProjectsDeleted  int            `json:"projects_deleted"`
	ProjectsByType   map[string]int `json:"projects_by_type"`
	ProjectsByStatus map[string]int `json:"projects_by_status"`
	RelationsActive  int            `json:"relations_active"`
	AssetsTotal      int            `json:"assets_total"`
}
