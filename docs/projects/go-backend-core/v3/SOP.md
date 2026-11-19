# Go-Backend-Core V3 SOP

> 版本：v1.9（L1/L2 手动维护切换；RustFS 固定在 ACloud）
> 生效日期：2026-11-18
> 适用范围：Go-Backend-Core V3.0 及后续 V3.x 业务模块迁移
> 关联：[V3 规划](../../../plans/V3-postgres-refactor-2026-11-18.md) · [V3 路线图](README.md)
> 执行方式：**AI 主执行 + 人工评审与验收**

---

## 1. 目的

本 SOP 规定 V3 的开发方式、PostgreSQL 规范、AI 协作协议、测试门禁、数据迁移、模块接入、部署与台账要求，确保：

1. V3 运行时只使用 PostgreSQL；业务模块暂留 V2，后续按模块迁移。
2. 所有 Schema 变更可追溯、可回滚，生产不依赖 GORM AutoMigrate。
3. AI 产出可验证、可评审、不越界；人工对合并与上线负责。
4. V3 项目在 developmentManager 中全程可追踪（需求/任务/依赖/风险/发布/进度）。
5. 新机器与老机器并行期间，数据归属、认证、路由、回滚边界清晰。
6. 新二级域名指向新机器，由公网反代/证书开放 HTTPS（1Panel 或 OpenResty，待定），容器内 80；管理员创建公网用户并授权；默认无自助注册。
7. V2 全面冻结：只运行与安全修复，V3 是唯一开发线。
8. 2026-12-15 上线：核心必上线，PM 高级前端 12.15 后补齐；1Panel 接入暂停。

---

## 2. 角色与职责

| 角色 | 承担者 | 职责 |
|---|---|---|
| 产品负责人（PO） | Tom | 需求优先级、验收标准、上线决策 |
| 架构/评审 | Tom + AI 评审代理 | 方案、Schema、迁移、安全、范围控制 |
| AI 实现 | AI 开发代理 | 读上下文、实现、测试、文档、任务日志 |
| AI 评审 | 子代理（reviewer） | 代码/SQL/安全/回归独立检查 |
| 运维 | Tom | 新机器、PG、Redis、S3、备份、监控、切换 |
| 台账 | AI + Tom | 任务状态、工时、阻塞、周报、资产 |

> AI 不拥有合并权、上线权、数据删除权；这些必须人工确认。

---

## 3. 核心原则

1. **范围冻结**：V3.0 不改业务模块（goodser/ia/login3rd/shopchecker）。发现范围外问题，登记新任务，不顺手改。
2. **迁移唯一**：Schema 只由 `migrations/postgres` 管理；生产 `auto_migrate=false`。
3. **PG 唯一**：V3 运行时 SQL 使用 `$n`、`jsonb`、`boolean`、`timestamptz`；不引入 MySQL 语法。
4. **契约优先**：保持 `/api/v1`；新增字段/端点向后兼容。
5. **小步提交**：一个任务一个分支，提交可回滚，提交信息带任务编号。
6. **证据优先**：测试、lint、迁移校验、接口冒烟必须留证据；不接受“应该可以”。
7. **人工合并**：AI 不自动合并、不自动发布、不自动删数据。
8. **单一事实源**：V3 项目管理/用户/RBAC 在 V3；V2 的 developmentManager 在 V3.0 上线前冻结。
9. **公网最小暴露**：只开放 443；80/29090/29092 不直连公网；管理接口 IP 白名单/内网。
10. **V2 冻结**：V2 不新增功能；所有新需求进 V3 里程碑。
11. **12.15 分批**：优先保迁移演练、PG 测试、公网安全；非关键 UI 后补；1Panel 接入暂停。

---

## 4. 开发流程（四段式 + AI）

```
① 需求（人工）
   - 在 DM 建/更新需求（RQM-V3-xxx），写清目标、范围、验收标准、非目标
   - PO 确认 priority/status=approved

② 规划（人工 + AI）
   - 建里程碑/任务（V3-Px-yy），拆依赖、预估、负责人
   - 高风险任务（Schema/迁移/认证/部署）先出方案，人工确认
   - AI 只读探索，产出方案与风险，不改代码

③ 开发（AI）
   - 按任务契约实现；小步提交；自测；子代理评审
   - 更新任务日志、文档、资产

④ 验收（人工）
   - 按 DoD 和验收标准逐条检查
   - 合并到 feature-V3；更新 DM 状态=done
   - 阶段末/发布前做里程碑评审
```

