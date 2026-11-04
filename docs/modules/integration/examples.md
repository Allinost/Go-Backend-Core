# 集成接入示例（n8n / OpenClaw / LangBot）

> 适用版本：P2（外盒分发器 + 连接器配置管理）
> 认证：外部工具统一使用 `X-API-Key`，不要使用控制台 JWT。
> 网络：生产容器内用 `http://gobackendcore:29090`，开发 EasyTier 用 `https://et.example.com:29090`；完整拓扑见 [connectivity.md](connectivity.md)。

本文命令中的 `$BACKEND_URL` 按环境替换：

```bash
# 生产容器内（n8n / OpenClaw / LangBot 所在容器）
export BACKEND_URL=http://gobackendcore:29090

# 开发机（EasyTier）
export BACKEND_URL=https://et.example.com:29090

# 本机直连
export BACKEND_URL=http://localhost:29090
```

## 1. 准备

| 使用方 | 建议作用域 | 用途 |
|---|---|---|
| n8n / OpenClaw | `owner:read`、`owner:write` | 库存、订单、退货、统计、组合销售 |
| LangBot | `customer:read`、`customer:order_request` | 客服只读查询、代客提交订单请求 |
| 集成管理台 | `integration:manage` | 连接器、事件、手动分发 |

请求头：

```http
X-API-Key: gbk_xxxxxxxxxxxxxxxx
Content-Type: application/json
```

统一响应：

```json
{ "code": 0, "message": "success", "data": {} }
```

## 2. 通用请求示例

```bash
# 库存查询（owner）
curl -X POST "$BACKEND_URL/api/v1/integration/actions/goodser.stock.query" \
  -H "X-API-Key: $OWNER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"params":{"inventory_id":"<inventory-id>","keyword":"徽章","page":1,"page_size":20}}'

# 统计总览（owner）
curl -X POST "$BACKEND_URL/api/v1/integration/actions/goodser.stats.summary" \
  -H "X-API-Key: $OWNER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"params":{"inventory_id":"<inventory-id>","days":30}}'

# 客户只读查询（LangBot）
curl -X POST "$BACKEND_URL/api/v1/integration/actions/goodser.customer.stock_query" \
  -H "X-API-Key: $CUSTOMER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"params":{"inventory_id":"<inventory-id>","keyword":"拍立得"}}'

# 客户提交订单请求（LangBot）
curl -X POST "$BACKEND_URL/api/v1/integration/actions/goodser.customer.order_request" \
  -H "X-API-Key: $CUSTOMER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "params":{
      "inventory_id":"<inventory-id>",
      "order_info":"微信客服代客下单",
      "items":[{"product_id":"<product-id>","quantity":1}]
    }
  }'
```

客户订单请求会创建 `pending` 预留并立即锁定库存，返回预留单号 `RSV-...`。

### 2.1 写操作幂等

写操作可以带 `request_id`，相同调用方 + 动作 + `request_id` 只执行一次：

```bash
curl -X POST "$BACKEND_URL/api/v1/integration/actions/goodser.order.create" \
  -H "X-API-Key: $OWNER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "request_id":"n8n-order-20261103-001",
    "params":{
      "inventory_id":"<inventory-id>",
      "order_info":"n8n 自动补货",
      "items":[{"product_id":"<product-id>","quantity":1}]
    }
  }'
```

命中缓存时响应里的 `data.replayed` 为 `true`。`request_id` 建议使用业务单号或 UUID，重试时保持不变。

## 3. n8n

推荐使用 **HTTP Request** 节点：

| 配置项 | 值 |
|---|---|
| Method | `POST` |
| URL | `http://gobackendcore:29090/api/v1/integration/actions/goodser.stock.query`（开发：`$BACKEND_URL`） |
| Authentication | None |
| Header `X-API-Key` | owner key |
| Body Content Type | JSON |
| Body | `{"params":{...}}` |

库存变化通知使用 Webhook 连接器，n8n 侧用 **Webhook** 节点接收，校验签名后处理。

## 4. OpenClaw

OpenClaw 按普通 HTTP 工具接入即可，建议把动作名与参数封装成固定工具：

```json
{
  "name": "goodser_order_create",
  "method": "POST",
  "url": "http://gobackendcore:29090/api/v1/integration/actions/goodser.order.create",
  "headers": { "X-API-Key": "<owner-key>" },
  "body": {
    "params": {
      "inventory_id": "<inventory-id>",
      "order_info": "OpenClaw 自动补货",
      "bundle": {
        "bundle_id": "<bundle-id>",
        "main_product_id": "<main-product-id>",
        "main_quantity": 1,
        "quantity": 1,
        "sub_items": [{ "product_id": "<sub-product-id>", "quantity": 1 }]
      }
    }
  }
}
```

