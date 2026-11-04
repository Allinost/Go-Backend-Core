# n8n / OpenClaw / LangBot 对接与网络拓扑

> 适用场景：`gobackendcore` 与 n8n、OpenClaw、LangBot 都接入 `1panel-network`。
> 开发环境：三者与后端通过 EasyTier 入口 `et.example.com` 访问。
> 相关文档：[集成基座](index.md)｜[接入示例](examples.md)｜[MCP 端点](mcp.md)

## 1. 结论先行

| 能力 | 推荐方案 | 当前状态 |
|---|---|---|
| n8n 调用后端 | HTTP Request + `X-API-Key` 调用 `/api/v1/integration/actions/:name` | ✅ 后端已具备 |
| 后端事件通知 n8n | Webhook 连接器 + HMAC 签名 | ✅ 后端已具备 |
| OpenClaw 调用后端 | MCP 端点 `/api/v1/integration/mcp` 或 Skill/HTTP 脚本 | ✅ MCP 端点已具备 |
| LangBot 客服查询/下单 | MCP 端点（customer 作用域）或插件 HTTP | ✅ MCP 端点已具备，仅开放 customer 作用域 |
| 后端事件通知 LangBot | 不建议，LangBot 以拉取为主 | — |
| 后端事件通知 OpenClaw | 可选，需配置 Gateway hooks | ⚠️ 非首版必需 |

> 补充：n8n 适合做自动化编排中心，但不建议替代 OpenClaw / LangBot 的渠道与会话层，详见第 13 节。

**统一原则**

1. 三者都不直连 MySQL/Redis，也不复用 Web 控制台 JWT。
2. 业务调用统一走动作网关：`POST /api/v1/integration/actions/{action}`。
3. 外部工具只拿 `X-API-Key`，按作用域最小授权。
4. 生产容器之间用 **容器名 / 服务名 + 容器端口**；开发机用 **`et.example.com` + 宿主机发布端口**。
5. 网络互通不等于可信：内部网络仍要校验 API Key，Webhook 仍要校验 HMAC。

## 2. 调用方向与拓扑

```text
                    ┌──────────────────────────────┐
                    │        gobackendcore          │
                    │  :29090 业务 API / actions    │
                    │  :29092 调试（不要对外）      │
                    └───────▲──────────┬───────────┘
                            │          │
          X-API-Key 调用动作 │          │ Webhook + HMAC
                            │          ▼
        ┌───────────────────┴──┐   ┌──────────────┐
        │ n8n / OpenClaw /     │   │    n8n       │
        │ LangBot              │   │ :5678        │
        └──────────────────────┘   └──────────────┘

  n8n        ：双向（调用动作 + 接收事件）
  OpenClaw   ：默认拉取（Agent 调后端），可选接收 hooks
  LangBot    ：默认拉取（客服调后端），不接收后端事件
```

生产环境：

```text
1panel-network
├── gobackendcore    :29090
├── n8n              :5678
├── openclaw         :18789
└── langbot          :5300
```

开发环境：

```text
开发机 ──EasyTier──► et.example.com
                     ├── :29090  gobackendcore
                     ├── :5678   n8n
                     ├── :18789  openclaw
                     └── :5300   langbot
```

## 3. 地址速查

> 下表端口是各项目的默认容器端口。1Panel 安装时可能改过宿主机映射端口，开发环境必须以实际映射为准。

| 服务 | 生产（1panel-network 内） | 开发（EasyTier） | 说明 |
|---|---|---|---|
| 后端 API | `http://gobackendcore:29090` | `https://et.example.com:29090` | 业务 API / actions |
| 后端调试 | `http://gobackendcore:29092` | 不建议暴露 | Swagger / 运维接口 |
| n8n | `http://n8n:5678` | `https://et.example.com:5678` | UI 与 Webhook 同端口 |
| OpenClaw Gateway | `http://openclaw:18789` | `https://et.example.com:18789` | WS + HTTP 同端口 |
| LangBot | `http://langbot:5300` | `https://et.example.com:5300` | HTTP API + WebUI |
| LangBot OneBot | `langbot:2280-2290` | 按映射 | 仅 IM 平台反连，后端不用 |

