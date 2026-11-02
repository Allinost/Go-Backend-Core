package goodser

import (
	"strconv"
	"strings"

	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// ListBundlesREST 捆模板列表
// @Summary      捆模板列表
// @Tags         zzz-goodser
// @Param        id      path   string  true   "库存目录 ID"
// @Param        status  query  string  false  "active/inactive"
// @Success      200  {object}  response.Response{data=[]Bundle}
// @Router       /zzz-goodser/inventories/{id}/bundles [get]
func (h *Handler) ListBundlesREST(c *gin.Context) {
	items, err := h.svc.ListBundles(c.Request.Context(), c.Param("id"), strings.TrimSpace(c.Query("status")))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

// CreateBundleREST 创建捆模板
// @Summary      创建捆模板
// @Tags         zzz-goodser
// @Param        id    path  string           true  "库存目录 ID"
// @Param        body  body  CreateBundleReq  true  "捆内容"
// @Success      200  {object}  response.Response{data=Bundle}
// @Router       /zzz-goodser/inventories/{id}/bundles [post]
func (h *Handler) CreateBundleREST(c *gin.Context) {
	var req CreateBundleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	req.InventoryID = c.Param("id")
	bundle, err := h.svc.CreateBundle(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, bundle)
}

// GetBundleREST 捆模板详情
// @Summary      捆模板详情
// @Tags         zzz-goodser
// @Param        bid  path  string  true  "捆 ID"
// @Success      200  {object}  response.Response{data=Bundle}
// @Router       /zzz-goodser/bundles/{bid} [get]
func (h *Handler) GetBundleREST(c *gin.Context) {
	bundle, err := h.svc.GetBundle(c.Request.Context(), c.Param("bid"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, bundle)
}

// UpdateBundleREST 更新捆模板
// @Summary      更新捆模板
// @Tags         zzz-goodser
// @Param        bid   path  string           true  "捆 ID"
// @Param        body  body  UpdateBundleReq  true  "更新字段"
// @Success      200  {object}  response.Response{data=Bundle}
// @Router       /zzz-goodser/bundles/{bid} [put]
func (h *Handler) UpdateBundleREST(c *gin.Context) {
	var req UpdateBundleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	req.ID = c.Param("bid")
	bundle, err := h.svc.UpdateBundle(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, bundle)
}

// DeleteBundleREST 删除捆模板
// @Summary      删除捆模板
// @Tags         zzz-goodser
// @Param        bid  path  string  true  "捆 ID"
// @Success      200  {object}  response.Response
// @Router       /zzz-goodser/bundles/{bid} [delete]
func (h *Handler) DeleteBundleREST(c *gin.Context) {
	if err := h.svc.DeleteBundle(c.Request.Context(), c.Param("bid")); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// ListBundleMainProductsREST 捆允许的主商品
// @Summary      捆允许的主商品
// @Tags         zzz-goodser
// @Param        bid      path   string  true   "捆 ID"
// @Param        keyword  query  string  false  "名称或编码关键词"
// @Success      200  {object}  response.Response{data=[]BundleMainProduct}
// @Router       /zzz-goodser/bundles/{bid}/main-products [get]
func (h *Handler) ListBundleMainProductsREST(c *gin.Context) {
	items, err := h.svc.ListBundleMainProducts(c.Request.Context(), c.Param("bid"), strings.TrimSpace(c.Query("keyword")))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

// ListBundleSubProductsREST 捆允许的副区商品
// @Summary      捆允许的副区商品
// @Tags         zzz-goodser
// @Param        bid             path   string  true   "捆 ID"
// @Param        keyword         query  string  false  "名称或编码关键词"
// @Param        only_available  query  bool    false  "仅看有库存"
// @Success      200  {object}  response.Response{data=[]BundleSubProduct}
// @Router       /zzz-goodser/bundles/{bid}/sub-products [get]
func (h *Handler) ListBundleSubProductsREST(c *gin.Context) {
	onlyAvailable := false
	if raw := c.Query("only_available"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			response.ParamErr(c, "only_available 参数错误")
			return
		}
		onlyAvailable = value
	}
	items, err := h.svc.ListBundleSubProducts(c.Request.Context(), c.Param("bid"),
		strings.TrimSpace(c.Query("keyword")), onlyAvailable)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

// PreviewBundleREST 校验捆选择并计算价格
// @Summary      捆下单预览
// @Description  校验主商品范围、副区商品和金额数量规则，不改变库存
// @Tags         zzz-goodser
// @Param        bid   path  string            true  "捆 ID"
// @Param        body  body  BundlePreviewReq  true  "选择内容"
// @Success      200  {object}  response.Response{data=BundlePreviewResp}
// @Router       /zzz-goodser/bundles/{bid}/preview [post]
func (h *Handler) PreviewBundleREST(c *gin.Context) {
	var req BundlePreviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	result, err := h.svc.PreviewBundle(c.Request.Context(), c.Param("bid"), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, result)
}