### 4.1 任务契约（每个任务必须先写）

```markdown
# 任务契约：V3-Px-yy 标题

## 目标
- ...

## 非目标
- ...

## 必读上下文
- docs/plans/V3-postgres-refactor-2026-11-18.md §x
- docs/projects/go-backend-core/v3/SOP.md §x
- 相关代码/迁移/模块文档

## 验收标准
- [ ] 可执行命令/测试/接口断言
- [ ] 文档/迁移/数据字典

## 约束
- PG-only；`$n`；不改业务模块；保持 /api/v1；生产禁用 AutoMigrate

## 交付物
- 代码/测试/迁移/文档/任务日志

## 风险与回滚
- ...
```

### 4.2 分支与提交

- 分支：`v3/<task-code>-<short-slug>`，例如 `v3/p1-02-database-manager`。
- 集成分支：`feature-V3`。
- 提交格式：

```
<type>(v3): <summary> (<task-code>)

<body: 做了什么/为什么/如何验证>
```

示例：`feat(v3): add pg main pool and health check (V3-P1-02)`。

- 一个任务可多个小提交；不要混合无关改动。
- 合并前 rebase/merge `feature-V3`，解决冲突后重跑门禁。

---

## 5. AI 开发协议

### 5.1 上下文包

AI 开始任务前必须读取：

1. `AGENTS.md`（如存在）与仓库根 `README.md`。
2. V3 规划、V3 路线图、本 SOP。
3. 任务相关的模块文档、迁移、仓库、测试。
4. `aiPrompt/v3/` 下的规则（任务模板、PG 规则、评审清单、迁移规则）。

若上下文缺失，AI 必须先补齐或提问，不得猜测。

### 5.2 AI 工作循环

```
读上下文 → 写方案 → 自检方案 → 实现 → 运行门禁 → 自审 diff
   → 子代理评审 → 修复 → 人工评审 → 合并 → 更新台账
```

### 5.3 硬性规则

| # | 规则 |
|---|---|
| A1 | 不直接提交 `main`/`feature-V3`；只在 `v3/<task-code>` 分支工作。 |
| A2 | 不修改 `zzz/goodser`、`zzz/ia`、`zzz/login3rd`、`zzz/shopchecker`（V3.0 范围外）。 |
| A3 | 不引入 MySQL 驱动/语法到 V3 运行时；测试工具除外。 |
| A4 | 不执行 `DROP`/`TRUNCATE`/无备份迁移/生产数据操作，除非人工书面确认。 |
| A5 | 不读取/输出密钥、token、真实 `.env`；不把密钥写入代码/日志。 |
| A6 | 不跳过测试；失败必须报告；不得伪造/篡改测试结果。 |
| A7 | 不擅自升级依赖/改公共接口；确需变更，先登记任务并人工确认。 |
| A8 | 不删除他人未提交改动；发现工作区脏，先询问。 |
| A9 | 提交/PR 必须带任务编号与验证证据。 |
| A10 | 完成后必须更新任务日志、文档、数据字典/资产。 |

### 5.4 AI 自检清单

- [ ] 变更范围与任务契约一致，无范围外文件。
- [ ] 新增/修改 SQL 全部 `$n`、无 MySQL 方言。
- [ ] 新增/修改表有迁移，生产不依赖 AutoMigrate。
- [ ] 单元测试 + PG 集成测试（如涉及仓库）通过。
- [ ] `go build ./...`、`go vet ./...`、`golangci-lint run` 通过。
- [ ] API 变更更新 Swagger/模块文档。
- [ ] 兼容 `/api/v1`。
- [ ] 任务日志/台账已更新。

### 5.5 子代理评审

高风险任务（Schema、迁移、认证、部署、数据）必须由独立评审代理检查：

1. **SQL 评审**：占位符、方言、索引、事务、锁、注入。
2. **回归评审**：路由/权限/错误码/兼容性。
3. **安全评审**：密钥、越权、日志泄露、文件路径。
4. **迁移评审**：可重复、可回滚、序列、校验、allowlist。

评审输出必须包含：问题、严重级别、修复建议、是否阻塞合并。

---

## 6. PostgreSQL 开发规范

### 6.1 SQL

