package servicenav

import (
	"database/sql"
	"errors"

	appErr "github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
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

// ---------- 服务类型 ----------

// ListTypes 获取服务类型列表
// @Summary      获取服务类型列表
// @Tags         servicenav
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.Response{data=[]ServiceType}
// @Router       /servicenav/types [get]
func (h *Handler) ListTypes(c *gin.Context) {
	items, err := h.svc.ListTypes(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []ServiceType{}
	}
	response.Success(c, items)
}

// CreateType 创建服务类型
// @Summary      创建服务类型
// @Tags         servicenav
// @Accept       json
// @Produce      json
// @Param        body  body  CreateTypeReq  true  "服务类型"
// @Success      200  {object}  response.Response{data=ServiceType}
// @Router       /servicenav/types [post]
func (h *Handler) CreateType(c *gin.Context) {
	var req CreateTypeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	t, err := h.svc.CreateType(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, t)
}

// UpdateType 更新服务类型
// @Summary      更新服务类型
// @Tags         servicenav
// @Accept       json
// @Produce      json
// @Param        id    path  string          true  "服务类型 ID"
// @Param        body  body  UpdateTypeReq  true  "更新字段"
// @Success      200  {object}  response.Response{data=ServiceType}
// @Router       /servicenav/types/{id} [put]
func (h *Handler) UpdateType(c *gin.Context) {
	id := c.Param("id")
	var req UpdateTypeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	t, err := h.svc.UpdateType(c.Request.Context(), id, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, t)
}

// DeleteType 删除服务类型
// @Summary      删除服务类型
// @Tags         servicenav
// @Accept       json
// @Produce      json
// @Param        id   path  string  true  "服务类型 ID"
// @Success      200  {object}  response.Response
// @Router       /servicenav/types/{id} [delete]
func (h *Handler) DeleteType(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.DeleteType(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// ---------- 服务条目 ----------

// ListItems 获取服务条目列表（分页 + 模糊搜索 + 类型筛选 + 排序）
// @Summary      获取服务条目列表
// @Tags         servicenav
// @Accept       json
// @Produce      json
// @Param        page        query  int     false  "页码，默认 1"
// @Param        page_size   query  int     false  "每页数量，默认 20，最大 100"
// @Param        keyword     query  string  false  "搜索关键词（匹配服务名，模糊）"
// @Param        type        query  string  false  "按服务类型筛选"
// @Param        sort_by     query  string  false  "排序字段：name / created_at，默认 name"
// @Param        sort_order  query  string  false  "排序方向：asc / desc，默认 asc"
// @Success      200  {object}  response.Response{data=PaginatedResp[ServiceItem]}
// @Router       /servicenav/services [get]
func (h *Handler) ListItems(c *gin.Context) {
	var req ListItemsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	req.Normalize()
	items, hasMore, total, err := h.svc.ListItems(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []ServiceItem{}
	}
	response.Success(c, PaginatedResp[ServiceItem]{Items: items, HasMore: hasMore, Total: total})
}

// CreateItem 创建服务条目
// @Summary      创建服务条目
// @Tags         servicenav
// @Accept       json
// @Produce      json
// @Param        body  body  CreateItemReq  true  "服务条目"
// @Success      200  {object}  response.Response{data=ServiceItem}
// @Router       /servicenav/services [post]
func (h *Handler) CreateItem(c *gin.Context) {
	var req CreateItemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.CreateItem(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// GetItem 获取服务条目详情
// @Summary      获取服务条目详情
// @Tags         servicenav
// @Accept       json
// @Produce      json
// @Param        id   path  string  true  "服务条目 ID"
// @Success      200  {object}  response.Response{data=ServiceItem}
// @Router       /servicenav/services/{id} [get]
func (h *Handler) GetItem(c *gin.Context) {
	id := c.Param("id")
	item, err := h.svc.GetItem(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// UpdateItem 更新服务条目
// @Summary      更新服务条目
// @Tags         servicenav
// @Accept       json
// @Produce      json
// @Param        id    path  string          true  "服务条目 ID"
// @Param        body  body  UpdateItemReq  true  "更新字段"
// @Success      200  {object}  response.Response{data=ServiceItem}
// @Router       /servicenav/services/{id} [put]
func (h *Handler) UpdateItem(c *gin.Context) {
	id := c.Param("id")
	var req UpdateItemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.UpdateItem(c.Request.Context(), id, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// ImportFromProjects 从项目部署接入点快速导入服务导航
// @Summary      从项目部署接入点快速导入服务导航
// @Tags         servicenav
// @Accept       json
// @Produce      json
// @Param        body  body  ImportFromProjectsReq  true  "导入参数"
// @Success      200  {object}  response.Response{data=ImportFromProjectsResp}
// @Router       /servicenav/import [post]
func (h *Handler) ImportFromProjects(c *gin.Context) {
	var req ImportFromProjectsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.ImportFromProjects(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, resp)
}

// DeleteItem 删除服务条目
// @Summary      删除服务条目
// @Tags         servicenav
// @Accept       json
// @Produce      json
// @Param        id   path  string  true  "服务条目 ID"
// @Success      200  {object}  response.Response
// @Router       /servicenav/services/{id} [delete]
func (h *Handler) DeleteItem(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.DeleteItem(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// handleError 统一错误处理：NavError → 业务错误码，其余 → 500
func (h *Handler) handleError(c *gin.Context, err error) {
	var navErr *NavError
	if errors.As(err, &navErr) {
		logger.Error().Err(err).Int("nav_code", navErr.Code).Msg("servicenav 业务错误")
		response.Fail(c, appErr.New(navErr.Code, navErr.Message))
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		response.Fail(c, appErr.New(appErr.CodeNotFound, "资源不存在"))
		return
	}
	logger.Error().Err(err).Msg("servicenav 服务内部错误")
	response.Fail(c, appErr.New(appErr.CodeSystemErr, "服务内部错误"))
}
