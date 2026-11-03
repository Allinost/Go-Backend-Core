# Goodser API 设计

## 1. 兼容性策略

前端（微信小程序）同时支持两种调用方式：

1. **Legacy POST 端点**（兼容旧 Rust 后端协议，前端通过统一 `_nasRequest` 调用）
2. **RESTful 端点**（推荐，新版前端或直连使用）

### 统一响应格式

```json
// 成功
{ "code": 0, "message": "ok", "data": { ... } }
// 失败
{ "code": 40001, "message": "参数错误", "detail": "xxx" }
```

### 统一请求头

```
# Web 控制台
Authorization: Bearer {jwt}
# 外部工具
X-API-Key: {api_key}

X-Request-ID: {uuid}    // 可选的请求追踪 ID
```

> 所有 Goodser 路由都要求认证。JWT 用户视为 owner 控制台；API Key 必须携带
> `owner:read` 或 `owner:write` 作用域，作用域定义见
> [集成基座文档](../../integration/index.md)。

---

## 2. Legacy 端点（POST /api/v1/zzz-goodser/legacy/{action}）

兼容旧 Rust 后端的扁平调用协议，前端通过 `{ baseUrl }/api/{action}` POST 调用。

| Action | 说明 | 请求体 | 响应体 |
|--------|------|--------|--------|
| `loadInventories` | 加载库存目录 | `{}` | `[{Inventory}]` |
| `loadProducts` | 加载商品列表 | `{inventory_id}` | `[{Product}]` |
| `queryProducts` | 查询商品 | `{keyword, inventory_id}` | `[{Product}]` |
| `loadOutboundOrders` | 加载出库单 | `{inventory_id}` | `[{OutboundOrder}]` |
| `loadInboundLogs` | 加载入库日志 | `{inventory_id}` | `[{InboundLog}]` |
| `loadTags` | 加载标签 | `{}` | `[{Tag}]` |
| `loadStatusCodes` | 加载状态码 | `{}` | `[{StatusCode}]` |
| `loadWhitelist` | 加载白名单 | `{}` | `[{WhitelistEntry}]` |
| `createInventory` | 创建仓库 | `{name}` | `{Inventory}` |
| `updateInventory` | 更新仓库 | `{id, name}` | `{Inventory}` |
| `deleteInventory` | 删除仓库 | `{id}` | `{}` |
| `createProduct` | 创建商品 | `{inventory_id, code, ...}` | `{Product}` |
| `updateProduct` | 更新商品 | `{id, ...}` | `{Product}` |
| `deleteProduct` | 删除商品 | `{id}` | `{}` |
| `allocateSeq` | 分配序号 | `{inventory_id, main_zone, sub_zone}` | `{seq_number}` |
| `inboundSingle` | 单品入库 | `{inventory_id, code, ...}` | `{Product}` |
| `inboundBatch` | 批量入库 | `{inventory_id, items: [...]}` | `{count}` |
| `inboundSearchImport` | 搜索导入 | `{inventory_id, items: [...]}` | `{count}` |
| `createInboundLog` | 创建入库日志 | `{inventory_id, type, items}` | `{InboundLog}` |
| `updateInboundLog` | 更新入库日志 | `{id, remark}` | `{InboundLog}` |
| `deleteInboundLog` | 删除入库日志 | `{id}` | `{}` |
| `createOutbound` | 创建出库单 | `{inventory_id, items}` | `{OutboundOrder}` |
| `confirmOutbound` | 确认出库 | `{id}` | `{OutboundOrder}` |
| `cancelOutbound` | 取消出库 | `{id}` | `{OutboundOrder}` |
| `cancelReserve` | 取消预留 | `{id}` | `{OutboundOrder}` |
| `reserveToOutbound` | 预留转出库 | `{id, order_no}` | `{OutboundOrder}` |
| `createTag` | 创建标签 | `{name, color}` | `{Tag}` |
| `updateTag` | 更新标签 | `{id, name, color}` | `{Tag}` |
| `deleteTag` | 删除标签 | `{id}` | `{}` |
| `addWhitelist` | 添加白名单 | `{openid, role}` | `{WhitelistEntry}` |
| `removeWhitelist` | 移除白名单 | `{id}` | `{}` |
| `checkWhitelist` | 检查白名单 | `{openid}` | `{allowed, role}` |
| `addStatusCode` | 添加状态码 | `{code, label}` | `{StatusCode}` |
| `updateStatusCode` | 更新状态码 | `{id, label}` | `{StatusCode}` |
| `removeStatusCode` | 删除状态码 | `{id}` | `{}` |
| `uploadImage` | 上传图片 | multipart/form-data | `{url}` |