| 规则 | 说明 |
|---|---|
| 占位符 | 一律 `$1..$n`；禁止 `?` |
| 标识符 | 全小写；动态标识符用 `Quote()`/`pgx.Identifier` |
| JSONB | `jsonb`；包含 `@>`；取值 `->>`；索引用 GIN |
| 时间 | `timestamptz`；应用 UTC；`now()` |
| 布尔 | `boolean`；禁止 `= 1` |
| 金额 | `numeric`；禁止 `float` |
| 分页 | `LIMIT $n OFFSET $m` |
| 排序 | 需要稳定时加 `id`；NULL 语义显式 `NULLS FIRST/LAST` |
| 搜索 | 大小写不敏感用 `ILIKE`/`citext` |
| UPSERT | `ON CONFLICT (...) DO UPDATE SET ... EXCLUDED` |
| 批量 | `pgx.CopyFrom` 或 `unnest($1::type[])` |
| 事务 | 短事务；不在事务内做外部 HTTP |
| 锁 | 大表 DDL 用 `CONCURRENTLY` 或分步 |

### 6.2 迁移

- 文件：`migrations/postgres/NNNN_snake_case.sql`。
- 已发布迁移不可修改；修复用新迁移。
- 每个迁移尽量单事务；特殊语句注释说明。
- Seed 用 `ON CONFLICT DO NOTHING`。
- 新增表必须：主键 identity、审计字段、必要索引、注释、数据字典。
- 软删除表建部分索引 `WHERE deleted = false`。
- 破坏性变更必须先备份/双写过渡/人工确认。

### 6.3 GORM

- 驱动：`gorm.io/driver/postgres`，连接复用 `database.Main()` 的 `*sql.DB`。
- 生产禁用 AutoMigrate；启动校验 `_migrations` 版本。
- 模型标签：`timestamptz`/`jsonb`/`numeric`/`boolean`；禁止 MySQL 类型。
- 表名显式 `TableName()`；不依赖隐式复数化。
- UPSERT：`clause.OnConflict`。
- 错误：开启 `TranslateError`，映射唯一键/外键。
- 复杂查询（统计/窗口/JSONB）优先手写 SQL/pgx。
- 每个 GORM Store 配 PG 集成测试。

### 6.4 连接与配置

```yaml
database:
  auto_migrate: false
  postgres:
    main:
      enabled: true
      host: ...
      port: 5432
      user: ...
      password: ""        # .env/secret
      dbname: go_backend_core_v3
      sslmode: disable    # 生产按需 require/verify-full
      search_path: gobackendcore,public  # 专用 schema；public 仅兜底
      timezone: UTC
      max_open: 30
      max_idle: 10
      conn_max_lifetime: 30m
```

- V3 不初始化 MySQL 主库；`migrations/mysql` 只作 V2 归档。
- **PG schema**：使用专用 schema `gobackendcore`（`search_path=gobackendcore,public`）。应用账号通常无权在 `public` 建表，但拥有数据库 `CREATE` 权限时，migrate 模块启动会自动创建并拥有该 schema；无需 superuser 授权。
- 连接池不超过 PG `max_connections` 的 70%；需要时上 pgbouncer。
- 健康检查暴露 `postgres:main`，不再暴露 `mysql:main`。

---

## 7. 模块接入 V3 SOP（V3.1+ 业务模块）

> V3.0 禁止接入业务模块；本节用于后续 V3.1 goodser、V3.2 ia、V3.3 login3rd、V3.4 shopchecker。

### 7.1 前置条件

- [ ] V3.0 稳定运行 ≥ 7 天。
- [ ] 该模块在 DM 有里程碑、需求、任务、验收标准。
- [ ] 明确新增/修改的业务需求（不只是数据库迁移）。
- [ ] 明确数据迁移窗口、回滚方案、负责人。

### 7.2 接入步骤

1. **立项**：DM 建里程碑/需求/任务；冻结范围。
2. **代码移植**：从 V2 分支复制模块到 V3；先保证编译，不改变业务行为。
3. **Schema**：新增该模块迁移；数据字典登记；索引评审。
4. **方言改造**：按 §6 改 SQL/GORM；补 PG 集成测试。
5. **业务回归**：核心流程 + 边界 + 权限 + 错误码。
6. **性能基线**：关键接口 p95、慢查询、索引。
7. **数据迁移**：
   - 冻结 V2 模块写入；
   - 快照/导出；
   - 迁移到 V3；
   - 校验（行数/主键/FK/JSON/时间/金额/抽样）；
   - 需要时做增量同步；
   - 切换路由到 V3；
   - V2 模块停用但保留数据。
