package integration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Allinost/go-backend-core/internal/pkg/authz"
	"github.com/Allinost/go-backend-core/internal/services/action"
	"github.com/gin-gonic/gin"
)

// MCP Streamable HTTP 端点：/api/v1/integration/mcp
//
// 该端点把动作网关动态暴露为 MCP 工具，OpenClaw、LangBot 等 MCP 客户端
// 无需为每个动作单独写 HTTP 封装。认证复用 X-API-Key / Bearer API Key，
// 作用域仍由调用方 API Key 决定。
const (
	mcpLatestVersion = "2025-06-18"
	mcpMaxBodyBytes  = 1 << 20
)

var mcpSupportedVersions = []string{
	"2025-06-18",
	"2025-03-26",
}

type mcpMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpTool struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	InputSchema map[string]any      `json:"inputSchema"`
	Annotations *mcpToolAnnotations `json:"annotations,omitempty"`
}

type mcpToolAnnotations struct {
	ReadOnlyHint    bool `json:"readOnlyHint"`
	DestructiveHint bool `json:"destructiveHint"`
	IdempotentHint  bool `json:"idempotentHint"`
	OpenWorldHint   bool `json:"openWorldHint"`
}

// MCP 处理 MCP Streamable HTTP 请求。
// POST 承载 JSON-RPC；DELETE 用于会话结束（当前实现无状态）；GET 不提供 SSE 流。
// @Summary     MCP 工具端点
// @Description 供 OpenClaw / LangBot 通过 MCP 调用集成动作，支持 initialize / tools/list / tools/call
// @Tags        integration
// @Accept      json
// @Produce     json
// @Success     200  {object}  object
// @Router      /integration/mcp [post]
// @Router      /integration/mcp [get]
// @Router      /integration/mcp [delete]
func (h *Handler) MCP(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodPost:
		h.mcpPost(c)
	case http.MethodDelete:
		c.Status(http.StatusNoContent)
	default:
		c.Header("Allow", "POST, DELETE")
		c.Status(http.StatusMethodNotAllowed)
	}
}

func (h *Handler) mcpPost(c *gin.Context) {
	contentType := strings.ToLower(strings.TrimSpace(c.GetHeader("Content-Type")))
	if contentType != "" && !strings.HasPrefix(contentType, "application/json") {
		writeMCPHTTPError(c, http.StatusUnsupportedMediaType, -32600, "Content-Type 必须是 application/json")
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, mcpMaxBodyBytes))
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeMCPHTTPError(c, http.StatusRequestEntityTooLarge, -32600, "请求体超过 1MB")
			return
		}
		writeMCPHTTPError(c, http.StatusBadRequest, -32700, "读取请求失败: "+err.Error())
		return
	}
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		writeMCPHTTPError(c, http.StatusBadRequest, -32700, "请求体为空")
		return
	}
	if !json.Valid(body) {
		writeMCPHTTPError(c, http.StatusBadRequest, -32700, "JSON 解析失败")
		return
	}
	if body[0] != '{' && body[0] != '[' {
		writeMCPHTTPError(c, http.StatusBadRequest, -32600, "JSON-RPC 请求必须是对象或数组")
		return
	}

	// 兼容旧版 MCP / JSON-RPC 批量请求；新版 MCP 虽然不再使用 batch，但支持它不影响单请求。
	if body[0] == '[' {
		var batch []json.RawMessage
		if err := json.Unmarshal(body, &batch); err != nil {
			writeMCPHTTPError(c, http.StatusBadRequest, -32700, "JSON 解析失败: "+err.Error())
			return
		}
		if len(batch) == 0 {
			writeMCPHTTPError(c, http.StatusBadRequest, -32600, "空批次")
			return
		}
		responses := make([]mcpResponse, 0, len(batch))
		for _, raw := range batch {
			resp, notification := h.handleMCPMessage(c, raw)
			if !notification {
				responses = append(responses, resp)
			}
		}
		writeMCPResponses(c, responses, true)
		return
	}

	resp, notification := h.handleMCPMessage(c, body)
	if notification {
		c.Status(http.StatusAccepted)
		return
	}
	writeMCPResponses(c, []mcpResponse{resp}, false)
}

// writeMCPResponses 同时兼容 application/json 与只接受 text/event-stream 的客户端。
// 按规范客户端通常同时声明两种 Accept，此时优先返回 JSON；只有客户端明确只要 SSE 时才回 SSE。
func writeMCPResponses(c *gin.Context, responses []mcpResponse, batch bool) {
	if len(responses) == 0 {
		c.Status(http.StatusAccepted)
		return
	}
	if mcpWantsSSE(c) {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Status(http.StatusOK)
		for _, resp := range responses {
			data, err := json.Marshal(resp)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(c.Writer, "event: message\ndata: %s\n\n", data)
		}
		c.Writer.Flush()
		return
	}
	if batch || len(responses) > 1 {
		c.JSON(http.StatusOK, responses)
		return
	}
	c.JSON(http.StatusOK, responses[0])
}