开发环境若 TLS 不通，先改用 `http://et.example.com:<端口>`；以实际证书和反代配置为准。`docs/login3rd` 中后端回调使用 `https://et.example.com:29090`，说明该入口通常有 TLS 终结。

### 3.1 先确认容器名和端口

在服务器执行：

```bash
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Ports}}' \
  | grep -Ei 'n8n|langbot|openclaw|gobackend|go-backend'
```

确认网络归属：

```bash
docker inspect -f '{{range $name, $_ := .NetworkSettings.Networks}}{{$name}} {{end}}' gobackendcore
docker inspect -f '{{range $name, $_ := .NetworkSettings.Networks}}{{$name}} {{end}}' n8n
```

确认实际端口映射：

```bash
docker port n8n
docker port langbot
docker port openclaw
docker port gobackendcore
```

如果容器名不是 `n8n` / `langbot` / `openclaw` / `gobackendcore`，把本文地址中的名字替换成实际容器名即可；同一 Docker 网络内容器名会作为 DNS 名解析。

## 4. 接入 1panel-network

### 4.1 确认网络存在

```bash
docker network inspect 1panel-network >/dev/null && echo OK
```

### 4.2 让容器加入同一网络

三种方式任选：

1. **1Panel 控制台**：容器 → 编辑 → 网络 → 勾选 `1panel-network` → 保存/重建。
2. **命令行临时接入**（容器重建后失效）：
   ```bash
   docker network connect 1panel-network n8n
   docker network connect 1panel-network langbot
   docker network connect 1panel-network openclaw
   ```
3. **Compose 声明**（推荐，持久化）：
   ```yaml
   services:
     app:
       container_name: gobackendcore
       networks:
         - 1panel-network

     n8n:
       networks:
         - 1panel-network

     openclaw:
       networks:
         - 1panel-network

     langbot:
       networks:
         - 1panel-network

   networks:
     1panel-network:
       external: true
   ```

> 本仓库的 `docker-compose.yml` 已把 `app` 和 `chrome` 接入 `1panel-network`，并给 `app` 增加了 `gobackendcore` 网络别名。若线上使用独立的 1Panel Compose 应用，请按同样方式加别名，或把本文地址替换成实际容器名。

### 4.3 验证 DNS 与连通性

从 `gobackendcore` 容器内测试另外三者：

```bash
docker exec gobackendcore sh -lc 'wget -S -O /dev/null http://n8n:5678/healthz'
docker exec gobackendcore sh -lc 'wget -S -O /dev/null http://langbot:5300/'
docker exec gobackendcore sh -lc 'wget -S -O /dev/null http://openclaw:18789/health'
```

从 n8n 容器内测试后端：

```bash
docker exec n8n sh -lc \
  "wget -qO- --header='X-API-Key: gbk_xxx' http://gobackendcore:29090/api/v1/integration/ping"
```

> 能解析但连不上，通常是端口没监听、容器没加入网络、或 1Panel 防火墙/反向代理拦截。

### 4.4 开发环境 EasyTier

1. 开发机先连上 EasyTier，确认 `et.example.com` 可解析、可访问。
2. 本机后端 `.env` 继续使用 `EASY_TIER_NAS_URL=et.example.com`（MySQL/Redis 等基础设施）。
3. 开发机访问 n8n/OpenClaw/LangBot 时，用 **宿主机发布端口**：
   ```bash
   curl -I http://et.example.com:5678
   curl -I http://et.example.com:5300
   curl -I http://et.example.com:18789
   ```
4. 开发机上的后端要作为被调方时，三者统一填：
   ```text
   https://et.example.com:29090
   ```
5. 如果 EasyTier 没有提供 DNS，可把 `et.example.com` 写入 hosts，或直接使用 EasyTier 虚拟 IP。
6. 只监听 `127.0.0.1` 的服务，EasyTier 侧访问不到；需要监听 `0.0.0.0` 并做好防火墙限制。

## 5. 认证与作用域

### 5.1 创建 API Key

先用管理员账号登录拿 JWT：

```bash
curl -s -X POST "$BACKEND_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"<password>"}'
```

再用 JWT 创建三个独立 Key（`raw_key` 只在创建时返回一次）：

