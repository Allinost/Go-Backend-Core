package goodser

import (
	"strconv"
	"strings"

	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// --- 字典 ---

// ListDictionariesREST 查询字典项
// @Summary      查询受控维度字典
// @Tags         zzz-goodser
// @Param        kind     query  string  false  "series/character/set_type/product_type"
// @Param        enabled  query  bool    false  "是否启用"
// @Success      200  {object}  response.Response{data=[]Dictionary}
// @Router       /zzz-goodser/settings/dictionaries [get]
func (h *Handler) ListDictionariesREST(c *gin.Context) {
	req := &ListDictionariesReq{Kind: strings.TrimSpace(c.Query("kind"))}
	if raw := c.Query("enabled"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			response.ParamErr(c, "enabled 参数错误")
			return
		}
		req.Enabled = &value
	}
	items, err := h.svc.ListDictionaries(c.Request.Context(), req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

// CreateDictionaryREST 创建字典项
// @Summary      创建受控维度字典
// @Tags         zzz-goodser
// @Param        body  body  CreateDictionaryReq  true  "字典内容"
// @Success      200  {object}  response.Response{data=Dictionary}
// @Router       /zzz-goodser/settings/dictionaries [post]
func (h *Handler) CreateDictionaryREST(c *gin.Context) {
	var req CreateDictionaryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	item, err := h.svc.CreateDictionary(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// UpdateDictionaryREST 更新字典项
// @Summary      更新受控维度字典
// @Tags         zzz-goodser
// @Param        did   path  string               true  "字典 ID"
// @Param        body  body  UpdateDictionaryReq  true  "更新字段"
// @Success      200  {object}  response.Response{data=Dictionary}
// @Router       /zzz-goodser/settings/dictionaries/{did} [put]
func (h *Handler) UpdateDictionaryREST(c *gin.Context) {
	var req UpdateDictionaryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	req.ID = c.Param("did")
	item, err := h.svc.UpdateDictionary(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// DeleteDictionaryREST 删除字典项
// @Summary      删除受控维度字典
// @Tags         zzz-goodser
// @Param        did  path  string  true  "字典 ID"
// @Success      200  {object}  response.Response
// @Router       /zzz-goodser/settings/dictionaries/{did} [delete]
func (h *Handler) DeleteDictionaryREST(c *gin.Context) {
	if err := h.svc.DeleteDictionary(c.Request.Context(), c.Param("did")); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// --- 系列视图 ---

// ListSeriesSummaryREST 系列聚合视图
// @Summary      系列聚合视图
// @Tags         zzz-goodser
// @Param        id  path  string  true  "库存目录 ID"
// @Success      200  {object}  response.Response{data=[]SeriesSummary}
// @Router       /zzz-goodser/inventories/{id}/series [get]
func (h *Handler) ListSeriesSummaryREST(c *gin.Context) {
	items, err := h.svc.ListSeriesSummary(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

// --- 组合包 ---

// ListCombosREST 组合包列表
// @Summary      组合包列表
// @Tags         zzz-goodser
// @Param        id      path   string  true   "库存目录 ID"
// @Param        status  query  string  false  "active/inactive"
// @Success      200  {object}  response.Response{data=[]Combo}
// @Router       /zzz-goodser/inventories/{id}/combos [get]
func (h *Handler) ListCombosREST(c *gin.Context) {
	items, err := h.svc.ListCombos(c.Request.Context(), c.Param("id"), strings.TrimSpace(c.Query("status")))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

// CreateComboREST 创建组合包
// @Summary      创建组合包
// @Tags         zzz-goodser
// @Param        id    path  string          true  "库存目录 ID"
// @Param        body  body  CreateComboReq  true  "组合包内容"
// @Success      200  {object}  response.Response{data=Combo}
// @Router       /zzz-goodser/inventories/{id}/combos [post]
func (h *Handler) CreateComboREST(c *gin.Context) {
	var req CreateComboReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	req.InventoryID = c.Param("id")
	combo, err := h.svc.CreateCombo(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, combo)
}

// GetComboREST 组合包详情
// @Summary      组合包详情
// @Tags         zzz-goodser
// @Param        cid  path  string  true  "组合包 ID"
// @Success      200  {object}  response.Response{data=Combo}
// @Router       /zzz-goodser/combos/{cid} [get]
func (h *Handler) GetComboREST(c *gin.Context) {
	combo, err := h.svc.GetCombo(c.Request.Context(), c.Param("cid"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, combo)
}

// UpdateComboREST 更新组合包
// @Summary      更新组合包
// @Tags         zzz-goodser
// @Param        cid   path  string          true  "组合包 ID"
// @Param        body  body  UpdateComboReq  true  "更新字段"
// @Success      200  {object}  response.Response{data=Combo}
// @Router       /zzz-goodser/combos/{cid} [put]
func (h *Handler) UpdateComboREST(c *gin.Context) {
	var req UpdateComboReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	req.ID = c.Param("cid")
	combo, err := h.svc.UpdateCombo(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, combo)
}

// DeleteComboREST 删除组合包
// @Summary      删除组合包
// @Tags         zzz-goodser
// @Param        cid  path  string  true  "组合包 ID"
// @Success      200  {object}  response.Response
// @Router       /zzz-goodser/combos/{cid} [delete]
func (h *Handler) DeleteComboREST(c *gin.Context) {
	if err := h.svc.DeleteCombo(c.Request.Context(), c.Param("cid")); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// --- 客户服务 ---

// CustomerStockQueryREST 客户只读库存查询
// @Summary      客户库存查询
// @Tags         zzz-goodser-customer
// @Param        body  body  CustomerStockQueryReq  true  "查询条件"
// @Success      200  {object}  response.Response{data=CustomerStockQueryResp}
// @Router       /zzz-goodser/customer/stock-query [post]
func (h *Handler) CustomerStockQueryREST(c *gin.Context) {
	var req CustomerStockQueryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	result, err := h.svc.CustomerStockQuery(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, result)
}

// CustomerOrderRequestREST 客户提交订单请求
// @Summary      客户提交订单请求
// @Description  创建 pending 预留并锁定库存，异步通知店主
// @Tags         zzz-goodser-customer
// @Param        body  body  CustomerOrderRequestReq  true  "订单请求"
// @Success      200  {object}  response.Response{data=CustomerOrderRequestResp}
// @Router       /zzz-goodser/customer/order-requests [post]
func (h *Handler) CustomerOrderRequestREST(c *gin.Context) {
	var req CustomerOrderRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	result, err := h.svc.CustomerOrderRequest(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, result)
}
