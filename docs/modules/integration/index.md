# 集成基座（integration）

> 代码位置：`internal/modules/integration`
> 路由前缀：`/api/v1/integration`
> 相关包：`internal/services/{outbox,action,connector}`、`internal/pkg/authz`
> 当前阶段：P2 外盒分发器 + 连接器配置管理 + MCP 工具端点
> 接入示例：[examples.md](examples.md)（n8n / OpenClaw / LangBot / Webhook 签名）
> 网络与部署拓扑：[connectivity.md](connectivity.md)（1panel-network / EasyTier / 验收清单）
> MCP 端点：[mcp.md](mcp.md)（OpenClaw / LangBot 工具调用）

## 1. 定位

n8n、OpenClaw、LangBot 等外部工具不直接访问业务表，而是通过统一入口调用动作或接收事件：

| 使用方 | 角色 | 建议作用域 |
|---|---|---|
| n8n / OpenClaw | 店主工具，库存、订单、统计、组合销售 | `owner:read`、`owner:write` |
| LangBot | 客服，只读查询 + 提交订单请求 | `customer:read`、`customer:order_request` |
| 集成管理台 | 连接器与事件管理 | `integration:manage` |

认证支持两种方式：

- `Authorization: Bearer <JWT>`：Web 控制台，视为 owner，不受 API Key 作用域限制
- `X-API-Key: <key>`：外部工具，按 `api_key_scopes` 精确放行
- `Authorization: Bearer gbk_...`：MCP 客户端兼容形式，等价于 `X-API-Key`

## 2. 作用域

定义在 `internal/pkg/authz/scope.go`，由 HTTP 中间件与动作网关共用：

| 作用域 | 含义 |
|---|---|
| `owner:read` | 读取库存、订单、退货、统计 |
| `owner:write` | 库存、订单、退货等写操作 |
| `customer:read` | 客户服务只读查询 |
| `customer:order_request` | 客户提交订单请求，不直接改库存 |
| `integration:manage` | 管理连接器、事件与集成配置 |

匹配规则：支持 `*`、`*:*`、`owner:*`，多个作用域可用逗号、空格或分号分隔。

## 3. 接口

| 方法 | 路径 | 作用域 | 说明 |
|---|---|---|---|
| GET | `/api/v1/integration/ping` | `owner:read` | 连通性测试 |
| GET | `/api/v1/integration/actions` | `owner:read` | 已注册动作列表 |
| POST | `/api/v1/integration/actions/:name` | 动作声明 | 调用动作，body 为 `{"params":{...},"request_id":"可选幂等键"}` |
| POST | `/api/v1/integration/mcp` | 动作声明 | MCP Streamable HTTP（JSON-RPC），供 OpenClaw / LangBot 使用 |
| GET / DELETE | `/api/v1/integration/mcp` | 动作声明 | 协议兼容；GET 返回 405，DELETE 返回 204 |
| GET | `/api/v1/integration/events` | `owner:read` | 查询外盒事件，支持 topic/status/limit/offset |
| POST | `/api/v1/integration/events` | `integration:manage` | 手动写入事件，用于联调与补偿 |
| POST | `/api/v1/integration/events/dispatch` | `integration:manage` | 立即执行一轮事件投递 |
| GET | `/api/v1/integration/connectors` | `integration:manage` | 连接器配置列表（secret 仅返回 `secret_set`） |
| POST | `/api/v1/integration/connectors` | `integration:manage` | 创建 webhook 连接器 |
| PUT | `/api/v1/integration/connectors/:id` | `integration:manage` | 更新连接器，`secret` 省略表示不变更 |
| DELETE | `/api/v1/integration/connectors/:id` | `integration:manage` | 删除连接器 |
| POST | `/api/v1/integration/connectors/:id/test` | `integration:manage` | 向连接器发送测试事件 |

动作调用会写入 `zzz_integration_action_logs`，记录调用者、认证方式、参数、`request_id`、是否重放、结果、耗时与错误。

### 3.1 Goodser 动作

Goodser 模块初始化时向动作网关注册以下动作：