```bash
# n8n / OpenClaw：店主工具
curl -s -X POST "$BACKEND_URL/api/v1/auth/api-keys" \
  -H "Authorization: Bearer $JWT" \
  -H 'Content-Type: application/json' \
  -d '{"name":"n8n-owner","scopes":"owner:read,owner:write","expires_in":"8760h"}'

# LangBot：客服
curl -s -X POST "$BACKEND_URL/api/v1/auth/api-keys" \
  -H "Authorization: Bearer $JWT" \
  -H 'Content-Type: application/json' \
  -d '{"name":"langbot-customer","scopes":"customer:read,customer:order_request","expires_in":"8760h"}'

# 集成管理：连接器/事件管理
curl -s -X POST "$BACKEND_URL/api/v1/auth/api-keys" \
  -H "Authorization: Bearer $JWT" \
  -H 'Content-Type: application/json' \
  -d '{"name":"integration-admin","scopes":"integration:manage,owner:read","expires_in":"8760h"}'
```

### 5.2 作用域对照

| 使用方 | 作用域 | 可调用 |
|---|---|---|
| n8n | `owner:read,owner:write` | 库存、订单、退货、统计、组合包、捆 |
| OpenClaw | `owner:read,owner:write`（或只读） | 同上；自动补货/查询建议限制为 read |
| LangBot | `customer:read,customer:order_request` | 仅 `goodser.customer.stock_query`、`goodser.customer.order_request` |
| 集成管理 | `integration:manage,owner:read` | 连接器、事件、手动分发 |

注意：

- 默认创建的 Key 如果不传 `scopes` 会是 `*:*`，生产不要这样。
- LangBot Key 调 owner 动作会返回 `403`，这是预期行为。
- 轮换时先建新 Key、切换配置、确认流量正常，再用 `PUT /api/v1/auth/api-keys/:id` 把旧 Key 置为禁用，最后删除。
- 不要把 Key 写进前端代码、n8n 普通节点文本或 Git；放在各平台的凭据/环境变量中。

## 6. n8n 对接

### 6.1 n8n → 后端：HTTP Request 节点

以库存查询为例：

| 配置项 | 值 |
|---|---|
| Method | `POST` |
| URL | `http://gobackendcore:29090/api/v1/integration/actions/goodser.stock.query` |
| Authentication | Generic Credential Type → Header Auth |
| Header Name | `X-API-Key` |
| Header Value | n8n 的 owner Key |
| Send Body | JSON |
| Body | `{"params":{"inventory_id":"<inventory-id>","keyword":"徽章","page":1,"page_size":20}}` |
| Timeout | `30000` |

开发环境把 URL 换成：

```text
https://et.example.com:29090/api/v1/integration/actions/goodser.stock.query
```

统一响应：

```json
{ "code": 0, "message": "success", "data": { "action": "...", "data": {}, "duration": 12345678 } }
```

注意结果有两层 `data`：外层是统一响应，内层 `data` 才是动作返回值。n8n 表达式通常写 `{{ $json.data.data }}`。

建议在 n8n 里封装一个 **子工作流**，入参为 `action`、`params` 与 `request_id`，内部只做一个 HTTP Request，避免每个流程重复配置 Header 和错误处理。

写操作（`goodser.order.create`、`goodser.return.create` 等）建议在 body 顶层带 `request_id`：

```json
{
  "request_id": "n8n-order-20261103-001",
  "params": { "inventory_id": "<inventory-id>", "items": [] }
}
```

n8n 的重试必须复用同一个 `request_id`，否则仍会重复执行。

### 6.2 后端 → n8n：Webhook 节点

在 n8n 新建工作流，Webhook 节点配置：

| 配置项 | 值 |
|---|---|
| HTTP Method | `POST` |
| Path | `goodser-events` |
| Authentication | `None` |
| Respond | `Using Respond to Webhook Node` |
| Raw Body | **开启** |
| Response Code | `200` |

生产 URL：

```text
http://n8n:5678/webhook/goodser-events
```

开发 URL：

```text
https://et.example.com:5678/webhook/goodser-events
```

> `webhook-test` 地址只监听一次，联调完成后必须激活工作流并使用 `/webhook/` 生产地址，否则连接器会一直失败重试。

### 6.3 校验 HMAC 签名

后端签名规则：

```text
X-Integration-Signature = "sha256=" + HMAC_SHA256(secret, timestamp + "." + raw_body)
```

