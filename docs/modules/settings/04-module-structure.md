# settings 模块结构

```
internal/modules/settings/
├── module.go       # Module：Name/Init/Close/RegisterRoutes，admin 鉴权
├── store.go        # JSON 存储：Load/Save/Exists/Path，0600 原子写入
├── handler.go      # GET/PUT、normalizeData、buildView、审计
└── settings_test.go
```

## 生命周期

1. `main.go` 注册顺序：在 `auth` 与 `login3rd` 之后（本模块依赖 auth 中间件，并在保存时调用 login3rd）。
2. `Init`：解析 `APP_SETTINGS_FILE`（默认 `data/settings.json`），取 `auth.Module.AuthMiddleware()` 与 `RBAC()`。
3. `RegisterRoutes`：`r.Use(authMW)` + `RequirePermission(rbac, "*", "*")`。
4. 保存设置：写文件 → 更新 `config.Server.PublicBaseURL` → `login3rd.SetPublicBaseURL` → 审计。

## 前端

| 文件 | 作用 |
|---|---|
| `src/components/SettingsDialog.vue` | 顶部「设置」弹窗（前端/后端两页签） |
| `src/components/AdminHeader.vue` | 设置入口（用户名左侧） |
| `src/api/settings.ts` | GET/PUT 封装 |
| `src/utils/appSettings.ts` | API 地址/站点标题的 localStorage 读写与归一化 |
| `src/api/request.ts` | axios baseURL 改为动态读取（默认 `/api/v1`） |

## 已知限制

- 敏感配置（数据库、Redis、JWT、Chrome）不能在线修改，避免运行时重建连接池；需改 `.env` 后重启。
- 外部工具地址目前用于展示/参考，不代替 `integration` 连接器的实际配置。
- 设置文件为单实例本地文件；多副本部署需改为共享存储或数据库。
- 前端 API 地址修改后必须刷新页面（axios 实例在启动时创建）。