---

## 3. RESTful 端点

### 3.1 库存目录 Inventories

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/zzz-goodser/inventories` | 列表 |
| POST | `/api/v1/zzz-goodser/inventories` | 创建 |
| PUT | `/api/v1/zzz-goodser/inventories/:id` | 更新 |
| DELETE | `/api/v1/zzz-goodser/inventories/:id` | 删除 |
| GET | `/api/v1/zzz-goodser/inventories/:id/stats` | 店主统计总览，支持 `days` 1-365（默认 30）或 `start_date`/`end_date` 自定义区间 |

### 3.2 商品 Products

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/zzz-goodser/inventories/:id/products` | 列表（支持筛选、排序、分页） |
| POST | `/api/v1/zzz-goodser/inventories/:id/products` | 创建 |
| POST | `/api/v1/zzz-goodser/inventories/:id/allocate-seq` | 分配序号 |
| GET | `/api/v1/zzz-goodser/products/:pid` | 详情 |
| PUT | `/api/v1/zzz-goodser/products/:pid` | 更新 |
| DELETE | `/api/v1/zzz-goodser/products/:pid` | 删除（可用与锁定库存均为 0） |

### 3.3 入库 Inbound

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/zzz-goodser/inventories/:id/inbound/single` | 单品入库 |
| POST | `/api/v1/zzz-goodser/inventories/:id/inbound/batch` | 批量入库 |
| POST | `/api/v1/zzz-goodser/inventories/:id/inbound/search-import` | 搜索导入入库 |
| GET | `/api/v1/zzz-goodser/inventories/:id/inbound-logs` | 入库日志列表 |

### 3.4 出库与预留 Outbound / Reserves

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/zzz-goodser/inventories/:id/outbound-orders` | 出库/预留单列表 |
| POST | `/api/v1/zzz-goodser/inventories/:id/outbound-orders` | 创建出库或预留单（创建即锁定库存） |
| GET | `/api/v1/zzz-goodser/inventories/:id/outbound-orders/:oid` | 单据详情 |
| POST | `/api/v1/zzz-goodser/inventories/:id/outbound-orders/:oid/confirm` | 确认出库（释放锁定） |
| POST | `/api/v1/zzz-goodser/inventories/:id/outbound-orders/:oid/cancel` | 取消单据（锁定退回可用） |
| POST | `/api/v1/zzz-goodser/inventories/:id/reserves` | 创建预留单 |
| POST | `/api/v1/zzz-goodser/inventories/:id/reserves/:rid/cancel` | 取消预留 |
| POST | `/api/v1/zzz-goodser/inventories/:id/reserves/:rid/convert` | 预留转出库（库存保持锁定） |

单据状态：`pending → confirmed / converted / cancelled`，退货后进入
`partially_returned` 或 `returned`。

