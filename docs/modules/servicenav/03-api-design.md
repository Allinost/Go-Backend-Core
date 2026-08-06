# 服务导航 API 设计

## 1. 统一规范

- 路由前缀：`/api/v1/servicenav`
- 统一响应格式同项目规范（`response.Response`：`code` / `message` / `data`）
- 分页响应结构：`{ "items": [...], "has_more": bool, "total": int }`

---

## 2. 服务条目 API

### 2.1 列表查询（模糊搜索 + 类型筛选 + 排序 + 分页）

```
GET /api/v1/servicenav/services
```

Query 参数：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，默认 1 |
| page_size | int | 否 | 每页条数，默认 20，最大 100 |
| keyword | string | 否 | 服务名模糊搜索（`LIKE %kw%`） |
| type | string | 否 | 按服务类型筛选（`JSON_CONTAINS(types, ...)`） |
| sort_by | string | 否 | 排序字段：`name` / `created_at`，默认 `name` |
| sort_order | string | 否 | 排序方向：`asc` / `desc`，默认 `asc` |

> 安全：`sort_by` 走白名单（仅 `name` / `created_at`），非法值回退 `name ASC`，防 SQL 注入。

### 2.2 创建服务条目

```
POST /api/v1/servicenav/services
```

```json
{
  "name": "File Browser",
  "types": ["web", "file"],
  "protocol": "https",
  "address": "https://files.home.lan",
  "intranet_port": 8080,
  "easytier_port": 20001,
  "account": "admin",
  "password_hint": "首次部署时设置的默认密码",
  "remark": "NAS 文件管理"
}
```

校验：`name` 必填且唯一、`types` 非空且必须在 `services_nav_type` 已注册。`protocol` 可选，不传或传空默认 `http`。

### 2.3 获取详情

```
GET /api/v1/servicenav/services/:id
```

返回完整服务条目（`intranet_port` 为 `null` 表示不支持内网访问；`password_hint` 明文返回）。

### 2.4 更新服务条目

```
PUT /api/v1/servicenav/services/:id
```

支持部分更新，未传字段不修改：

```json
{
  "intranet_port": null,
  "clear_intranet_port": true,
  "remark": "已迁移到新服务器"
}
```

| 场景 | 用法 |
|------|------|
| 修改某字段 | 传入该字段值 |
| 不修改 | 不传该字段 |
| 将 `intranet_port` 置为 NULL | `"intranet_port": null, "clear_intranet_port": true` |

### 2.5 删除服务条目

```
DELETE /api/v1/servicenav/services/:id
```

---

## 3. 服务类型 API

### 3.1 类型列表

```
GET /api/v1/servicenav/types
```

返回全部类型（按名称升序）。

### 3.2 创建类型

```
POST /api/v1/servicenav/types
```

```json
{
  "name": "web",
  "remark": "Web 服务"
}
```

校验：`name` 必填且唯一。

### 3.3 更新类型

```
PUT /api/v1/servicenav/types/:id
```

支持部分更新（`name` / `remark`），重命名时若被条目引用会自动生效。

### 3.4 删除类型

```
DELETE /api/v1/servicenav/types/:id
```

删除保护：若该类型已被任一服务条目引用，返回错误 `41102`（服务类型已被服务条目引用，无法删除）。

---

## 4. 业务错误码

| 错误码 | 说明 |
|--------|------|
| 41001 | 服务条目不存在 |
| 41002 | 服务条目名称已存在 |
| 41003 | 包含无效的服务类型 |
| 41100 | 服务类型不存在 |
| 41101 | 服务类型名称已存在 |
| 41102 | 服务类型已被服务条目引用，无法删除 |
| 41200 | 名称不能为空 |
