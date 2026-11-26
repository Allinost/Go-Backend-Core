package goodser

import (
	"strconv"

	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// PreviewExport 计算平铺图导出的价格与布局摘要。
// @Summary      平铺图导出预览
// @Tags         zzz-goodser
// @Accept       json
// @Produce      json
// @Param        id    path  string         true  "目录 ID"
// @Param        body  body  ExportRequest  true  "导出参数"
// @Success      200  {object}  response.Response{data=ExportPreview}
// @Router       /zzz-goodser/inventories/{id}/exports/preview [post]
func (h *Handler) PreviewExport(c *gin.Context) {
	var req ExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	preview, err := h.svc.PreviewExport(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, preview)
}

// CreateExport 创建平铺图导出任务。
// @Summary      创建平铺图导出
// @Tags         zzz-goodser
// @Accept       json
// @Produce      json
// @Param        id    path  string         true  "目录 ID"
// @Param        body  body  ExportRequest  true  "导出参数"
// @Success      200  {object}  response.Response{data=ExportJob}
// @Router       /zzz-goodser/inventories/{id}/exports [post]
func (h *Handler) CreateExport(c *gin.Context) {
	var req ExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	job, err := h.svc.CreateExport(c.Request.Context(), c.Param("id"), &req, int64(c.GetUint("user_id")))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, job)
}

// GetExport 查询导出任务。
// @Summary      查询导出任务
// @Tags         zzz-goodser
// @Produce      json
// @Param        eid  path  string  true  "导出 ID"
// @Success      200  {object}  response.Response{data=ExportJob}
// @Router       /zzz-goodser/exports/{eid} [get]
func (h *Handler) GetExport(c *gin.Context) {
	job, err := h.svc.GetExport(c.Request.Context(), c.Param("eid"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, job)
}

// ListExports 查询导出历史。
// @Summary      导出历史
// @Tags         zzz-goodser
// @Produce      json
// @Param        id     path   string  true   "目录 ID"
// @Param        limit  query  int     false  "条数，默认 20"
// @Success      200  {object}  response.Response{data=[]ExportJob}
// @Router       /zzz-goodser/inventories/{id}/exports [get]
func (h *Handler) ListExports(c *gin.Context) {
	limit := 20
	if raw := c.Query("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			limit = v
		}
	}
	items, err := h.svc.ListExports(c.Request.Context(), c.Param("id"), limit)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []ExportJob{}
	}
	response.Success(c, items)
}

// DeleteExport 删除导出记录。
// @Summary      删除导出记录
// @Tags         zzz-goodser
// @Produce      json
// @Param        eid  path  string  true  "导出 ID"
// @Success      200  {object}  response.Response
// @Router       /zzz-goodser/exports/{eid} [delete]
func (h *Handler) DeleteExport(c *gin.Context) {
	if err := h.svc.DeleteExport(c.Request.Context(), c.Param("eid")); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}
