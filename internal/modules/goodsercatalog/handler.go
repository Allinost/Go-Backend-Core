package goodsercatalog

import (
	"errors"
	"strings"

	appErr "github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) actorID(c *gin.Context) int64 {
	if value, ok := c.Get("user_id"); ok {
		switch v := value.(type) {
		case uint:
			return int64(v)
		case int64:
			return v
		case float64:
			return int64(v)
		}
	}
	return 0
}

func (h *Handler) handleError(c *gin.Context, err error) {
	if errors.Is(err, ErrNotFound) {
		response.Fail(c, appErr.New(appErr.CodeNotFound, "图鉴不存在"))
		return
	}
	response.Fail(c, appErr.New(appErr.CodeSystemErr, err.Error()))
}

// ListCatalogs GET /catalogs
func (h *Handler) ListCatalogs(c *gin.Context) {
	var req ListCatalogsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	items, err := h.svc.ListCatalogs(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

// CreateCatalog POST /catalogs
func (h *Handler) CreateCatalog(c *gin.Context) {
	var req CreateCatalogReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	item, err := h.svc.CreateCatalog(c.Request.Context(), &req, h.actorID(c))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// GetCatalog GET /catalogs/:id
func (h *Handler) GetCatalog(c *gin.Context) {
	item, err := h.svc.GetCatalog(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// UpdateCatalog PUT /catalogs/:id
func (h *Handler) UpdateCatalog(c *gin.Context) {
	var req UpdateCatalogReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	item, err := h.svc.UpdateCatalog(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// DeleteCatalog DELETE /catalogs/:id
func (h *Handler) DeleteCatalog(c *gin.Context) {
	if err := h.svc.DeleteCatalog(c.Request.Context(), c.Param("id")); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// AddImage POST /catalogs/:id/images
// 支持 multipart 的 image 字段，或 JSON { image_url, width, height }
func (h *Handler) AddImage(c *gin.Context) {
	catalogID := c.Param("id")
	if file, header, err := c.Request.FormFile("image"); err == nil {
		defer file.Close()
		contentType := header.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "image/jpeg"
		}
		item, err := h.svc.UploadImage(c.Request.Context(), catalogID, header.Filename, contentType, file)
		if err != nil {
			h.handleError(c, err)
			return
		}
		response.Success(c, item)
		return
	}
	var req AddImageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "缺少图片文件或 image_url")
		return
	}
	if strings.TrimSpace(req.ImageURL) == "" {
		response.ParamErr(c, "image_url 不能为空")
		return
	}
	item, err := h.svc.AddImage(c.Request.Context(), catalogID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// SegmentImage POST /catalog-images/:id/segment
func (h *Handler) SegmentImage(c *gin.Context) {
	result, err := h.svc.SegmentImage(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, result)
}

// CreateItem POST /catalogs/:id/items
func (h *Handler) CreateItem(c *gin.Context) {
	var req CreateItemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	item, err := h.svc.CreateItem(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// UpdateItem PUT /catalog-items/:id
func (h *Handler) UpdateItem(c *gin.Context) {
	var req UpdateItemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	item, err := h.svc.UpdateItem(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

// DeleteItem DELETE /catalog-items/:id
func (h *Handler) DeleteItem(c *gin.Context) {
	if err := h.svc.DeleteItem(c.Request.Context(), c.Param("id")); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}