Code 节点（Run Once for Each Item）：

```js
const crypto = require('crypto');

const item = $input.first();
const headers = item.json.headers || {};

// Webhook 节点开启 Raw Body 后，原始字节在 binary.data.data（base64）
const raw = item.binary?.data?.data
  ? Buffer.from(item.binary.data.data, 'base64')
  : Buffer.from(JSON.stringify(item.json.body));

const ts = String(headers['x-integration-timestamp'] || '');
const sig = String(headers['x-integration-signature'] || '');
const secret = $env.GOODSER_WEBHOOK_SECRET;

if (!ts || !sig || !secret) return [{ json: { valid: false, reason: 'missing signature' } }];

const now = Math.floor(Date.now() / 1000);
if (!Number.isFinite(Number(ts)) || Math.abs(now - Number(ts)) > 300) {
  return [{ json: { valid: false, reason: 'expired timestamp' } }];
}

const expected = 'sha256=' + crypto
  .createHmac('sha256', secret)
  .update(Buffer.concat([Buffer.from(`${ts}.`, 'utf8'), raw]))
  .digest('hex');

const a = Buffer.from(expected);
const b = Buffer.from(sig);
const valid = a.length === b.length && crypto.timingSafeEqual(a, b);

const envelope = item.json.body || {};
return [{
  json: {
    valid,
    event: envelope.event,
    event_id: envelope.event_id,
    occurred_at: envelope.occurred_at,
    payload: envelope.payload,
    metadata: envelope.metadata,
  },
}];
```

后续节点：

1. `IF` 节点判断 `{{ $json.valid }}`。
2. `false` → `Respond to Webhook` 返回 `401`，结束。
3. `true` → `Respond to Webhook` 立即返回 `200`。
4. 再执行业务分支（发通知、写表、调后端动作）。

注意：

- n8n 1.x 默认禁止 Code 节点读取环境变量。用 `$env` 时需要在 n8n 容器设置 `N8N_BLOCK_ENV_ACCESS_IN_NODE=false`，或改用工作流变量/凭据方案。
- 必须用原始请求体验签，不能用重新 `JSON.stringify` 的 body；否则字段顺序、空格、中文编码都会导致签名不一致。
- 建议按 `X-Integration-Event-ID` 做去重（Redis/数据库/工作流静态数据），并限制时间戳偏差 ±5 分钟。
- n8n 必须在约 10 秒内返回 2xx；耗时逻辑放在 `Respond to Webhook` 之后异步执行。

### 6.4 在 Goodser 侧创建连接器

用管理 Key 调用：

```bash
curl -s -X POST "$BACKEND_URL/api/v1/integration/connectors" \
  -H "X-API-Key: $ADMIN_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "name":"n8n-goodser-events",
    "type":"webhook",
    "url":"http://n8n:5678/webhook/goodser-events",
    "secret":"replace-with-a-long-random-secret",
    "topics":["goodser.*"],
    "timeout_seconds":10,
    "enabled":true
  }'
```

开发环境把 `url` 改为：

```text
https://et.example.com:5678/webhook/goodser-events
```

创建后：

1. `POST /api/v1/integration/connectors/:id/test` 发送 `integration.test` 测试事件。
2. `POST /api/v1/integration/events/dispatch` 手动跑一轮分发。
3. 在 n8n 执行记录里确认收到事件且验签通过。

### 6.5 事件主题

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
| `integration.test` | 连接器测试 |

`topics` 留空表示订阅全部；支持 `*`、`goodser.*`、`goodser.order.*` 和精确主题。

## 7. OpenClaw 对接

### 7.1 方向

OpenClaw 是 Agent/Gateway，推荐让它 **主动调用** 后端动作：

```text
用户在 IM 下指令 → OpenClaw Agent → 工具 → gobackendcore action → 返回结果
```

默认不建议后端直接推事件给 OpenClaw；如果确实需要事件唤醒 Agent，再配置 Gateway 入站 hooks。

### 7.2 方案 A：MCP 端点（推荐）

后端已提供 `/api/v1/integration/mcp`，把动作注册表动态暴露为 MCP 工具，OpenClaw 与 LangBot 可复用同一套工具：

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

MCP 工具按动作生成：