8. **观察 7 天**：错误率、延迟、数据一致性。
9. **文档/台账**：模块文档四件套、发布说明、任务关闭。
10. **老机器**：该模块稳定后，评估是否可从 V2 下线；全部模块完成才评估整机下线。

### 7.3 路由切换

- 前端/网关按模块把 `/api/v1/<module>` 指向 V3。
- 切换前双环境健康检查、JWT 兼容、CORS 白名单。
- 切换后保留 V2 只读/回退开关（短期），确认稳定后关闭。
- 不做同域随机分流；按模块整体切换，避免数据双写。

---

## 8. 数据迁移 SOP

### 8.1 V3.0 bootstrap

- 仅迁移 allowlist 表：auth/RBAC/APIKey、scheduler（active）、fileref、audit/feature（可选）、integration、webhook、servicenav、developmentManager、settings。
- 不迁移业务模块表。
- 使用 `cmd/v3migrate`；默认拒绝非 allowlist 表。
- 两次演练 + 校验报告 + 签字。

### 8.2 业务模块迁移（V3.1+）

- 按模块 allowlist；先结构后数据。
- 冻结写入 → 快照 → 迁移 → 校验 → 增量（如需要）→ 切路由。
- 迁移脚本必须可重复执行（清空目标表后可重来）。
- 记录迁移前/后行数、校验和、耗时。
- 回滚：保留 V2 数据快照 ≥ 7 天；V3 目标表可清空重来。

### 8.3 校验清单

- [ ] 行数一致。
- [ ] 主键无重复、序列大于 max(id)。
- [ ] 外键无孤儿。
- [ ] JSON 合法。
- [ ] 时间/时区一致（UTC）。
- [ ] 金额 `SUM/MIN/MAX` 一致。
- [ ] 抽样查询结果一致。
- [ ] 报告归档。

---

## 9. 测试与验收

### 9.1 命令门禁

```bash
# 1. 编译
go build ./...

# 2. 静态检查
go vet ./...
golangci-lint run

# 3. 单元测试
go test -short -count=1 ./...

# 4. PG 集成测试（需要 TEST_POSTGRES_DSN 或 testcontainers）
go test -tags=integration -count=1 ./...

# 5. 迁移测试
go test -tags=integration ./internal/services/migrate/...

# 6. Swagger（API 变更时）
make swagger
```

> Windows 开发按 `docs/README.md` 约定使用 Git Bash；中文文件用编辑器/heredoc，禁止 `Set-Content` 改写。

### 9.2 任务 DoD

- [ ] 代码 + 单元测试 + lint 通过。
- [ ] SQL 变更配 PG 集成测试或 golden SQL 测试。
- [ ] 表结构变更配迁移 + 数据字典 + 文档。
- [ ] API 变更配 Swagger + 模块文档。
- [ ] 提交信息带任务编号。
- [ ] 人工评审通过 + 任务日志留痕。
- [ ] 未越界修改业务模块（V3.0）。

### 9.3 公网安全检查（12.15 前必做）

- [ ] 只开放 443；80 跳转 443；应用端口不直连公网。
- [ ] 证书有效/续期正常；HSTS（如启用）配置正确。
- [ ] `TrustedProxies` 只信任反代出口；真实 IP 正确。
- [ ] 安全头（Content-Type/Frame/Referrer/CSP）配置。
- [ ] 登录/写接口限流、失败锁定、审计生效。
- [ ] 默认无自助注册；管理员授权流程可用。
- [ ] RBAC 越权测试（普通用户/管理员/未登录）通过。
- [ ] 管理接口（auth/admin、ops、migrate）IP 白名单/内网限制。
- [ ] 调试端口 29092、Swagger（如公网）不暴露。
- [ ] 上传大小/类型限制；路径穿越测试通过。
- [ ] 日志不泄露密钥/Token/密码。
- [ ] 依赖漏洞扫描无高危；反代版本无已知高危。
- [ ] 管理面板（如 1Panel）仅内网 + 专用 Token + 审计。

### 9.4 阶段验收

