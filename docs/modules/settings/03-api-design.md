# settings API 设计

前缀：`/api/v1/settings` ｜ 鉴权：登录 + admin（`*/*`）

## 1. 读取设置

```
GET /api/v1/settings
```

响应：

```json
{
  "code": 0,
  "data": {
    "settings": {
      "public_base_url": "https://et.hailong.site:29090",
      "frontend_title": "go-backend-core",
      "frontend_api_base_url": "/api/v1",
      "browser_url": "ws://chrome-browser:3000",
      "n8n_url": "http://n8n:5678",
      "openclaw_url": "http://openclaw:18789",
      "langbot_url": "http://langbot:5300"
    },
    "status": [
      { "name": "mysql:main", "status": "ok" },
      { "name": "redis:main", "status": "ok" }
    ],
    "meta": {
      "version": "v0.1.0",
      "mode": "release",
      "server_name": "go-backend-core",
      "settings_file": "data/settings.json",
      "persisted": true,
      "chrome_remote_url": "ws://chrome-browser:3000",
      "read_only_keys": ["mysql", "redis", "jwt", "postgres", "rustfs", "chrome"]
    }
  }
}
```

## 2. 更新设置

```
PUT /api/v1/settings
Content-Type: application/json
```

请求体为完整 `settings` 对象（未传字段按空处理）：

```json
{
  "public_base_url": "https://et.hailong.site:29090",
  "frontend_title": "go-backend-core",
  "frontend_api_base_url": "/api/v1",
  "browser_url": "ws://chrome-browser:3000",
  "n8n_url": "http://n8n:5678",
  "openclaw_url": "http://openclaw:18789",
  "langbot_url": "http://langbot:5300"
}
```

校验：

- `public_base_url`：空或 `http/https`，自动去尾斜杠
- `frontend_api_base_url`：`/` 开头或 `http/https`
- 外部工具地址：空或 `http/https/ws/wss`
- 非法值返回 400（参数错误）

保存后立即应用 `public_base_url`（config + login3rd），并写审计日志。

## 3. 非管理员

返回 403（标准 `CodeForbidden`），不区分读写。

## 4. 注意

该接口不修改 `.env`；数据库/Redis/JWT/Chrome 等连接类配置修改后仍需重启后端。