func mcpWantsSSE(c *gin.Context) bool {
	accept := strings.ToLower(c.GetHeader("Accept"))
	return strings.Contains(accept, "text/event-stream") && !strings.Contains(accept, "application/json")
}

func (h *Handler) handleMCPMessage(c *gin.Context, raw json.RawMessage) (mcpResponse, bool) {
	var msg mcpMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return mcpErrorResponse(nil, -32700, "JSON 解析失败: "+err.Error()), false
	}
	if msg.JSONRPC != "" && msg.JSONRPC != "2.0" {
		return mcpErrorResponse(msg.ID, -32600, "不支持的 JSON-RPC 版本: "+msg.JSONRPC), false
	}
	if strings.TrimSpace(msg.Method) == "" {
		if len(msg.ID) == 0 {
			return mcpResponse{}, true
		}
		return mcpErrorResponse(msg.ID, -32600, "缺少 method"), false
	}

	// JSON-RPC 通知不需要响应；当前只用到 notifications/initialized。
	if len(msg.ID) == 0 {
		return mcpResponse{}, true
	}

	switch msg.Method {
	case "initialize":
		return mcpResult(msg.ID, h.mcpInitializeResult(msg.Params)), false
	case "ping":
		return mcpResult(msg.ID, map[string]any{}), false
	case "tools/list":
		return mcpResult(msg.ID, h.mcpToolsList(c)), false
	case "tools/call":
		return h.mcpToolsCall(c, msg.ID, msg.Params), false
	case "resources/list":
		return mcpResult(msg.ID, map[string]any{"resources": []any{}}), false
	case "resources/templates/list":
		return mcpResult(msg.ID, map[string]any{"resourceTemplates": []any{}}), false
	case "prompts/list":
		return mcpResult(msg.ID, map[string]any{"prompts": []any{}}), false
	case "logging/setLevel":
		return mcpResult(msg.ID, map[string]any{}), false
	default:
		return mcpErrorResponse(msg.ID, -32601, "不支持的方法: "+msg.Method), false
	}
}

func (h *Handler) mcpInitializeResult(params json.RawMessage) map[string]any {
	var req struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(params, &req)

	return map[string]any{
		"protocolVersion": negotiateMCPVersion(req.ProtocolVersion),
		"capabilities": map[string]any{
			"tools": map[string]any{"listChanged": false},
		},
		"serverInfo": map[string]any{
			"name":    "go-backend-core-integration",
			"version": "1.0.0",
		},
		"instructions": "本服务把 gobackendcore 集成动作暴露为 MCP 工具。工具名中的下划线对应动作名的点号；" +
			"调用参数放在 arguments.params 中，字段与对应动作的 params 一致。" +
			"只读动作需要 owner:read 或 customer:read；写操作需要 owner:write；客服下单需要 customer:order_request。",
	}
}

func (h *Handler) mcpToolsList(c *gin.Context) map[string]any {
	scopes := callerScopes(c)
	tools := make([]mcpTool, 0)
	for _, def := range h.gateway.Registry().List() {
		if !authz.Has(scopes, def.Scopes...) {
			continue
		}
		tools = append(tools, mcpToolFromDefinition(def))
	}
	return map[string]any{"tools": tools}
}

func (h *Handler) mcpToolsCall(c *gin.Context, id json.RawMessage, params json.RawMessage) mcpResponse {
	var req struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return mcpErrorResponse(id, -32602, "tools/call 参数格式错误: "+err.Error())
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return mcpErrorResponse(id, -32602, "tools/call 缺少 name")
	}

	def, ok := h.findMCPTool(req.Name)
	if !ok {
		return mcpErrorResponse(id, -32602, "未知工具: "+req.Name)
	}
	if !authz.Has(callerScopes(c), def.Scopes...) {
		return mcpToolErrorResult(id, fmt.Sprintf("缺少作用域: %s 需要 %s", def.Name, strings.Join(def.Scopes, " 或 ")))
	}

	actionParams, requestID := parseMCPCallArguments(req.Arguments)
	result, err := h.gateway.Invoke(c.Request.Context(), action.Request{
		Action:    def.Name,
		Actor:     callerIdentity(c),
		Scopes:    callerScopes(c),
		Params:    actionParams,
		RequestID: requestID,
	})
	h.recordAction(c, def.Name, actionParams, requestID, result, err)
	if err != nil {
		return mcpToolErrorResult(id, "动作执行失败: "+err.Error())
	}
	return mcpToolSuccessResult(id, result.Data, result.Replayed)
}

