package goodser

import (
	"strings"

	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// ListBundlePoolsREST 捆物池列表
// @Summary      捆物池列表
// @Tags         zzz-goodser
// @Param        inventory_id  query  string  true   "库存目录 ID"
// @Param        status        query  string  false  "active/inactive"
// @Success      200  {object}  response.Response{data=[]BundlePool}
// @Router       /zzz-goodser/bundle-pools [get]
func (h *Handler) ListBundlePoolsREST(c *gin.Context) {
	inventoryID := strings.TrimSpace(c.Query("inventory_id"))
	items, err := h.svc.ListBundlePools(c.Request.Context(), inventoryID, strings.TrimSpace(c.Query("status")))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

// CreateBundlePoolREST 创建捆物池
// @Summary      创建捆物池
// @Tags         zzz-goodser
// @Param        body  body  CreateBundlePoolReq  true  "捆物池内容"
// @Success      200  {object}  response.Response{data=BundlePool}
// @Router       /zzz-goodser/bundle-pools [post]
func (h *Handler) CreateBundlePoolREST(c *gin.Context) {
	var req CreateBundlePoolReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	pool, err := h.svc.CreateBundlePool(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, pool)
}

// GetBundlePoolREST 捆物池详情
// @Summary      捆物池详情
// @Tags         zzz-goodser
// @Param        pid  path  string  true  "捆物池 ID"
// @Success      200  {object}  response.Response{data=BundlePool}
// @Router       /zzz-goodser/bundle-pools/{pid} [get]
func (h *Handler) GetBundlePoolREST(c *gin.Context) {
	pool, err := h.svc.GetBundlePool(c.Request.Context(), c.Param("pid"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, pool)
}

// UpdateBundlePoolREST 更新捆物池
// @Summary      更新捆物池
// @Tags         zzz-goodser
// @Param        pid   path  string                true  "捆物池 ID"
// @Param        body  body  UpdateBundlePoolReq   true  "更新字段"
// @Success      200  {object}  response.Response{data=BundlePool}
// @Router       /zzz-goodser/bundle-pools/{pid} [put]
func (h *Handler) UpdateBundlePoolREST(c *gin.Context) {
	var req UpdateBundlePoolReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	req.ID = c.Param("pid")
	pool, err := h.svc.UpdateBundlePool(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, pool)
}

// DeleteBundlePoolREST 删除捆物池
// @Summary      删除捆物池
// @Tags         zzz-goodser
// @Param        pid  path  string  true  "捆物池 ID"
// @Success      200  {object}  response.Response
// @Router       /zzz-goodser/bundle-pools/{pid} [delete]
func (h *Handler) DeleteBundlePoolREST(c *gin.Context) {
	if err := h.svc.DeleteBundlePool(c.Request.Context(), c.Param("pid")); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}