| 工具 | 后端动作 | 作用域 |
|---|---|---|
| `goodser_stock_query` | `goodser.stock.query` | `owner:read` |
| `goodser_order_list` | `goodser.order.list` | `owner:read` |
| `goodser_order_create` | `goodser.order.create` | `owner:write` |
| `goodser_stats_summary` | `goodser.stats.summary` | `owner:read` |
| `goodser_customer_stock_query` | `goodser.customer.stock_query` | `customer:read` |
| `goodser_customer_order_request` | `goodser.customer.order_request` | `customer:order_request` |

`tools/list` 会按 Key 作用域过滤；LangBot 的 customer Key 只能看到最后两个工具。完整协议、错误码与冒烟测试见 [mcp.md](mcp.md)。

### 7.3 方案 B：Skill + HTTP 脚本（兼容旧客户端）

在 OpenClaw 的 Skill 目录放一个脚本，通过环境变量读取 Key：

```python
# goodser_client.py
import os
import requests

BASE = os.environ["GOODSER_BASE_URL"]          # http://gobackendcore:29090
KEY = os.environ["GOODSER_API_KEY"]

def invoke(action: str, params: dict) -> dict:
    resp = requests.post(
        f"{BASE}/api/v1/integration/actions/{action}",
        headers={"X-API-Key": KEY, "Content-Type": "application/json"},
        json={"params": params},
        timeout=15,
    )
    resp.raise_for_status()
    body = resp.json()
    if body.get("code") != 0:
        raise RuntimeError(body)
    return body["data"]["data"]
```

在 `SKILL.md` 中明确：

- 查库存用 `goodser.stock.query`。
- 建单/确认/取消用对应 `owner:*` 动作。
- 任何时候都不要把 API Key 输出给用户。
- 写操作前必须复述商品、数量、单据类型让用户确认。

> 用 `exec` 跑脚本时要限制可执行命令和工作目录，避免 Agent 获得过大的主机权限。

### 7.4 可选：OpenClaw 入站 hooks

如果后端事件需要唤醒 Agent，可在 OpenClaw 配置中启用 hooks：

```json5
{
  hooks: {
    enabled: true,
    path: "/hooks",
    token: "<long-random-token>"
  }
}
```

连接器目标形如：

```text
http://openclaw:18789/hooks/<mapping>
```

注意：

- hooks 需要声明 mapping，后端当前的通用事件信封不保证与 mapping 期望的字段直接匹配。
- 需要把 `event/payload` 映射为 Agent 输入，必要时加一层转换服务。
- 首版建议只做拉取，不做事件推送，降低联调复杂度。

## 8. LangBot 对接

### 8.1 只开放两个动作

LangBot 是客服角色，只允许：

| 动作 | 作用域 | 用途 |
|---|---|---|
| `goodser.customer.stock_query` | `customer:read` | 查商品、组合包、捆与可售数量 |
| `goodser.customer.order_request` | `customer:order_request` | 代客提交预留单 |

禁止使用 owner Key，禁止调用库存写操作、统计、成本等接口。

### 8.2 调用示例

```bash
# 查询
curl -s -X POST "$BACKEND_URL/api/v1/integration/actions/goodser.customer.stock_query" \
  -H "X-API-Key: $LANG_BOT_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"params":{"inventory_id":"<inventory-id>","keyword":"拍立得","page":1,"page_size":10}}'

# 提交订单请求
curl -s -X POST "$BACKEND_URL/api/v1/integration/actions/goodser.customer.order_request" \
  -H "X-API-Key: $LANG_BOT_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "params":{
      "inventory_id":"<inventory-id>",
      "customer_name":"张三",
      "contact":"wechat:zhangsan",
      "order_info":"客服代客下单",
      "items":[{"product_id":"<product-id>","quantity":1}]
    }
  }'
```

成功返回 `RSV-...` 预留单号；库存会在创建时锁定，店主确认后转出库。

### 8.3 接入方式

1. **MCP（推荐）**：LangBot 挂载 `http://gobackendcore:29090/api/v1/integration/mcp`，Header 带 customer Key；`tools/list` 会只返回两个 customer 工具。
2. **插件 HTTP 调用（兼容）**：在 LangBot 的 Tool/插件中使用 Python `requests`，Base URL 指向 `http://gobackendcore:29090`，Header 带 `X-API-Key`。
3. **REST 端点**：后端也有 `/api/v1/goodser/customer/stock-query` 与 `/order-requests`，同样支持 API Key；但优先使用 MCP/动作网关，便于统一日志与作用域。

