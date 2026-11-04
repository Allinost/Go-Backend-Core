# MCP 端点（/api/v1/integration/mcp）

> 用途：OpenClaw、LangBot 等 MCP 客户端直接调用集成动作，不需要为每个动作单独写 HTTP 封装。
> 实现：`internal/modules/integration/mcp.go`
> 协议：MCP Streamable HTTP + JSON-RPC 2.0
> 相关文档：[集成基座](index.md)｜[网络与部署](connectivity.md)｜[接入示例](examples.md)

## 1. 端点与认证

| 项目 | 值 |
|---|---|
| URL（1panel-network） | `http://gobackendcore:29090/api/v1/integration/mcp` |
| URL（EasyTier 开发） | `https://et.example.com:29090/api/v1/integration/mcp` |
| Method | `POST`（JSON-RPC）；`DELETE` 结束会话；`GET` 不提供 SSE |
| 认证 | `X-API-Key: gbk_xxx` 或 `Authorization: Bearer gbk_xxx` |
| 协议版本 | `2025-06-18` / `2025-03-26`（Streamable HTTP） |
| 会话 | 无状态，不返回 `Mcp-Session-Id` |
| 幂等 | 写操作可在 `arguments.request_id` 传幂等键 |
| 限流 | 按 API Key 每分钟配额（`integration.rate_limit`） |

MCP 客户端推荐用 Header 传 Key；不支持自定义 Header 的客户端可用 Bearer 形式。控制台 JWT 也能通过认证，但不建议给外部工具使用。

## 2. 客户端配置

### 2.1 OpenClaw

```json5
{
  mcp: {
    servers: {
      goodser: {
        url: "http://gobackendcore:29090/api/v1/integration/mcp",
        transport: "streamable-http",
        enabled: true,
        headers: { "X-API-Key": "${GOODSER_API_KEY}" },
        toolFilter: {
          include: ["goodser_*"]
        }
      }
    }
  }
}
```

开发环境把 `url` 换成 `https://et.example.com:29090/api/v1/integration/mcp`。

### 2.2 LangBot

在 LangBot 的 MCP 服务配置中新增一个 Streamable HTTP 服务：

- URL：`http://gobackendcore:29090/api/v1/integration/mcp`
- Header：`X-API-Key: <langbot-customer-key>`
- 只保留 customer 工具（或通过 LangBot 侧工具白名单限制）

不同 LangBot 版本的字段名以管理界面为准；协议层与 OpenClaw 相同。

## 3. 协议交互

### 3.1 initialize

请求：

```bash
curl -s -X POST http://gobackendcore:29090/api/v1/integration/mcp \
  -H "X-API-Key: $GOODSER_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "jsonrpc":"2.0",
    "id":1,
    "method":"initialize",
    "params":{
      "protocolVersion":"2025-06-18",
      "capabilities":{},
      "clientInfo":{"name":"openclaw","version":"1.0"}
    }
  }'
```

响应：

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2025-06-18",
    "capabilities": { "tools": { "listChanged": false } },
    "serverInfo": {
      "name": "go-backend-core-integration",
      "version": "1.0.0"
    },
    "instructions": "..."
  }
}
```

### 3.2 tools/list

```bash
curl -s -X POST http://gobackendcore:29090/api/v1/integration/mcp \
  -H "X-API-Key: $GOODSER_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'
```

工具名规则：动作名中的 `.` 替换为 `_`。

| 动作 | MCP 工具名 |
|---|---|
| `goodser.stock.query` | `goodser_stock_query` |
| `goodser.order.create` | `goodser_order_create` |
| `goodser.customer.stock_query` | `goodser_customer_stock_query` |
| `goodser.customer.order_request` | `goodser_customer_order_request` |

`tools/list` 会按当前 API Key 的作用域过滤，只返回可调用的工具。

### 3.3 tools/call

```bash
curl -s -X POST http://gobackendcore:29090/api/v1/integration/mcp \
  -H "X-API-Key: $GOODSER_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "jsonrpc":"2.0",
    "id":3,
    "method":"tools/call",
    "params":{
      "name":"goodser_stock_query",
      "arguments":{
        "params":{
          "inventory_id":"<inventory-id>",
          "keyword":"徽章",
          "page":1,
          "page_size":20
        }
      }
    }
  }'
```

响应：

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      { "type": "text", "text": "{\"items\":[...]}" }
    ],
    "structuredContent": { "items": [] },
    "isError": false
  }
}
```

说明：

- `arguments.params` 与动作网关的 `params` 完全一致。
- 也兼容直接传 `arguments`（不包 `params`），但推荐统一包一层，避免与动作字段重名。
- 动作执行失败时返回 `isError: true`，`content[0].text` 是错误信息；协议错误才返回 JSON-RPC `error`。
- 服务端会把 MCP 调用写入 `zzz_integration_action_logs`，`auth_method` 为 `apikey`。

### 3.4 写操作幂等

