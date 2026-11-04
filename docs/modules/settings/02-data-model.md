# settings 数据模型

## 存储位置

- 默认：`data/settings.json`（相对进程工作目录）
- 可用 `APP_SETTINGS_FILE` 覆盖路径
- Compose 已将 `./data` 挂载到容器 `/app/data`，容器重建后保留

## 文件结构

```json
{
  "public_base_url": "https://et.example.com:29090",
  "frontend_title": "go-backend-core",
  "frontend_api_base_url": "/api/v1",
  "browser_url": "ws://chrome-browser:3000",
  "n8n_url": "http://n8n:5678",
  "openclaw_url": "http://openclaw:18789",
  "langbot_url": "http://langbot:5300"
}
```

| 字段 | 含义 | 校验 |
|---|---|---|
| public_base_url | 对外访问地址，用于第三方登录回调/远程登录 | 空或 http/https，去尾斜杠 |
| frontend_title | 前端默认标题 | 可空 |
| frontend_api_base_url | 前端默认 API 地址 | `/` 开头或 http/https |
| browser_url | Chrome/Browserless 远程地址（保存后写入 `CHROME_REMOTE_URL`，重启后采集器完全生效） | 空或 ws/wss/http/https |
| n8n_url / openclaw_url / langbot_url | 外部工具地址（展示/参考） | 空或 http/https/ws/wss |

## 不落库的内容

数据库密码、JWT Secret、数据库 DSN、Redis 密码、对象存储密钥等敏感配置不写入设置文件，仍由 `.env`/Docker Secret 管理。`browser_url` 不是密钥，允许在线保存；未保存时默认取 `.env` / `CHROME_REMOTE_URL`。