### 8.4 提示词与风控

建议在系统提示词中固定：

```text
你是客服助手，只能查询库存和提交预留单。
- 查询使用 goodser.customer.stock_query。
- 下单使用 goodser.customer.order_request。
- 下单前必须确认商品、数量、联系人。
- 不得承诺一定有货，以返回的 available 为准。
- 不得调用任何 owner 接口，不得输出 API Key。
```

`inventory_id` 由部署方按店铺固定配置，禁止让聊天用户自由输入；否则可能查到其他库存或把订单建到错误仓库。

库存字段说明：

- `available`：可售数量。
- `total`：总库存。
- `reserved`：已锁定数量。

### 8.5 客户下单后的通知链路

```text
LangBot → customer.order_request
        → 创建 pending 预留并锁定库存
        → 写入 goodser.customer.order_requested 外盒事件
        → n8n Webhook 收到事件并通知店主
```

LangBot 不接收该事件；它只负责把 `RSV-...` 返回给客户。

## 9. 部署顺序与验收

### 9.1 建议顺序

1. 确认 `1panel-network` 存在，四个容器都在网络内。
2. 后端设置 `APP_CRYPTO_MASTER_KEY_HEX`，启用 `integration.dispatcher.enabled`。
3. 创建 n8n / OpenClaw / LangBot / 管理四个 API Key。
4. 配置 n8n 的 HTTP Request 凭据和 Webhook 工作流，并激活。
5. 在 Goodser 创建 n8n Webhook 连接器。
6. 配 OpenClaw 的 MCP 端点（`/api/v1/integration/mcp`）。
7. 配 LangBot 的 MCP 端点，只使用 customer Key。
8. 跑验收清单。

### 9.2 验收清单

| 检查 | 命令/操作 | 期望 |
|---|---|---|
| 网络 DNS | `docker exec gobackendcore nslookup n8n` | 返回 IP |
| 后端可达 | n8n 里调用 `/integration/ping` | `code=0` |
| 动作列表 | owner key `GET /api/v1/integration/actions` | 返回 20+ 动作 |
| MCP initialize | `POST /integration/mcp` 发 `initialize` | 返回 `protocolVersion` 与 `serverInfo` |
| MCP tools/list | owner / customer key 各调一次 | 工具列表按作用域过滤 |
| MCP tools/call | 调 `goodser_stock_query` | `isError=false`，返回库存数据 |
| 库存查询 | n8n 调 `goodser.stock.query` | 返回分页数据 |
| 连接器测试 | `POST /integration/connectors/:id/test` | n8n 收到 `integration.test` |
| 事件分发 | `POST /integration/events/dispatch` | `delivered > 0` |
| 验签 | n8n Code 节点返回 `valid=true` | 无签名错误 |
| LangBot 查询 | 调 `goodser.customer.stock_query` | 返回 available/total/reserved |
| LangBot 下单 | 调 `goodser.customer.order_request` | 返回 `RSV-...` |
| 越权拦截 | LangBot Key 调 owner 动作 | `403` |
| 事件落库 | `GET /integration/events?topic=goodser.*` | 能看到事件与状态 |

### 9.3 失败重试机制

- 分发器每 `interval_seconds`（默认 30s）轮询 `pending` 和到期 `failed`。
- 投递失败按 `backoff_seconds × 2^(attempts-1)` 退避，上限 `max_backoff_seconds`。
- 达到 `max_attempts` 后保留 `failed`，不会自动删除。
- 无匹配连接器的事件会直接标记 `delivered`。
- 需要立刻补投时调用 `POST /api/v1/integration/events/dispatch`。

## 10. 安全与运维

