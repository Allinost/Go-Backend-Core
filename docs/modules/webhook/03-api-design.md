# Webhook API 设计

## 1. 统一规范

路由前缀：`/api/v1/webhook`

统一响应格式同项目规范。

---

## 2. Webhook 配置管理

### 2.1 配置列表

```
GET /api/v1/webhook/configs
```

Query 参数：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，默认 1 |
| page_size | int | 否 | 每页条数，默认 20 |
| type | string | 否 | 按类型筛选 |
| status | string | 否 | active / paused / disabled |
| source | string | 否 | 按来源筛选 |
| tag | string | 否 | 按标签筛选 |
| keyword | string | 否 | 按名称/描述搜索 |

### 2.2 创建配置

```
POST /api/v1/webhook/configs
```

```json
{
  "name": "主力群降价通知",
  "type": "feishu",
  "url": "https://open.feishu.cn/open-apis/bot/v2/hook/xxx",
  "source": "shopchecker",
  "app": "price-monitor",
  "description": "ShopChecker 降价通知主力群",
  "tags": ["shopchecker", "price"],
  "signing_key": "",
  "whitelist_keywords": ["降价", "缺货"],
  "whitelist_mode": "any",
  "max_per_minute": 20,
  "max_per_hour": 200,
  "dedup_window": 300,
  "max_retries": 3,
  "retry_interval": 30,
  "timeout_ms": 10000
}
```

### 2.3 获取详情

```
GET /api/v1/webhook/configs/:id
```

响应自动脱敏敏感字段（signing_key / access_token / auth_value 显示 `***` 后缀）。

### 2.4 更新配置

```
PUT /api/v1/webhook/configs/:id
```

支持部分更新。

### 2.5 删除配置

```
DELETE /api/v1/webhook/configs/:id
```

### 2.6 切换状态

```
POST /api/v1/webhook/configs/:id/status
```

```json
{ "status": "paused" }
// 或 "active" / "disabled"
```

### 2.7 测试发送

```
POST /api/v1/webhook/configs/:id/test
```

```json
{
  "test_msg": "这是一条测试消息，来自 Webhook 管理模块"
}
```

向指定 Webhook 发送一条测试消息，并返回投递结果。

---

## 3. 投递服务

### 3.1 发送消息（内部 API / 业务模块调用）

```
POST /api/v1/webhook/send
```

```json
{
  "webhook_ids": ["wh-xxx", "wh-yyy"],
  "source": "shopchecker",
  "title": "💰 降价提醒",
  "content": "**商品**: [某商品](https://...)\n**原价**: ¥100\n**现价**: ¥80\n**降幅**: 20%",
  "msg_type": "markdown"
}
```

或按标签发送：

```json
{
  "tags": ["shopchecker", "price"],
  "source": "shopchecker",
  "title": "💰 降价提醒",
  "content": "**商品**: ...",
  "msg_type": "markdown"
}
```

### 3.2 投递日志列表

```
GET /api/v1/webhook/delivery-logs
```

Query 参数：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| webhook_id | string | 否 | 按 Webhook 筛选 |
| source | string | 否 | 按来源筛选 |
| status | string | 否 | success / failed / rate_limited / deduped |
| page | int | 否 | 页码 |
| page_size | int | 否 | 每页条数 |

---

## 4. 统计

### 4.1 概览统计

```
GET /api/v1/webhook/stats
```

响应：

```json
{
  "total_configs": 15,
  "active_configs": 12,
  "total_sent_today": 234,
  "success_today": 228,
  "failed_today": 6,
  "rate_limited_today": 2,
  "by_type": {
    "feishu": 8,
    "dingtalk": 3,
    "github": 2,
    "generic": 2
  },
  "by_source": {
    "shopchecker": 180,
    "system": 54
  }
}
```

### 4.2 单个 Webhook 统计

```
GET /api/v1/webhook/configs/:id/stats
```