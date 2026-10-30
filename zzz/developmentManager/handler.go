package developmentmanager

import (
	"database/sql"
	"errors"
	"strconv"

	"github.com/Allinost/go-backend-core/internal/config"
	appErr "github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/Allinost/go-backend-core/internal/services/audit"
	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	svc *Service
}

// NewHandler 创建 Handler
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ---------- 辅助 ----------

// actorName 当前操作人用户名（auth 中间件注入）
func actorName(c *gin.Context) string {
	if v, ok := c.Get("username"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// actorID 当前操作人用户 ID 字符串
func actorID(c *gin.Context) string {
	if v, ok := c.Get("user_id"); ok {
		if id, ok := v.(uint); ok {
			return strconv.FormatUint(uint64(id), 10)
		}
	}
	return ""
}

// statusForCode 模块业务错误码 → HTTP 状态码（标准码之外的补充映射）
func statusForCode(code int) int {
	switch code {
	case ErrProjectNotFound, ErrRelationNotFound, ErrAssetNotFound, ErrTargetProjectMissing,
		ErrContainerNotFound, ErrResourceNotFound, ErrVolumeNotFound, ErrAccessPointNotFound,
		ErrWhitelistNotFound, ErrWhitelistMemberNotFound, ErrBackupNotFound,
		ErrTableNotFound, ErrColumnNotFound:
		return 404
	case ErrDuplicateProjectKey, ErrProjectHasDependents, ErrSelfRelation,
		ErrDuplicateContainerName, ErrDuplicateWhitelistName,
		ErrContainerInWhitelist, ErrWhitelistInUse,
		ErrDuplicateTable, ErrDuplicateColumn:
		return 409
	case ErrInvalidProjectKey, ErrInvalidProjectType, ErrInvalidRelationType,
		ErrInvalidAssetCategory, ErrInvalidProjectName,
		ErrInvalidContainerName, ErrInvalidPort, ErrInvalidProtocol, ErrInvalidExposure,
		ErrInvalidProxyType, ErrInvalidNetworkZone, ErrCrossProjectRef, ErrInvalidVolumePath,
		ErrInvalidBackupType, ErrInvalidBackupStatus, ErrInvalidVolumeMode,
		ErrInvalidWhitelistName, ErrInvalidReplicas, ErrInvalidResourceAlloc,
		ErrInvalidDBType, ErrInvalidTableName, ErrInvalidTableStatus,
		ErrInvalidColumnName, ErrScanUnsupported:
		return 400
	default:
		return 500
	}
}

// handleError 统一错误处理（模块错误带正确的 HTTP 状态，其余按标准错误处理）
func (h *Handler) handleError(c *gin.Context, err error) {
	var dmErr *DevelopmentManagerError
	if errors.As(err, &dmErr) {
		logger.Error().Err(err).Int("developmentmanager_code", dmErr.Code).Msg("developmentmanager 业务错误")
		resp := response.Response{Code: dmErr.Code, Message: dmErr.Message}
		if dmErr.Detail != "" {
			if cfg := config.Get(); cfg != nil && cfg.Server.Mode != "release" {
				resp.Detail = dmErr.Detail
			}
		}
		c.AbortWithStatusJSON(statusForCode(dmErr.Code), resp)
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		response.Fail(c, appErr.New(appErr.CodeNotFound, "资源不存在"))
		return
	}
	logger.Error().Err(err).Msg("developmentmanager 服务内部错误")
	response.Fail(c, appErr.New(appErr.CodeSystemErr, "服务内部错误"))
}

// recordAudit 记录审计日志（失败不阻断业务）
func recordAudit(c *gin.Context, action, resource, detail string) {
	var err error
	switch action {
	case "create":
		err = audit.RecordCreate(c.Request.Context(), actorID(c), actorName(c), resource, detail)
	case "update":
		err = audit.RecordUpdate(c.Request.Context(), actorID(c), actorName(c), resource, detail)
	case "delete":
		err = audit.RecordDelete(c.Request.Context(), actorID(c), actorName(c), resource, detail)
	}
	if err != nil {
		logger.Warn().Err(err).Str("action", action).Str("resource", resource).Msg("developmentmanager 审计记录失败")
	}
}

// ---------- 项目 ----------

// ListProjects 获取项目列表
// @Summary      获取项目列表
// @Tags         developmentManager
// @Produce      json
// @Param        page      query  int     false  "页码，默认 1"
// @Param        page_size query  int     false  "每页数量，默认 20，最大 100"
// @Param        type      query  string  false  "项目类型 base/template/backend_module/frontend_module"
// @Param        status    query  string  false  "状态 active/archived"
// @Param        keyword   query  string  false  "关键词，匹配名称/唯一键/简介"
// @Param        deleted   query  bool    false  "是否查询已删除（默认否）"
// @Success      200  {object}  response.Response{data=PaginatedResp[Project]}
// @Router       /developmentmanager/projects [get]
// @Security     BearerAuth
func (h *Handler) ListProjects(c *gin.Context) {
	var req ListProjectsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	items, hasMore, total, err := h.svc.ListProjects(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []Project{}
	}
	response.Success(c, PaginatedResp[Project]{Items: items, HasMore: hasMore, Total: total})
}

// CreateProject 新建项目
// @Summary      新建项目
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        body  body  CreateProjectReq  true  "项目信息"
// @Success      200  {object}  response.Response{data=Project}
// @Router       /developmentmanager/projects [post]
// @Security     BearerAuth
func (h *Handler) CreateProject(c *gin.Context) {
	var req CreateProjectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	p, err := h.svc.CreateProject(c.Request.Context(), &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "create", "developmentmanager:project", p.ProjectKey)
	response.Success(c, p)
}

// GetProject 获取项目详情
// @Summary      获取项目详情
// @Tags         developmentManager
// @Produce      json
// @Param        id   path  int  true  "项目 ID"
// @Success      200  {object}  response.Response{data=Project}
// @Router       /developmentmanager/projects/{id} [get]
// @Security     BearerAuth
func (h *Handler) GetProject(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	p, err := h.svc.GetProject(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, p)
}

// UpdateProject 更新项目（全量）
// @Summary      更新项目
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int                true  "项目 ID"
// @Param        body  body  UpdateProjectReq   true  "项目信息（全量）"
// @Success      200  {object}  response.Response{data=Project}
// @Router       /developmentmanager/projects/{id} [put]
// @Security     BearerAuth
func (h *Handler) UpdateProject(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req UpdateProjectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	p, err := h.svc.UpdateProject(c.Request.Context(), id, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "update", "developmentmanager:project", p.ProjectKey)
	response.Success(c, p)
}

// DeleteProject 删除项目（软删除，级联标记关联与资产）
// @Summary      删除项目
// @Tags         developmentManager
// @Produce      json
// @Param        id     path  int   true   "项目 ID"
// @Param        force  query  bool  false  "强制删除（存在被依赖关联时需为 1）"
// @Success      200  {object}  response.Response
// @Router       /developmentmanager/projects/{id} [delete]
// @Security     BearerAuth
func (h *Handler) DeleteProject(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	force := c.Query("force") == "1" || c.Query("force") == "true"
	if err := h.svc.DeleteProject(c.Request.Context(), id, force); err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "delete", "developmentmanager:project", "id="+strconv.FormatInt(id, 10))
	response.Success(c, gin.H{})
}

// ---------- 关联 ----------

// ListRelations 获取项目关联列表
// @Summary      获取项目关联列表
// @Tags         developmentManager
// @Produce      json
// @Param        id        path  int    true   "项目 ID"
// @Param        page      query  int    false  "页码"
// @Param        page_size query  int    false  "每页数量"
// @Param        type      query  string false  "关联类型筛选"
// @Param        deleted   query  bool   false  "是否查询已删除"
// @Success      200  {object}  response.Response{data=PaginatedResp[ProjectRelation]}
// @Router       /developmentmanager/projects/{id}/relations [get]
// @Security     BearerAuth
func (h *Handler) ListRelations(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req ListRelationsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	items, hasMore, total, err := h.svc.ListRelations(c.Request.Context(), id, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []ProjectRelation{}
	}
	response.Success(c, PaginatedResp[ProjectRelation]{Items: items, HasMore: hasMore, Total: total})
}

// CreateRelation 新建项目关联
// @Summary      新建项目关联
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int                 true  "发起方项目 ID"
// @Param        body  body  CreateRelationReq   true  "关联信息"
// @Success      200  {object}  response.Response{data=ProjectRelation}
// @Router       /developmentmanager/projects/{id}/relations [post]
// @Security     BearerAuth
func (h *Handler) CreateRelation(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req CreateRelationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	rel, err := h.svc.CreateRelation(c.Request.Context(), id, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "create", "developmentmanager:relation",
		"from="+strconv.FormatInt(id, 10)+" to="+strconv.FormatInt(req.ToProjectID, 10)+" type="+req.RelationType)
	response.Success(c, rel)
}

// DeleteRelation 删除项目关联（软删除）
// @Summary      删除项目关联
// @Tags         developmentManager
// @Produce      json
// @Param        id       path  int  true  "项目 ID"
// @Param        rel_id   path  int  true  "关联 ID"
// @Success      200  {object}  response.Response
// @Router       /developmentmanager/projects/{id}/relations/{rel_id} [delete]
// @Security     BearerAuth
func (h *Handler) DeleteRelation(c *gin.Context) {
	relID, ok := parseID(c, "rel_id")
	if !ok {
		return
	}
	if err := h.svc.DeleteRelation(c.Request.Context(), relID); err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "delete", "developmentmanager:relation", "rel_id="+strconv.FormatInt(relID, 10))
	response.Success(c, gin.H{})
}

