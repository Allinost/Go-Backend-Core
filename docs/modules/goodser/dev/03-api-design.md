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
Authorization: Bearer {api_key}
X-Request-ID: {uuid}    // 可选的请求追踪 ID
```

---

## 2. Legacy 端点（POST /api/v1/goodser/legacy/{action}）

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
| GET | `/api/v1/goodser/inventories` | 列表 |
| POST | `/api/v1/goodser/inventories` | 创建 |
| PUT | `/api/v1/goodser/inventories/:id` | 更新 |
| DELETE | `/api/v1/goodser/inventories/:id` | 删除 |
| GET | `/api/v1/goodser/inventories/:id/stats` | 统计信息 |

### 3.2 商品 Products

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/goodser/inventories/:id/products` | 列表 |
| POST | `/api/v1/goodser/inventories/:id/products` | 创建 |
| GET | `/api/v1/goodser/inventories/:id/products/:pid` | 详情 |
| PUT | `/api/v1/goodser/inventories/:id/products/:pid` | 更新 |
| DELETE | `/api/v1/goodser/inventories/:id/products/:pid` | 删除 |
| POST | `/api/v1/goodser/inventories/:id/products/search` | 搜索 |
| POST | `/api/v1/goodser/inventories/:id/products/allocate-seq` | 分配序号 |

### 3.3 入库 Inbound

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/goodser/inventories/:id/inbound/single` | 单品入库 |
| POST | `/api/v1/goodser/inventories/:id/inbound/batch` | 批量入库 |
| POST | `/api/v1/goodser/inventories/:id/inbound/search-import` | 搜索导入入库 |
| GET | `/api/v1/goodser/inventories/:id/inbound/logs` | 入库日志列表 |
| GET | `/api/v1/goodser/inventories/:id/inbound/logs/:logId` | 入库日志详情 |
| PUT | `/api/v1/goodser/inventories/:id/inbound/logs/:logId` | 更新入库日志 |
| DELETE | `/api/v1/goodser/inventories/:id/inbound/logs/:logId` | 删除入库日志 |

### 3.4 出库 Outbound

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/goodser/inventories/:id/outbound/orders` | 出库单列表 |
| POST | `/api/v1/goodser/inventories/:id/outbound/orders` | 创建出库单 |
| GET | `/api/v1/goodser/inventories/:id/outbound/orders/:oid` | 出库单详情 |
| POST | `/api/v1/goodser/inventories/:id/outbound/orders/:oid/confirm` | 确认出库 |
| POST | `/api/v1/goodser/inventories/:id/outbound/orders/:oid/cancel` | 取消出库 |
| POST | `/api/v1/goodser/inventories/:id/outbound/reserves` | 创建预留单 |
| POST | `/api/v1/goodser/inventories/:id/outbound/reserves/:rid/cancel` | 取消预留 |
| POST | `/api/v1/goodser/inventories/:id/outbound/reserves/:rid/to-outbound` | 预留转出库 |

### 3.5 图片 Images

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/goodser/images/presign` | 获取预签名上传 URL |
| POST | `/api/v1/goodser/images/confirm` | 确认上传完成 |
| GET | `/api/v1/goodser/images/:key/url` | 获取图片访问 URL |

### 3.6 设置 Settings

| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST | `/api/v1/goodser/settings/whitelist` | 列表/添加白名单 |
| DELETE | `/api/v1/goodser/settings/whitelist/:id` | 移除白名单 |
| POST | `/api/v1/goodser/settings/whitelist/check` | 检查白名单 |
| GET/POST | `/api/v1/goodser/settings/status-codes` | 列表/添加状态码 |
| PUT/DELETE | `/api/v1/goodser/settings/status-codes/:id` | 更新/删除状态码 |
| GET/POST | `/api/v1/goodser/settings/tags` | 列表/创建标签 |
| PUT/DELETE | `/api/v1/goodser/settings/tags/:id` | 更新/删除标签 |

---

## 4. 路由注册实现

在 `module.go` 中，所有 Legacy 端点统一挂载到 `/api/v1/goodser/legacy` 组下，RESTful 端点挂载到 `/api/v1/goodser/` 下：

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