| 阶段 | 验收人 | 产出 |
|---|---|---|
| P1 数据层 | Tom | PG 连接/健康检查证据 |
| P2 Schema | Tom + 评审代理 | 迁移执行/回滚记录 |
| P3 方言 | Tom + 评审代理 | 方言门禁 0；PG 测试报告 |
| P4 基础/公共 | Tom | 模块冒烟清单 |
| PM 项目管控 | Tom | 里程碑/任务/看板/报表演示 |
| P6 数据 | Tom | bootstrap 校验报告 |
| P7 测试/CI | Tom | CI 绿；覆盖率 |
| P8 部署 | Tom | 新机器部署/备份/监控证据 |
| P9 上线 | Tom | 7 天观察记录；V2 正常 |

---

## 10. 部署与环境

### 10.1 新机器（V3）

- PostgreSQL 16+（独立库/账号/强密码；生产按需 SSL）。
- Redis、RustFS/MinIO、Chrome（按现有 compose 能力）。
- 容器内监听 **80**；调试端口 29092 仅内网。
- 新二级域名指向新机器；公网反代/证书/HTTPS（1Panel 或 OpenResty，实现待定），反代本机 V3:80。
- 1Panel API/数据/MCP 接入暂停；V3 不依赖 1Panel API。
- 老二级域名/老机器保持不变，继续服务 V2。
- 环境变量：`APP_DATABASE_POSTGRES_MAIN_*`、`APP_AUTH_JWT_SECRET` 等。
- 启动前：迁移完成、Schema 版本匹配、备份可用。
- 启动后：`/api/v1/health/health`、`/api/v1/health/modules`、PM 核心 API 冒烟。

### 10.2 公网反代 / 证书（实现待定）

- 新机器：新二级域名站点 443/HTTPS，证书申请/续期，反代本机 V3:80。
- 公网只开 443；V3 容器 80 仅本机/内网；应用 29090/29092、管理面板不对公网。
- 转发头：`Host`、`X-Real-IP`、`X-Forwarded-For`、`X-Forwarded-Proto https`；Gin `TrustedProxies` 只信任反代出口。
- WebSocket：`proxy_http_version 1.1` + `Upgrade/Connection`；读超时 ≥ 300s。
- 上传：`client_max_body_size` 按附件需求（建议 50–100MB）。
- 安全头：`X-Content-Type-Options`、`X-Frame-Options`、`Referrer-Policy`、CSP（按前端）。
- 限流：登录/写接口在反代 + 应用双层限流。
- 管理接口：`auth/admin`、`ops`、`migrate` 加 IP 白名单/内网限制。
- 证书：由反代/证书工具统一管理；上线前验证续期与 HTTPS 链。
- **V3 不手工改反代配置**；变更由运维在 1Panel/OpenResty 操作并记录。

### 10.3 老机器（V2 + 老 1Panel，冻结）

- 继续运行 MySQL + 全部业务模块 + 老二级域名，但**不新增功能**。
- 老机器 1Panel 只服务老域名/V2；不参与新机器 V3 的生产反代。
- V3.0 上线前冻结 V2 developmentManager 写入；上线后 V2 PM 只读/归档。
- 不升级 V2 数据库；不做双写。
- V2 只允许安全/数据修复；新需求转 V3 里程碑。
- 老机器下线条件：所有业务模块迁移到 V3、**边缘迁移方案完成**、V3 稳定 ≥ 30 天、数据/回滚演练通过、PO 签字。

### 10.4 备份与监控

- `pg_dump` 每日全量 + WAL 归档（按需）；保留 ≥ 30 天。
- 恢复演练每月一次。
- 监控：连接数、慢查询、锁等待、磁盘、复制延迟（如有）、错误率、p95。
- 告警：连接耗尽、慢查询 > 1s、备份失败、磁盘 > 80%、证书到期。
- 反代 access/error 日志接入监控，保留真实 IP。

### 10.5 1Panel（V3.0 暂停，后续待定）

> 本节保留为后续评估资料；V3.0 不排 1Panel 任务，只要求公网反代/证书可用。

- **边缘**：1Panel 或 OpenResty 均可；若用 1Panel，面板仅内网/VPN/IP 白名单，不对公网。
- **数据源/MCP**：V3 直连本机/内网 1Panel v2；专用最小权限 Token；只读 + 工具白名单；禁止原始 URL 代理。
- **受限操作**：12.15 后开启；二次确认 + 审计 + 限流。
- **不允许 1Panel 管理 V3 Schema**；迁移仍由 V3 migrate 管理。
- API 适配不可用时 V3 核心功能不受影响；边缘依赖需有回源/备用预案。
- 详见规划 §7.6。