// ---------- 资产 ----------

// ListAssets 获取项目资产索引
// @Summary      获取项目资产索引
// @Tags         developmentManager
// @Produce      json
// @Param        id        path  int    true   "项目 ID"
// @Param        page      query  int    false  "页码"
// @Param        page_size query  int    false  "每页数量"
// @Param        category  query  string false  "类别 requirement/code/doc/asset"
// @Success      200  {object}  response.Response{data=PaginatedResp[ProjectAsset]}
// @Router       /developmentmanager/projects/{id}/assets [get]
// @Security     BearerAuth
func (h *Handler) ListAssets(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req ListAssetsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	items, hasMore, total, err := h.svc.ListAssets(c.Request.Context(), id, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []ProjectAsset{}
	}
	response.Success(c, PaginatedResp[ProjectAsset]{Items: items, HasMore: hasMore, Total: total})
}

// CreateAsset 登记项目资产索引
// @Summary      登记项目资产索引
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int                true  "项目 ID"
// @Param        body  body  CreateAssetReq     true  "资产信息"
// @Success      200  {object}  response.Response{data=ProjectAsset}
// @Router       /developmentmanager/projects/{id}/assets [post]
// @Security     BearerAuth
func (h *Handler) CreateAsset(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req CreateAssetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	a, err := h.svc.CreateAsset(c.Request.Context(), id, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "create", "developmentmanager:asset", a.URI)
	response.Success(c, a)
}

