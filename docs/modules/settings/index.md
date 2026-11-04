# settings — 应用设置模块

> 代码位置：`internal/modules/settings` ｜ 路由前缀：`/api/v1/settings`
> 前端入口：顶部导航「设置」（用户名左侧），含「前端设置 / 后端设置」

## 目标

最小化启动后的配置成本：

- 前端可在页面内切换后端 API 地址、站点标题、内网/EasyTier 域名与主题，无需重新构建；
- 管理员可在页面内配置公开访问地址、前端默认参数、Browser 地址、n8n / OpenClaw / LangBot 地址；
- 展示 MySQL / Redis / PostgreSQL / RustFS 健康状态；
- 密码、JWT、数据库连接等敏感项仍由 `.env` 管理，只读展示，不在页面落库。

## 文档目录

| 文档 | 内容 |
|------|------|
| [01-architecture-overview.md](01-architecture-overview.md) | 架构、鉴权、运行时应用 |
| [02-data-model.md](02-data-model.md) | `data/settings.json` 结构 |
| [03-api-design.md](03-api-design.md) | GET/PUT 接口、校验与错误 |
| [04-module-structure.md](04-module-structure.md) | 文件组织、生命周期、限制 |

## 关键决策

1. **文件存储**：设置持久化到 `data/settings.json`（0600，原子写入），不新增数据库表。
2. **管理员限定**：接口使用 `RequirePermission(rbac, "*", "*")`，仅 admin 可读写。
3. **敏感项只读**：数据库/Redis/JWT 等仍需 `.env`，修改后重启；接口只返回健康状态与脱敏元信息。`browser_url` 可在线保存，未保存时回退 `CHROME_REMOTE_URL`。
4. **运行时应用**：保存 `public_base_url` 后立即更新 `config.Server.PublicBaseURL` 并调用 `login3rd.SetPublicBaseURL`；保存 `browser_url` 后同步更新运行时配置与环境变量。
5. **前端地址本地优先**：API 地址、站点标题、内网/EasyTier 域名保存在浏览器 localStorage；默认值来自构建时 `VITE_*` 环境变量，不依赖后端即可切换环境。