`goodser.order.create` 的 `type` 可省略（出库）或传 `reserve`（预留）。组合包使用 `combo_id` + `combo_quantity`，捆使用 `bundle`，两者不要同时出现。

## 5. LangBot

LangBot 只允许两类调用：

1. `goodser.customer.stock_query`：查询商品、组合包、捆与可售数量。
2. `goodser.customer.order_request`：提交订单请求，不直接改库存。

推荐话术链路：

```text
用户：还有某某徽章吗？
LangBot -> customer.stock_query -> 返回 available / total / reserved
用户：帮我留 1 个
LangBot -> customer.order_request -> 返回 RSV-... 预留单号
```

客户 key 访问 owner 动作会返回 `403`，不要在生产环境复用 owner key。

## 6. 配置 Webhook 连接器

```bash
curl -X POST "$BACKEND_URL/api/v1/integration/connectors" \
  -H "X-API-Key: $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name":"n8n 生产通知",
    "type":"webhook",
    "url":"https://n8n.example.com/webhook/goodser",
    "secret":"replace-with-random-secret",
    "topics":["goodser.order.*","goodser.return.*"],
    "timeout_seconds":10,
    "enabled":true
  }'
```

- `secret` 由服务端 AES-256-GCM 加密保存，读取接口只返回 `secret_set`。
- `topics` 留空表示订阅全部；支持 `goodser.*`、`goodser.order.*`、精确主题。
- 更新时省略 `secret` 表示不变更，传 `""` 表示清除。
- 可在 Web「集成管理」或 `POST /integration/connectors/:id/test` 发送测试事件。
- 手动补投：`POST /api/v1/integration/events/dispatch`（`integration:manage`）。

## 7. 接收端校验签名

Webhook 请求头：

| 头 | 说明 |
|---|---|
| `X-Integration-Event` | 事件主题 |
| `X-Integration-Event-ID` | 事件 ID |
| `X-Integration-Timestamp` | Unix 秒 |
| `X-Integration-Signature` | `sha256=HMAC_SHA256(secret, timestamp + "." + body)` |

Node.js：

```js
const crypto = require('crypto')

function verify(rawBody, timestamp, signature, secret) {
  const expected = 'sha256=' + crypto
    .createHmac('sha256', secret)
    .update(timestamp + '.' + rawBody)
    .digest('hex')
  return crypto.timingSafeEqual(Buffer.from(expected), Buffer.from(signature))
}
```

Python：

```python
import hmac, hashlib

def verify(raw_body: bytes, timestamp: str, signature: str, secret: str) -> bool:
    expected = "sha256=" + hmac.new(
        secret.encode(), f"{timestamp}.".encode() + raw_body, hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(expected, signature)
```

生产环境应同时限制时间戳偏差（例如 ±5 分钟）并按 `X-Integration-Event-ID` 去重。

## 8. 事件主题

| 主题 | 触发时机 |
|---|---|
| `goodser.order.created` | 创建出库/预留单 |
| `goodser.order.confirmed` | 确认出库 |
| `goodser.order.cancelled` | 取消出库 |
| `goodser.reserve.converted` | 预留转出库 |
| `goodser.reserve.cancelled` | 取消预留 |
| `goodser.return.created` | 创建退货单 |
| `goodser.return.confirmed` | 确认退货 |
| `goodser.customer.order_requested` | 客户提交订单请求 |
| `integration.test` | 连接器测试事件 |

## 9. 排查

| 现象 | 检查 |
|---|---|
| `401/403` | API Key 是否正确、是否缺少作用域 |
| 事件一直 `pending` | 分发器是否启用、是否有匹配 topic 的连接器 |
| 事件 `failed` | 查看 `last_error`，确认接收端 2xx、签名密钥一致 |
| Webhook 返回非 2xx | 分发器会按指数退避重试，超过 `max_attempts` 后停止 |
| 连接器启动时被跳过 | 检查 URL、密钥是否可解密、配置是否合法 |

## 10. MCP 端点（OpenClaw / LangBot 推荐）

OpenClaw 和 LangBot 可以直接挂载后端 MCP 端点，不必为每个动作写 HTTP 封装：

```text
http://gobackendcore:29090/api/v1/integration/mcp
```

认证：

```http
X-API-Key: gbk_xxxxxxxxxxxxxxxx
# 或
Authorization: Bearer gbk_xxxxxxxxxxxxxxxx
```

最小冒烟请求：

```bash
curl -s -X POST "$BACKEND_URL/api/v1/integration/mcp" \
  -H "X-API-Key: $OWNER_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

工具名规则：`goodser.stock.query` → `goodser_stock_query`，调用参数放在 `arguments.params`。写操作可加 `arguments.request_id` 做幂等。完整协议、OpenClaw / LangBot 配置与错误码见 [mcp.md](mcp.md)。
