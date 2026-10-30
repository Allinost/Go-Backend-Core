package developmentmanager

import (
	"strconv"

	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// ---------- 容器 ----------

// ListContainers 获取项目容器列表
// @Summary      获取项目容器列表
// @Tags         developmentManager
// @Produce      json
// @Param        id       path   int   true   "项目 ID"
// @Param        deleted  query  bool  false  "是否查询已删除（默认否）"
// @Success      200  {object}  response.Response{data=[]Container}
// @Router       /developmentmanager/projects/{id}/containers [get]
// @Security     BearerAuth
func (h *Handler) ListContainers(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	includeDeleted := c.Query("deleted") == "1" || c.Query("deleted") == "true"
	items, err := h.svc.ListContainers(c.Request.Context(), projectID, includeDeleted)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []Container{}
	}
	response.Success(c, items)
}

// CreateContainer 新建容器
// @Summary      新建容器
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int                    true  "项目 ID"
// @Param        body  body  CreateContainerReq     true  "容器信息"
// @Success      200  {object}  response.Response{data=Container}
// @Router       /developmentmanager/projects/{id}/containers [post]
// @Security     BearerAuth
func (h *Handler) CreateContainer(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req CreateContainerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.CreateContainer(c.Request.Context(), projectID, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "create", "developmentmanager:container", item.ContainerName)
	response.Success(c, item)
}

