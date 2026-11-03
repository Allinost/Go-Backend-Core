package integration

import (
	"context"
	"encoding/json"
	stdErrors "errors"
	"strconv"
	"strings"
	"time"

	appErrors "github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/Allinost/go-backend-core/internal/services/action"
	"github.com/Allinost/go-backend-core/internal/services/connector"
	"github.com/Allinost/go-backend-core/internal/services/outbox"
	"github.com/gin-gonic/gin"
)

// Handler 集成模块 HTTP 处理器
type Handler struct {
	gateway      *action.Gateway
	outbox       *outbox.Service
	connectors   *connector.Registry
	connectorMgr *ConnectorManager
	dispatcher   *outbox.Dispatcher
	logger       ActionLogger
	now          func() time.Time
}

// NewHandler 创建处理器
func NewHandler(gateway *action.Gateway, outboxSvc *outbox.Service, connectors *connector.Registry, logger ActionLogger) *Handler {
	return &Handler{gateway: gateway, outbox: outboxSvc, connectors: connectors, logger: logger, now: time.Now}
}

// SetConnectorManager 注入连接器配置管理器
func (h *Handler) SetConnectorManager(manager *ConnectorManager) {
	h.connectorMgr = manager
}

// SetDispatcher 注入外盒分发器，用于手动触发
func (h *Handler) SetDispatcher(dispatcher *outbox.Dispatcher) {
	h.dispatcher = dispatcher
}

// ListActions 返回已注册动作
// @Summary     查询已注册集成动作
// @Tags        integration
// @Success     200  {object}  response.Response
// @Router      /integration/actions [get]
func (h *Handler) ListActions(c *gin.Context) {
	response.Success(c, gin.H{"items": h.gateway.Registry().List()})
}