| 动作 | 作用域 | 说明 |
|---|---|---|
| `goodser.inventory.list` | `owner:read` | 库存目录列表 |
| `goodser.stock.query` | `owner:read` | 库存查询，支持维度/状态/关键词 |
| `goodser.order.list` | `owner:read` | 出库/预留单列表 |
| `goodser.order.create` | `owner:write` | 创建出库/预留单，支持组合包与捆 |
| `goodser.order.confirm` | `owner:write` | 确认出库 |
| `goodser.order.cancel` | `owner:write` | 取消单据并释放锁定 |
| `goodser.return.create` | `owner:write` | 创建退货单 |
| `goodser.return.confirm` | `owner:write` | 确认退货并回库 |
| `goodser.combo.list` | `owner:read` | 组合包及可售数量 |
| `goodser.combo.create` | `owner:write` | 创建组合包 |
| `goodser.bundle.list` | `owner:read` | 捆模板及主/副商品配置 |
| `goodser.bundle.get` | `owner:read` | 捆模板详情 |
| `goodser.bundle.create` | `owner:write` | 创建捆模板 |
| `goodser.bundle.update` | `owner:write` | 更新捆模板 |
| `goodser.bundle.delete` | `owner:write` | 删除捆模板 |
| `goodser.bundle.preview` | `owner:read` | 校验捆选择并计算价格 |
| `goodser.dictionary.list` | `owner:read` | 受控维度字典 |
| `goodser.stats.summary` | `owner:read` | 店主统计总览：库存、订单、退货、系列分布与趋势 |
| `goodser.customer.stock_query` | `customer:read` | 客户只读库存查询，含组合包与捆 |
| `goodser.customer.order_request` | `customer:order_request` | 客户提交订单请求，支持商品、组合包或捆 |

客户订单请求会写入 `goodser.customer.order_requested` 外盒事件，并尽力推送到
所有已注册连接器；通知失败不影响预留单创建结果。

### 3.2 MCP 端点

`/api/v1/integration/mcp` 把动作注册表动态暴露为 MCP 工具：

- 工具名：动作名的 `.` 替换为 `_`，例如 `goodser.stock.query` → `goodser_stock_query`
- `tools/list` 按 API Key 作用域过滤，客户 Key 看不到 owner 工具
- `tools/call` 参数放在 `arguments.params`，与动作网关的 `params` 一致
- Goodser 主要动作带字段级 `params_schema`，模型可直接看到 `inventory_id`、`items` 等字段
- 写操作可在 `arguments.request_id` 传幂等键，重复提交返回首次结果
- 认证支持 `X-API-Key` 与 `Authorization: Bearer gbk_...`
- 动作 / MCP 调用按 API Key 限流（`integration.rate_limit`）
- 无状态 Streamable HTTP，不提供 SSE 服务端推送
- 调用同样写入 `zzz_integration_action_logs`

完整协议示例、OpenClaw / LangBot 配置和错误码见 [mcp.md](mcp.md)。

### 3.3 写操作幂等

- REST：body 顶层带 `request_id`，例如 `{"request_id":"order-001","params":{...}}`
- MCP：`arguments.request_id`
- 缓存键：调用方 + 动作 + `request_id`
- 只缓存成功结果，默认保留 24 小时；命中时 REST 结果 `replayed=true`，MCP 结果 `_meta.replayed=true`
- 未传 `request_id` 时不启用幂等

### 3.4 限流

`POST /integration/actions/:name` 与 `/integration/mcp` 按 API Key 限流，默认 600 次/分钟；Redis 可用时使用分布式固定窗口，否则降级为进程内存。超过配额返回 `429`。

## 4. 外盒事件

`internal/services/outbox` 提供：

- `Event`：`id/topic/source/aggregate_type/aggregate_id/payload/status/attempts/last_error`
- `Store`：`Append/Get/List/MarkDelivered/MarkFailed`
- `Service`：`Emit/List/MarkDelivered/MarkFailed`
- `MemoryStore`（单测）与 `MySQLStore`（生产）

状态流转：

```
pending --投递成功--> delivered
   |
   +--投递失败--> failed --重试--> delivered / failed
```

业务模块通过 `integration.Module.Outbox()` 拿到 `*outbox.Service`，在业务事务内写入事件。分发器按 topic 路由到连接器：

- 后台按 `integration.dispatcher.interval_seconds` 轮询 `pending` 与到期的 `failed` 事件。
- 连接器 `topics` 为空表示订阅全部；支持精确值、`*`、`goodser.*`、`goodser*`。
- 无匹配连接器时事件直接标记 `delivered`，避免无意义堆积。
- 投递失败累加 `attempts`，按 `backoff_seconds × 2^(attempts-1)` 退避，上限 `max_backoff_seconds`。
- `attempts >= max_attempts` 后不再自动重试，保留 `failed` 状态供人工排查。
- 可通过 `POST /integration/events/dispatch` 手动触发一轮，用于联调或补偿。

Goodser 当前写入的业务事件：

| 主题 | 触发时机 |
|---|---|
| `goodser.order.created` | 创建出库/预留单后 |
| `goodser.order.confirmed` | 确认出库后 |
| `goodser.order.cancelled` | 取消出库单后 |
| `goodser.reserve.converted` | 预留转出库后 |
| `goodser.reserve.cancelled` | 取消预留后 |
| `goodser.return.created` | 创建退货单后 |
| `goodser.return.confirmed` | 确认退货后 |
| `goodser.customer.order_requested` | 客户提交订单请求后 |

