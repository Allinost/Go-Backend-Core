# settings 架构总览

## 定位

应用级设置模块：把“可安全在线修改”的配置集中到管理端，减少启动后改 `.env`/重建镜像的次数。

## 分层

```
internal/modules/settings/
├── module.go    # Module 实现：Init 装配 auth/rbac、RegisterRoutes
├── store.go     # data/settings.json 读写（原子写入、0600）
├── handler.go   # GET/PUT、校验、健康状态、运行时应用
└── settings_test.go
```

## 依赖

| 依赖 | 用途 |
|---|---|
| `internal/config` | 读取默认值、更新 `Server.PublicBaseURL` |
| `internal/modules/auth` | `AuthMiddleware()` + `RBAC()` |
| `internal/middleware.RequirePermission` | admin 限定（`*/*`） |
| `internal/database.Health()` | MySQL/Redis/PostgreSQL/RustFS 状态 |
| `zzz/login3rd.SetPublicBaseURL` | 立即应用回调/远程登录地址 |

## 鉴权

- 全部路由先过登录中间件；
- `GET/PUT` 均要求 `resource=* action=*`（admin）；
- 非管理员前端不显示「后端设置」页签。

## 运行时应用范围

| 设置 | 是否立即生效 |
|---|---|
| 公开访问地址 | 是（config + login3rd） |
| 前端标题/API 地址 | 保存到后端，供参考；前端实际值以 localStorage 为准 |
| n8n / OpenClaw / LangBot 地址 | 保存后展示/供前端读取，不主动发起连接 |
| Browser 地址 | 是（config + `CHROME_REMOTE_URL`）；已在运行的采集器重启后端后完全生效 |
| DB/Redis/JWT | 否，仍需 `.env` + 重启 |
