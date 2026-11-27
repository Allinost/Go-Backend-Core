package goodser

import (
	"io"
	"strconv"

	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// CreateRecognition 创建识别任务。支持 multipart image 或 JSON image_url。
// @Summary      图片识别
// @Tags         zzz-goodser
// @Accept       multipart/form-data
// @Produce      json
// @Param        id     path  string  true  "目录 ID"
// @Param        image  formData  file  false  "图片"
// @Param        mode   formData  string  false  "multi | similarity"
// @Success      200  {object}  response.Response{data=RecognitionJob}
// @Router       /zzz-goodser/inventories/{id}/recognitions [post]
func (h *Handler) CreateRecognition(c *gin.Context) {
	var (
		imageBytes []byte
		imageURL   string
		mode       string
	)
	if file, _, err := c.Request.FormFile("image"); err == nil {
		defer file.Close()
		data, readErr := io.ReadAll(io.LimitReader(file, 20<<20))
		if readErr != nil || len(data) == 0 {
			response.ParamErr(c, "图片为空")
			return
		}
		imageBytes = data
		mode = c.PostForm("mode")
	} else {
		var req struct {
			ImageURL string `json:"image_url"`
			Mode     string `json:"mode"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ParamErr(c, "参数错误")
			return
		}
		imageURL = req.ImageURL
		mode = req.Mode
	}
	job, err := h.svc.CreateRecognition(c.Request.Context(), c.Param("id"), imageBytes, imageURL, mode, int64(c.GetUint("user_id")))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, job)
}

// ListRecognitionJobs 识别历史。
// @Summary      识别历史
// @Tags         zzz-goodser
// @Produce      json
// @Param        id     path   string  true   "目录 ID"
// @Param        limit  query  int     false  "条数"
// @Success      200  {object}  response.Response{data=[]RecognitionJob}
// @Router       /zzz-goodser/inventories/{id}/recognitions [get]
func (h *Handler) ListRecognitionJobs(c *gin.Context) {
	limit := 20
	if raw := c.Query("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			limit = v
		}
	}
	items, err := h.svc.ListRecognitionJobs(c.Request.Context(), c.Param("id"), limit)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []RecognitionJob{}
	}
	response.Success(c, items)
}

// GetRecognition 查询识别任务。
// @Summary      查询识别任务
// @Tags         zzz-goodser
// @Produce      json
// @Param        rid  path  string  true  "识别 ID"
// @Success      200  {object}  response.Response{data=RecognitionJob}
// @Router       /zzz-goodser/recognitions/{rid} [get]
func (h *Handler) GetRecognition(c *gin.Context) {
	job, err := h.svc.GetRecognition(c.Request.Context(), c.Param("rid"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, job)
}

// ConfirmRecognitionItem 人工确认/修改识别项。
// @Summary      确认识别项
// @Tags         zzz-goodser
// @Accept       json
// @Produce      json
// @Param        rid   path  string  true  "识别 ID"
// @Param        iid   path  int     true  "识别项 ID"
// @Success      200  {object}  response.Response{data=RecognitionJob}
// @Router       /zzz-goodser/recognitions/{rid}/items/{iid}/confirm [post]
func (h *Handler) ConfirmRecognitionItem(c *gin.Context) {
	itemID, err := strconv.ParseInt(c.Param("iid"), 10, 64)
	if err != nil {
		response.ParamErr(c, "识别项 ID 不合法")
		return
	}
	var req struct {
		ProductID *string `json:"product_id"`
		Quantity  *int    `json:"quantity"`
		Confirmed *bool   `json:"confirmed"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	job, err := h.svc.ConfirmRecognitionItem(c.Request.Context(), c.Param("rid"), itemID, req.ProductID, req.Quantity, req.Confirmed)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, job)
}

// ApplyRecognition 提交复核结果，返回入库草稿（不直接改库存）。
// @Summary      提交识别结果
// @Tags         zzz-goodser
// @Accept       json
// @Produce      json
// @Param        rid  path  string  true  "识别 ID"
// @Success      200  {object}  response.Response{data=RecognitionJob}
// @Router       /zzz-goodser/recognitions/{rid}/apply [post]
func (h *Handler) ApplyRecognition(c *gin.Context) {
	var req RecognitionApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		response.ParamErr(c, "参数错误")
		return
	}
	job, err := h.svc.ApplyRecognition(c.Request.Context(), c.Param("rid"), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, job)
}

// RebuildEmbeddings 重建商品图片向量索引。
// @Summary      重建向量索引
// @Tags         zzz-goodser
// @Produce      json
// @Param        id  path  string  true  "目录 ID"
// @Success      200  {object}  response.Response{data=EmbeddingRebuildResult}
// @Router       /zzz-goodser/inventories/{id}/embeddings/rebuild [post]
func (h *Handler) RebuildEmbeddings(c *gin.Context) {
	result, err := h.svc.RebuildEmbeddings(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, result)
}

// EmbeddingStats 向量索引进度。
// @Summary      向量索引统计
// @Tags         zzz-goodser
// @Produce      json
// @Param        id  path  string  true  "目录 ID"
// @Success      200  {object}  response.Response
// @Router       /zzz-goodser/inventories/{id}/embeddings/stats [get]
func (h *Handler) EmbeddingStats(c *gin.Context) {
	result, err := h.svc.EmbeddingStats(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, result)
}