写操作（`mutating=true`）可以在 `arguments` 里带 `request_id`：

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "goodser_order_create",
    "arguments": {
      "request_id": "n8n-order-20261103-001",
      "params": {
        "inventory_id": "<inventory-id>",
        "order_info": "n8n 自动补货",
        "items": [{ "product_id": "<product-id>", "quantity": 1 }]
      }
    }
  }
}
```

规则：

- 相同 `调用方 + 动作 + request_id` 只执行一次，重复提交返回首次结果。
- 幂等缓存默认保留 24 小时（`integration.idempotency.ttl_seconds`）。
- 只缓存成功结果；失败后可安全重试。
- 命中缓存时，MCP 结果会带 `_meta.replayed=true`。
- 不传 `request_id` 时保持原行为，不做幂等。
- `request_id` 由调用方生成，建议用业务单号或 UUID，不要每次重试都换新值。

### 3.5 通知与关闭

- `notifications/initialized`：返回 `202 Accepted`，无响应体。
- `DELETE /api/v1/integration/mcp`：返回 `204 No Content`，当前无状态，仅用于协议兼容。
- `GET /api/v1/integration/mcp`：返回 `405`，不支持服务端主动推送。
- 如果客户端只接受 `Accept: text/event-stream`，POST 响应会用 SSE 返回单次 JSON-RPC 结果；同时接受 JSON 时优先返回 JSON。

## 4. 作用域与工具可见性

| API Key 作用域 | 可见工具 |
|---|---|
| `owner:read` | 库存、订单查询、统计、组合包、捆、字典 |
| `owner:write` | 订单创建/确认/取消、退货、组合包/捆写入 |
| `customer:read` | `goodser_customer_stock_query` |
| `customer:order_request` | `goodser_customer_order_request` |
| `*` / `*:*` | 全部（仅限控制台/管理员，不建议外部工具使用） |

LangBot 必须使用 customer Key；即使它拿到 owner 工具名，服务端也会返回 `isError: true`。

## 5. 错误码

| 场景 | 响应 |
|---|---|
| JSON 不合法 / 请求体为空 | HTTP 400，JSON-RPC `-32700` |
| 不是对象/数组、缺少 method | HTTP 400，JSON-RPC `-32600` |
| 未知 method | JSON-RPC `-32601` |
| 未知工具、缺少工具名、参数格式错误 | JSON-RPC `-32602` |
| 缺少作用域 / 动作执行失败 | HTTP 200，`result.isError = true` |
| 未认证 / Key 无效 | HTTP 401 |
| 超过 API Key 每分钟配额 | HTTP 429 |
| Content-Type 不是 application/json | HTTP 415 |
| 请求体超过 1MB | HTTP 413 |

## 6. 冒烟测试

在 n8n / OpenClaw / LangBot 所在容器内：

```bash
# 1. 初始化
docker exec n8n sh -lc "wget -qO- \
  --header='X-API-Key: gbk_xxx' \
  --header='Content-Type: application/json' \
  --post-data='{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2025-06-18\"}}' \
  http://gobackendcore:29090/api/v1/integration/mcp"

# 2. 工具列表
docker exec n8n sh -lc "wget -qO- \
  --header='X-API-Key: gbk_xxx' \
  --header='Content-Type: application/json' \
  --post-data='{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/list\"}' \
  http://gobackendcore:29090/api/v1/integration/mcp"
```

开发机可直接用 `curl`，地址换成 `https://et.example.com:29090/api/v1/integration/mcp`。

## 7. 当前限制

- Goodser 的主要动作已提供字段级 `params` JSON Schema；未配置 schema 的动作仍回退为通用 `params` 对象。
- 无状态实现，不支持服务端主动通知；工具调用是请求-响应模式。客户端只接受 SSE 时可用 SSE 返回单次响应，但 GET 不保持长连接。
- 仅支持 Streamable HTTP 传输（`2025-03-26` 及以上），不支持旧版 `2024-11-05` 的 HTTP+SSE 双端点传输。
- 幂等缓存单实例内用 singleflight 合并并发；多实例部署时依赖 Redis 存储，极端并发下仍建议调用方使用稳定 `request_id` 并做结果校验。
- 不支持 OAuth 授权流程，使用现有 API Key。
- 工具名做了点号转下划线处理；如果客户端允许点号，也接受原始动作名。
- MCP 端点与动作网关共享同一套作用域和日志，不额外引入权限模型。

## 8. 安全建议

1. 只允许 1panel-network / EasyTier 访问，不要公网暴露。
2. OpenClaw / LangBot 各用独立 Key，LangBot 只给 customer 作用域。
3. 写操作工具在 Agent 侧要求二次确认，尤其是 `goodser_order_create`、`goodser_order_cancel`、`goodser_return_confirm`。
4. 不要把 `*:*` Key 配给任何外部工具。
5. 写操作重试必须复用同一个 `request_id`；不要每次重试生成新值。
6. 关注 `HTTP 429`，按 API Key 的每分钟配额调整 `integration.rate_limit.requests_per_minute`。
7. 定期轮换 Key，并检查 `zzz_integration_action_logs` 的异常调用与 `replayed` 记录。
