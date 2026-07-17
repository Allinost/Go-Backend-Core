package goodser

import (
	"database/sql"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	appErr "github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/Allinost/go-backend-core/internal/services/storage"
	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	svc   *Service
	store storage.Storage
}

// NewHandler 创建 Handler
func NewHandler(svc *Service, store storage.Storage) *Handler {
	return &Handler{svc: svc, store: store}
}

// --- Legacy 兼容端点 ---

// LoadInventories 获取所有库存目录
// @Summary      获取目录列表
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.Response{data=[]Inventory}
// @Router       /zzz-goodser/legacy/loadInventories [post]
func (h *Handler) LoadInventories(c *gin.Context) {
	items, err := h.svc.ListInventories(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []Inventory{}
	}
	response.Success(c, items)
}

// LoadProducts 获取指定库存目录下的商品列表
// @Summary      获取商品列表
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  PaginatedReq  true  "分页参数"
// @Success      200  {object}  response.Response{data=PaginatedResp[Product]}
// @Router       /zzz-goodser/legacy/loadProducts [post]
func (h *Handler) LoadProducts(c *gin.Context) {
	var req PaginatedReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	if req.InventoryID == "" {
		response.ParamErr(c, "缺少 inventory_id")
		return
	}
	req.Normalize()
	items, hasMore, total, err := h.svc.ListProductsPaginated(c.Request.Context(), req.InventoryID, req.Page, req.PageSize)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []Product{}
	}
	response.Success(c, PaginatedResp[Product]{Items: items, HasMore: hasMore, Total: total})
}

