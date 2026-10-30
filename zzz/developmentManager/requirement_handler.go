package developmentmanager

import (
	"strconv"

	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// ListRequirements 获取项目需求列表
// @Summary      获取项目需求列表
// @Tags         developmentManager
// @Produce      json
// @Param        id        path   int    true   "项目 ID"
// @Param        page      query  int    false  "页码，默认 1"
// @Param        page_size query  int    false  "每页数量，默认 20，最大 100"
// @Param        status    query  string false  "状态筛选"
// @Param        type      query  string false  "类型筛选"
// @Param        priority  query  string false  "优先级筛选"
// @Param        keyword   query  string false  "关键词（标题/编号/正文）"
// @Param        deleted   query  bool   false  "是否查询已删除（默认否）"
// @Success      200  {object}  response.Response{data=PaginatedResp[Requirement]}
// @Router       /developmentmanager/projects/{id}/requirements [get]
// @Security     BearerAuth
func (h *Handler) ListRequirements(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req ListRequirementsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	items, hasMore, total, err := h.svc.ListRequirements(c.Request.Context(), projectID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []Requirement{}
	}
	response.Success(c, PaginatedResp[Requirement]{Items: items, HasMore: hasMore, Total: total})
}

// CreateRequirement 新建需求
// @Summary      新建需求
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int                    true  "项目 ID"
// @Param        body  body  CreateRequirementReq   true  "需求信息"
// @Success      200  {object}  response.Response{data=Requirement}
// @Router       /developmentmanager/projects/{id}/requirements [post]
// @Security     BearerAuth
func (h *Handler) CreateRequirement(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req CreateRequirementReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.CreateRequirement(c.Request.Context(), projectID, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "create", "developmentmanager:requirement", item.Code+" "+item.Title)
	response.Success(c, item)
}

// GetRequirement 获取需求详情
// @Summary      获取需求详情
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "需求 ID"
// @Success      200  {object}  response.Response{data=Requirement}
// @Router       /developmentmanager/requirements/{id} [get]
// @Security     BearerAuth
func (h *Handler) GetRequirement(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.svc.GetRequirement(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// UpdateRequirement 更新需求（自动追加版本快照，不改变状态）
// @Summary      更新需求
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int                    true  "需求 ID"
// @Param        body  body  UpdateRequirementReq   true  "需求信息（全量，不含状态）"
// @Success      200  {object}  response.Response{data=Requirement}
// @Router       /developmentmanager/requirements/{id} [put]
// @Security     BearerAuth
func (h *Handler) UpdateRequirement(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req UpdateRequirementReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.UpdateRequirement(c.Request.Context(), id, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "update", "developmentmanager:requirement", item.Code+" v"+strconv.Itoa(item.LatestVersion))
	response.Success(c, item)
}

// ChangeRequirementStatus 需求状态流转
// @Summary      需求状态流转
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int                            true  "需求 ID"
// @Param        body  body  ChangeRequirementStatusReq     true  "目标状态与变更说明"
// @Success      200  {object}  response.Response{data=Requirement}
// @Router       /developmentmanager/requirements/{id}/status [post]
// @Security     BearerAuth
func (h *Handler) ChangeRequirementStatus(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req ChangeRequirementStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.ChangeRequirementStatus(c.Request.Context(), id, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "update", "developmentmanager:requirement", item.Code+" status="+item.Status)
	response.Success(c, item)
}

// DeleteRequirement 删除需求（软删除 + 生成已删除快照）
// @Summary      删除需求
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "需求 ID"
// @Success      200  {object}  response.Response
// @Router       /developmentmanager/requirements/{id} [delete]
// @Security     BearerAuth
func (h *Handler) DeleteRequirement(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteRequirement(c.Request.Context(), id, actorName(c)); err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "delete", "developmentmanager:requirement", "id="+strconv.FormatInt(id, 10))
	response.Success(c, gin.H{})
}

// ListRequirementVersions 获取需求版本历史
// @Summary      获取需求版本历史
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "需求 ID"
// @Success      200  {object}  response.Response{data=[]RequirementVersion}
// @Router       /developmentmanager/requirements/{id}/versions [get]
// @Security     BearerAuth
func (h *Handler) ListRequirementVersions(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	items, err := h.svc.ListRequirementVersions(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []RequirementVersion{}
	}
	response.Success(c, items)
}
