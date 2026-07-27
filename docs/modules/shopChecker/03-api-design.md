# ShopChecker API 设计

## 1. 统一规范

### 响应格式

```json
// 成功
{ "code": 0, "message": "ok", "data": { ... } }
// 失败
{ "code": 30001, "message": "参数错误", "detail": "xxx" }
```

路由前缀：`/api/v1/shopchecker`

---

## 2. 商品管理（Products）

### 2.1 商品列表

```
GET /api/v1/shopchecker/products
```

Query 参数：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，默认 1 |
| page_size | int | 否 | 每页条数，默认 20 |
| platform | string | 否 | 按平台筛选：jd / taobao / pdd |
| enabled | bool | 否 | 按启用状态筛选 |
| keyword | string | 否 | 按名称搜索 |

响应：`{ items: [Product], total: int, page: int, page_size: int }`

### 2.2 创建商品

```
POST /api/v1/shopchecker/products
```

请求体：

```json
{
  "name": "某商品",
  "platform": "jd",
  "url": "https://item.jd.com/xxx.html",
  "image_url": "https://img.example.com/xxx.jpg",
  "target_price": 199.00,
  "price_drop_ratio": 0,
  "check_interval": "1h",
  "custom_webhook_ids": ["wh-xxx", "wh-yyy"],
  "remark": "关注双11价格"
}
```

> `custom_webhook_ids` 为空时，使用 Webhook 公共模块中标签为 `shopchecker` 的 Webhook。Webhook 配置的创建和管理见 `docs/modules/webhook/`。
```

### 2.3 获取商品详情

```
GET /api/v1/shopchecker/products/:id
```

### 2.4 更新商品

```
PUT /api/v1/shopchecker/products/:id
```

允许部分更新，字段同创建接口。

### 2.5 删除商品

```
DELETE /api/v1/shopchecker/products/:id
```

### 2.6 手动执行检查

```
POST /api/v1/shopchecker/products/:id/check
```

立即对该商品执行一次价格检查（忽略 next_check_at）。

响应：

```json
{
  "price": 189.00,
  "original_price": 259.00,
  "title": "某商品标题",
  "is_dropped": true,
  "is_notified": true
}
```

### 2.7 商品价格历史

```
GET /api/v1/shopchecker/products/:id/price-records
```

Query 参数：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| start | string | 否 | 开始时间 (RFC3339) |
| end | string | 否 | 结束时间 (RFC3339) |
| page | int | 否 | 页码 |
| page_size | int | 否 | 每页条数 |

---

## 3. 通知与投递

ShopChecker 不自建 Webhook 配置管理，**所有 Webhook 的创建、编辑、测试、投递日志查询均在 `zzz/webhook` 模块中完成**，详见 `docs/modules/webhook/03-api-design.md`。

ShopChecker 仅提供降价通知的**业务记录查询**：

### 3.1 降价通知记录

```
GET /api/v1/shopchecker/products/:id/notification-logs
```

返回该商品的降价通知发送记录（仅含业务上下文，投递详情需查 webhook 模块的投递日志）。

---

## 4. 仪表盘

### 5.1 概览统计

```
GET /api/v1/shopchecker/dashboard
```

响应：

```json
{
  "total_products": 10,
  "enabled_products": 8,
  "checked_today": 45,
  "price_drops_today": 3,
  "notifications_today": 2,
  "platforms": {
    "jd": 5,
    "taobao": 3,
    "pdd": 2
  }
}
```