# 集成基座（integration）

> 代码位置：`internal/modules/integration`
> 路由前缀：`/api/v1/integration`
> 相关包：`internal/services/{outbox,action,connector}`、`internal/pkg/authz`
> 当前阶段：P0-4 骨架（动作网关 + 外盒事件 + 连接器注册表）

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
| POST | `/api/v1/integration/actions/:name` | 动作声明 | 调用动作，body 为 `{"params":{...}}` |
| GET | `/api/v1/integration/events` | `owner:read` | 查询外盒事件，支持 topic/status/limit/offset |
| POST | `/api/v1/integration/events` | `integration:manage` | 手动写入事件，用于联调与补偿 |
| GET | `/api/v1/integration/connectors` | `integration:manage` | 已注册连接器 |

动作调用会写入 `zzz_integration_action_logs`，记录调用者、认证方式、参数、结果、耗时与错误。

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
| `goodser.customer.stock_query` | `customer:read` | 客户只读库存查询，含组合包与捆 |
| `goodser.customer.order_request` | `customer:order_request` | 客户提交订单请求，支持商品、组合包或捆 |

客户订单请求会写入 `goodser.customer.order_requested` 外盒事件，并尽力推送到
所有已注册连接器；通知失败不影响预留单创建结果。

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

业务模块通过 `integration.Module.Outbox()` 拿到 `*outbox.Service`，在业务事务内写入事件。分发器在 P2 接入，按 topic 路由到连接器。

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

## 6. 数据表

迁移：`migrations/mysql/011_integration_foundation.sql`

| 表 | 用途 |
|---|---|
| `zzz_integration_outbox` | 外盒事件 |
| `zzz_integration_connectors` | 连接器配置（P2 启用） |
| `zzz_integration_action_logs` | 动作调用日志 |

## 7. 已知限制

- 连接器配置表尚未接入 CRUD，P2 补充加密存储与动态加载
- 外盒分发器与重试调度尚未启用，P2 补充
- 客户订单事件目前是提交后写入外盒，尚未与业务事务合并
- 单租户模型：所有 JWT 用户共享现有库存，不做 owner 行级隔离
