package shopchecker

import (
	"database/sql"
	"errors"

	appErr "github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/Allinost/go-backend-core/zzz/shopchecker/collector"
	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	svc      *Service
	registry *collector.Registry
}

// NewHandler 创建 Handler
func NewHandler(svc *Service, registry *collector.Registry) *Handler {
	return &Handler{svc: svc, registry: registry}
}

// ListProducts 获取商品列表
// @Summary      获取商品列表
// @Tags         shopchecker
// @Accept       json
// @Produce      json
// @Param        page      query  int     false  "页码，默认 1"
// @Param        page_size query  int     false  "每页数量，默认 20，最大 100"
// @Param        platform  query  string  false  "筛选平台：jd（京东桌面版）/ jdm（京东移动版）/ taobao（淘宝）/ pdd（拼多多）/ qixian（七鲜）"
// @Param        keyword   query  string  false  "搜索关键词，匹配商品名称"
// @Param        enabled   query  bool    false  "筛选启用状态"
// @Success      200  {object}  response.Response{data=PaginatedResp[Product]}
// @Router       /shopchecker/products [get]
func (h *Handler) ListProducts(c *gin.Context) {
	var req ListProductsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	req.Normalize()
	items, hasMore, total, err := h.svc.ListProducts(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []Product{}
	}
	response.Success(c, PaginatedResp[Product]{Items: items, HasMore: hasMore, Total: total})
}

// CreateProduct 创建商品监控
// @Summary      创建商品监控
// @Tags         shopchecker
// @Accept       json
// @Produce      json
// @Description  创建一个新的商品价格监控任务。支持以下平台和 URL 格式：
// @Description  - platform: jd（京东桌面版）、jdm（京东移动版）、taobao（淘宝/天猫）、pdd（拼多多）、qixian（七鲜）
// @Description  - url: 商品页完整 URL、分享短链接（如 e.tb.cn/xxx）、或含 ID 的链接均可自动识别
// @Description  - check_interval: 检查间隔，如 "1h"（1小时）、"30m"（30分钟）、"24h"（24小时），默认 "1h"
// @Description  - target_price: 目标价格，当现价低于此值时触发通知（淘宝同时支持国补/百亿补贴后价格对比）
// @Description  - price_drop_ratio: 降价百分比阈值，降幅达到此比例时触发通知
// @Description  - cron_expr: cron 表达式，优先级高于 check_interval（如 "0 9 * * *" 表示每天 9 点检查）
// @Param        body  body  CreateProductReq  true  "商品监控配置"
// @Success      200  {object}  response.Response{data=Product}
// @Failure      400  {object}  response.Response  "参数错误（平台不支持/URL 无效/间隔格式错误）"
// @Failure      500  {object}  response.Response  "服务内部错误"
// @Router       /shopchecker/products [post]
func (h *Handler) CreateProduct(c *gin.Context) {
	var req CreateProductReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	product, err := h.svc.CreateProduct(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, product)
}

// GetProduct 获取商品详情
// @Summary      获取商品详情
// @Tags         shopchecker
// @Accept       json
// @Produce      json
// @Description  获取单个商品监控的完整配置信息，包含当前价格、上次检查时间、下次检查时间等
// @Param        id   path  string  true  "商品 ID"
// @Success      200  {object}  response.Response{data=Product}
// @Failure      404  {object}  response.Response  "商品不存在"
// @Router       /shopchecker/products/{id} [get]
func (h *Handler) GetProduct(c *gin.Context) {
	id := c.Param("id")
	product, err := h.svc.GetProduct(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, product)
}

// UpdateProduct 更新商品监控
// @Summary      更新商品监控
// @Tags         shopchecker
// @Accept       json
// @Produce      json
// @Description  更新商品监控配置，仅传入需要修改的字段即可（部分更新）
// @Param        id    path  string           true  "商品 ID"
// @Param        body  body  UpdateProductReq  true  "更新字段（仅传入需修改的字段）"
// @Success      200  {object}  response.Response{data=Product}
// @Failure      404  {object}  response.Response  "商品不存在"
// @Router       /shopchecker/products/{id} [put]
func (h *Handler) UpdateProduct(c *gin.Context) {
	id := c.Param("id")
	var req UpdateProductReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	product, err := h.svc.UpdateProduct(c.Request.Context(), id, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, product)
}

