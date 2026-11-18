# V3 AI 上下文包：项目地图与硬约束

> 用法：AI 开始任何 V3 任务前必读。
> 上游：[V3 规划](../../docs/plans/V3-postgres-refactor-2026-11-18.md) · [V3 SOP](../../docs/projects/go-backend-core/v3/SOP.md) · [V3 路线图](../../docs/projects/go-backend-core/v3/README.md)

## 1. 当前任务

Go-Backend-Core V3.0：把基座、公共能力、项目管理模块迁移到 PostgreSQL，部署到新机器，通过 OpenResty 开放公网；业务模块继续留在 V2 老机器，后续按模块迁移。

**关键时间与形态：**

- 上线目标：**2026-12-15**（核心必上线；PM 高级前端/1Panel 后补）。
- 公网：新二级域名 → 新机器 1Panel v2 托管证书/反代，443/HTTPS → 本机 V3 容器 80；管理员创建用户并授权；默认无自助注册。
- V2：**全面冻结**，只运行与安全修复；V3 是唯一开发线。
- **1Panel 接入暂停**：API/数据/MCP/多实例/密钥入库不进入 V3.0，后续单独立项。
- V3.0 只要求公网反代/证书可用（1Panel 或 OpenResty，实现待定）；V3 只依赖容器 80 + 转发头 + HTTPS。
- **配置中心**：能配置就配置；模块声明 ConfigSpec；运行时参数 DB 动态化，启动/安全密钥走 Secret；禁止写死 URL/阈值/开关。
- **多主机管理**：V3.0 做主机台账（cloud/host/backup）+ SSH 连通/状态；凭据加密；不做任意命令/批量操作。
- **移除 s0/s1/s2**：health/metrics/info/modules 合并到 `ops`。

## 2. V3.0 范围

**允许修改：**

- `internal/config`、`internal/database`、`internal/middleware`
- `internal/modules/{s0,s1,s2,auth,migrate,ops,settings,integration,webhook,servicenav}`
- `internal/services/*`（V3.0 需要的公共能力）
- `internal/pkg/*`、`pkg/*`（仅必要改动）
- `zzz/developmentManager/*`（含项目管理扩展）
- `cmd/server`、新增 `cmd/v3migrate`
- `migrations/postgres/*`
- 相关文档、测试、脚本

**禁止修改（V3.0）：**

- `zzz/goodser/*`
- `zzz/ia/*`
- `zzz/login3rd/*`
- `zzz/shopchecker/*`
- 它们的路由注册、表、数据、前端模块

> 发现范围外问题：登记到任务日志/新任务，不要顺手改。

## 3. 关键约束

1. **PG-only**：V3 运行时不连接 MySQL；SQL 用 `$n`；禁止 MySQL 方言。
2. **可配置优先**：新增能力必须提供配置项/Schema/默认值/校验；禁止硬编码 URL、阈值、开关、保留天数。
2. **迁移唯一**：Schema 只由 `migrations/postgres` 管理；生产 `auto_migrate=false`。
3. **GORM 保留**：PG 驱动；连接复用 `database.Main()`；禁止生产 AutoMigrate。
4. **API 契约**：保持 `/api/v1`；新增字段/端点向后兼容。
5. **业务模块冻结**：V3.0 不改/不注册业务模块。
6. **V3 为 PM/用户/RBAC 唯一事实源**：V2 在上线前冻结，不做双向同步。
7. **公网最小暴露**：新机器反代只开放 443；V3 容器 80 仅本机/内网；80/29090/29092/管理面板不直连公网；管理接口白名单。
8. **人工合并**：AI 不自动合并/发布/删除数据。
9. **证据**：测试、lint、迁移校验、接口冒烟、公网安全检查必须贴结果。
10. **12.15 分批**：不因赶功能跳过迁移演练/公网安全/回滚演练。

## 4. 仓库地图

```
cmd/server/                  # 唯一入口；V3.0 只注册基座/公共/PM
internal/
  config/                    # Viper 配置；database.postgres.main
  database/                  # Manager；Main()/PG()；健康检查；breaker
  middleware/                # logger/CORS/ratelimit/auth/rbac/metrics
  modules/                   # 基座模块（s0/auth/migrate/ops/...）
  services/                  # 23 个公共服务
  pkg/                       # logger/response/errors/crypto/authz
zzz/developmentManager/      # V3 项目管理模块（重点）
migrations/postgres/         # V3 Schema（唯一事实源）
migrations/mysql/            # V2 归档，只读
docs/plans/                  # V3 规划
docs/projects/go-backend-core/v3/  # 路线图 + SOP
aiPrompt/v3/                 # 本上下文包
```

## 5. 常用命令

```bash
go build ./...
go test -short -count=1 ./...
go test -tags=integration -count=1 ./...
go vet ./...
golangci-lint run
make swagger
```

Windows 下使用 Git Bash；中文文件用编辑器/heredoc，禁止 `Set-Content` 改写。

## 6. 关键文档

| 文档 | 用途 |
|---|---|
| `docs/plans/V3-postgres-refactor-2026-11-18.md` | 范围、决策、架构、WBS、风险 |
| `docs/projects/go-backend-core/v3/README.md` | 需求/任务/依赖/验收 |
| `docs/projects/go-backend-core/v3/SOP.md` | 开发流程、AI 协议、PG 规范、迁移/部署 |
| `aiPrompt/v3/pg-rules.md` | PG 速查 |
| `aiPrompt/v3/task-template.md` | 任务契约模板 |
| `aiPrompt/v3/review-checklist.md` | 评审清单 |
| `aiPrompt/v3/migration-rules.md` | 数据迁移规则 |

## 7. 升级路径

以下情况必须停下来问人：

- 需要改业务模块或 `/api/v1` 契约。
- 需要删除/重建表、清空数据、改生产配置。
- 需要新增依赖/升级 Go/改公共接口。
- 上下文缺失、测试无法运行、发现安全/数据风险。
- 任务范围与规划冲突。

## 8. 交付前自检

- [ ] 只改任务相关文件。
- [ ] 无 MySQL 方言/驱动进入 V3 运行时。
- [ ] 迁移/GORM/类型符合规范。
- [ ] 测试/lint/build 通过。
- [ ] 文档/数据字典/任务日志已更新。
- [ ] 提交信息带任务编号。