### 10.6 1Panel v2 只读接入规范（V3.0 暂停）

| 项 | 规范 |
|---|---|
| 版本 | 1Panel 社区版 **2.5.1**；API 前缀 `/api/v2` |
| 开发机 | `http://10.144.144.10:58331`（仅开发/联调） |
| 云服务器 | `https://gtp.hailong.site`；安全入口 `EternallyAscend` |
| 生产 | 新机器本机/内网地址；面板不公网 |
| Swagger | `{host}:{port}/1panel/swagger/index.html`（需登录；未登录 401） |
| Header | `1Panel-Token`、`1Panel-Timestamp`（秒级） |
| 签名 | 推荐 `HMAC-SHA256(API-Key, "1panel:"+ts)`；兼容 `MD5("1panel"+API-Key+ts)` |
| API Key | 面板创建/启用；IP 白名单 + 有效期；存 Secret |
| 时间 | 客户端与 1Panel 必须 NTP 同步 |
| 只读范围 | dashboard/monitoring/containers/websites/certs/apps/logs/cronjobs/task-center/nodes |
| 只读动词 | GET，或 1Panel-skills 的只读 search POST；按模块白名单 |
| 禁止 | 创建/更新/删除/重启/执行脚本等写操作（12.15 后另评） |
| 代理 | 只允许适配器内固定路径；禁止原始 URL/任意路径代理 |

> 官方参考：
> - API 鉴权：https://docs.fit2cloud.com/1panel/dev_manual/api_manual/
> - 运维技能只读模块：https://docs.fit2cloud.com/1panel/dev_manual/skills
> - Swagger：`{host}:{port}/1panel/swagger/index.html`（开发机需登录后访问）

### 10.7 多实例与 API Key 托管（V3.0 暂停）

- 表：`zzz_ops_onepanel_instance`；字段含 `name/base_url/api_key_ciphertext/api_key_hint/auth_mode/environment/last_status`。
- API Key 用 AES-GCM 加密后入库；列表/详情只返回末 4 位；日志/审计/错误脱敏。
- 多实例：`instance_id` 显式传入；不设默认实例；按实例做权限/审计。
- 管理 API：`/api/v1/ops/onepanel/instances`（read/manage/test）。
- 首次接入：通过管理 API/CLI 写入实例；不要把 Key 写进 `config.yaml`/代码/文档。
- 1Panel-skills：按其只读模块（monitoring/containers/websites/apps/logs/cronjobs/task-center/nodes）实现 Go 适配器；不复制其代码（许可证待确认，README MIT/仓库 GPL-3.0 不一致）。
- 当前云服务器 `gtp.hailong.site` 联调结果：`401 API 接口禁止访问`；响应头 `Server: cloudflare`，需启用 API/Key 白名单并处理 Cloudflare 回源 IP 后复测。该项阻塞 P0-04，不阻塞 V3 核心。

### 10.8 配置中心规范（V3.0）

- **能配置就配置**：新增能力必须提供配置项，禁止写死 URL/阈值/开关/保留天数。
- 每个模块声明 `ConfigSpec`：key/type/default/enum/min/max/sensitive/reloadable/env_locked/description/group/validate。
- 优先级：
  - 启动/安全：Secret/ENV > config.yaml > 默认值；不入 DB 明文。
  - 运行时：DB > config.yaml > 默认值；可 `env_locked` 锁定。
- 变更：校验 → 持久化 → 发布 `config.changed` → 模块应用；失败自动回滚。
- API：`/api/v1/settings/config/{schema,validate,history,rollback}`；权限 `settings.config.read/write`。
- 敏感项只存密文/Secret 引用；接口/日志/历史脱敏。
- **env 边界**：env 只放启动/节点/数据库/Redis/RustFS 等基础设施配置；**业务账号与外部密钥（如 JD、1Panel）不写 env**，统一走 `system_config`，由设置页维护。
- 敏感项使用 `crypto.master_key_hex`（AES-GCM）加密后落库；接口/日志/历史只回显脱敏值；未配置密钥时启动告警。
- 表：`system_config`、`system_config_history`。
- 测试：优先级、校验、热更新、回滚、脱敏、env_locked。