// DeleteProduct 删除商品监控
// @Summary      删除商品监控
// @Tags         shopchecker
// @Accept       json
// @Produce      json
// @Param        id   path  string  true  "商品 ID"
// @Success      200  {object}  response.Response
// @Router       /shopchecker/products/{id} [delete]
func (h *Handler) DeleteProduct(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.DeleteProduct(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// ManualCheck 手动触发价格检查
// @Summary      手动触发价格检查
// @Tags         shopchecker
// @Accept       json
// @Produce      json
// @Description  立即执行一次价格采集，不等待定时调度。返回本次采集到的最新价格记录
// @Param        id   path  string  true  "商品 ID"
// @Success      200  {object}  response.Response{data=PriceRecord}
// @Failure      404  {object}  response.Response  "商品不存在"
// @Failure      500  {object}  response.Response  "采集失败（可能需要登录或反爬拦截）"
// @Router       /shopchecker/products/{id}/check [post]
func (h *Handler) ManualCheck(c *gin.Context) {
	id := c.Param("id")
	record, err := h.svc.ManualCheck(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, record)
}

// ListPriceRecords 获取价格记录
// @Summary      获取价格记录
// @Tags         shopchecker
// @Accept       json
// @Produce      json
// @Description  按时间倒序分页获取商品的历史价格记录。每条记录包含采集时的价格、标题、库存状态和错误信息
// @Param        id        path  string  true  "商品 ID"
// @Param        page      query  int     false  "页码，默认 1"
// @Param        page_size query  int     false  "每页数量，默认 20"
// @Success      200  {object}  response.Response{data=PaginatedResp[PriceRecord]}
// @Router       /shopchecker/products/{id}/price-records [get]
func (h *Handler) ListPriceRecords(c *gin.Context) {
	id := c.Param("id")
	var req ListPriceRecordsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	req.Normalize()
	items, hasMore, total, err := h.svc.ListPriceRecords(c.Request.Context(), id, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []PriceRecord{}
	}
	response.Success(c, PaginatedResp[PriceRecord]{Items: items, HasMore: hasMore, Total: total})
}

// ListNotificationLogs 获取通知记录
// @Summary      获取通知记录
// @Tags         shopchecker
// @Accept       json
// @Produce      json
// @Description  按时间倒序分页获取商品的通知记录。包含通知类型（price_drop 降价 / target_reached 到达目标价 / stock_restore 补货）、新旧价格、发送结果
// @Param        id        path  string  true  "商品 ID"
// @Param        page      query  int     false  "页码，默认 1"
// @Param        page_size query  int     false  "每页数量，默认 20"
// @Success      200  {object}  response.Response{data=PaginatedResp[NotificationLog]}
// @Router       /shopchecker/products/{id}/notification-logs [get]
func (h *Handler) ListNotificationLogs(c *gin.Context) {
	id := c.Param("id")
	var req ListNotificationLogsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	req.Normalize()
	items, hasMore, total, err := h.svc.ListNotificationLogs(c.Request.Context(), id, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []NotificationLog{}
	}
	response.Success(c, PaginatedResp[NotificationLog]{Items: items, HasMore: hasMore, Total: total})
}

// Dashboard 获取仪表盘
// @Summary      获取仪表盘统计
// @Tags         shopchecker
// @Accept       json
// @Produce      json
// @Description  返回全局统计数据：监控商品总数、已启用数、今日检查次数、今日通知次数、最近降价列表
// @Success      200  {object}  response.Response{data=DashboardResp}
// @Router       /shopchecker/dashboard [get]
func (h *Handler) Dashboard(c *gin.Context) {
	data, err := h.svc.Dashboard(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, data)
}

// handleError 统一错误处理
func (h *Handler) handleError(c *gin.Context, err error) {
	var scErr *ShopCheckerError
	if errors.As(err, &scErr) {
		logger.Error().Err(err).Int("shopchecker_code", scErr.Code).Msg("shopchecker 业务错误")
		response.Fail(c, appErr.New(scErr.Code, scErr.Message))
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		response.Fail(c, appErr.New(appErr.CodeNotFound, "资源不存在"))
		return
	}
	logger.Error().Err(err).Msg("shopchecker 服务内部错误")
	response.Fail(c, appErr.New(appErr.CodeSystemErr, "服务内部错误"))
}
