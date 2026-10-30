package developmentmanager

import (
	"strconv"

	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// ---------- 数据表 ----------

// ListTables 获取项目数据表字典
// @Summary      获取项目数据表字典
// @Tags         developmentManager
// @Produce      json
// @Param        id           path   int    true   "项目 ID"
// @Param        status       query  string false  "状态 active/deprecated/dropped"
// @Param        module_name  query  string false  "归属模块"
// @Param        keyword      query  string false  "关键词（匹配表名/用途）"
// @Success      200  {object}  response.Response{data=[]DBTable}
// @Router       /developmentmanager/projects/{id}/tables [get]
// @Security     BearerAuth
func (h *Handler) ListTables(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req ListTablesReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	items, err := h.svc.ListTables(c.Request.Context(), projectID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []DBTable{}
	}
	response.Success(c, items)
}

// CreateTable 登记数据表
// @Summary      登记数据表
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int              true  "项目 ID"
// @Param        body  body  CreateTableReq   true  "数据表信息"
// @Success      200  {object}  response.Response{data=DBTable}
// @Router       /developmentmanager/projects/{id}/tables [post]
// @Security     BearerAuth
func (h *Handler) CreateTable(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req CreateTableReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.CreateTable(c.Request.Context(), projectID, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "create", "developmentmanager:db_table", item.TableName)
	response.Success(c, item)
}

// GetTable 获取数据表详情
// @Summary      获取数据表详情
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "数据表 ID"
// @Success      200  {object}  response.Response{data=DBTable}
// @Router       /developmentmanager/tables/{id} [get]
// @Security     BearerAuth
func (h *Handler) GetTable(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.svc.GetTable(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// UpdateTable 更新数据表（全量）
// @Summary      更新数据表
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int              true  "数据表 ID"
// @Param        body  body  UpdateTableReq   true  "数据表信息（全量）"
// @Success      200  {object}  response.Response{data=DBTable}
// @Router       /developmentmanager/tables/{id} [put]
// @Security     BearerAuth
func (h *Handler) UpdateTable(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req UpdateTableReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.UpdateTable(c.Request.Context(), id, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "update", "developmentmanager:db_table", item.TableName)
	response.Success(c, item)
}

// DeleteTable 删除数据表登记
// @Summary      删除数据表登记
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "数据表 ID"
// @Success      200  {object}  response.Response
// @Router       /developmentmanager/tables/{id} [delete]
// @Security     BearerAuth
func (h *Handler) DeleteTable(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteTable(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "delete", "developmentmanager:db_table", "id="+strconv.FormatInt(id, 10))
	response.Success(c, gin.H{})
}

// ---------- 字段 ----------

// ListColumns 获取表字段字典
// @Summary      获取表字段字典
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "数据表 ID"
// @Success      200  {object}  response.Response{data=[]DBColumn}
// @Router       /developmentmanager/tables/{id}/columns [get]
// @Security     BearerAuth
func (h *Handler) ListColumns(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	items, err := h.svc.ListColumns(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []DBColumn{}
	}
	response.Success(c, items)
}

// CreateColumn 新增字段
// @Summary      新增字段
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int               true  "数据表 ID"
// @Param        body  body  CreateColumnReq   true  "字段信息"
// @Success      200  {object}  response.Response{data=DBColumn}
// @Router       /developmentmanager/tables/{id}/columns [post]
// @Security     BearerAuth
func (h *Handler) CreateColumn(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req CreateColumnReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.CreateColumn(c.Request.Context(), id, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "create", "developmentmanager:db_column", "table="+strconv.FormatInt(id, 10)+" column="+item.ColumnName)
	response.Success(c, item)
}

// UpdateColumn 更新字段（全量）
// @Summary      更新字段
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int               true  "字段 ID"
// @Param        body  body  UpdateColumnReq   true  "字段信息（全量）"
// @Success      200  {object}  response.Response{data=DBColumn}
// @Router       /developmentmanager/columns/{id} [put]
// @Security     BearerAuth
func (h *Handler) UpdateColumn(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req UpdateColumnReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.UpdateColumn(c.Request.Context(), id, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "update", "developmentmanager:db_column", "id="+strconv.FormatInt(id, 10))
	response.Success(c, item)
}

// DeleteColumn 删除字段
// @Summary      删除字段
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "字段 ID"
// @Success      200  {object}  response.Response
// @Router       /developmentmanager/columns/{id} [delete]
// @Security     BearerAuth
func (h *Handler) DeleteColumn(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteColumn(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "delete", "developmentmanager:db_column", "id="+strconv.FormatInt(id, 10))
	response.Success(c, gin.H{})
}

// ScanImportColumns 从 information_schema 扫描导入字段初稿
// @Summary      从库扫描导入字段初稿
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "数据表 ID"
// @Success      200  {object}  response.Response{data=ScanImportResp}
// @Router       /developmentmanager/tables/{id}/scan-import [post]
// @Security     BearerAuth
func (h *Handler) ScanImportColumns(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.svc.ScanImportColumns(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "update", "developmentmanager:db_column", "scan_import table="+strconv.FormatInt(id, 10))
	response.Success(c, item)
}
