# 前端对接 - 商品价格监控（ShopChecker）API

## 基础信息

- **Base URL**: `/api/v1/shopchecker`
- **认证方式**: JWT Bearer Token（Header `Authorization: Bearer {token}`）
- **通用响应格式**:

```json
{
  "code": 0,
  "message": "成功",
  "data": {},
  "trace_id": "xxx"
}
```

错误时 `code != 0`，`message` 为错误描述，`detail` 仅开发环境可见。

---

## 支持平台

| 平台标识 | 名称 | URL 格式说明 |
|---------|------|-------------|
| `jd` | 京东（桌面版） | `https://item.jd.com/{skuId}.html` |
| `jdm` | 京东（移动版） | `https://item.m.jd.com/product/{skuId}.html` |
| `taobao` | 淘宝/天猫 | 支持完整 URL、分享短链接（`e.tb.cn/xxx`），自动识别商品 ID |
| `pdd` | 拼多多 | `https://mobile.yangkeduo.com/goods.html?goods_id=xxx` |
| `qixian` | 七鲜 | 通过京东移动版采集 |

> **淘宝特别说明**: 支持从分享口令中自动提取链接，如 `【淘宝】7天无理由退换 https://e.tb.cn/h.xxx?tk=yyy HU071 「商品名」`，前端只需粘贴完整分享文本或纯链接均可。淘宝价格采集依赖登录态，需在「账号管理」模块完成淘宝账号登录。

---

## 数据模型

### Product（商品监控配置）

```typescript
interface Product {
  _id: string;                    // 商品 ID（UUID）
  name: string;                   // 商品名称
  platform: "jd" | "jdm" | "taobao" | "pdd" | "qixian";
  url: string;                    // 商品 URL（已规范化为商品页地址）
  image_url?: string;             // 商品图片 URL
  target_price: number;           // 目标价格（0 表示未设置）
  price_drop_ratio: number;       // 降价百分比阈值（0 表示未设置）
  last_price: number;             // 上次采集到的价格
  last_stock_status: "in_stock" | "out_of_stock" | "unknown";
  last_checked_at?: string;       // 上次检查时间（ISO 8601），null 表示尚未检查
  next_check_at?: string;         // 下次计划检查时间（ISO 8601）
  check_interval: string;         // 检查间隔（如 "1h"）
  cron_expr?: string;             // cron 表达式
  custom_webhook_ids?: string[];  // 自定义通知 webhook ID
  enabled: boolean;               // 是否启用
  remark?: string;                // 备注
  created_at: string;             // 创建时间
  updated_at: string;             // 更新时间
}
```

### PriceRecord（价格记录）

```typescript
interface PriceRecord {
  id: number;
  product_id: string;
  price: number;                  // 当前价格（到手价）
  original_price: number;         // 原价/划线价
  title: string;                  // 采集到的商品标题
  stock_status: "in_stock" | "out_of_stock" | "unknown";
  is_notified: boolean;           // 是否已触发通知
  error_msg: string;              // 采集错误信息（为空表示成功）
  created_at: string;
}
```

### NotificationLog（通知记录）

```typescript
interface NotificationLog {
  id: number;
  product_id: string;
  price_record_id: number;
  old_price: number;
  new_price: number;
  old_stock_status: string;
  new_stock_status: string;
  notify_type: "price_drop" | "target_reached" | "stock_restore" | "login_expired";
  result_summary: "all_success" | "partial" | "all_failed";
  webhook_ids: string[];
  created_at: string;
}
```

### DashboardResp（仪表盘）

```typescript
interface DashboardResp {
  total_products: number;
  enabled_products: number;
  today_checks: number;
  today_notified: number;
  recent_drops: Array<{
    product_id: string;
    name: string;
    platform: string;
    old_price: number;
    new_price: number;
    drop_percent: number;
    dropped_at: string;
  }>;
}
```

### 分页响应

```typescript
interface PaginatedResp<T> {
  items: T[];
  has_more: boolean;
  total: number;
}
```