1. **不要公网暴露** `29090/29092/5678/18789/5300`；只通过 1Panel 网络、EasyTier、或带鉴权的反代访问。
2. **最小权限**：三个使用方三把 Key，禁止共用；LangBot 绝不能拿 owner Key。
3. **密钥保管**：Key 放各平台凭据/环境变量；连接器 secret 由 `APP_CRYPTO_MASTER_KEY_HEX` 加密，读取接口只返回 `secret_set`。
4. **Webhook 防重放**：校验时间戳 ±5 分钟，按事件 ID 去重。
5. **超时**：连接器 `timeout_seconds` 默认 10s，最大 120s；接收端先回 200 再异步处理。
6. **审计**：动作调用写入 `zzz_integration_action_logs`；事件在 `zzz_integration_outbox`。
7. **保留策略**：已投递事件默认保留 30 天；失败事件保留供排查。
8. **EasyTier**：只给开发机加入网络；离职/换机及时移除设备，避免虚拟内网成为后门。
9. **写操作重试**：n8n / Agent 重试写操作必须复用同一个 `request_id`；幂等缓存默认保留 24 小时。
10. **限流**：动作与 MCP 默认按 API Key 600 次/分钟限流，可通过 `integration.rate_limit` 调整。

## 11. 当前缺口与后续建议

| 缺口 | 影响 | 建议 |
|---|---|---|
| MCP 幂等缓存跨实例极端并发 | 两个实例可能同时执行同一写操作 | 调用方使用稳定 `request_id`；多实例共享 Redis，必要时再加分布式锁 |
| MCP 无状态、无服务端主动推送 | 不能主动通知 Agent | 当前工具调用足够；POST 已兼容只接受 SSE 的客户端 |
| OpenClaw hooks 映射未定义 | 事件无法直接唤醒 Agent | 首版先拉取；需要时再加 payload 转换 |
| LangBot 无事件接收 | 客户下单只能由 n8n 通知店主 | 保持现状，避免 LangBot 承担通知职责 |
| 连接器只有 webhook | 无法直接对接 MQ/消息平台 | 后续按需增加 connector 类型 |

> 推荐优先级：**n8n 模板化子工作流 > 动作级限流 > OpenClaw hooks 映射 > 其他连接器类型**。

## 12. 故障排查

| 现象 | 排查 |
|---|---|
| `nslookup` / `wget` 解析失败 | 容器不在同一网络；容器名不对；网络不是 `1panel-network` |
| `401` | `X-API-Key` 缺失、错误、过期或被禁用 |
| `403` | Key 缺少动作所需作用域；LangBot 误用 owner 动作 |
| `404` | 动作名拼写错误；模块未初始化；路由前缀少了 `/api/v1` |
| 事件一直 `pending` | 分发器未启用；没有匹配 `topics` 的连接器；连接器 `enabled=false` |
| 事件 `failed` | 看 `last_error`；接收端非 2xx；URL/DNS 不通；超时 |
| n8n 验签失败 | Raw Body 未开启；用了测试 URL；secret 不一致；反代改了 body |
| n8n 收不到事件 | 工作流未激活；用了 `/webhook-test/`；`topics` 不匹配 |
| MCP 401 | `X-API-Key` / `Authorization: Bearer gbk_...` 缺失或 Key 无效 |
| MCP `tools/list` 少了工具 | Key 作用域不足；`tools/list` 会按作用域过滤 |
| MCP `tools/call` 返回 `isError` | 看 `content[0].text`；常见是缺少作用域或动作参数不合法 |
| MCP 405 | 用了 GET；工具调用必须 POST JSON-RPC |
| MCP 415 | `Content-Type` 不是 `application/json` |
| 开发环境连不上 | 没连 EasyTier；端口没发布；只监听 127.0.0.1；TLS/证书不匹配 |
| 容器重启后网络丢失 | 用的是 `docker network connect` 临时接入；改 Compose 持久化 |

## 13. n8n 接管 OpenClaw / LangBot 的边界

“接管”有三种含义，结论不同：

| 说法 | 含义 | 结论 |
|---|---|---|
| n8n 作为编排中心 | 自动化流程、事件路由、定时任务统一放 n8n；OpenClaw / LangBot 只做渠道与 Agent | ✅ 推荐 |
| n8n 代理所有动作调用 | LangBot / OpenClaw → n8n → 后端 | ⚠️ 谨慎，只适合复杂流程，不适合所有同步查询 |
| n8n 完全替代两者 | 不再部署 OpenClaw / LangBot | ❌ 不推荐，能力不对等 |