### 10.9 多主机管理规范（V3.0 最小）

- 表：`zzz_ops_host`；角色 `cloud/host/backup`；地址、SSH 端口、用户、凭据密文、状态。
- 凭据 AES-GCM 加密；只回提示；SSH 指纹校验；命令白名单；超时/重试/熔断。
- 权限：`ops.host.read/manage/connect`；所有连接审计。
- V3.0 只做台账 + SSH 连通/状态；不做任意命令/文件分发/批量重启。
- V3.1 再做：批量运维、数据库迁移/备份（主机间、冷备份）。
- 模块调整：`s0` 改名 `health`，保留 ping/health/metrics/info/modules；删除 s1/s2 占位模块。

### 10.10 L1/L2 手动维护（V3.0）

- **默认 `database.failover_mode=manual`**：不做连接失败自动切库，避免脑裂。
- **URL 归属**：指向 L1 的（V3 主入口/PG/Redis/L1 工具）随维护切换；指向 ACloud 的（RustFS/n8n/OpenClaw/LangBot/MCP/TTS）**固定不变**。
- **三种模式**：
  - `shutdown` 停机：直接停 L1，不启用 L2；结束直接回 L1。
  - `migrate_return` 迁移并迁回：L1→L2 同步确认 → L2 运行 → L2→L1 反向同步确认 → 回 L1。
  - `migrate_no_return` 迁移不迁回：L1→L2 同步确认 → L2 长期运行；L1 停用/退役，不做反向同步。
- 开始维护：`migrate_*` 需勾选“L1→L2 已同步”；停止 L1 V3，在 L2 启动 V3（`APP_PROFILE=backup`）。
- 结束维护：按模式处理；`migrate_return` 必须反向同步确认。
- 状态/操作审计：维护状态表 + 模式/操作人/时间/同步确认；权限 `ops.maintenance.read/start/end`。
- 不做运行中热切连接；L1/L2 各自 V3 进程连接本机服务。

### 10.11 管理员账号初始化（V3.0）

首次部署后使用 `v3admin` 创建管理员，不依赖开放注册。

```powershell
$env:V3_ADMIN_PASSWORD='至少8位密码'
go run ./cmd/v3admin -username admin -nickname 管理员
```

- 默认角色 `admin`；`-role user` 可创建普通用户后另行授权。
- 用户已存在时只补齐角色，不重置密码。
- 密码也可用 `-password` 传入，但推荐环境变量，避免进入命令历史。
- 表不存在时先执行 `POST /api/v1/migrate/schema/migrate`。
- 首次登录后立即修改密码；默认不开放自助注册（P4-06 关闭 `/auth/register`）。

---

## 11. 项目管控与周报

### 11.1 台账字段

每个任务必须维护：

- `code`、`title`、`description`（验收标准）
- `milestone_id`、`requirement_id`、`iteration_id`
- `type`、`priority`、`status`
- `estimate_hours`、`actual_hours`
- `owner`、`start_date`、`due_date`
- 依赖、检查项、评论、标签、资产
- 任务日志（状态变更/工时/阻塞/验收）

### 11.2 状态流转

```
todo → doing → review → done
  ↑        ↓       ↓
  └──── blocked ←──┘
done → doing（验收不通过打回）
任意 → cancelled（人工确认）
```

### 11.3 周报

每周五生成，内容：

- 本周完成（任务/人日）
- 进行中（进度/预计完成）
- 阻塞/风险（影响/决策人）
- 指标：里程碑进度、需求完成、阻塞数、方言门禁、AI 返工率
- 下周计划

### 11.4 AI 试点

V3-SOP-03 选一个真实任务试点（建议 V3-P1-02 `database.Manager`）：

1. 写任务契约。
2. AI 按 SOP 实现。
3. 子代理评审 + 人工评审。
4. 记录：耗时、返工点、上下文缺失、改进项。
5. 更新 SOP / 上下文包。

---

## 12. 模板

### 12.1 任务契约模板

见 §4.1。

### 12.2 评审清单