---

## 接口详情

### 1. 获取商品列表

```
GET /api/v1/shopchecker/products
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，默认 1 |
| page_size | int | 否 | 每页数量，默认 20，最大 100 |
| platform | string | 否 | 筛选平台：jd / jdm / taobao / pdd / qixian |
| keyword | string | 否 | 搜索关键词，模糊匹配商品名称 |
| enabled | bool | 否 | 筛选启用状态 |

**响应示例**:

```json
{
  "code": 0,
  "data": {
    "items": [
      {
        "_id": "a1b2c3d4-...",
        "name": "Apple/苹果 iPhone 17e",
        "platform": "taobao",
        "url": "https://item.taobao.com/item.htm?id=1027760304544",
        "last_price": 3999,
        "last_stock_status": "in_stock",
        "target_price": 4000,
        "enabled": true,
        "check_interval": "1h",
        "next_check_at": "2026-08-27T12:00:00Z",
        "created_at": "2026-08-27T10:00:00Z",
        "updated_at": "2026-08-27T11:00:00Z"
      }
    ],
    "has_more": false,
    "total": 1
  }
}
```

### 2. 创建商品监控

```
POST /api/v1/shopchecker/products
```

**请求体**:

```json
{
  "name": "Apple/苹果 iPhone 17e",
  "platform": "taobao",
  "url": "https://e.tb.cn/h.8Pz6TAsvVLBXFPH?tk=BO2CT3crXeE",
  "target_price": 4000,
  "price_drop_ratio": 5,
  "check_interval": "1h",
  "remark": "国补商品，补贴后约3999"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | **是** | 商品名称 |
| platform | string | **是** | 平台标识（jd/jdm/taobao/pdd/qixian） |
| url | string | **是** | 商品 URL（支持完整链接、短链接、分享口令文本） |
| image_url | string | 否 | 商品图片 URL |
| target_price | number | 否 | 目标价格，低于此值触发通知 |
| price_drop_ratio | number | 否 | 降价百分比阈值 |
| check_interval | string | 否 | 检查间隔，默认 "1h"，支持 "30m"/"1h"/"24h" 等 |
| cron_expr | string | 否 | cron 表达式，优先级高于 check_interval |
| custom_webhook_ids | string[] | 否 | 自定义通知 webhook ID 列表 |
| remark | string | 否 | 备注 |

**响应**: 返回创建后的 Product 对象。

**淘宝 URL 支持格式**:

```
# 分享短链接（推荐）
https://e.tb.cn/h.8Pz6TAsvVLBXFPH?tk=BO2CT3crXeE

# 完整商品页链接
https://item.taobao.com/item.htm?id=1027760304544

# 含分享文本的完整口令（自动提取链接）
【淘宝】7天无理由退换 https://e.tb.cn/h.8Pz6TAsvVLBXFPH?tk=BO2CT3crXeE HU071 「Apple/苹果 iPhone 17e」点击链接直接打开
```

### 3. 获取商品详情

```
GET /api/v1/shopchecker/products/{id}
```

**响应**: 返回 Product 对象。

### 4. 更新商品监控

```
PUT /api/v1/shopchecker/products/{id}
```

**请求体**（部分更新，仅传入需修改的字段）:

```json
{
  "target_price": 3800,
  "enabled": false
}
```

### 5. 删除商品监控

```
DELETE /api/v1/shopchecker/products/{id}
```

### 6. 手动触发价格检查

```
POST /api/v1/shopchecker/products/{id}/check
```

**响应**: 返回本次采集到的 PriceRecord。

> 此接口可能耗时较长（淘宝最长 120s），前端需设置合理超时（建议 150s）并展示加载状态。

### 7. 获取价格记录

```
GET /api/v1/shopchecker/products/{id}/price-records?page=1&page_size=20
```

**响应**: 分页返回 PriceRecord 列表。

### 8. 获取通知记录

```
GET /api/v1/shopchecker/products/{id}/notification-logs?page=1&page_size=20
```

**响应**: 分页返回 NotificationLog 列表。

### 9. 获取仪表盘

```
GET /api/v1/shopchecker/dashboard
```

**响应**: 返回 DashboardResp 对象。

---

## 前端实现建议

### 创建商品表单

```typescript
// 平台选项
const PLATFORM_OPTIONS = [
  { value: "jd", label: "京东" },
  { value: "jdm", label: "京东（移动版）" },
  { value: "taobao", label: "淘宝/天猫" },
  { value: "pdd", label: "拼多多" },
  { value: "qixian", label: "七鲜" },
];

// 检查间隔选项
const INTERVAL_OPTIONS = [
  { value: "30m", label: "30 分钟" },
  { value: "1h", label: "1 小时" },
  { value: "2h", label: "2 小时" },
  { value: "6h", label: "6 小时" },
  { value: "12h", label: "12 小时" },
  { value: "24h", label: "24 小时" },
];
```

### 通知类型标签颜色

```typescript
const NOTIFY_TYPE_CONFIG = {
  price_drop:      { label: "降价通知", color: "green" },
  target_reached:  { label: "到达目标价", color: "blue" },
  stock_restore:   { label: "补货通知", color: "orange" },
  login_expired:   { label: "登录失效", color: "red" },
};

const RESULT_SUMMARY_CONFIG = {
  all_success: { label: "全部成功", color: "green" },
  partial:     { label: "部分成功", color: "orange" },
  all_failed:  { label: "全部失败", color: "red" },
};
```

### 平台图标映射

```typescript
const PLATFORM_ICONS = {
  jd:    "京东",
  jdm:   "京东",
  taobao:"淘宝",
  pdd:   "拼多多",
  qixian:"七鲜",
};
```

### 价格显示格式化

```typescript
function formatPrice(price: number, platform: string): string {
  if (price <= 0) return "--";
  return `¥${price.toFixed(2)}`;
}

function formatStockStatus(status: string): { label: string; color: string } {
  switch (status) {
    case "in_stock":    return { label: "有货", color: "green" };
    case "out_of_stock": return { label: "无货", color: "red" };
    default:            return { label: "未知", color: "gray" };
  }
}
```

### 手动检查 Loading 状态

```typescript
// 手动检查可能耗时较长（淘宝 120s），建议：
// 1. 按钮点击后禁用，显示加载动画
// 2. 设置 150s 超时
// 3. 成功后刷新价格记录列表

async function handleManualCheck(productId: string) {
  setLoading(true);
  try {
    const res = await fetch(`/api/v1/shopchecker/products/${productId}/check`, {
      method: "POST",
      signal: AbortSignal.timeout(150000),
    });
    const data = await res.json();
    if (data.code === 0) {
      // 刷新价格记录
      refreshPriceRecords(productId);
    } else {
      showError(data.message);
    }
  } catch (err) {
    if (err.name === "TimeoutError") {
      showError("采集超时，请稍后重试");
    } else {
      showError("采集失败");
    }
  } finally {
    setLoading(false);
  }
}
```

---

## 常见问题

### Q: 淘宝商品采集报"需要登录"
A: 需要在「账号管理 → login3rd」模块完成淘宝账号登录。登录后采集器会自动使用会话 cookie。

### Q: 手动检查接口响应很慢
A: 淘宝采集最长需要 120 秒（浏览器渲染路径），建议前端设置 150s 超时并展示加载动画。

### Q: 价格记录中 price 和 original_price 的区别
A: `price` 是当前到手价（含所有优惠/补贴后），`original_price` 是原价/划线价。淘宝的国补/百亿补贴会体现在 price 中。

### Q: 如何区分降价通知和目标价通知
A: 查看 NotificationLog 的 `notify_type` 字段：`price_drop` 为降价通知（价格下降），`target_reached` 为目标价通知（价格低于设定值）。
