package developmentmanager

import (
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// ---------- Iteration ----------

func (h *Handler) ListIterations(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req ListProjectItemsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	items, hasMore, total, err := h.pm.ListIterations(c.Request.Context(), projectID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []Iteration{}
	}
	response.Success(c, PaginatedResp[Iteration]{Items: items, HasMore: hasMore, Total: total})
}

func (h *Handler) CreateIteration(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req CreateIterationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.pm.CreateIteration(c.Request.Context(), projectID, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "create", "developmentmanager:iteration", item.Name)
	response.Success(c, item)
}

func (h *Handler) GetIteration(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.pm.GetIteration(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *Handler) UpdateIteration(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req UpdateIterationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.pm.UpdateIteration(c.Request.Context(), id, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "update", "developmentmanager:iteration", item.Name)
	response.Success(c, item)
}

func (h *Handler) DeleteIteration(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.pm.DeleteIteration(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "delete", "developmentmanager:iteration", "iteration:"+c.Param("id"))
	response.Success(c, gin.H{"deleted": true})
}

// ---------- Risk ----------

func (h *Handler) ListRisks(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req ListProjectItemsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	items, hasMore, total, err := h.pm.ListRisks(c.Request.Context(), projectID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []Risk{}
	}
	response.Success(c, PaginatedResp[Risk]{Items: items, HasMore: hasMore, Total: total})
}

func (h *Handler) CreateRisk(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req CreateRiskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.pm.CreateRisk(c.Request.Context(), projectID, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "create", "developmentmanager:risk", item.Title)
	response.Success(c, item)
}

func (h *Handler) GetRisk(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.pm.GetRisk(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *Handler) UpdateRisk(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req UpdateRiskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.pm.UpdateRisk(c.Request.Context(), id, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "update", "developmentmanager:risk", item.Title)
	response.Success(c, item)
}

func (h *Handler) DeleteRisk(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.pm.DeleteRisk(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "delete", "developmentmanager:risk", "risk:"+c.Param("id"))
	response.Success(c, gin.H{"deleted": true})
}

// ---------- Release ----------

func (h *Handler) ListReleases(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req ListProjectItemsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	items, hasMore, total, err := h.pm.ListReleases(c.Request.Context(), projectID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []Release{}
	}
	response.Success(c, PaginatedResp[Release]{Items: items, HasMore: hasMore, Total: total})
}

func (h *Handler) CreateRelease(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req CreateReleaseReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.pm.CreateRelease(c.Request.Context(), projectID, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "create", "developmentmanager:release", item.Version+" "+item.Name)
	response.Success(c, item)
}

func (h *Handler) GetRelease(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.pm.GetRelease(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *Handler) UpdateRelease(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req UpdateReleaseReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.pm.UpdateRelease(c.Request.Context(), id, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "update", "developmentmanager:release", item.Version+" "+item.Name)
	response.Success(c, item)
}

func (h *Handler) DeleteRelease(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.pm.DeleteRelease(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "delete", "developmentmanager:release", "release:"+c.Param("id"))
	response.Success(c, gin.H{"deleted": true})
}

// ---------- Baseline ----------

func (h *Handler) ListBaselines(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	items, err := h.pm.ListBaselines(c.Request.Context(), projectID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []Baseline{}
	}
	response.Success(c, gin.H{"items": items})
}

func (h *Handler) CreateBaseline(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req CreateBaselineReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.pm.CreateBaseline(c.Request.Context(), projectID, &req, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "create", "developmentmanager:baseline", item.Name)
	response.Success(c, item)
}

func (h *Handler) GetBaseline(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.pm.GetBaseline(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *Handler) DeleteBaseline(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.pm.DeleteBaseline(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, "delete", "developmentmanager:baseline", "baseline:"+c.Param("id"))
	response.Success(c, gin.H{"deleted": true})
}

// ---------- Comment / checklist ----------

func (h *Handler) ListTaskComments(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	items, err := h.pm.ListTaskComments(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []Comment{}
	}
	response.Success(c, gin.H{"items": items})
}

func (h *Handler) AddTaskComment(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req CreateTaskCommentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.pm.AddTaskComment(c.Request.Context(), id, req.Content, actorName(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *Handler) DeleteComment(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.pm.DeleteComment(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *Handler) ListChecklist(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	items, err := h.pm.ListChecklist(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []ChecklistItem{}
	}
	response.Success(c, gin.H{"items": items})
}

func (h *Handler) CreateChecklistItem(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req CreateChecklistReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.pm.CreateChecklistItem(c.Request.Context(), id, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *Handler) UpdateChecklistItem(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req UpdateChecklistReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	if err := h.pm.UpdateChecklistItem(c.Request.Context(), id, &req); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"updated": true})
}

func (h *Handler) DeleteChecklistItem(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.pm.DeleteChecklistItem(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// ---------- Label / task label / task asset ----------

func (h *Handler) ListLabels(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	items, err := h.pm.ListLabels(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []Label{}
	}
	response.Success(c, gin.H{"items": items})
}

func (h *Handler) CreateLabel(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req CreateLabelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.pm.CreateLabel(c.Request.Context(), id, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *Handler) DeleteLabel(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.pm.DeleteLabel(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *Handler) ListTaskLabels(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	items, err := h.pm.ListTaskLabels(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []Label{}
	}
	response.Success(c, gin.H{"items": items})
}

func (h *Handler) AddTaskLabel(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req AddTaskLabelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.pm.AddTaskLabel(c.Request.Context(), id, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *Handler) RemoveTaskLabel(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	labelID, ok := parseID(c, "label_id")
	if !ok {
		return
	}
	if err := h.pm.RemoveTaskLabel(c.Request.Context(), id, labelID); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *Handler) ListTaskAssets(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	items, err := h.pm.ListTaskAssets(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []TaskAsset{}
	}
	response.Success(c, gin.H{"items": items})
}

func (h *Handler) AddTaskAsset(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req AddTaskAssetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.pm.AddTaskAsset(c.Request.Context(), id, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *Handler) RemoveTaskAsset(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	assetID, ok := parseID(c, "asset_id")
	if !ok {
		return
	}
	if err := h.pm.RemoveTaskAsset(c.Request.Context(), id, assetID); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}