### 13.1 职责分层

| 层 | 负责 | 不应该由 n8n 替代 |
|---|---|---|
| `gobackendcore` | 库存/订单事实、动作网关、外盒事件、审计 | 业务规则不能散落到 n8n |
| n8n | 定时任务、事件扇出、通知、重试、人工审批、跨系统编排 | 不承担 IM 长连接和客服会话 |
| OpenClaw | 店主/运维 Agent 的会话入口、渠道接入、工具/MCP、设备能力 | 不负责库存事实和订单事务 |
| LangBot | 客户服务会话入口、IM 渠道、RAG、客服话术 | 不负责库存写操作和店主统计 |

n8n 不能直接替代的能力：

- QQ / 微信 / 飞书 / Telegram / WhatsApp / iMessage 等 IM 渠道接入与配对。
- 长会话上下文、人工坐席接管、消息流式输出。
- LangBot 的客服插件、知识库/RAG 与多平台适配。
- OpenClaw 的 Gateway、设备节点、语音、渠道权限控制。
- 后端的事务一致性、库存锁定、退货回库等业务规则。

### 13.2 推荐形态：n8n 做自动化中心，不做同步查询的必经之路

```text
客户/店主消息
   │
   ├─ LangBot（客服）──customer Key──► gobackendcore
   │        │
   │        └─ 下单事件 ──► outbox ──► n8n ──► 通知店主
   │
   └─ OpenClaw（店主/运维 Agent）
            ├─ 简单查询/操作 ──owner Key──► gobackendcore
            └─ 复杂流程 ──webhook──► n8n ──► 多个后端动作/外部系统

定时任务、对账、补货、日报：n8n ──owner Key──► gobackendcore
```

分工建议：

1. **LangBot 的同步查询和下单直连后端**：延迟低，后端能按 customer Key 做作用域隔离，n8n 挂了也不影响客服主链路。
2. **OpenClaw 的简单工具直连后端**；涉及多步、需要人工审批或跨系统的流程再交给 n8n。
3. **后端事件统一进 n8n**：`goodser.customer.order_requested` 通知店主、`return.confirmed` 触发对账、`order.cancelled` 触发补偿。
4. **n8n 负责定时任务**：日报、库存巡检、补货建议、失败事件补偿。
5. **n8n 可以反向驱动两者**：调用 OpenClaw 的 `/tools/invoke` 或 `/v1/responses` 唤醒 Agent，调用 LangBot 的 Service API 发消息；但这属于“n8n 驱动渠道”，不是“n8n 取代渠道”。

### 13.3 如果坚持让 n8n 代理所有调用

需要接受的代价：

- 后端只看到 n8n 的 API Key，无法区分调用方是 LangBot 还是 OpenClaw；作用域隔离要在 n8n 内部实现，安全性弱于“每个入口一把 Key”。
- 多一跳网络和一次工作流执行，客服同步查询的延迟、失败率都会上升。
- n8n 成为单点：n8n 故障会同时影响客服、店主工具和事件通知。
- n8n 的重试可能造成重复下单。动作网关已支持 `request_id` 幂等，但 n8n 重试时必须复用同一个 `request_id`。
- n8n 执行历史会保存请求参数，可能包含客户姓名、联系方式等敏感数据，需要限制访问和保留期。

如果确实要代理，至少做到：

- 每个角色独立子工作流、独立后端 Key。
- 子工作流只暴露固定动作白名单，不接受任意 `action` 名。
- 写操作带稳定 `request_id`，失败重试复用同一个值；仍建议“一次成功即停，失败转人工”。
- 关键读操作仍允许 LangBot 直连后端，避免 n8n 成为客服主链路的单点。

### 13.4 结论

**n8n 可以接管“流程”，但不建议接管“入口”。**

最稳的分层是：

```text
OpenClaw / LangBot  = 渠道与会话 Agent
n8n                 = 自动化编排与事件中心
gobackendcore       = 业务事实、动作网关与外盒事件
```

这样既保留了 LangBot 的客服渠道和 OpenClaw 的 Agent 能力，又让 n8n 承担它最擅长的定时、编排、通知和重试。OpenClaw / LangBot 的同步工具调用统一走 [MCP 端点](mcp.md)，n8n 不进入这条同步链路。