// GetContainer 获取容器详情
// @Summary      获取容器详情
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "容器 ID"
// @Success      200  {object}  response.Response{data=Container}
// @Router       /developmentmanager/containers/{id} [get]
// @Security     BearerAuth
func (h *Handler) GetContainer(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.svc.GetContainer(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// UpdateContainer 更新容器（全量）
// @Summary      更新容器
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int                    true  "容器 ID"
// @Param        body  body  UpdateContainerReq     true  "容器信息（全量）"
// @Success      200  {object}  response.Response{data=Container}
// @Router       /developmentmanager/containers/{id} [put]
// @Security     BearerAuth
func (h *Handler) UpdateContainer(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req UpdateContainerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.UpdateContainer(c.Request.Context(), id, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "update", "developmentmanager:container", item.ContainerName)
	response.Success(c, item)
}

// DeleteContainer 删除容器（软删除，级联标记卷与接入点）
// @Summary      删除容器
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "容器 ID"
// @Success      200  {object}  response.Response
// @Router       /developmentmanager/containers/{id} [delete]
// @Security     BearerAuth
func (h *Handler) DeleteContainer(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteContainer(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "delete", "developmentmanager:container", "id="+strconv.FormatInt(id, 10))
	response.Success(c, gin.H{})
}

// ---------- 资源分配 ----------

// GetResourceAlloc 获取容器资源分配
// @Summary      获取容器资源分配
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "容器 ID"
// @Success      200  {object}  response.Response{data=ResourceAlloc}
// @Router       /developmentmanager/containers/{id}/resources [get]
// @Security     BearerAuth
func (h *Handler) GetResourceAlloc(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.svc.GetResourceAlloc(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// UpsertResourceAlloc 写入容器资源分配（1:1）
// @Summary      写入容器资源分配
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int                 true  "容器 ID"
// @Param        body  body  UpsertResourceReq   true  "资源分配"
// @Success      200  {object}  response.Response{data=ResourceAlloc}
// @Router       /developmentmanager/containers/{id}/resources [put]
// @Security     BearerAuth
func (h *Handler) UpsertResourceAlloc(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req UpsertResourceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.UpsertResourceAlloc(c.Request.Context(), id, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "update", "developmentmanager:resource", "container_id="+strconv.FormatInt(id, 10))
	response.Success(c, item)
}

// ---------- 卷映射 ----------

// ListVolumes 获取容器卷映射
// @Summary      获取容器卷映射
// @Tags         developmentManager
// @Produce      json
// @Param        id       path   int   true   "容器 ID"
// @Param        deleted  query  bool  false  "是否查询已删除（默认否）"
// @Success      200  {object}  response.Response{data=[]VolumeMapping}
// @Router       /developmentmanager/containers/{id}/volumes [get]
// @Security     BearerAuth
func (h *Handler) ListVolumes(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	includeDeleted := c.Query("deleted") == "1" || c.Query("deleted") == "true"
	items, err := h.svc.ListVolumes(c.Request.Context(), id, includeDeleted)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []VolumeMapping{}
	}
	response.Success(c, items)
}

// CreateVolume 新建卷映射
// @Summary      新建卷映射
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int               true  "容器 ID"
// @Param        body  body  CreateVolumeReq   true  "卷映射信息"
// @Success      200  {object}  response.Response{data=VolumeMapping}
// @Router       /developmentmanager/containers/{id}/volumes [post]
// @Security     BearerAuth
func (h *Handler) CreateVolume(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req CreateVolumeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.CreateVolume(c.Request.Context(), id, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "create", "developmentmanager:volume", item.HostPath+" -> "+item.ContainerPath)
	response.Success(c, item)
}

// UpdateVolume 更新卷映射
// @Summary      更新卷映射
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int               true  "卷映射 ID"
// @Param        body  body  UpdateVolumeReq   true  "卷映射信息（全量）"
// @Success      200  {object}  response.Response{data=VolumeMapping}
// @Router       /developmentmanager/volumes/{id} [put]
// @Security     BearerAuth
func (h *Handler) UpdateVolume(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req UpdateVolumeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.UpdateVolume(c.Request.Context(), id, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "update", "developmentmanager:volume", "id="+strconv.FormatInt(id, 10))
	response.Success(c, item)
}

// DeleteVolume 删除卷映射（软删除）
// @Summary      删除卷映射
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "卷映射 ID"
// @Success      200  {object}  response.Response
// @Router       /developmentmanager/volumes/{id} [delete]
// @Security     BearerAuth
func (h *Handler) DeleteVolume(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteVolume(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "delete", "developmentmanager:volume", "id="+strconv.FormatInt(id, 10))
	response.Success(c, gin.H{})
}

// ---------- 网络接入点 ----------

// ListAccessPoints 获取项目网络接入点
// @Summary      获取项目网络接入点
// @Tags         developmentManager
// @Produce      json
// @Param        id            path   int    true   "项目 ID"
// @Param        exposure      query  string false  "暴露范围 none/intranet/extranet"
// @Param        network_zone  query  string false  "网络区域 intranet/extranet/proxy_net"
// @Param        deleted       query  bool   false  "是否查询已删除（默认否）"
// @Success      200  {object}  response.Response{data=[]AccessPoint}
// @Router       /developmentmanager/projects/{id}/access-points [get]
// @Security     BearerAuth
func (h *Handler) ListAccessPoints(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req ListDeploymentReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	items, err := h.svc.ListAccessPoints(c.Request.Context(), projectID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []AccessPoint{}
	}
	response.Success(c, items)
}

// CreateAccessPoint 新建网络接入点
// @Summary      新建网络接入点
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int                     true  "项目 ID"
// @Param        body  body  CreateAccessPointReq    true  "接入点信息"
// @Success      200  {object}  response.Response{data=AccessPoint}
// @Router       /developmentmanager/projects/{id}/access-points [post]
// @Security     BearerAuth
func (h *Handler) CreateAccessPoint(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req CreateAccessPointReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.CreateAccessPoint(c.Request.Context(), projectID, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "create", "developmentmanager:access_point", "port="+strconv.Itoa(item.ListenPort))
	response.Success(c, item)
}

// GetAccessPoint 获取网络接入点详情
// @Summary      获取网络接入点详情
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "接入点 ID"
// @Success      200  {object}  response.Response{data=AccessPoint}
// @Router       /developmentmanager/access-points/{id} [get]
// @Security     BearerAuth
func (h *Handler) GetAccessPoint(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.svc.GetAccessPoint(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// UpdateAccessPoint 更新网络接入点（全量）
// @Summary      更新网络接入点
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int                     true  "接入点 ID"
// @Param        body  body  UpdateAccessPointReq    true  "接入点信息（全量）"
// @Success      200  {object}  response.Response{data=AccessPoint}
// @Router       /developmentmanager/access-points/{id} [put]
// @Security     BearerAuth
func (h *Handler) UpdateAccessPoint(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req UpdateAccessPointReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.UpdateAccessPoint(c.Request.Context(), id, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "update", "developmentmanager:access_point", "id="+strconv.FormatInt(id, 10))
	response.Success(c, item)
}

// DeleteAccessPoint 删除网络接入点（软删除）
// @Summary      删除网络接入点
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "接入点 ID"
// @Success      200  {object}  response.Response
// @Router       /developmentmanager/access-points/{id} [delete]
// @Security     BearerAuth
func (h *Handler) DeleteAccessPoint(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteAccessPoint(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "delete", "developmentmanager:access_point", "id="+strconv.FormatInt(id, 10))
	response.Success(c, gin.H{})
}

// ---------- 白名单 ----------

// ListWhitelists 获取项目白名单组
// @Summary      获取项目白名单组
// @Tags         developmentManager
// @Produce      json
// @Param        id       path   int   true   "项目 ID"
// @Param        deleted  query  bool  false  "是否查询已删除（默认否）"
// @Success      200  {object}  response.Response{data=[]Whitelist}
// @Router       /developmentmanager/projects/{id}/whitelists [get]
// @Security     BearerAuth
func (h *Handler) ListWhitelists(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	includeDeleted := c.Query("deleted") == "1" || c.Query("deleted") == "true"
	items, err := h.svc.ListWhitelists(c.Request.Context(), projectID, includeDeleted)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []Whitelist{}
	}
	response.Success(c, items)
}

// CreateWhitelist 新建白名单组（允许 0 成员）
// @Summary      新建白名单组
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int                  true  "项目 ID"
// @Param        body  body  CreateWhitelistReq   true  "白名单组信息"
// @Success      200  {object}  response.Response{data=Whitelist}
// @Router       /developmentmanager/projects/{id}/whitelists [post]
// @Security     BearerAuth
func (h *Handler) CreateWhitelist(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req CreateWhitelistReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.CreateWhitelist(c.Request.Context(), projectID, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "create", "developmentmanager:whitelist", item.Name)
	response.Success(c, item)
}

// GetWhitelist 获取白名单组详情（含成员数）
// @Summary      获取白名单组详情
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "白名单组 ID"
// @Success      200  {object}  response.Response{data=Whitelist}
// @Router       /developmentmanager/whitelists/{id} [get]
// @Security     BearerAuth
func (h *Handler) GetWhitelist(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.svc.GetWhitelist(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// UpdateWhitelist 更新白名单组
// @Summary      更新白名单组
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int                  true  "白名单组 ID"
// @Param        body  body  UpdateWhitelistReq   true  "白名单组信息（全量）"
// @Success      200  {object}  response.Response{data=Whitelist}
// @Router       /developmentmanager/whitelists/{id} [put]
// @Security     BearerAuth
func (h *Handler) UpdateWhitelist(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req UpdateWhitelistReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.UpdateWhitelist(c.Request.Context(), id, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "update", "developmentmanager:whitelist", item.Name)
	response.Success(c, item)
}

// DeleteWhitelist 删除白名单组
// @Summary      删除白名单组
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "白名单组 ID"
// @Success      200  {object}  response.Response
// @Router       /developmentmanager/whitelists/{id} [delete]
// @Security     BearerAuth
func (h *Handler) DeleteWhitelist(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteWhitelist(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "delete", "developmentmanager:whitelist", "id="+strconv.FormatInt(id, 10))
	response.Success(c, gin.H{})
}

// ListWhitelistMembers 获取白名单成员（允许空组）
// @Summary      获取白名单成员
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "白名单组 ID"
// @Success      200  {object}  response.Response{data=[]WhitelistMember}
// @Router       /developmentmanager/whitelists/{id}/members [get]
// @Security     BearerAuth
func (h *Handler) ListWhitelistMembers(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	items, err := h.svc.ListWhitelistMembers(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []WhitelistMember{}
	}
	response.Success(c, items)
}

// AddWhitelistMember 添加白名单成员
// @Summary      添加白名单成员
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int                        true  "白名单组 ID"
// @Param        body  body  CreateWhitelistMemberReq   true  "成员信息"
// @Success      200  {object}  response.Response{data=WhitelistMember}
// @Router       /developmentmanager/whitelists/{id}/members [post]
// @Security     BearerAuth
func (h *Handler) AddWhitelistMember(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req CreateWhitelistMemberReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.AddWhitelistMember(c.Request.Context(), id, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "create", "developmentmanager:whitelist_member",
		"whitelist="+strconv.FormatInt(id, 10)+" container="+strconv.FormatInt(req.ContainerID, 10))
	response.Success(c, item)
}

// RemoveWhitelistMember 移除白名单成员
// @Summary      移除白名单成员
// @Tags         developmentManager
// @Produce      json
// @Param        id         path  int  true  "白名单组 ID"
// @Param        member_id  path  int  true  "成员 ID"
// @Success      200  {object}  response.Response
// @Router       /developmentmanager/whitelists/{id}/members/{member_id} [delete]
// @Security     BearerAuth
func (h *Handler) RemoveWhitelistMember(c *gin.Context) {
	whitelistID, ok := parseID(c, "id")
	if !ok {
		return
	}
	memberID, ok := parseID(c, "member_id")
	if !ok {
		return
	}
	if err := h.svc.RemoveWhitelistMember(c.Request.Context(), whitelistID, memberID); err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "delete", "developmentmanager:whitelist_member", "member_id="+strconv.FormatInt(memberID, 10))
	response.Success(c, gin.H{})
}

// ---------- 备份 ----------

// ListBackups 获取项目备份记录
// @Summary      获取项目备份记录
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "项目 ID"
// @Success      200  {object}  response.Response{data=[]Backup}
// @Router       /developmentmanager/projects/{id}/backups [get]
// @Security     BearerAuth
func (h *Handler) ListBackups(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	items, err := h.svc.ListBackups(c.Request.Context(), projectID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []Backup{}
	}
	response.Success(c, items)
}

// CreateBackup 新建备份记录
// @Summary      新建备份记录
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int               true  "项目 ID"
// @Param        body  body  CreateBackupReq   true  "备份信息"
// @Success      200  {object}  response.Response{data=Backup}
// @Router       /developmentmanager/projects/{id}/backups [post]
// @Security     BearerAuth
func (h *Handler) CreateBackup(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req CreateBackupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.CreateBackup(c.Request.Context(), projectID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "create", "developmentmanager:backup", item.BackupType)
	response.Success(c, item)
}

// GetBackup 获取备份记录详情
// @Summary      获取备份记录详情
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "备份 ID"
// @Success      200  {object}  response.Response{data=Backup}
// @Router       /developmentmanager/backups/{id} [get]
// @Security     BearerAuth
func (h *Handler) GetBackup(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.svc.GetBackup(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// UpdateBackup 更新备份记录（含最近执行结果回报）
// @Summary      更新备份记录
// @Tags         developmentManager
// @Accept       json
// @Produce      json
// @Param        id    path  int               true  "备份 ID"
// @Param        body  body  UpdateBackupReq   true  "备份信息"
// @Success      200  {object}  response.Response{data=Backup}
// @Router       /developmentmanager/backups/{id} [put]
// @Security     BearerAuth
func (h *Handler) UpdateBackup(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req UpdateBackupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.UpdateBackup(c.Request.Context(), id, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "update", "developmentmanager:backup", "id="+strconv.FormatInt(id, 10))
	response.Success(c, item)
}

// DeleteBackup 删除备份记录
// @Summary      删除备份记录
// @Tags         developmentManager
// @Produce      json
// @Param        id  path  int  true  "备份 ID"
// @Success      200  {object}  response.Response
// @Router       /developmentmanager/backups/{id} [delete]
// @Security     BearerAuth
func (h *Handler) DeleteBackup(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteBackup(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "delete", "developmentmanager:backup", "id="+strconv.FormatInt(id, 10))
	response.Success(c, gin.H{})
}
