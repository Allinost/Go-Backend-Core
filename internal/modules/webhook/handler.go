package webhook

import (
	"database/sql"
	"errors"
	"strconv"

	appErr "github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	svc *Service
}

// NewHandler 创建 Handler
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ListConfigs 获取 Webhook 配置列表（分页+筛选）
// @Summary      获取 Webhook 配置列表
// @Tags         webhook
// @Accept       json
// @Produce      json
// @Param        page       query  int     false  "页码，默认 1"
// @Param        page_size  query  int     false  "每页数量，默认 20，最大 100"
// @Param        type       query  string  false  "筛选类型：feishu / dingtalk / wecom / github / generic"
// @Param        status     query  string  false  "筛选状态：active / paused / disabled"
// @Param        source     query  string  false  "筛选来源"
// @Param        keyword    query  string  false  "搜索关键词（匹配 name / url）"
// @Success      200  {object}  response.Response{data=PaginatedResp[WebhookConfig]}
// @Router       /webhook/configs [get]
func (h *Handler) ListConfigs(c *gin.Context) {
	var req ListConfigsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	req.Normalize()
	items, hasMore, total, err := h.svc.ListConfigs(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []WebhookConfig{}
	}
	response.Success(c, PaginatedResp[WebhookConfig]{Items: items, HasMore: hasMore, Total: total})
}

// CreateConfig 创建 Webhook 配置
// @Summary      创建 Webhook 配置
// @Tags         webhook
// @Accept       json
// @Produce      json
// @Param        body  body  CreateConfigReq  true  "Webhook 配置"
// @Success      200  {object}  response.Response{data=WebhookConfig}
// @Router       /webhook/configs [post]
func (h *Handler) CreateConfig(c *gin.Context) {
	var req CreateConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	config, err := h.svc.CreateConfig(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, config)
}

// GetConfig 获取单个 Webhook 配置详情（脱敏）
// @Summary      获取 Webhook 配置详情
// @Tags         webhook
// @Accept       json
// @Produce      json
// @Param        id   path  string  true  "Webhook 配置 ID"
// @Success      200  {object}  response.Response{data=WebhookConfig}
// @Router       /webhook/configs/{id} [get]
func (h *Handler) GetConfig(c *gin.Context) {
	id := c.Param("id")
	config, err := h.svc.GetConfig(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, config.Mask())
}

// UpdateConfig 更新 Webhook 配置
// @Summary      更新 Webhook 配置
// @Tags         webhook
// @Accept       json
// @Produce      json
// @Param        id    path  string          true  "Webhook 配置 ID"
// @Param        body  body  UpdateConfigReq  true  "更新字段"
// @Success      200  {object}  response.Response{data=WebhookConfig}
// @Router       /webhook/configs/{id} [put]
func (h *Handler) UpdateConfig(c *gin.Context) {
	id := c.Param("id")
	var req UpdateConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	config, err := h.svc.UpdateConfig(c.Request.Context(), id, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, config.Mask())
}