```markdown
## 评审：V3-Px-yy

### 范围
- [ ] 只改任务相关文件
- [ ] 未改业务模块（V3.0）

### SQL/PG
- [ ] 全部 `$n`，无 MySQL 方言
- [ ] JSONB/boolean/timestamptz 正确
- [ ] 索引/事务/锁合理

### Schema/GORM
- [ ] 迁移可重复/可回滚
- [ ] 未依赖 AutoMigrate
- [ ] 模型与迁移一致

### 测试
- [ ] 单元测试
- [ ] PG 集成测试/golden SQL
- [ ] 冒烟/回归

### 安全/兼容
- [ ] 无密钥泄露
- [ ] 权限正确
- [ ] `/api/v1` 兼容

### 文档/台账
- [ ] 模块文档/Swagger
- [ ] 数据字典/资产
- [ ] 任务日志
```

### 12.3 迁移报告模板

```markdown
# V3 bootstrap/模块迁移报告

- 日期/执行人/环境：
- 源：MySQL V2（主机/库/快照）
- 目标：PostgreSQL V3（主机/库）
- 表清单/行数：
- 耗时：
- 校验结果：行数/主键/FK/JSON/时间/金额/抽样/序列
- 差异与修复：
- 回滚点：
- 签字：
```

---

## 13. 禁止事项

1. 在 V3.0 修改/注册业务模块。
2. 生产启用 GORM AutoMigrate。
3. 使用 `?` 占位符或 MySQL 方言。
4. 直接改已发布迁移文件。
5. 无备份执行破坏性 SQL。
6. 跳过测试/伪造证据。
7. AI 自动合并/发布/删数据。
8. 在 V3 与 V2 同时对同一业务数据双写。
9. 把密钥/真实凭据写入代码、日志、文档。
10. 未经确认改变 `/api/v1` 契约。
11. 把 80/29090/29092、MySQL、1Panel 面板直接暴露公网。
12. 开放公网自助注册（默认禁止；如需，必须审批流 + 人工确认）。
13. 在 V2 上新增功能（V2 冻结，只允许安全/数据修复）。
14. 让 1Panel 管理 V3 Schema 或作为 V3 身份源/唯一监控源。
15. 在 12.15 前为赶功能跳过迁移演练/公网安全评审/回滚演练。

---

## 14. 附录：常用命令

```bash
# 构建/测试
go build ./...
go test -short -count=1 ./...
go test -tags=integration -count=1 ./...
golangci-lint run

# 迁移（本地/测试）
# 通过 migrate 模块 API 或测试基座执行；生产由发布流程执行

# Swagger
make swagger

# V3 bootstrap（示例，参数以实际实现为准）
go run ./cmd/v3migrate plan --config config.yaml
go run ./cmd/v3migrate dump --out ./tmp/v2dump
go run ./cmd/v3migrate load --dsn "$V3_PG_DSN"
go run ./cmd/v3migrate verify --report ./tmp/v3-verify.md

# 前端（Go-Frontend-Core）
npm run dev
```

---

## 15. 变更记录

| 日期 | 版本 | 变更 |
|---|---|---|
| 2026-11-18 | v1.0 | 初版：AI 开发、PG 规范、模块接入、数据迁移、部署、台账 |
| 2026-11-18 | v1.1 | 公网 OpenResty/HTTPS、容器 80、管理员授权、V2 冻结、12.15 分批、1Panel 可选适配 |
| 2026-11-18 | v1.2 | 1Panel 托管反代/证书（老机器回源）；核心上线；管理员先创建用户；PM-07 12/31 |
| 2026-11-18 | v1.3 | 新二级域名 → 新机器；新机器 1Panel v2 本地直连；数据源/MCP 最小只读；旧/新边缘独立 |
| 2026-11-18 | v1.4 | 1Panel 2.5.1 API `/api/v2`、HMAC-SHA256、只读模块、开发机地址；域名待定 |
| 2026-11-18 | v1.5 | 1Panel 多实例/Key 加密入库；1Panel-skills 只读；云服务器 401 待放行 |
| 2026-11-18 | v1.6 | 1Panel 接入暂停；V3.0 只要求公网反代/证书；57 任务/61.0 人日 |
| 2026-11-18 | v1.7 | 配置中心、多主机管理、移除 s0/s1/s2；60 任务/65.0 人日 |
| 2026-11-18 | v1.8 | s0 改名 health，保留探活能力；s1/s2 删除 |
| 2026-11-19 | v1.9 | L1/L2 手动维护；RustFS 固定 ACloud；默认关闭自动 fallback |