// DeleteAsset 删除项目资产索引（软删除）
// @Summary      删除项目资产索引
// @Tags         developmentManager
// @Produce      json
// @Param        id        path  int  true  "项目 ID"
// @Param        asset_id  path  int  true  "资产 ID"
// @Success      200  {object}  response.Response
// @Router       /developmentmanager/projects/{id}/assets/{asset_id} [delete]
// @Security     BearerAuth
func (h *Handler) DeleteAsset(c *gin.Context) {
	assetID, ok := parseID(c, "asset_id")
	if !ok {
		return
	}
	if err := h.svc.DeleteAsset(c.Request.Context(), assetID); err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "delete", "developmentmanager:asset", "asset_id="+strconv.FormatInt(assetID, 10))
	response.Success(c, gin.H{})
}

// ---------- 统计与权限元数据 ----------

// Stats 获取仪表盘统计
// @Summary      获取仪表盘统计
// @Tags         developmentManager
// @Produce      json
// @Success      200  {object}  response.Response{data=StatsResp}
// @Router       /developmentmanager/stats [get]
// @Security     BearerAuth
func (h *Handler) Stats(c *gin.Context) {
	stats, err := h.svc.Stats(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, stats)
}

// PermissionMeta 获取本模块权限点元数据（供管理端展示与分配）
// @Summary      获取本模块权限点元数据
// @Tags         developmentManager
// @Produce      json
// @Success      200  {object}  response.Response{data=[]PermSpec}
// @Router       /developmentmanager/permissions/meta [get]
// @Security     BearerAuth
func (h *Handler) PermissionMeta(c *gin.Context) {
	response.Success(c, PermissionSpecs)
}

// parseID 解析路径整型参数
func parseID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		response.ParamErr(c, "无效的 "+name)
		return 0, false
	}
	return id, true
}