// DeleteConfig 删除 Webhook 配置
// @Summary      删除 Webhook 配置
// @Tags         webhook
// @Accept       json
// @Produce      json
// @Param        id   path  string  true  "Webhook 配置 ID"
// @Success      200  {object}  response.Response
// @Router       /webhook/configs/{id} [delete]
func (h *Handler) DeleteConfig(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.DeleteConfig(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// UpdateStatus 更新 Webhook 配置状态
// @Summary      更新 Webhook 状态
// @Tags         webhook
// @Accept       json
// @Produce      json
// @Param        id    path  string             true  "Webhook 配置 ID"
// @Param        body  body  UpdateStatusReq    true  "状态：active / paused / disabled"
// @Success      200  {object}  response.Response
// @Router       /webhook/configs/{id}/status [post]
func (h *Handler) UpdateStatus(c *gin.Context) {
	id := c.Param("id")
	var req UpdateStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.UpdateStatus(c.Request.Context(), id, req.Status); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// TestSend 测试发送 Webhook 消息
// @Summary      测试发送
// @Tags         webhook
// @Accept       json
// @Produce      json
// @Param        id    path  string    true  "Webhook 配置 ID"
// @Param        body  body  TestReq   false  "可选自定义测试消息内容"
// @Success      200  {object}  response.Response{data=WebhookResult}
// @Router       /webhook/configs/{id}/test [post]
func (h *Handler) TestSend(c *gin.Context) {
	id := c.Param("id")
	var req TestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		req.WebhookID = id
	}
	req.WebhookID = id
	result, err := h.svc.TestSend(c.Request.Context(), id, req.TestMsg)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, result)
}

// Send 发送 Webhook 消息（可选指定多个 Webhook ID 或按标签匹配）
// @Summary      发送消息
// @Description  向一个或多个 Webhook 发送消息。可通过 webhook_ids 精确指定，或通过 tags 按标签匹配，均不传时推送给所有活跃配置
// @Tags         webhook
// @Accept       json
// @Produce      json
// @Param        body  body  SendReq  true  "消息内容"
// @Success      200  {object}  response.Response{data=SendResult}
// @Router       /webhook/send [post]
func (h *Handler) Send(c *gin.Context) {
	var req SendReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	if req.Source == "" {
		req.Source = "api"
	}
	if req.MsgType == "" {
		req.MsgType = "markdown"
	}
	result, err := h.svc.Send(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, result)
}

// ListDeliveryLogs 获取投递日志列表
// @Summary      获取投递日志
// @Tags         webhook
// @Accept       json
// @Produce      json
// @Param        page        query  int     false  "页码，默认 1"
// @Param        page_size   query  int     false  "每页数量，默认 20"
// @Param        webhook_id  query  string  false  "按 Webhook ID 筛选"
// @Param        status      query  string  false  "筛选状态：success / failed / rate_limited / deduped"
// @Param        source      query  string  false  "按来源筛选"
// @Success      200  {object}  response.Response{data=PaginatedResp[DeliveryLog]}
// @Router       /webhook/delivery-logs [get]
func (h *Handler) ListDeliveryLogs(c *gin.Context) {
	var req ListDeliveryLogsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	req.Normalize()
	items, hasMore, total, err := h.svc.repo.ListDeliveryLogs(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []DeliveryLog{}
	}
	response.Success(c, PaginatedResp[DeliveryLog]{Items: items, HasMore: hasMore, Total: total})
}

// Stats 获取全局统计概览
// @Summary      全局统计概览
// @Tags         webhook
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.Response{data=StatsResp}
// @Router       /webhook/stats [get]
func (h *Handler) Stats(c *gin.Context) {
	stats, err := h.svc.repo.GetStats(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, stats)
}

// ConfigStats 获取单个 Webhook 配置的统计信息
// @Summary      配置统计
// @Tags         webhook
// @Accept       json
// @Produce      json
// @Param        id   path  string  true  "Webhook 配置 ID"
// @Success      200  {object}  response.Response{data=ConfigStatsResp}
// @Router       /webhook/configs/{id}/stats [get]
func (h *Handler) ConfigStats(c *gin.Context) {
	id := c.Param("id")
	stats, err := h.svc.repo.GetConfigStats(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, stats)
}

// DeleteDeliveryLog 删除单条投递日志
// @Summary      删除投递日志
// @Tags         webhook
// @Accept       json
// @Produce      json
// @Param        id   path  int  true  "投递日志 ID"
// @Success      200  {object}  response.Response
// @Router       /webhook/delivery-logs/{id} [delete]
func (h *Handler) DeleteDeliveryLog(c *gin.Context) {
	idStr := c.Param("id")
	id, err := parseInt64(idStr)
	if err != nil {
		response.ParamErr(c, "无效的日志 ID")
		return
	}
	if err := h.svc.DeleteDeliveryLog(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// BatchDeleteDeliveryLogs 批量删除投递日志
// @Summary      批量删除投递日志
// @Tags         webhook
// @Accept       json
// @Produce      json
// @Param        body  body  BatchDeleteDeliveryLogsReq  true  "要删除的日志 ID 列表"
// @Success      200  {object}  response.Response
// @Router       /webhook/delivery-logs/batch-delete [post]
func (h *Handler) BatchDeleteDeliveryLogs(c *gin.Context) {
	var req BatchDeleteDeliveryLogsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: 请提供 ids 数组")
		return
	}
	if err := h.svc.BatchDeleteDeliveryLogs(c.Request.Context(), req.IDs); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// handleError 统一错误处理：WebhookError → 业务错误码，其余 → 500
func (h *Handler) handleError(c *gin.Context, err error) {
	var whErr *WebhookError
	if errors.As(err, &whErr) {
		logger.Error().Err(err).Int("webhook_code", whErr.Code).Msg("webhook 业务错误")
		response.Fail(c, appErr.New(whErr.Code, whErr.Message))
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		response.Fail(c, appErr.New(appErr.CodeNotFound, "资源不存在"))
		return
	}
	logger.Error().Err(err).Msg("webhook 服务内部错误")
	response.Fail(c, appErr.New(appErr.CodeSystemErr, "服务内部错误"))
}

// parseInt64 字符串转 int64 辅助函数
func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