### 3.5 退货 Returns

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/zzz-goodser/inventories/:id/outbound-orders/:oid/returns` | 创建退货单（支持多商品、回库开关） |
| GET | `/api/v1/zzz-goodser/inventories/:id/returns` | 退货单列表，可按 `outbound_order_id` 过滤 |
| GET | `/api/v1/zzz-goodser/inventories/:id/returns/:rid` | 退货单详情 |
| POST | `/api/v1/zzz-goodser/inventories/:id/returns/:rid/confirm` | 确认退货（按明细回库） |
| POST | `/api/v1/zzz-goodser/inventories/:id/returns/:rid/cancel` | 取消退货单 |

退货原因：`refund_only`（仅退款，强制不回库）、`return_refund`（退货退款，默认回库）、
`shipping_damage`（运损，默认不回库）、`other`。商品级 `restock` 可覆盖默认值，
但 `refund_only` 始终不回库。

### 3.6 图片 Images

仅提供 Legacy 上传端点 `POST /api/v1/zzz-goodser/legacy/uploadImage`（multipart 字段 `image`），
返回 `{url}`。返回值使用对象存储公开读地址（不带签名查询串），避免预签名 URL 7 天后失效；
商品读取时会把历史预签名 URL 归一为公开地址。REST 版上传接口尚未实现。
前端在商品表单上传/预览/移除，列表展示缩略图，详情展示大图。

### 3.7 设置 Settings

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/zzz-goodser/settings/tags` | 标签列表 |
| GET | `/api/v1/zzz-goodser/settings/status-codes` | 状态码列表 |
| POST | `/api/v1/zzz-goodser/legacy/createTag` | 创建标签 |
| POST | `/api/v1/zzz-goodser/legacy/addStatusCode` | 新增状态码 |