func (h *Handler) findMCPTool(name string) (action.Definition, bool) {
	for _, def := range h.gateway.Registry().List() {
		if def.Name == name || mcpToolName(def.Name) == name {
			return def, true
		}
	}
	return action.Definition{}, false
}

func mcpToolFromDefinition(def action.Definition) mcpTool {
	description := strings.TrimSpace(def.Description)
	if description == "" {
		description = def.Name
	}
	description = fmt.Sprintf("%s（动作：%s", description, def.Name)
	if len(def.Scopes) > 0 {
		description += "；作用域：" + strings.Join(def.Scopes, " / ")
	}
	if def.Mutating {
		description += "；写操作"
	}
	description += "）"

	paramsSchema := any(map[string]any{
		"type":                 "object",
		"description":          "动作参数，字段与 " + def.Name + " 的 params 一致",
		"additionalProperties": true,
	})
	if len(def.ParamsSchema) > 0 {
		var parsed any
		if err := json.Unmarshal(def.ParamsSchema, &parsed); err == nil {
			paramsSchema = parsed
		}
	}

	properties := map[string]any{"params": paramsSchema}
	if def.Mutating {
		properties["request_id"] = map[string]any{
			"type":        "string",
			"description": "可选幂等键；相同调用方 + 动作 + request_id 只执行一次，重复提交返回首次结果",
		}
	}
	inputSchema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(def.ParamsSchema) > 0 {
		inputSchema["required"] = []string{"params"}
	}

	return mcpTool{
		Name:        mcpToolName(def.Name),
		Description: description,
		InputSchema: inputSchema,
		Annotations: &mcpToolAnnotations{
			ReadOnlyHint:    !def.Mutating,
			DestructiveHint: def.Mutating,
			IdempotentHint:  !def.Mutating,
			OpenWorldHint:   false,
		},
	}
}

// mcpToolName 把动作名转换为 MCP 工具名。
// 多数 MCP 客户端/模型对工具名只接受字母、数字、下划线和短横线，因此把点号换成下划线。
func mcpToolName(name string) string {
	replacer := strings.NewReplacer(".", "_", ":", "_", "/", "_", " ", "_")
	return replacer.Replace(strings.TrimSpace(name))
}

func parseMCPCallArguments(raw json.RawMessage) (json.RawMessage, string) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, ""
	}
	var args struct {
		Params    json.RawMessage `json:"params"`
		RequestID string          `json:"request_id"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return raw, ""
	}
	if len(bytes.TrimSpace(args.Params)) > 0 {
		return args.Params, args.RequestID
	}

	// 兼容直接传参数（不包 params）的客户端；同时移除 request_id，避免动作处理器看到伪字段。
	var direct map[string]json.RawMessage
	if err := json.Unmarshal(raw, &direct); err != nil {
		return raw, args.RequestID
	}
	delete(direct, "request_id")
	if len(direct) == 0 {
		return nil, args.RequestID
	}
	payload, err := json.Marshal(direct)
	if err != nil {
		return raw, args.RequestID
	}
	return payload, args.RequestID
}

func mcpResult(id json.RawMessage, result any) mcpResponse {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	return mcpResponse{JSONRPC: "2.0", ID: id, Result: result}
}

func mcpErrorResponse(id json.RawMessage, code int, message string) mcpResponse {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	return mcpResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &mcpError{Code: code, Message: message},
	}
}

func mcpToolErrorResult(id json.RawMessage, message string) mcpResponse {
	return mcpResult(id, map[string]any{
		"content": []map[string]any{{"type": "text", "text": message}},
		"isError": true,
	})
}

func mcpToolSuccessResult(id json.RawMessage, data any, replayed bool) mcpResponse {
	text, err := json.Marshal(data)
	if err != nil {
		return mcpToolErrorResult(id, "动作结果序列化失败: "+err.Error())
	}
	result := map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(text)}},
		"isError": false,
	}
	if replayed {
		result["_meta"] = map[string]any{"replayed": true}
	}
	var structured any
	if err := json.Unmarshal(text, &structured); err == nil {
		if _, ok := structured.(map[string]any); ok {
			result["structuredContent"] = structured
		}
	}
	return mcpResult(id, result)
}

func writeMCPHTTPError(c *gin.Context, status, code int, message string) {
	c.JSON(status, mcpErrorResponse(nil, code, message))
}

func negotiateMCPVersion(requested string) string {
	for _, version := range mcpSupportedVersions {
		if requested == version {
			return version
		}
	}
	return mcpLatestVersion
}