> 事件写入发生在业务事务提交之后，属于尽力而为；写入失败只记录日志。

## 5. 连接器

`internal/services/connector` 提供：

- `Connector` 接口：`ID/Type/Send`
- `Registry`：注册、查询、列表、移除
- `Webhook`：签名 JSON POST

Webhook 请求头：

| 头 | 说明 |
|---|---|
| `X-Integration-Event` | 事件主题 |
| `X-Integration-Event-ID` | 事件 ID |
| `X-Integration-Timestamp` | Unix 秒级时间戳 |
| `X-Integration-Signature` | `sha256=HMAC_SHA256(secret, timestamp + "." + body)` |

接收方应校验签名并限制时间戳偏差，避免重放。

### 5.1 连接器配置

连接器配置保存在 `zzz_integration_connectors.config`，创建/更新示例：

```json
{
  "name": "n8n 生产通知",
  "type": "webhook",
  "url": "https://n8n.example.com/webhook/goodser",
  "secret": "仅创建或轮换时传入",
  "topics": ["goodser.order.*", "goodser.return.*"],
  "timeout_seconds": 10,
  "enabled": true
}
```

- `secret` 使用 `crypto.master_key_hex` 进行 AES-256-GCM 加密，密文以 `v1:` 开头。
- 读取接口只返回 `secret_set`，不返回 secret 明文或密文。
- 更新时 `secret` 省略或为 `null` 表示保持不变，传空字符串表示清除。
- 未配置 `crypto.master_key_hex` 时，带 secret 的创建/更新会被拒绝。
- 连接器配置变更后立即刷新注册表，无需重启服务。
- `POST /integration/connectors/:id/test` 发送 `integration.test` 测试事件，便于联调。
- 已投递事件默认保留 30 天，由后台清理器定期删除；失败事件不会自动清理。

## 6. 数据表

迁移：`migrations/mysql/011_integration_foundation.sql`、`015_integration_action_idempotency.sql`

| 表 | 用途 |
|---|---|
| `zzz_integration_outbox` | 外盒事件 |
| `zzz_integration_connectors` | 连接器配置（P2 已接入 CRUD、加密与动态加载） |
| `zzz_integration_action_logs` | 动作调用日志（含 `request_id`、`replayed`） |

## 7. 配置

```yaml
crypto:
  master_key_hex: ""          # 通过 APP_CRYPTO_MASTER_KEY_HEX 注入，64 位十六进制

integration:
  retention_days: 30          # 已投递事件保留天数，0 表示不清理
  idempotency:
    enabled: true             # 写操作幂等缓存（Redis 优先，Redis 不可用时降级为进程内存）
    ttl_seconds: 86400        # 幂等结果保留时间
  rate_limit:
    enabled: true             # 动作 / MCP 调用按 API Key 限流
    requests_per_minute: 600  # 每个 Key 每分钟请求上限
  dispatcher:
    enabled: true
    interval_seconds: 30
    batch_size: 50
    max_attempts: 5
    backoff_seconds: 30
    max_backoff_seconds: 1800
    send_timeout_seconds: 10
```

## 8. 已知限制

- 客户订单事件目前是提交后写入外盒，尚未与业务事务合并
- 单租户模型：所有 JWT 用户共享现有库存，不做 owner 行级隔离
- Goodser 主要动作已有字段级 `params_schema`；未配置 schema 的动作仍回退为通用 `params` 对象
- MCP 无状态实现，不支持服务端主动通知；客户端只接受 SSE 时可用 SSE 返回单次响应
- 写操作幂等缓存单实例内用 singleflight 合并并发；多实例共享 Redis 存储，但极端并发仍建议使用稳定 `request_id`

## 9. 网络与部署

- **生产**：`gobackendcore` 与 n8n / OpenClaw / LangBot 同处 `1panel-network`，容器之间使用「容器名 + 容器端口」，例如 `http://gobackendcore:29090`。
- **开发**：三者通过 EasyTier 入口 `et.hailong.site` + 宿主机发布端口访问，例如 `https://et.hailong.site:29090`。
- **OpenClaw / LangBot**：通过 `/api/v1/integration/mcp` 调用动作，详见 [mcp.md](mcp.md)。
- 网络互通不替代鉴权：外部工具仍使用 `X-API-Key`，Webhook 仍校验 HMAC。
- 完整地址表、1Panel 接入步骤、n8n / OpenClaw / LangBot 配置与验收清单见 [connectivity.md](connectivity.md)。