// InvokeAction 调用动作
// @Summary     调用集成动作
// @Tags        integration
// @Param       name  path  string  true  "动作名称"
// @Param       body  body  object  false  "动作参数，格式为 {params:{...}}"
// @Success     200  {object}  response.Response
// @Router      /integration/actions/{name} [post]
func (h *Handler) InvokeAction(c *gin.Context) {
	var body struct {
		Params    json.RawMessage `json:"params"`
		RequestID string          `json:"request_id"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, appErrors.New(appErrors.CodeParamErr, "参数格式错误: "+err.Error()))
			return
		}
	}

	result, err := h.gateway.Invoke(c.Request.Context(), action.Request{
		Action:    c.Param("name"),
		Actor:     callerIdentity(c),
		Scopes:    callerScopes(c),
		Params:    body.Params,
		RequestID: body.RequestID,
	})
	h.recordAction(c, c.Param("name"), body.Params, body.RequestID, result, err)
	if err != nil {
		failAction(c, err)
		return
	}
	response.Success(c, result)
}

// ListEvents 查询外盒事件
// @Summary     查询外盒事件
// @Tags        integration
// @Param       topic     query  string  false  "事件主题"
// @Param       status    query  string  false  "投递状态"
// @Param       limit     query  int     false  "每页数量"
// @Param       offset    query  int     false  "偏移量"
// @Success     200  {object}  response.Response
// @Router      /integration/events [get]
func (h *Handler) ListEvents(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	filter := outbox.Filter{
		Topic:         strings.TrimSpace(c.Query("topic")),
		Source:        strings.TrimSpace(c.Query("source")),
		Status:        strings.TrimSpace(c.Query("status")),
		AggregateType: strings.TrimSpace(c.Query("aggregate_type")),
		AggregateID:   strings.TrimSpace(c.Query("aggregate_id")),
		Limit:         limit,
		Offset:        offset,
	}
	events, err := h.outbox.List(c.Request.Context(), filter)
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, "查询事件失败: "+err.Error()))
		return
	}
	response.Success(c, gin.H{"items": events, "limit": limit, "offset": offset})
}

// EmitEvent 手动写入外盒事件，供集成联调与补偿使用
// @Summary     手动写入外盒事件
// @Tags        integration
// @Param       body  body  outbox.EmitInput  true  "事件内容"
// @Success     200  {object}  response.Response
// @Router      /integration/events [post]
func (h *Handler) EmitEvent(c *gin.Context) {
	var body outbox.EmitInput
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeParamErr, "参数格式错误: "+err.Error()))
		return
	}
	event, err := h.outbox.Emit(c.Request.Context(), body)
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeParamErr, err.Error()))
		return
	}
	response.Success(c, event)
}

// ListConnectors 返回已注册连接器
// @Summary     查询集成连接器
// @Tags        integration
// @Success     200  {object}  response.Response
// @Router      /integration/connectors [get]
func (h *Handler) ListConnectors(c *gin.Context) {
	if h.connectorMgr != nil {
		items, err := h.connectorMgr.List(c.Request.Context())
		if err != nil {
			response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
			return
		}
		response.Success(c, gin.H{"items": items})
		return
	}
	response.Success(c, gin.H{"items": h.connectors.List()})
}

// CreateConnector 创建连接器
// @Summary     创建集成连接器
// @Tags        integration
// @Accept      json
// @Produce     json
// @Param       body  body  ConnectorReq  true  "连接器配置"
// @Success     200  {object}  response.Response{data=ConnectorDTO}
// @Router      /integration/connectors [post]
func (h *Handler) CreateConnector(c *gin.Context) {
	var req ConnectorReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "连接器参数格式错误")
		return
	}
	if h.connectorMgr == nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, "连接器管理器未初始化"))
		return
	}
	item, err := h.connectorMgr.Create(c.Request.Context(), &req)
	if err != nil {
		failConnector(c, err)
		return
	}
	response.Success(c, item)
}

// UpdateConnector 更新连接器
// @Summary     更新集成连接器
// @Tags        integration
// @Accept      json
// @Produce     json
// @Param       id    path  string        true  "连接器 ID"
// @Param       body  body  ConnectorReq  true  "连接器配置"
// @Success     200  {object}  response.Response{data=ConnectorDTO}
// @Router      /integration/connectors/{id} [put]
func (h *Handler) UpdateConnector(c *gin.Context) {
	var req ConnectorReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "连接器参数格式错误")
		return
	}
	if h.connectorMgr == nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, "连接器管理器未初始化"))
		return
	}
	item, err := h.connectorMgr.Update(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		failConnector(c, err)
		return
	}
	response.Success(c, item)
}

// DeleteConnector 删除连接器
// @Summary     删除集成连接器
// @Tags        integration
// @Param       id  path  string  true  "连接器 ID"
// @Success     200  {object}  response.Response
// @Router      /integration/connectors/{id} [delete]
func (h *Handler) DeleteConnector(c *gin.Context) {
	if h.connectorMgr == nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, "连接器管理器未初始化"))
		return
	}
	if err := h.connectorMgr.Delete(c.Request.Context(), c.Param("id")); err != nil {
		failConnector(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// TestConnector 向指定连接器发送测试事件
// @Summary     测试集成连接器
// @Tags        integration
// @Param       id  path  string  true  "连接器 ID"
// @Success     200  {object}  response.Response
// @Router      /integration/connectors/{id}/test [post]
func (h *Handler) TestConnector(c *gin.Context) {
	id := c.Param("id")
	item, err := h.connectors.Get(id)
	if err != nil {
		failConnector(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	err = item.Send(ctx, connector.Message{
		EventID:    "test-" + strconv.FormatInt(h.now().UnixNano(), 10),
		Event:      "integration.test",
		OccurredAt: h.now().UTC(),
		Payload:    json.RawMessage(`{"test":true}`),
		Metadata:   map[string]string{"connector_id": id},
	})
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeBadGateway, "测试事件发送失败: "+err.Error()))
		return
	}
	response.Success(c, gin.H{"sent": true, "connector_id": id})
}

// DispatchEvents 手动触发一轮外盒事件投递// @Summary     手动分发外盒事件
// @Tags        integration
// @Success     200  {object}  response.Response
// @Router      /integration/events/dispatch [post]
func (h *Handler) DispatchEvents(c *gin.Context) {
	if h.dispatcher == nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, "分发器未初始化"))
		return
	}
	delivered, failed, err := h.dispatcher.RunOnce(c.Request.Context())
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
		return
	}
	response.Success(c, gin.H{"delivered": delivered, "failed": failed})
}

func failConnector(c *gin.Context, err error) {
	switch {
	case stdErrors.Is(err, connector.ErrNotFound):
		response.Fail(c, appErrors.New(appErrors.CodeNotFound, err.Error()))
	case stdErrors.Is(err, connector.ErrInvalidConfig):
		response.ParamErr(c, err.Error())
	default:
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
	}
}

// Ping 集成连通性测试
// @Summary     集成连通性测试
// @Tags        integration
// @Success     200  {object}  response.Response
// @Router      /integration/ping [get]
func (h *Handler) Ping(c *gin.Context) {
	response.Success(c, gin.H{
		"ok":          true,
		"server_time": h.now().UTC(),
		"auth_method": c.GetString("auth_method"),
	})
}

func callerIdentity(c *gin.Context) string {
	if username := c.GetString("username"); username != "" {
		return username
	}
	if userID, exists := c.Get("user_id"); exists {
		return "user:" + toString(userID)
	}
	return ""
}

// callerScopes 返回当前调用方作用域。控制台 JWT 视为 owner，不受 API Key 作用域限制。
func callerScopes(c *gin.Context) string {
	if c.GetString("auth_method") == "apikey" {
		return c.GetString("api_key_scopes")
	}
	return "*"
}

func toString(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case uint:
		return strconv.FormatUint(uint64(value), 10)
	case int:
		return strconv.Itoa(value)
	default:
		return ""
	}
}

func (h *Handler) recordAction(c *gin.Context, actionName string, params json.RawMessage, requestID string, result action.Result, invokeErr error) {
	if h.logger == nil {
		return
	}
	entry := ActionLog{
		Action:     actionName,
		Actor:      callerIdentity(c),
		AuthMethod: c.GetString("auth_method"),
		Status:     "success",
		Params:     params,
		RequestID:  requestID,
		Replayed:   result.Replayed,
		DurationMS: int(result.Duration.Milliseconds()),
	}
	if entry.Action == "" {
		entry.Action = result.Action
	}
	if len(entry.RequestID) > 100 {
		entry.RequestID = entry.RequestID[:100]
	}
	if value, exists := c.Get("api_key_id"); exists {
		switch id := value.(type) {
		case uint:
			entry.APIKeyID = id
		case int:
			if id > 0 {
				entry.APIKeyID = uint(id)
			}
		}
	}
	if invokeErr != nil {
		entry.Status = "error"
		entry.Error = invokeErr.Error()
	} else if data, err := json.Marshal(result.Data); err == nil {
		entry.Result = data
	}
	_ = h.logger.Write(c.Request.Context(), entry)
}

func failAction(c *gin.Context, err error) {
	switch {
	case stdErrors.Is(err, action.ErrNotFound):
		response.Fail(c, appErrors.New(appErrors.CodeNotFound, err.Error()))
	case stdErrors.Is(err, action.ErrForbidden):
		response.Fail(c, appErrors.New(appErrors.CodeForbidden, err.Error()))
	default:
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
	}
}