### 3.8 受控维度、组合包与捆 Dimensions / Combos / Bundles

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/zzz-goodser/settings/dictionaries` | 字典列表，`kind` 支持 series/character/set_type/product_type |
| POST | `/api/v1/zzz-goodser/settings/dictionaries` | 创建字典项 |
| PUT | `/api/v1/zzz-goodser/settings/dictionaries/:did` | 更新字典项 |
| DELETE | `/api/v1/zzz-goodser/settings/dictionaries/:did` | 删除字典项（被引用时拒绝） |
| GET | `/api/v1/zzz-goodser/inventories/:id/series` | 系列聚合视图（商品数、总量、可用量） |
| GET | `/api/v1/zzz-goodser/inventories/:id/combos` | 组合包列表，含可售数量 |
| POST | `/api/v1/zzz-goodser/inventories/:id/combos` | 创建组合包（BOM） |
| GET | `/api/v1/zzz-goodser/combos/:cid` | 组合包详情 |
| PUT | `/api/v1/zzz-goodser/combos/:cid` | 更新组合包 |
| DELETE | `/api/v1/zzz-goodser/combos/:cid` | 删除组合包 |
| GET | `/api/v1/zzz-goodser/inventories/:id/bundles` | 捆模板列表 |
| POST | `/api/v1/zzz-goodser/inventories/:id/bundles` | 创建捆模板 |
| GET | `/api/v1/zzz-goodser/bundles/:bid` | 捆模板详情，含主商品与副区商品 |
| PUT | `/api/v1/zzz-goodser/bundles/:bid` | 更新捆模板 |
| DELETE | `/api/v1/zzz-goodser/bundles/:bid` | 删除捆模板 |
| GET | `/api/v1/zzz-goodser/bundles/:bid/main-products` | 捆允许的主商品，支持 `keyword` |
| GET | `/api/v1/zzz-goodser/bundles/:bid/sub-products` | 捆允许的副区商品，支持 `keyword`、`only_available` |
| POST | `/api/v1/zzz-goodser/bundles/:bid/preview` | 校验捆选择并计算价格，不改变库存 |

商品创建、更新、入库均支持 `series_id`、`set_type_id`、`product_type_id` 与
`character_ids`。`tags` 仍作为自由标签保留。字典为全局共享，角色可通过
`parent_id` 归属系列。

更新商品时维度字段省略或传 `null` 表示“不修改”，传空字符串 `""` 表示“清空”。
`character_ids` 传 `[]` 即清空角色关联。

组合包不持有独立库存，`availability = min(组件可用量 / 组件数量)`。创建出库/
预留单时传 `combo_id` + `combo_quantity`，服务端会展开为组件商品并按组件扣减库存。

捆由主商品条件与店主选定的副区商品组成。主商品可显式指定，也可通过
`main_filter` 按系列、角色、套型、类型、状态、关键词动态筛选，两者同时存在时按
AND 校验。更新捆时 `main_filter` 省略或传 `null` 表示不修改，传 `{}` 表示清空筛选；
清空后必须仍有显式主商品，否则返回“缺少主商品条件”。下单时传：

```json
{
  "inventory_id": "inv-1",
  "type": "reserve",
  "bundle": {
    "bundle_id": "bundle-1",
    "main_product_id": "product-main",
    "main_quantity": 1,
    "quantity": 1,
    "sub_items": [
      {"product_id": "product-sub-1", "quantity": 2},
      {"product_id": "product-sub-2", "quantity": 1}
    ]
  }
}
```

`main_quantity`、`sub_items[].quantity` 与 `quantity` 均按单捆计，实际出库数量为
单捆数量乘以 `quantity`。使用捆下单时 `items` 必须留空，服务端会：

1. 校验主商品在允许范围内，且满足主商品最低数量、最低金额。
2. 校验副区商品都来自捆配置，满足副区最低/最多数量和最低金额。
3. 按 `price_mode` 计算单捆价与总价。
4. 展开为主商品与副区商品明细，按 D1 规则锁定库存。
5. 在出库单写入 `bundle_snapshot`，保存选择内容、价格与规则快照。

`preview` 接口使用同样的校验逻辑，但只返回价格与明细，不创建单据、不改变库存，
适合 Web 控制台与 LangBot 在下单前确认。

### 3.9 统计 Stats

| 方法 | 路径 | 作用域 | 说明 |
|------|------|--------|------|
| GET | `/api/v1/zzz-goodser/inventories/:id/stats?days=30` | `owner:read` | 库存数量与价值、订单/退货状态数、按天趋势、系列分布、热销商品 |
| GET | `/api/v1/zzz-goodser/inventories/:id/stats?start_date=2026-07-01&end_date=2026-07-31` | `owner:read` | 自定义区间统计（含首尾，最长 366 天）；响应回填 `start_date`/`end_date` |
| Action | `goodser.stats.summary` | `owner:read` | 集成动作版统计，参数 `inventory_id`、`days` |

统计口径：

- `stock.available` = `SUM(quantity)`，`stock.reserved` = `SUM(reserved_quantity)`，
  `stock.total` = 两者之和；`stock_value` 优先使用 `expected_price`，为 0 时回退 `market_price`。
- `orders` 与 `returns` 按单据创建时间落在区间内计数；「已确认出库」包含
  后续进入 `partially_returned` / `returned` 的单据。
- `trend` 按天补齐缺失日期，出库量取确认出库流水，退货量取确认且回库流水。
- `series` 按商品系列聚合，未设置系列归入“未设置”。
- `top_products` 按确认出库流水倒序，最多 10 条。

### 3.10 客户服务 Customer

| 方法 | 路径 | 作用域 | 说明 |
|------|------|--------|------|
| POST | `/api/v1/zzz-goodser/customer/stock-query` | `customer:read` | 按关键词、系列、角色、套型、类型、状态查询库存、组合包与捆 |
| POST | `/api/v1/zzz-goodser/customer/order-requests` | `customer:order_request` | 提交订单请求，支持商品、组合包或捆，创建 pending 预留并锁定库存 |

客户接口严格只读，不接受任何直接改库存的字段。订单请求会写入
`goodser.customer.order_requested` 外盒事件，并尽力通知已注册连接器。
客户使用捆下单时，`order-requests` 的 `bundle` 字段与店主下单结构一致，
`items` 与 `combo_id` 必须留空。响应会返回 `bundle_id` 与 `total_price`。

---

## 4. 路由注册实现

在 `module.go` 中，所有 Legacy 端点统一挂载到 `/api/v1/zzz-goodser/legacy` 组下，RESTful 端点挂载到 `/api/v1/zzz-goodser/` 下：

```go
func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
    // Legacy 兼容端点
    legacy := r.Group("/legacy")
    legacy.POST("/loadInventories", m.h.LoadInventories)
    legacy.POST("/loadProducts", m.h.LoadProducts)
    // ...

    // RESTful 端点
    r.GET("/inventories", m.h.ListInventories)
    r.POST("/inventories", m.h.CreateInventory)
    // ...
}
```
