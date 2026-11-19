package developmentmanager

import (
	"strconv"

	appErrors "github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *Handler) pmReady(c *gin.Context) bool {
	if h.pm == nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, "里程碑/任务服务未初始化"))
		return false
	}
	return true
}

// ---------- 里程碑 ----------

func (h *Handler) ListMilestones(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req ListMilestonesReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	items, hasMore, total, err := h.pm.ListMilestones(c.Request.Context(), projectID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []Milestone{}
	}
	response.Success(c, PaginatedResp[Milestone]{Items: items, HasMore: hasMore, Total: total})
}

func (h *Handler) CreateMilestone(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req CreateMilestoneReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.pm.CreateMilestone(c.Request.Context(), projectID, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "create", "developmentmanager:milestone", item.Version+" "+item.Name)
	response.Success(c, item)
}

func (h *Handler) GetMilestone(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.pm.GetMilestone(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *Handler) UpdateMilestone(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req UpdateMilestoneReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.pm.UpdateMilestone(c.Request.Context(), id, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "update", "developmentmanager:milestone", item.Version+" "+item.Name)
	response.Success(c, item)
}

func (h *Handler) DeleteMilestone(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.pm.DeleteMilestone(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "delete", "developmentmanager:milestone", "milestone:"+c.Param("id"))
	response.Success(c, gin.H{"deleted": true})
}

func (h *Handler) MilestoneProgress(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.pm.MilestoneProgress(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// ---------- 任务 ----------

func (h *Handler) ListTasks(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req ListTasksReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	items, hasMore, total, err := h.pm.ListTasks(c.Request.Context(), projectID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []Task{}
	}
	response.Success(c, PaginatedResp[Task]{Items: items, HasMore: hasMore, Total: total})
}

func (h *Handler) CreateTask(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req CreateTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.pm.CreateTask(c.Request.Context(), projectID, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "create", "developmentmanager:task", item.Code+" "+item.Title)
	response.Success(c, item)
}

func (h *Handler) GetTask(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.pm.GetTask(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *Handler) UpdateTask(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req UpdateTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.pm.UpdateTask(c.Request.Context(), id, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "update", "developmentmanager:task", item.Code+" "+item.Title)
	response.Success(c, item)
}

func (h *Handler) DeleteTask(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.pm.DeleteTask(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "delete", "developmentmanager:task", "task:"+c.Param("id"))
	response.Success(c, gin.H{"deleted": true})
}

func (h *Handler) ChangeTaskStatus(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req ChangeTaskStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.pm.ChangeTaskStatus(c.Request.Context(), id, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "update", "developmentmanager:task", item.Code+" status="+item.Status)
	response.Success(c, item)
}

func (h *Handler) ListTaskLogs(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	limit := 100
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	items, err := h.pm.ListTaskLogs(c.Request.Context(), id, limit)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []TaskLog{}
	}
	response.Success(c, gin.H{"items": items})
}

func (h *Handler) AddTaskLog(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req AddTaskLogReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.pm.AddTaskLog(c.Request.Context(), id, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "update", "developmentmanager:task", "task:"+c.Param("id")+" log")
	response.Success(c, item)
}

func (h *Handler) ListDependencies(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	items, err := h.pm.ListDependencies(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []TaskDependency{}
	}
	response.Success(c, gin.H{"items": items})
}

func (h *Handler) AddDependency(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req AddTaskDependencyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.pm.AddDependency(c.Request.Context(), id, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "update", "developmentmanager:task", "task:"+c.Param("id")+" dependency")
	response.Success(c, item)
}

func (h *Handler) RemoveDependency(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	taskID, ok := parseID(c, "id")
	if !ok {
		return
	}
	depID, ok := parseID(c, "dep_id")
	if !ok {
		return
	}
	if err := h.pm.RemoveDependency(c.Request.Context(), taskID, depID); err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "delete", "developmentmanager:task", "task:"+c.Param("id")+" dependency:"+c.Param("dep_id"))
	response.Success(c, gin.H{"deleted": true})
}
