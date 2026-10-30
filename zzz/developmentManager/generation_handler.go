package developmentmanager

import (
	"strconv"

	appErr "github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// ensureAutoCommitPermission auto_commit 的二次权限校验（dm.generation:autocommit）
func (h *Handler) ensureAutoCommitPermission(c *gin.Context) bool {
	if h.rbac == nil {
		response.FailCode(c, appErr.CodeForbidden)
		return false
	}
	v, ok := c.Get("user_id")
	if !ok {
		response.FailCode(c, appErr.CodeUnauth)
		return false
	}
	uid, ok := v.(uint)
	if !ok {
		response.FailCode(c, appErr.CodeUnauth)
		return false
	}
	if !h.rbac.HasPermission(uid, "dm.generation", "autocommit") {
		response.FailCode(c, appErr.CodeForbidden)
		return false
	}
	return true
}

// CreateGeneration 触发生成任务
// @Summary      触发生成任务（plan/code/doc）
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int                    true  "需求 ID"
// @Param        body  body  CreateGenerationReq    true  "生成参数"
// @Success      200  {object}  response.Response{data=Generation}
// @Router       /developmentmanager/requirements/{id}/generations [post]
// @Security     BearerAuth
func (h *Handler) CreateGeneration(c *gin.Context) {
	requirementID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req CreateGenerationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	if req.AutoCommit && !h.ensureAutoCommitPermission(c) {
		return
	}
	item, err := h.svc.CreateGeneration(c.Request.Context(), requirementID, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "create", "developmentmanager:generation",
		item.Kind+" engine="+item.EngineUsed+" status="+item.Status+" ref="+item.OutputRef)
	response.Success(c, item)
}

// ListGenerations 获取需求下的生成任务
// @Summary      获取需求下的生成任务
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "需求 ID"
// @Success      200  {object}  response.Response{data=[]Generation}
// @Router       /developmentmanager/requirements/{id}/generations [get]
// @Security     BearerAuth
func (h *Handler) ListGenerations(c *gin.Context) {
	requirementID, ok := parseID(c, "id")
	if !ok {
		return
	}
	items, err := h.svc.ListGenerations(c.Request.Context(), requirementID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []Generation{}
	}
	response.Success(c, items)
}

// GetGeneration 获取生成任务详情
// @Summary      获取生成任务详情
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "生成任务 ID"
// @Success      200  {object}  response.Response{data=Generation}
// @Router       /developmentmanager/generations/{id} [get]
// @Security     BearerAuth
func (h *Handler) GetGeneration(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.svc.GetGeneration(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// RetryGeneration 重试失败的生成任务
// @Summary      重试生成任务
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "生成任务 ID"
// @Success      200  {object}  response.Response{data=Generation}
// @Router       /developmentmanager/generations/{id}/retry [post]
// @Security     BearerAuth
func (h *Handler) RetryGeneration(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	existing, err := h.svc.GetGeneration(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if existing.AutoCommit && !h.ensureAutoCommitPermission(c) {
		return
	}
	item, err := h.svc.RetryGeneration(c.Request.Context(), id, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "update", "developmentmanager:generation", "retry id="+strconv.FormatInt(id, 10))
	response.Success(c, item)
}