// QueryProducts 按关键词搜索商品（匹配 name / code / remark）
// @Summary      搜索商品
// @Description  根据关键词搜索商品，匹配 name / code / remark 字段
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  object{inventory_id=string,keyword=string}  true  "搜索参数"
// @Success      200  {object}  response.Response{data=[]Product}
// @Router       /zzz-goodser/legacy/queryProducts [post]
func (h *Handler) QueryProducts(c *gin.Context) {
	var req struct {
		InventoryID string `json:"inventory_id"`
		Keyword     string `json:"keyword"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	items, err := h.svc.SearchProducts(c.Request.Context(), req.InventoryID, req.Keyword)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []Product{}
	}
	response.Success(c, items)
}

// CreateInventory 创建库存目录
// @Summary      创建目录
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  CreateInventoryReq  true  "目录信息"
// @Success      200  {object}  response.Response{data=Inventory}
// @Router       /zzz-goodser/legacy/createInventory [post]
func (h *Handler) CreateInventory(c *gin.Context) {
	var req CreateInventoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	item, err := h.svc.CreateInventory(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// UpdateInventory 更新库存目录
// @Summary      更新目录
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  UpdateInventoryReq  true  "更新信息"
// @Success      200  {object}  response.Response{data=Inventory}
// @Router       /zzz-goodser/legacy/updateInventory [post]
func (h *Handler) UpdateInventory(c *gin.Context) {
	var req UpdateInventoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	item, err := h.svc.UpdateInventory(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// DeleteInventory 删除库存目录
// @Summary      删除目录
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  DeleteInventoryReq  true  "目录 ID"
// @Success      200  {object}  response.Response
// @Router       /zzz-goodser/legacy/deleteInventory [post]
func (h *Handler) DeleteInventory(c *gin.Context) {
	var req DeleteInventoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	if err := h.svc.DeleteInventory(c.Request.Context(), req.ID); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// CreateProduct 创建商品
// @Summary      创建商品
// @Description  直接创建一个新商品，需先通过 allocateSeq 获取序号
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  CreateProductReq  true  "商品信息"
// @Success      200  {object}  response.Response{data=Product}
// @Failure      400  {object}  response.Response
// @Router       /zzz-goodser/legacy/createProduct [post]
func (h *Handler) CreateProduct(c *gin.Context) {
	var req CreateProductReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	product, err := h.svc.CreateProduct(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, product)
}

// DeleteProduct 删除商品（含序号回收）
// @Summary      删除商品
// @Description  删除指定商品并回收其序号
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  object{id=string}  true  "商品 ID"
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.Response
// @Router       /zzz-goodser/legacy/deleteProduct [post]
func (h *Handler) DeleteProduct(c *gin.Context) {
	var req struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	if err := h.svc.DeleteProduct(c.Request.Context(), req.ID); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// UpdateProductREST 更新商品
// @Summary      更新商品
// @Description  部分更新商品信息，仅传需要修改的字段
// @Tags         zzz-goodser
// @Accept       json
// @Produce      json
// @Param        id    path  string          true  "商品 ID"
// @Param        body  body  UpdateProductReq  true  "更新字段"
// @Success      200  {object}  response.Response{data=Product}
// @Failure      400  {object}  response.Response
// @Router       /zzz-goodser/products/{id} [put]
func (h *Handler) UpdateProductREST(c *gin.Context) {
	productID := c.Param("id")
	var req UpdateProductReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	req.ID = productID
	product, err := h.svc.UpdateProduct(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, product)
}

// DeleteProductREST 删除商品（REST）
// @Summary      删除商品
// @Description  删除指定商品并回收其序号
// @Tags         zzz-goodser
// @Produce      json
// @Param        id  path  string  true  "商品 ID"
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.Response
// @Router       /zzz-goodser/products/{id} [delete]
func (h *Handler) DeleteProductREST(c *gin.Context) {
	productID := c.Param("id")
	if err := h.svc.DeleteProduct(c.Request.Context(), productID); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// AllocateSeq 分配商品序号（优先复用回收序号）
// @Summary      分配序号
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  AllocateSeqReq  true  "分配参数"
// @Success      200  {object}  response.Response{data=AllocateSeqResp}
// @Router       /zzz-goodser/legacy/allocateSeq [post]
func (h *Handler) AllocateSeq(c *gin.Context) {
	var req AllocateSeqReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	seq, err := h.svc.AllocateSeq(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, AllocateSeqResp{SeqNumber: seq})
}

// InboundSingle 单品入库
// @Summary      单品入库
// @Description  创 build 或叠加商品库存，写入库日志
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  InboundSingleReq  true  "入库信息"
// @Success      200  {object}  response.Response{data=Product}
// @Router       /zzz-goodser/legacy/inboundSingle [post]
func (h *Handler) InboundSingle(c *gin.Context) {
	var req InboundSingleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	product, err := h.svc.InboundSingle(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, product)
}

// InboundBatch 批量入库
// @Summary      批量入库
// @Description  批量创建或增加商品库存，已存在的按 code 匹配叠加数量
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  InboundBatchReq  true  "入库项列表"
// @Success      200  {object}  response.Response{data=InboundBatchResp}
// @Failure      400  {object}  response.Response
// @Router       /zzz-goodser/legacy/inboundBatch [post]
func (h *Handler) InboundBatch(c *gin.Context) {
	var req InboundBatchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	resp, err := h.svc.InboundBatch(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, resp)
}

// InboundSearchImport 搜索导入入库
// @Summary      搜索导入入库
// @Description  将搜索结果中的商品批量导入（按 product_id 叠加库存）
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  InboundSearchImportReq  true  "导入项列表"
// @Success      200  {object}  response.Response{data=InboundSearchImportResp}
// @Failure      400  {object}  response.Response
// @Router       /zzz-goodser/legacy/inboundSearchImport [post]
func (h *Handler) InboundSearchImport(c *gin.Context) {
	var req InboundSearchImportReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	resp, err := h.svc.InboundSearchImport(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, resp)
}

// LoadOutboundOrders 获取出库单列表
// @Summary      获取出库单
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  PaginatedReq  true  "分页参数"
// @Success      200  {object}  response.Response{data=PaginatedResp[OutboundOrder]}
// @Router       /zzz-goodser/legacy/loadOutboundOrders [post]
func (h *Handler) LoadOutboundOrders(c *gin.Context) {
	var req PaginatedReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	if req.InventoryID == "" {
		response.ParamErr(c, "缺少 inventory_id")
		return
	}
	req.Normalize()
	items, hasMore, total, err := h.svc.ListOutboundOrdersPaginated(c.Request.Context(), req.InventoryID, req.Page, req.PageSize)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []OutboundOrder{}
	}
	response.Success(c, PaginatedResp[OutboundOrder]{Items: items, HasMore: hasMore, Total: total})
}

// CreateOutbound 创建出库单（同时锁定预留库存）
// @Summary      创建出库单
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  CreateOutboundReq  true  "出库信息"
// @Success      200  {object}  response.Response{data=OutboundOrder}
// @Router       /zzz-goodser/legacy/createOutbound [post]
func (h *Handler) CreateOutbound(c *gin.Context) {
	var req CreateOutboundReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	order, err := h.svc.CreateOutboundOrder(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, order)
}

// ConfirmOutboundLegacy 确认出库
// @Summary      确认出库
// @Description  确认后扣除库存、取消预留
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  ConfirmOutboundReq  true  "出库单 ID"
// @Success      200  {object}  response.Response{data=OutboundOrder}
// @Router       /zzz-goodser/legacy/confirmOutbound [post]
func (h *Handler) ConfirmOutboundLegacy(c *gin.Context) {
	var req ConfirmOutboundReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	order, err := h.svc.ConfirmOutbound(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, order)
}

// CancelOutboundLegacy 取消出库
// @Summary      取消出库
// @Description  释放已锁定的预留库存
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  CancelOutboundReq  true  "出库单 ID"
// @Success      200  {object}  response.Response{data=OutboundOrder}
// @Router       /zzz-goodser/legacy/cancelOutbound [post]
func (h *Handler) CancelOutboundLegacy(c *gin.Context) {
	var req CancelOutboundReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	order, err := h.svc.CancelOutbound(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, order)
}

// LoadInboundLogs 获取入库日志
// @Summary      获取入库日志
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  PaginatedReq  true  "分页参数"
// @Success      200  {object}  response.Response{data=PaginatedResp[InboundLog]}
// @Router       /zzz-goodser/legacy/loadInboundLogs [post]
func (h *Handler) LoadInboundLogs(c *gin.Context) {
	var req PaginatedReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	if req.InventoryID == "" {
		response.ParamErr(c, "缺少 inventory_id")
		return
	}
	req.Normalize()
	items, hasMore, total, err := h.svc.ListInboundLogsPaginated(c.Request.Context(), req.InventoryID, req.Page, req.PageSize)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []InboundLog{}
	}
	response.Success(c, PaginatedResp[InboundLog]{Items: items, HasMore: hasMore, Total: total})
}

// LoadTags 获取标签列表
// @Summary      获取标签
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.Response{data=[]Tag}
// @Router       /zzz-goodser/legacy/loadTags [post]
func (h *Handler) LoadTags(c *gin.Context) {
	items, err := h.svc.ListTags(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []Tag{}
	}
	response.Success(c, items)
}

// CreateTag 创建标签
// @Summary      创建标签
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  CreateTagReq  true  "标签信息"
// @Success      200  {object}  response.Response{data=Tag}
// @Router       /zzz-goodser/legacy/createTag [post]
func (h *Handler) CreateTag(c *gin.Context) {
	var req CreateTagReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	item, err := h.svc.CreateTag(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// LoadStatusCodes 获取状态编码列表
// @Summary      获取状态编码
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.Response{data=[]StatusCode}
// @Router       /zzz-goodser/legacy/loadStatusCodes [post]
func (h *Handler) LoadStatusCodes(c *gin.Context) {
	items, err := h.svc.ListStatusCodes(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []StatusCode{}
	}
	response.Success(c, items)
}

// UpdateProductLegacy 更新商品（Legacy POST）
// @Summary      更新商品
// @Description  部分更新商品信息，仅传需要修改的字段
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  UpdateProductReq  true  "更新字段"
// @Success      200  {object}  response.Response{data=Product}
// @Failure      400  {object}  response.Response
// @Router       /zzz-goodser/legacy/updateProduct [post]
func (h *Handler) UpdateProductLegacy(c *gin.Context) {
	var req UpdateProductReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	product, err := h.svc.UpdateProduct(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, product)
}

// UpdateTagLegacy 更新标签
// @Summary      更新标签
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  UpdateTagReq  true  "标签信息"
// @Success      200  {object}  response.Response{data=Tag}
// @Router       /zzz-goodser/legacy/updateTag [post]
func (h *Handler) UpdateTagLegacy(c *gin.Context) {
	var req UpdateTagReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	item, err := h.svc.UpdateTag(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// DeleteTagLegacy 删除标签
// @Summary      删除标签
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  object{id=string}  true  "标签 ID"
// @Success      200  {object}  response.Response{data=object{success=bool}}
// @Router       /zzz-goodser/legacy/deleteTag [post]
func (h *Handler) DeleteTagLegacy(c *gin.Context) {
	var req struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	if err := h.svc.DeleteTag(c.Request.Context(), req.ID); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"success": true})
}

// AddStatusCode 创建状态编码
// @Summary      新增状态编码
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  AddStatusCodeReq  true  "状态编码信息"
// @Success      200  {object}  response.Response{data=StatusCode}
// @Router       /zzz-goodser/legacy/addStatusCode [post]
func (h *Handler) AddStatusCode(c *gin.Context) {
	var req AddStatusCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	item, err := h.svc.CreateStatusCode(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// UpdateStatusCode 更新状态编码
// @Summary      更新状态编码
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  UpdateStatusCodeReq  true  "更新信息"
// @Success      200  {object}  response.Response{data=StatusCode}
// @Router       /zzz-goodser/legacy/updateStatusCode [post]
func (h *Handler) UpdateStatusCode(c *gin.Context) {
	var req UpdateStatusCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	item, err := h.svc.UpdateStatusCode(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// RemoveStatusCode 删除状态编码
// @Summary      删除状态编码
// @Description  删除前会检查是否有商品正在使用此编码，如有则返回错误
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  object{id=string}  true  "状态编码 ID"
// @Success      200  {object}  response.Response{data=object{success=bool}}
// @Router       /zzz-goodser/legacy/removeStatusCode [post]
func (h *Handler) RemoveStatusCode(c *gin.Context) {
	var req struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	if err := h.svc.DeleteStatusCode(c.Request.Context(), req.ID); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"success": true})
}

// CreateInboundLogHandler 创建入库日志
// @Summary      创建入库日志
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  CreateInboundLogReq  true  "入库日志"
// @Success      200  {object}  response.Response{data=InboundLog}
// @Router       /zzz-goodser/legacy/createInboundLog [post]
func (h *Handler) CreateInboundLogHandler(c *gin.Context) {
	var req CreateInboundLogReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	item, err := h.svc.CreateInboundLog(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// UpdateInboundLogHandler 更新入库日志
// @Summary      更新入库日志
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  UpdateInboundLogReq  true  "更新信息"
// @Success      200  {object}  response.Response{data=InboundLog}
// @Router       /zzz-goodser/legacy/updateInboundLog [post]
func (h *Handler) UpdateInboundLogHandler(c *gin.Context) {
	var req UpdateInboundLogReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	item, err := h.svc.UpdateInboundLog(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// DeleteInboundLogHandler 删除入库日志
// @Summary      删除入库日志
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  object{id=string}  true  "入库日志 ID"
// @Success      200  {object}  response.Response{data=object{success=bool}}
// @Router       /zzz-goodser/legacy/deleteInboundLog [post]
func (h *Handler) DeleteInboundLogHandler(c *gin.Context) {
	var req struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	if err := h.svc.DeleteInboundLog(c.Request.Context(), req.ID); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"success": true})
}

// CancelReserveHandler 取消预留单
// @Summary      取消预约
// @Description  取消指定预约单，释放已锁定的预留库存
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  CancelReserveReq  true  "预约单 ID"
// @Success      200  {object}  response.Response{data=OutboundOrder}
// @Router       /zzz-goodser/legacy/cancelReserve [post]
func (h *Handler) CancelReserveHandler(c *gin.Context) {
	var req CancelReserveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	order, err := h.svc.CancelReserve(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, order)
}

// ReserveToOutboundHandler 预留单转出库单
// @Summary      预约转出库
// @Description  将预约单转为正式出库单，需重新指定出库信息
// @Tags         zzz-goodser-legacy
// @Accept       json
// @Produce      json
// @Param        body  body  ReserveToOutboundReq  true  "转出库信息"
// @Success      200  {object}  response.Response{data=OutboundOrder}
// @Router       /zzz-goodser/legacy/reserveToOutbound [post]
func (h *Handler) ReserveToOutboundHandler(c *gin.Context) {
	var req ReserveToOutboundReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	order, err := h.svc.ReserveToOutbound(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, order)
}

// UploadImage 上传图片
// @Summary      上传图片
// @Description  上传图片到对象存储，返回可访问的 URL
// @Tags         zzz-goodser-legacy
// @Accept       multipart/form-data
// @Produce      json
// @Param        image  formData  file  true  "图片文件"
// @Success      200  {object}  response.Response{data=object{url=string}}
// @Failure      400  {object}  response.Response
// @Router       /zzz-goodser/legacy/uploadImage [post]
func (h *Handler) UploadImage(c *gin.Context) {
	if h.store == nil {
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "图片存储未配置"))
		return
	}

	file, header, err := c.Request.FormFile("image")
	if err != nil {
		response.ParamErr(c, "缺少图片文件")
		return
	}
	defer file.Close()

	ext := strings.ToLower(path.Ext(header.Filename))
	if ext == "" {
		ext = ".jpg"
	}
	objectKey := fmt.Sprintf("goodser/images/%s%s", uuid.New().String(), ext)

	if _, err := h.store.Upload(c.Request.Context(), objectKey, file, storage.WithContentType(header.Header.Get("Content-Type"))); err != nil {
		logger.Error().Err(err).Str("key", objectKey).Msg("上传图片到 S3 失败")
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "上传图片失败: "+err.Error()))
		return
	}

	signedURL, err := h.store.SignedURL(c.Request.Context(), objectKey, 7*24*time.Hour)
	if err != nil {
		logger.Error().Err(err).Str("key", objectKey).Msg("生成预签名 URL 失败")
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "生成图片 URL 失败: "+err.Error()))
		return
	}

	response.Success(c, gin.H{"url": signedURL})
}

// SyncAll 全量同步所有数据
// @Summary      全量同步
// @Description  一次性同步所有库存目录、商品、出库单、入库日志、标签和状态编码
// @Tags         zzz-goodser
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.Response{data=SyncAllResp}
// @Router       /zzz-goodser/syncAll [post]
func (h *Handler) SyncAll(c *gin.Context) {
	resp, err := h.svc.SyncAll(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, resp)
}

// NotSupported 返回不支持的操作提示
func (h *Handler) NotSupported(c *gin.Context) {
	response.Fail(c, appErr.New(appErr.CodeSystemErr, "该操作不支持，请通过数据库直接管理"))
}

// --- RESTful 端点 ---

// ListInventoriesREST 获取库存目录列表
// @Summary      获取目录列表
// @Tags         zzz-goodser
// @Produce      json
// @Success      200  {object}  response.Response{data=[]Inventory}
// @Router       /zzz-goodser/inventories [get]
func (h *Handler) ListInventoriesREST(c *gin.Context) {
	h.LoadInventories(c)
}

// CreateInventoryREST 创建库存目录
// @Summary      创建目录
// @Tags         zzz-goodser
// @Accept       json
// @Produce      json
// @Param        body  body  CreateInventoryReq  true  "目录名"
// @Success      200  {object}  response.Response{data=Inventory}
// @Router       /zzz-goodser/inventories [post]
func (h *Handler) CreateInventoryREST(c *gin.Context) {
	h.CreateInventory(c)
}

// ListProductsREST 获取指定目录下的商品列表
// @Summary      获取商品列表
// @Tags         zzz-goodser
// @Produce      json
// @Param        id        path  int  true  "库存目录 ID"
// @Param        page      query int  false "页码，默认 1"
// @Param        page_size query int  false "每页条数，默认 20"
// @Success      200  {object}  response.Response{data=PaginatedResp[Product]}
// @Router       /zzz-goodser/inventories/{id}/products [get]
func (h *Handler) ListProductsREST(c *gin.Context) {
	inventoryID := c.Param("id")
	req := PaginatedReq{
		InventoryID: inventoryID,
		Page:        parseInt(c.Query("page"), 1),
		PageSize:    parseInt(c.Query("page_size"), 20),
	}
	req.Normalize()
	items, hasMore, total, err := h.svc.ListProductsPaginated(c.Request.Context(), req.InventoryID, req.Page, req.PageSize)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []Product{}
	}
	response.Success(c, PaginatedResp[Product]{Items: items, HasMore: hasMore, Total: total})
}

// GetInventoryREST 获取单个库存目录
// @Summary      获取目录
// @Tags         zzz-goodser
// @Produce      json
// @Param        id  path  string  true  "目录 ID"
// @Success      200  {object}  response.Response{data=Inventory}
// @Router       /zzz-goodser/inventories/{id} [get]
func (h *Handler) GetInventoryREST(c *gin.Context) {
	id := c.Param("id")
	item, err := h.svc.GetInventory(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// UpdateInventoryREST 更新库存目录
// @Summary      更新目录
// @Tags         zzz-goodser
// @Accept       json
// @Produce      json
// @Param        id    path  string            true  "目录 ID"
// @Param        body  body  UpdateInventoryReq  true  "更新字段"
// @Success      200  {object}  response.Response{data=Inventory}
// @Router       /zzz-goodser/inventories/{id} [put]
func (h *Handler) UpdateInventoryREST(c *gin.Context) {
	id := c.Param("id")
	var req UpdateInventoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	req.ID = id
	item, err := h.svc.UpdateInventory(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// DeleteInventoryREST 删除库存目录
// @Summary      删除目录
// @Tags         zzz-goodser
// @Produce      json
// @Param        id  path  string  true  "目录 ID"
// @Success      200  {object}  response.Response
// @Router       /zzz-goodser/inventories/{id} [delete]
func (h *Handler) DeleteInventoryREST(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.DeleteInventory(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// ListOutboundOrdersREST 获取出库单列表（REST）
// @Summary      获取出库单
// @Tags         zzz-goodser
// @Produce      json
// @Param        id        path  string  true  "库存目录 ID"
// @Param        page      query int     false "页码，默认 1"
// @Param        page_size query int     false "每页条数，默认 20"
// @Success      200  {object}  response.Response{data=PaginatedResp[OutboundOrder]}
// @Router       /zzz-goodser/inventories/{id}/outbound-orders [get]
func (h *Handler) ListOutboundOrdersREST(c *gin.Context) {
	inventoryID := c.Param("id")
	req := PaginatedReq{
		InventoryID: inventoryID,
		Page:        parseInt(c.Query("page"), 1),
		PageSize:    parseInt(c.Query("page_size"), 20),
	}
	req.Normalize()
	items, hasMore, total, err := h.svc.ListOutboundOrdersPaginated(c.Request.Context(), req.InventoryID, req.Page, req.PageSize)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []OutboundOrder{}
	}
	response.Success(c, PaginatedResp[OutboundOrder]{Items: items, HasMore: hasMore, Total: total})
}

// ListInboundLogsREST 获取入库日志列表（REST）
// @Summary      获取入库日志
// @Tags         zzz-goodser
// @Produce      json
// @Param        id        path  string  true  "库存目录 ID"
// @Param        page      query int     false "页码，默认 1"
// @Param        page_size query int     false "每页条数，默认 20"
// @Success      200  {object}  response.Response{data=PaginatedResp[InboundLog]}
// @Router       /zzz-goodser/inventories/{id}/inbound-logs [get]
func (h *Handler) ListInboundLogsREST(c *gin.Context) {
	inventoryID := c.Param("id")
	req := PaginatedReq{
		InventoryID: inventoryID,
		Page:        parseInt(c.Query("page"), 1),
		PageSize:    parseInt(c.Query("page_size"), 20),
	}
	req.Normalize()
	items, hasMore, total, err := h.svc.ListInboundLogsPaginated(c.Request.Context(), req.InventoryID, req.Page, req.PageSize)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []InboundLog{}
	}
	response.Success(c, PaginatedResp[InboundLog]{Items: items, HasMore: hasMore, Total: total})
}

func parseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

// --- 错误处理 ---

func (h *Handler) handleError(c *gin.Context, err error) {
	var gErr *GoodserError
	if errors.As(err, &gErr) {
		logger.Error().Err(err).Int("goodser_code", gErr.Code).Msg("goodser 业务错误")
		response.Fail(c, appErr.New(gErr.Code, gErr.Message))
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		response.Fail(c, appErr.New(appErr.CodeNotFound, "资源不存在"))
		return
	}
	logger.Error().Err(err).Msg("goodser 服务内部错误")
	response.Fail(c, appErr.New(appErr.CodeSystemErr, "服务内部错误"))
}

// 确保 Handler 实现了 http.Handler 接口
var _ interface{} = (*Handler)(nil)
